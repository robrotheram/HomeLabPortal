package main

import (
	"github.com/kataras/iris"
	"github.com/kataras/iris/middleware/logger"
	"github.com/kataras/iris/middleware/recover"

	"io/ioutil"
	"log"
	"net/http"

	"bytes"
	"encoding/json"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/kataras/iris/sessions"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Frontend struct {
	EntryPoints          []string         `json:"entryPoints,omitempty"`
	Backend              string           `json:"backend,omitempty"`
	Routes               map[string]Route `json:"routes,omitempty"`
	PassHostHeader       bool             `json:"passHostHeader,omitempty"`
	PassTLSCert          bool             `json:"passTLSCert,omitempty"`
	Priority             int              `json:"priority"`
	BasicAuth            []string         `json:"basicAuth"`
	WhitelistSourceRange []string         `json:"whitelistSourceRange,omitempty"`
}

// Route holds route configuration.
type Route struct {
	Rule string `json:"rule,omitempty"`
}

type Backend struct {
	Servers        map[string]Server `json:"servers,omitempty"`
	CircuitBreaker *CircuitBreaker   `json:"circuitBreaker,omitempty"`
	LoadBalancer   *LoadBalancer     `json:"loadBalancer,omitempty"`
	MaxConn        *MaxConn          `json:"maxConn,omitempty"`
	HealthCheck    *HealthCheck      `json:"healthCheck,omitempty"`
	Buffering      *Buffering        `json:"buffering,omitempty"`
}

// LoadBalancer holds load balancing configuration.
type LoadBalancer struct {
	Method     string      `json:"method,omitempty"`
	Sticky     bool        `json:"sticky,omitempty"` // Deprecated: use Stickiness instead
	Stickiness *Stickiness `json:"stickiness,omitempty"`
}

// Stickiness holds sticky session configuration.
type Stickiness struct {
	CookieName string `json:"cookieName,omitempty"`
}

// MaxConn holds maximum connection configuration
type MaxConn struct {
	Amount        int64  `json:"amount,omitempty"`
	ExtractorFunc string `json:"extractorFunc,omitempty"`
}

// CircuitBreaker holds circuit breaker configuration.
type CircuitBreaker struct {
	Expression string `json:"expression,omitempty"`
}

// Buffering holds request/response buffering configuration/
type Buffering struct {
	MaxRequestBodyBytes  int64  `json:"maxRequestBodyBytes,omitempty"`
	MemRequestBodyBytes  int64  `json:"memRequestBodyBytes,omitempty"`
	MaxResponseBodyBytes int64  `json:"maxResponseBodyBytes,omitempty"`
	MemResponseBodyBytes int64  `json:"memResponseBodyBytes,omitempty"`
	RetryExpression      string `json:"retryExpression,omitempty"`
}

// HealthCheck holds HealthCheck configuration
type HealthCheck struct {
	Path     string `json:"path,omitempty"`
	Port     int    `json:"port,omitempty"`
	Interval string `json:"interval,omitempty"`
}

type TConfiguration struct {
	Backends  map[string]*Backend  `json:"backends,omitempty"`
	Frontends map[string]*Frontend `json:"frontends,omitempty"`
}

// Server holds server configuration.
type Server struct {
	URL    string `json:"url,omitempty"`
	Weight int    `json:"weight"`
}

type Service struct {
	Name        string
	BackendUrl  string
	BackendPath string
	FrontendUrl string
	Icon        string
	Status      string
}
type Infrastructure struct {
	Name   string
	PDUURL string
	Active bool
}
type Configuration struct {
	Host           string
	Port           int
	Username       string
	Password       string
	Traefik        bool
	Infrastructure Infrastructure
	Services       []Service
	Hosts          []string
}

var (
	cookieNameForSessionID = "mycookiesessionnameid"
	sess                   = sessions.New(sessions.Config{Cookie: cookieNameForSessionID})
	ticker                 = time.NewTicker(time.Minute)
	quit                   = make(chan struct{})
	tconfig                = TConfiguration{}
)

func getConfig() (Configuration, error) {
	filename := "config/config.json"
	file, err := os.Open(filename)
	config := Configuration{}
	if err != nil {
		return config, err
	}
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&config)
	return config, err
}

func getRules() {
	filename := "rules.toml"
	file, err := os.Open(filename)
	if err != nil {
	}
	buff := new(bytes.Buffer)
	buff.ReadFrom(file)
	if _, err := toml.Decode(buff.String(), &tconfig); err != nil {
		// handle error
	}
	data, err := json.Marshal(tconfig)
	fmt.Printf("%s\n", data)
}

func writeConfig(config *Configuration) {
	for i := range config.Services {
		attr := &config.Services[i]
		log.Print(attr.Name)
	}

	configJson, _ := json.MarshalIndent(config, "", "  ")
	log.Printf("%s", configJson)

	err := ioutil.WriteFile("config/config.json", configJson, 0644)

	if err != nil {
		log.Fatal(err)
	}

	if config.Traefik {
		go func() {
			time.Sleep(1 * time.Second)
			writeTraefik(config)
		}()

	}
}

func writeTraefik(config *Configuration) {

	tconfig.Backends = make(map[string]*Backend)
	tconfig.Frontends = make(map[string]*Frontend)

	server := "http://" + config.Host + ":" + strconv.Itoa(config.Port)
	servers := make(map[string]Server)
	servers["portal"] = Server{server, 1}
	back := Backend{Servers: servers}
	tconfig.Backends["portal"] = &back

	for _, host := range config.Hosts {
		routes := make(map[string]Route)
		routes["portal-"+host] = Route{Rule: "Host:" + "portal." + host}
		front := Frontend{Backend: "portal", Routes: routes, PassHostHeader: true}
		tconfig.Frontends["portal-"+host] = &front
	}

	for i := range config.Services {
		if i < len(config.Services) {
			attr := &config.Services[i]
			servers := make(map[string]Server)
			servers[attr.Name] = Server{attr.BackendUrl, 1}
			back := Backend{Servers: servers}
			tconfig.Backends[attr.Name] = &back

			for _, host := range config.Hosts {
				routes := make(map[string]Route)
				log.Println("Backgorund: " + attr.BackendPath)
				if attr.BackendPath != "" {
					routes[attr.Name] = Route{Rule: "Host:" + attr.Name + "." + host + ";AddPrefix:/" + attr.BackendPath}
				} else {
					routes[attr.Name] = Route{Rule: "Host:" + attr.Name + "." + host}
				}

				front := Frontend{Backend: attr.Name, Routes: routes, PassHostHeader: true}
				name := attr.FrontendUrl + "." + host
				tconfig.Frontends[name] = &front
			}

		}
	}

	data, _ := json.Marshal(tconfig)
	fmt.Printf("%s\n", data)

	buf := new(bytes.Buffer)
	if err := toml.NewEncoder(buf).Encode(tconfig); err != nil {
		log.Fatal(err)
	}
	ioutil.WriteFile("rules.toml", buf.Bytes(), 0644)

}

func AddService(config *Configuration) iris.Handler {
	return func(ctx iris.Context) {
		if auth, _ := sess.Start(ctx).GetBoolean("authenticated"); !auth {
			ctx.Redirect("/login")
			return
		}
		serviceName := ctx.PostValue("service-name")
		seriviceURl := ctx.PostValue("service-backend")
		serivicePath := ctx.PostValue("service-backendpath")
		seriviceFrontURl := ctx.PostValue("service-frontend")
		serviceIcon := ctx.PostValue("service-icon")

		config.Services = append(config.Services, Service{Name: serviceName, BackendUrl: seriviceURl, FrontendUrl: seriviceFrontURl, Icon: serviceIcon, BackendPath: serivicePath})
		writeConfig(config)
		ctx.Redirect("/")
	}
}

func GetIndex(config *Configuration) iris.Handler {
	return func(ctx iris.Context) {
		session := sess.Start(ctx)
		auth, _ := session.GetBoolean("authenticated")
		if (config.Infrastructure != Infrastructure{}) {
			ctx.ViewData("InfrastructureName", config.Infrastructure.Name)
			if isPowerOn(config) {
				ctx.ViewData("Active", "active")
			}
		}
		hostArr := strings.Split(ctx.Host(), ".")
		host := config.Hosts[0]
		if len(hostArr) == 3 {
			host = strings.Join(append(hostArr[:0], hostArr[1:]...), ".")
		}

		ctx.ViewData("Host", host)
		ctx.ViewData("auth", auth)
		ctx.ViewData("Name", "iris")
		ctx.ViewData("Services", config.Services)
		ctx.ViewData("Traefik", config.Traefik)
		if config.Traefik {
			ctx.ViewData("Hosts", config.Hosts[0])
		}

		ctx.Gzip(true)
		ctx.View("index.html")
	}
}

func UpateService(config *Configuration) iris.Handler {
	return func(ctx iris.Context) {
		if auth, _ := sess.Start(ctx).GetBoolean("authenticated"); !auth {
			ctx.Redirect("/login")
			return
		}

		serviceOldName := ctx.PostValue("service-oldname")
		serviceName := ctx.PostValue("service-name")
		seriviceURl := ctx.PostValue("service-backend")
		serivicePath := ctx.PostValue("service-backendpath")
		seriviceFrontURl := ctx.PostValue("service-frontend")
		serviceIcon := ctx.PostValue("service-icon")

		for i := range config.Services {
			attr := &config.Services[i]
			if attr.Name == serviceOldName {
				attr.Name = serviceName
				attr.BackendUrl = seriviceURl
				attr.BackendPath = serivicePath
				attr.FrontendUrl = seriviceFrontURl
				attr.Icon = serviceIcon
				break
			}
		}
		writeConfig(config)
		ctx.Redirect("/")
	}
}

func DeleteService(config *Configuration) iris.Handler {
	return func(ctx iris.Context) {
		if auth, _ := sess.Start(ctx).GetBoolean("authenticated"); !auth {
			ctx.Redirect("/login")
			return
		}
		serviceName := ctx.PostValue("serviceName")
		for i := range config.Services {
			attr := &config.Services[i]
			if attr.Name == serviceName {
				config.Services = append(config.Services[:i], config.Services[i+1:]...)
				break
			}
		}
		writeConfig(config)
		ctx.WriteString("{'status':'DELTEED'}")
	}
}

func powerOFF(config *Configuration) iris.Handler {
	return func(ctx iris.Context) {
		if auth, _ := sess.Start(ctx).GetBoolean("authenticated"); !auth {
			ctx.Redirect("/login")
			return
		}
		var username string = "admin"
		var passwd string = ""
		client := &http.Client{}
		req, err := http.NewRequest("GET", config.Infrastructure.PDUURL+"/SetPower.cgi?p1=0+p2=0+p3=0+p4=0", nil)
		req.SetBasicAuth(username, passwd)
		resp, err := client.Do(req)
		if err != nil {
			log.Print(err)
		}
		_, err = ioutil.ReadAll(resp.Body)
		ctx.WriteString("{'status':'OFF'}")
	}
}

func powerOn(config *Configuration) iris.Handler {
	return func(ctx iris.Context) {
		if auth, _ := sess.Start(ctx).GetBoolean("authenticated"); !auth {
			ctx.Redirect("/login")
			return
		}
		var username string = "admin"
		var passwd string = ""
		client := &http.Client{}
		req, err := http.NewRequest("GET", config.Infrastructure.PDUURL+"/SetPower.cgi?p1=0+p2=1+p3=1+p4=0", nil)
		req.SetBasicAuth(username, passwd)
		resp, err := client.Do(req)
		if err != nil {
			log.Print(err)
		}
		_, err = ioutil.ReadAll(resp.Body)
		ctx.WriteString("{'status':'ON'}")
	}
}

func checkURL(url string) bool {
	client := &http.Client{
		Timeout: time.Duration(time.Second),
	}
	req, err := http.NewRequest("GET", url, nil)
	resp, err := client.Do(req)
	if err != nil {
		//log.Fatal(err)
		return false
	}
	_, err = ioutil.ReadAll(resp.Body)
	return true
}

func isPowerOn(config *Configuration) bool {
	client := &http.Client{}
	req, err := http.NewRequest("GET", config.Infrastructure.PDUURL+"/GetPower.cgi", nil)
	resp, err := client.Do(req)
	if err != nil {
		log.Print(err)
	}
	bodyText, err := ioutil.ReadAll(resp.Body)

	r := regexp.MustCompile(`Power Control = (.+)?;<p>`)
	res := r.FindStringSubmatch(string(bodyText))
	if len(res) >= 2 {
		split := strings.Split(res[1], ",")
		for _, v := range split {
			if v == "P2:1" {
				return true
			} else if v == "P3:1" {
				return true
			}
		}
	}
	return false
}

func main() {
	config, err := getConfig()
	if err != nil {
		println(err.Error())
		return
	}
	if config.Traefik {
		writeTraefik(&config)
	}

	go func(conf *Configuration) {
		for {
			select {
			case <-ticker.C:
				for i := range conf.Services {
					if i < len(conf.Services) {
						attr := &conf.Services[i]
						if checkURL(attr.BackendUrl) {
							attr.Status = "online"
						} else {
							attr.Status = "offline"
						}
					}
				}
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}(&config)

	app := iris.New()
	app.Logger().SetLevel("debug")
	app.Use(recover.New())
	app.Use(logger.New())

	app.RegisterView(iris.HTML("./templates", ".html"))
	app.StaticWeb("/static", "./static")

	app.Get("/", GetIndex(&config))
	app.Get("/turnOFF", powerOFF(&config))
	app.Get("/turnOn", powerOn(&config))

	app.Post("/add-service", AddService(&config))
	app.Post("/update-service", UpateService(&config))
	app.Post("/delete-service", DeleteService(&config))

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

		if user == config.Username && password == config.Password {
			session := sess.Start(ctx)
			session.Set("authenticated", true)
			ctx.Redirect("/")
		} else {
			ctx.ViewData("Error", "wrong username or password")
			ctx.View("login.html")
		}
	})
	app.Run(iris.Addr(config.Host+":"+strconv.Itoa(config.Port)), iris.WithoutServerError(iris.ErrServerClosed))
}
