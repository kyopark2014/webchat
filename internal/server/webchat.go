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
		newGroupInfo := make(chan string)

		so.On("chat", func(msg string) {
			newMessages <- msg
		})

		so.On("group", func(groupEvent string) {
			newGroupInfo <- groupEvent
		})

		quit := make(chan struct{})               // 1-to-1
		quitMap := make(map[string]chan struct{}) // group

		so.On("disconnection", func() {
			user := userMap[so.Id()]
			log.D("disconnected... %v (%v)", so.Id(), user)

			delete(currentUser, user)
			delete(userMap, so.Id())
			close(quit)

			for k := range quitMap {
				log.D("Quit: ", k)
				close(quitMap[k])
			}

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
			subscribeEvent(user, userEvent, quit)

			// check stored message
			getEventList(user, userEvent)

			go func() { // send to redis
				for {
					select {
					case event := <-receive:
						// if online, push into redis
						// if offline, backup received messages into QUEUE using rpush
						if event.To[0] == 'g' { // groupchat
							if event.EvtType == "message" {
								log.D("PUBLISH %v %v %v", event.EvtType, event.To, event.MsgID)
								publishEvent(event.To, &event)
							} else {
								log.D("PUBLISH %v %v %v", event.EvtType, event.Originated, event.MsgID)
								publishEvent(event.Originated, &event)
							}
						} else { // 1-to-1
							if currentUser[event.To] {
								publishEvent(event.To, &event)
							} else {
								pushEvent(&event)
							}
						}

					case <-quit:
						return
					}
				}
			}()

			go func() { // receive from client or redis
				for {
					select {
					case event := <-userEvent: // received event
						log.D("Event(%v): (%v)->(%v) %v %v (%v)", event.EvtType, event.From, event.To, event.MsgID, event.Text, so.Id())

						if event.EvtType == "subscribe" { // if received event is "subscribe", do "SUBSCRIBE"
							log.D("SUBSCRIBE: (%v)->(%v)", user, event.From)

							quitMap[event.From] = make(chan struct{})
							subscribeEvent(event.From, userEvent, quitMap[event.From])

							// get groupinfo
							grpInfo := getGroupInfo(event.From)
							log.D("Load Groupchat info and then send to clients")
							if grpInfo != nil {
								so.Emit("groupInfo", grpInfo)
							}
						} else { // --> web client (socket.io)
							if event.Originated != user {
								so.Emit("chat", event)
							}
						}

					case msg := <-newMessages: // received message from client(browser)
						var newMSG data.Message
						json.Unmarshal([]byte(msg), &newMSG)
						log.D("new message(%v): (%v %v)->(%v) %v %v (%v)", newMSG.EvtType, newMSG.From, newMSG.Originated, newMSG.To, newMSG.MsgID, newMSG.Text, so.Id())

						receive <- NewEvent(newMSG.EvtType, newMSG.From, newMSG.Originated, newMSG.To, newMSG.MsgID, int(newMSG.Timestamp), newMSG.Text)

					case grpEvent := <-newGroupInfo: // received group info from client(browser)
						var grpInfo data.GroupInfo
						json.Unmarshal([]byte(grpEvent), &grpInfo)
						log.D("Group(%v): %v %v %v %v", grpInfo.EvtType, grpInfo.From, grpInfo.To, grpInfo.Timestamp, grpInfo.Participants)

						if grpInfo.EvtType == "create" {
							// save participant information into redis
							setGroupInfo(&grpInfo)
							// subscribe the groupcaht
							quitMap[grpInfo.To] = make(chan struct{})
							subscribeEvent(grpInfo.To, userEvent, quitMap[grpInfo.To])

							for i := range grpInfo.Participants {
								log.D("subscribe request to %v", grpInfo.Participants[i])
								event := NewEvent("subscribe", grpInfo.To, "", grpInfo.Participants[i], "", grpInfo.Timestamp, "")

								// request publish to participants
								if grpInfo.Participants[i] != user {
									publishEvent(grpInfo.Participants[i], &event)
								}
							}
						} else if grpInfo.EvtType == "refer" {
							for i := range grpInfo.Participants {
								log.D("subscribe request to %v", grpInfo.Participants[i])
								event := NewEvent("subscribe", grpInfo.To, "", grpInfo.Participants[i], "", grpInfo.Timestamp, "")

								// request publish to participants
								if grpInfo.Participants[i] != user {
									publishEvent(grpInfo.Participants[i], &event)
								}
							}
						} else if grpInfo.EvtType == "depart" {
							log.D("Depart: %v", grpInfo.To)
							close(quitMap[grpInfo.To])
						}

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
func (p *WebchatService) OnTerminate() error {
	log.I("WebchatService was terminated")

	// To-Do: add codes for error cases if requires
	return nil
}

// NewEvent is to create an new event
func NewEvent(evtType string, from string, originated string, to string, msgID string, timestamp int, msg string) data.Event {
	return data.Event{evtType, from, originated, to, msgID, timestamp, msg}
}

func publishEvent(key string, event *data.Event) {
	//log.D("event: %v %v %v %v %v", event.EvtType, event.From, event.To, event.Timestamp, event.Text)

	raw, err := json.Marshal(event)
	if err != nil {
		log.E("Cannot encode to Json", err)
	}

	_, errRedis := rediscache.Publish(key, raw)
	if errRedis != nil {
		log.E("Error of Publish: %v", errRedis)
	}
}

func subscribeEvent(channel string, userEvent chan data.Event, quit chan struct{}) {
	log.D("channel: %v", channel)

	redisChan := make(chan []byte, 10)

	if err := rediscache.Subscribe(channel, redisChan, quit); err != nil {
		log.E("%s", err)
	}

	go func() {
		for {
			raw := <-redisChan
			log.D("Received event: %v", string(raw))

			var event data.Event
			errJSON := json.Unmarshal([]byte(raw), &event)
			if errJSON != nil {
				log.E("%v: %v", channel, errJSON)
			}

			if event.To[0] == 'g' { // for groupchat, "originated" needs to set by the sender
				userEvent <- NewEvent(event.EvtType, event.To, event.From, channel, event.MsgID, event.Timestamp, event.Text) // send event
			} else {
				userEvent <- event
			}
		}
	}()
}

// pushEvent is to save a message
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

// getEventList is to get event list
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

// getEvent is to get the information of event saved in radis
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

// setGroupInfo is to save a group information
func setGroupInfo(grpInfo *data.GroupInfo) {
	log.D("group: %v", grpInfo.Participants)

	// generate key
	key := grpInfo.To // UID to identify the profile

	raw, err := json.Marshal(grpInfo)
	if err != nil {
		log.E("Cannot encode to Json", err)
	}
	log.D("key: %v value: %v", key, string(raw))

	_, errRedis := rediscache.SetCache(key, raw)
	if errRedis != nil {
		log.E("Error of setEvent: %v", errRedis)
	}
}

// getGroupInfo is to get the information of group info
func getGroupInfo(key string) *data.GroupInfo {
	raw, err := rediscache.GetCache(key)
	if err != nil {
		log.E("Error: %v", err)
	}
	log.D("raw: %v", string(raw))

	var value *data.GroupInfo
	err = json.Unmarshal([]byte(raw), &value)
	if err != nil {
		log.E("%v: %v", key, err)
	}

	log.D("participants: %v", value.Participants) // To-Do

	return value
}
