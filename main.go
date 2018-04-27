package main

import (
	"github.com/kataras/iris"

	"github.com/kataras/iris/middleware/logger"
	"github.com/kataras/iris/middleware/recover"
	"net/http"
	"log"
	"io/ioutil"
	"regexp"
	"strings"
	"os"
	"encoding/json"
	"strconv"
	"github.com/kataras/iris/sessions"
	"time"
	"fmt"
)

type Service struct {
	Name string
	Url string
	Icon string
	Status string
}
type Infrastructure struct{
	Name string
	PDUURL string
}
type Configuration struct {
	Host string
	Port int
	Username string
	Password string
	Infrastructure Infrastructure
	Services [] Service

}

var (
	cookieNameForSessionID 	=	 "mycookiesessionnameid"
	sess                   	=	 sessions.New(sessions.Config{Cookie: cookieNameForSessionID})
	ticker 					=	 time.NewTicker(5 * time.Second)
	quit 					=	  make(chan struct{})
	config					= 	  Configuration{}
)

func getConfig() (Configuration, error) {
	filename:= "config/config.json"
	file, err := os.Open(filename)
	if err != nil { return  config, err }
	decoder := json.NewDecoder(file)

	err = decoder.Decode(&config)
	return config, err;
}

func writeConfig() error{
	configJson, _ := json.MarshalIndent(config, "", "  ")
	err := ioutil.WriteFile("config/config.json", configJson, 0644)
	fmt.Printf("%+v", configJson)
	return err
}


func main() {
	config, err := getConfig()
	if(err != nil){
		println(err.Error())
		return;
	}


	go func() {
		for {
			select {
			case <- ticker.C:
				for i  := range  config.Services {
					if(i < len(config.Services)) {
						attr := &config.Services[i];
						if checkURL(attr.Url) {
							attr.Status = "online"
						} else {
							attr.Status = "offline"
						}
					}
				}
			case <- quit:
				ticker.Stop()
				return
			}
		}
	}()



	app := iris.New()
	app.Logger().SetLevel("debug")
	app.Use(recover.New())
	app.Use(logger.New())

	app.RegisterView(iris.HTML("./templates", ".html"))
	app.StaticWeb("/static", "./static")


	app.Get("/turnOn", func(ctx iris.Context) {
		if auth, _ := sess.Start(ctx).GetBoolean("authenticated"); !auth {
			ctx.Redirect("/login")
			return
		}
		powerOn();
		ctx.WriteString("{'status':'ON'}")
	})

	app.Get("/turnOFF", func(ctx iris.Context) {
		if auth, _ := sess.Start(ctx).GetBoolean("authenticated"); !auth {
			ctx.Redirect("/login")
			return
		}
		powerOFF();
		ctx.WriteString("{'status':'OFF'}")
	})


	app.Get("/", func(ctx iris.Context) {
		session := sess.Start(ctx)
		auth, _ :=  session.GetBoolean("authenticated")
		if (config.Infrastructure != Infrastructure{}){
			ctx.ViewData("InfrastructureName", config.Infrastructure.Name)
			if(isPowerOn()){
				ctx.ViewData("Active", "active")
			}
		}

		ctx.ViewData("auth", auth)
		ctx.ViewData("Name", "iris")
		ctx.ViewData("Services", config.Services)

		ctx.Gzip(true)
		ctx.View("index.html")
	})

	app.Get("/logout", func(ctx iris.Context) {
		session := sess.Start(ctx)
		session.Set("authenticated", false)

		ctx.Redirect("/")
	})

	app.Get("/login", func(ctx iris.Context) {
		ctx.ViewData("Name", "iris")
		ctx.Gzip(true)
		ctx.View("login.html")
	})

	app.Post("/login", func(ctx iris.Context) {
		user := ctx.PostValue("username")
		password := ctx.PostValue("password")

		if(user == config.Username && password == config.Password ){
			session := sess.Start(ctx)
			session.Set("authenticated", true)
			ctx.Redirect("/")
		}else{
			ctx.ViewData("Error", "wrong username or password")
			ctx.View("login.html")
		}
	})


	app.Post("/add-service", func(ctx iris.Context) {
		if auth, _ := sess.Start(ctx).GetBoolean("authenticated"); !auth {
			ctx.Redirect("/login")
			return
		}
		serviceName := ctx.PostValue("service-name")
		seriviceURl := ctx.PostValue("service-backend")
		serviceIcon := ctx.PostValue("service-icon")
		config.Services = append(config.Services, Service{Name:serviceName, Url:seriviceURl, Icon:serviceIcon})
		writeConfig()
		ctx.Redirect("/")
	})

	app.Post("/update-service", func(ctx iris.Context) {
		if auth, _ := sess.Start(ctx).GetBoolean("authenticated"); !auth {
			ctx.Redirect("/login")
			return
		}

		serviceOldName := ctx.PostValue("service-oldname")
		serviceName := ctx.PostValue("service-name")
		seriviceURl := ctx.PostValue("service-backend")
		serviceIcon := ctx.PostValue("service-icon")

		for i  := range  config.Services {
			attr := &config.Services[i];
			if(attr.Name == serviceOldName){
				attr.Name = serviceName
				attr.Url = seriviceURl
				attr.Icon = serviceIcon
				break;
			}
		}
		writeConfig()
		ctx.Redirect("/")
	})

	app.Post("/delete-service", func(ctx iris.Context) {
		if auth, _ := sess.Start(ctx).GetBoolean("authenticated"); !auth {
			ctx.Redirect("/login")
			return
		}


		serviceName := ctx.PostValue("serviceName")
		println("FOUND!!!! " + serviceName)

		for i  := range  config.Services {
			attr := &config.Services[i];
			if(attr.Name == serviceName){
				config.Services = append(config.Services[:i], config.Services[i+1:]...)
				break;
			}
		}
		writeConfig()
		ctx.WriteString("{'status':'DELTEED'}")
	})


	app.Run(iris.Addr(config.Host+":"+strconv.Itoa(config.Port)), iris.WithoutServerError(iris.ErrServerClosed))
}


func checkURL(url string) bool {
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	resp, err := client.Do(req)
	if err != nil{
		//log.Fatal(err)
		return false;
	}
	_, err = ioutil.ReadAll(resp.Body)
	return true;
}

func isPowerOn() bool {
	client := &http.Client{}
	req, err := http.NewRequest("GET", config.Infrastructure.PDUURL+"/GetPower.cgi", nil)
	resp, err := client.Do(req)
	if err != nil{
		log.Fatal(err)
	}
	bodyText, err := ioutil.ReadAll(resp.Body)

	r := regexp.MustCompile(`Power Control = (.+)?;<p>`)
	res := r.FindStringSubmatch(string(bodyText));
	if(len(res) >= 2) {
		split := strings.Split(res[1], ",")
		for _, v := range split {
			if v == "P2:1" {
				return true;
			} else if v == "P3:1" {
				return true;
			}
		}
	}
	return false;
}


func powerOFF()  {
	var username string = "admin"
	var passwd string = ""
	client := &http.Client{}
	req, err := http.NewRequest("GET", config.Infrastructure.PDUURL+"/SetPower.cgi?p1=0+p2=0+p3=0+p4=0", nil)
	req.SetBasicAuth(username, passwd)
	resp, err := client.Do(req)
	if err != nil{
		log.Fatal(err)
	}
	_, err = ioutil.ReadAll(resp.Body)
}

func powerOn()  {
	var username string = "admin"
	var passwd string = ""
	client := &http.Client{}
	req, err := http.NewRequest("GET", config.Infrastructure.PDUURL+"/SetPower.cgi?p1=0+p2=1+p3=1+p4=0", nil)
	req.SetBasicAuth(username, passwd)
	resp, err := client.Do(req)
	if err != nil{
		log.Fatal(err)
	}
	_, err = ioutil.ReadAll(resp.Body)
}

