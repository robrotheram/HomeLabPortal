# Home Lab Portal
Just a simple golang service that reads a config file checks that it can reach the endpoints listed and beable to turn on and off my server via a remote powerswitch. The server is configured to boot on power.

Its mainly a pet project for me to experiment with golang again using a new go framework iris. 

The main page is protected by a very simple session cookie. Is is secure no, but better than nothing anyhow its only for internal use 

##### Traefik
Now includes some basic support for [treafik.io](https://traefik.io/) reverse proxy. Adding new services will update the proxy with the new routes. Support for this is very basic.


Screenshot of the final result 

![screenshot](https://screenshotscdn.firefoxusercontent.com/images/dc37f864-ed0e-4770-8c66-cdb309fee9de.png)

Edit mode

![screenshot](https://screenshotscdn.firefoxusercontent.com/images/3c0fb895-90e2-4cef-80cf-2779bcfd4723.png)

