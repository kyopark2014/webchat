package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"webchat/internal/config"
	"webchat/internal/data"
	"webchat/internal/rediscache"

	socketio "github.com/nkovacs/go-socket.io"
)

// WebchatService is a list of service
type WebchatService struct{}

var host string
var port int

// Init is to start Profile Service
func (p *WebchatService) Init(conf *config.AppConfig) error {
	// initiate somethings
	host = conf.ChatConfig.Host
	port = conf.ChatConfig.Port

	return nil
}

// Start is to run Profile Server
func (p *WebchatService) Start() error {
	log.I("start Webchat Server...")
	server, err := socketio.NewServer(nil)
	if err != nil {
		log.E("%v", err)
	}

	userMap := make(map[string]string)   // hashmap to memorize the pair of socket id and user id
	currentUser := make(map[string]bool) // To-do it will be move to DB

	server.On("connection", func(so socketio.Socket) {
		log.D("connected... %v", so.Id())

		newMessages := make(chan string)

		so.On("chat", func(msg string) {
			newMessages <- msg
		})

		// var quit chan struct{}
		quit := make(chan struct{})

		so.On("disconnection", func() {
			user := userMap[so.Id()]
			log.D("disconnected... %v (%v)", so.Id(), user)

			delete(currentUser, user)
			delete(userMap, so.Id())
			close(quit)

			for s, u := range userMap {
				log.D("Last session: %v, uid: %v", s, u)
			}
		})

		userEvent := make(chan data.Event, 10)
		receive := make(chan data.Event, 10) // received event

		so.On("join", func(user string) {
			log.D("Join...%v (%v)", user, so.Id())

			currentUser[user] = true

			for s, u := range userMap {
				log.D("session: %v, uid: %v", s, u)
				if u == user { // delete duplicated session
					delete(userMap, s)

					log.D("%v is trying to join again!", user)
				}
			}

			userMap[so.Id()] = user
			log.D("SUBSCRIBE request: %v", user)
			subscribeEvent(user, userEvent)

			// check stored message
			getEventList(user, userEvent)

			// make connection with redis server
			go func() { // server <-> QUEUE / PUBSUB
				for {
					select {
					case event := <-receive: //
						log.D("sent message: %v", event.Text)

						// if online, push into redis
						// if offline, backup received messages into QUEUE using rpush
						if currentUser[event.To] {
							publishEvent(&event)
						} else {
							pushEvent(&event)
						}

					case <-quit:
						return
					}
				}
			}()

			go func() { // cient <-> server (WEB Socket)
				for {
					select {
					case event := <-userEvent: // sending event to client(browser)
						log.D("sending event to browsers: %v %v %v %v %v %v (%v)", event.EvtType, event.From, event.To, event.MsgID, event.Timestamp, event.Text, so.Id())
						so.Emit("chat", event)

					case msg := <-newMessages: // received message from client(browser)
						var newMSG data.Message
						json.Unmarshal([]byte(msg), &newMSG)
						log.D("receiving message from browser: %v %v %v %v %v (%v)", newMSG.From, newMSG.To, newMSG.MsgID, newMSG.Timestamp, newMSG.Text, so.Id())

						receive <- NewEvent(newMSG.EvtType, newMSG.From, newMSG.To, newMSG.MsgID, int(newMSG.Timestamp), newMSG.Text)
					case <-quit:
						return
					}
				}
			}()
		})
	})

	http.HandleFunc("/socket.io/", func(w http.ResponseWriter, r *http.Request) {
		// origin to excape Cross-Origin Resource Sharing (CORS)
		if origin := r.Header.Get("Origin"); origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		// address
		r.RemoteAddr = host
		log.I("Address: %v", r.RemoteAddr)

		server.ServeHTTP(w, r)
	})

	http.Handle("/", http.FileServer(http.Dir("./asset")))

	log.I("Serving at %v:%v", host, port)
	log.E("%v", http.ListenAndServe(fmt.Sprintf(":%v", port), nil))

	return err
}

// OnTerminate is to close the servcie
func (p *ProfileService) OnTerminate() error {
	log.I("Profile Server was terminated")

	// To-Do: add codes for error cases if requires
	return nil
}

// NewEvent is to create an new event
func NewEvent(evtType string, from string, to string, msgID string, timestamp int, msg string) data.Event {
	return data.Event{evtType, from, to, msgID, timestamp, msg}
}

// setEvent is to save a message
func publishEvent(event *data.Event) {
	log.D("event: %v %v %v %v %v", event.EvtType, event.From, event.To, event.Timestamp, event.Text)

	// generate key
	key := event.To // UID to identify the profile

	raw, err := json.Marshal(event)
	if err != nil {
		log.E("Cannot encode to Json", err)
	}
	log.D("key: %v value: %v", key, string(raw))

	_, errRedis := rediscache.Publish(key, raw)
	if errRedis != nil {
		log.E("Error of Publish: %v", errRedis)
	}
}

func subscribeEvent(channel string, e chan data.Event) {
	log.D("channel: %v", channel)

	d := make(chan []byte, 10)

	if err := rediscache.Subscribe(channel, d); err != nil {
		log.E("%s", err)
	}

	go func() {
		for {
			raw := <-d
			log.D("Received Data: %v", string(raw))

			var event data.Event
			errJSON := json.Unmarshal([]byte(raw), &event)
			if errJSON != nil {
				log.E("%v: %v", channel, errJSON)
			}

			e <- event // send event

			/*	if event == nil {
					log.D("No cache in Redis")
				} else {
					log.D("value: %v", event.Text) // To-Do
				} */
		}
	}()
}

// setEvent is to save a message
func pushEvent(event *data.Event) {
	// generate key
	key := event.To // UID to identify the profile

	raw, err := json.Marshal(event)
	if err != nil {
		log.E("Cannot encode to Json", err)
	}
	log.D("pushed message: key: %v value: %v", key, string(raw))

	_, errRedis := rediscache.PushList(key, raw)
	if errRedis != nil {
		log.E("Error of pushEvent: %v", errRedis)
	}
}

// GetUserInfo is getting the identification from the url
func getEventList(key string, e chan data.Event) {
	raw, err := rediscache.GetList(key)
	if err != nil {
		log.E("Error: %v", err)
	}

	if err = rediscache.Del(key); err != nil {
		log.E("Fail to delete: key: %v errMsg: %v", key, err)
	}

	var event *data.Event
	for index := range raw {
		log.D("raw[%v] : %v", index, string(raw[index]))
		err = json.Unmarshal([]byte(raw[index]), &event)
		if err != nil {
			log.E("%v: %v", key, err)
		}

		if event == nil {
			log.D("No cache in Redis")
		} else {
			log.D("loaded message: %v", event.MsgID) // To-Do
		}

		e <- *event
	}
}

// setEvent is to save a message
func setEvent(event *data.Event) {
	log.D("event: %v %v %v %v %v %v", event.EvtType, event.From, event.To, event.MsgID, event.Timestamp, event.Text)

	// generate key
	key := event.From // UID to identify the profile

	raw, err := json.Marshal(event)
	if err != nil {
		log.E("Cannot encode to Json", err)
	}
	log.D("key: %v value: %v", key, string(raw))

	_, errRedis := rediscache.SetCache(key, raw)
	if errRedis != nil {
		log.E("Error of setEvent: %v", errRedis)
	}
}

// GetUserInfo is getting the identification from the url
func getEvent(key string) *data.Event {
	raw, err := rediscache.GetCache(key)
	if err != nil {
		log.E("Error: %v", err)
	}
	log.D("raw: %v", string(raw))

	var value *data.Event
	err = json.Unmarshal([]byte(raw), &value)
	if err != nil {
		log.E("%v: %v", key, err)
	}

	if value == nil {
		log.D("No cache in Redis")
	} else {
		log.D("value: %v", value.Text) // To-Do
	}

	return value
}
