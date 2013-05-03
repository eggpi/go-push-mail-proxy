package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"go-imap"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type ServerConfig struct {
	Debug        bool   `json:"debug"`
	Hostname     string `json:"hostname"`
	Port         string `json:"port"`
	UseTLS       bool   `json:"useTLS"`
	CertFilename string `json:"certFilename"`
	KeyFilename  string `json:"keyFilename"`
	IDLETimeout  int    `json:"IDLETimeoutMinutes"`
}

var gServerConfig ServerConfig

func sendNotification(endpoint string) {
	body := strings.NewReader("version=" + string(int32(time.Now().Unix())))
	r, err := http.NewRequest("PUT", endpoint, body)
	if err != nil {
		log.Println(err)
		return
	}

	var client *http.Client

	if gServerConfig.UseTLS {
		client = &http.Client{}
	} else {
		config := &tls.Config{InsecureSkipVerify: true} // this line here
		tr := &http.Transport{TLSClientConfig: config}
		client = &http.Client{Transport: tr}
	}

	_, err = client.Do(r)
	if err != nil {
		log.Println(err)
		return
	}
}

func notifyNewMessageHandler(request *registrationRequest) {
	sendNotification(request.OnNewMessageURL)
}

func notifyReconnectHandler(request *registrationRequest) {
	sendNotification(request.OnReconnectURL)
}

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

func registerHandler(w http.ResponseWriter, r *http.Request) {
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

	respExamine, err := im.Examine("inbox")
	log.Println("EXISTS: ", respExamine.Exists)

	log.Println("Beginnig to IDLE")
	idleChan, err := im.Idle()

	if err != nil {
		log.Println("failed to send IDLE command")
	}

	go func() {
		for {
			select {
			case message, open := <-idleChan:
				if !open {
					log.Println("Attempting reconnect")
					notifyReconnectHandler(request)
					return
				}

				switch message := message.(type) {
				case *imap.ResponseExists:
					log.Println("Got EXISTS ", message.Count)
					notifyNewMessageHandler(request)
				case *imap.ResponseStatus:
					if message.Status != imap.OK {
						panic(fmt.Sprintf("Non-OK response from IDLE: %+v", message))
					}

					log.Println("Restarting IDLE")
					idleChan, err = im.Idle()
				}
			case <-time.After(time.Duration(gServerConfig.IDLETimeout) * time.Minute):
				/* RFC 2177:
				 * (...) clients using IDLE are advised to terminate the IDLE and
				 * re-issue it at least every 29 minutes to avoid being logged
				 * off.
				 */
				log.Println("Sending DONE")
				im.Done()
			}
		}
	}()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK\n"))
}

func main() {

	readConfig()

	http.HandleFunc("/register", registerHandler)

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
