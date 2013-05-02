package main

import (
	"crypto/tls"
	"encoding/json"
	"go-imap"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

type ServerConfig struct {
	Hostname     string `json:"hostname"`
	Port         string `json:"port"`
	UseTLS       bool   `json:"useTLS"`
	CertFilename string `json:"certFilename"`
	KeyFilename  string `json:"keyFilename"`
}

var gServerConfig ServerConfig

func readConfig() {

	var data []byte
	var err error

	data, err = ioutil.ReadFile("config.json")
	if err != nil {
		log.Println("Not configured.  Could not find config.json")
		os.Exit(-1)
	}

	err = json.Unmarshal(data, &gServerConfig)
	if err != nil {
		log.Println("Could not unmarshal config.json", err)
		os.Exit(-1)
		return
	}
}

type registrationRequest struct {
	Username        string `json:"username"`
	Password        string `json:"password"`
	OnNewMessageURL string `json:"onNewMessageURL"`
	OnReconnectURL  string `json:"onReconnectURL"`
}

func notifyHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Got notification from app server ", r.URL)
	if r.Method != "POST" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Not a post request"))
		return
	}
	// receive posted data
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Could not access body of request"))
		return
	}

	request := new(registrationRequest)
	err = json.Unmarshal(body, request)

	if err != nil {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Could not unmarshal body, "))
		log.Println(err)
		return
	}

	log.Println("got request: ", request)
	log.Println("got username: ", request.Username)
	log.Println("got password: ", request.Password)
	log.Println("got onNewMessageURL: ", request.OnNewMessageURL)
	log.Println("got onReconnectURL: ", request.OnReconnectURL)

	// do a bunch of magic here

	conn, err := tls.Dial("tcp", "imap.gmail.com:993", nil)
	var reader io.Reader = conn
	im := imap.New(reader, conn)
	im.Unsolicited = make(chan interface{}, 100)

	hello, err := im.Start()
	if err != nil {
		log.Println("imap didn't start...")
		return
	}
	log.Println("server hello: %s", hello)

	log.Println("logging in...")

	resp, caps, err := im.Auth(request.Username, request.Password)
	if err != nil {
		log.Println("server failed to auth: %s", err)
		return
	}

	log.Println("%s", resp)
	log.Println("server capabilities: %s", caps)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK\n"))
}

func main() {

	readConfig()

	http.HandleFunc("/register", notifyHandler)

	log.Println("Listening on", gServerConfig.Hostname+":"+gServerConfig.Port)

	var err error
	if gServerConfig.UseTLS {
		err = http.ListenAndServeTLS(gServerConfig.Hostname+":"+gServerConfig.Port,
			gServerConfig.CertFilename,
			gServerConfig.KeyFilename,
			nil)
	} else {
		for i := 0; i < 5; i++ {
			log.Println("This is a really unsafe way to run the push server.  Really.  Don't do this in production.")
		}
		err = http.ListenAndServe(gServerConfig.Hostname+":"+gServerConfig.Port, nil)
	}

	log.Println("Exiting... ", err)
}