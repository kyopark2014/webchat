package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
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

	userMap := make(map[string]string)             // hashmap to memorize the pair of socket id and user id
	onlineUser := make(map[string]bool)            // To-do it will be move to DB, all subscribers in this server
	groupParticipants := make(map[string][]string) // all of groupchat participants

	// once the server is booted , load online user list
	userList := getOnlineUserInfo()
	for i := range userList {
		onlineUser[userList[i]] = true
	}

	server.On("connection", func(so socketio.Socket) {
		log.D("connected... %v", so.Id())

		newMessages := make(chan string)

		so.On("chat", func(msg string) {
			newMessages <- msg
		})

		quit := make(chan struct{})               // 1-to-1
		quitMap := make(map[string]chan struct{}) // group

		so.On("disconnection", func() {
			user := userMap[so.Id()]
			log.D("disconnected... %v (%v)", so.Id(), user)

			removeOnlineUserInfo(user)
			delete(onlineUser, user)

			delete(userMap, so.Id())
			close(quit)

			for k := range quitMap {
				log.D("Quit: %v", k)
				close(quitMap[k])
				delete(quitMap, k)
			}

			for s, u := range userMap {
				log.D("Last session: %v, uid: %v", s, u)
			}
		})

		// PUBSUB -> IM
		userEvent := make(chan data.Event, 10)

		so.On("join", func(user string) {
			log.D("Join...%v (%v)", user, so.Id())

			onlineUser[user] = true
			addOnlineUserInfo(user)

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

			// restore undelivered message when the user is offline
			log.D("Check S&F messages in redis")
			getEventList(user, userEvent)

			go func() {
				for { // received event
					select {
					case event := <-userEvent: // PUBSUB -> IM -> client
						log.D("Event(%v): (%v)->(%v) %v %v (%v)", event.EvtType, event.Originated, event.To, event.MsgID, event.Body, so.Id())

						if event.EvtType == "subscribe" { // if the event type of received event is "subscribe", do "SUBSCRIBE"
							log.D("SUBSCRIBE: (%v)->(%v)", user, event.From)
							quitMap[event.From] = make(chan struct{})
							subscribeEvent(event.From, userEvent, quitMap[event.From])

							so.Emit("chat", event)
						} else if event.EvtType == "depart" {
							log.D("Notify the departure infomation")
							so.Emit("chat", event)
						} else if event.EvtType == "join" {
							log.D("Notify the join infomation")
							so.Emit("chat", event)
						} else { // --> web client (socket.io)
							if event.Originated != user {
								so.Emit("chat", event)
							}
						}

					case msg := <-newMessages: // client -> IM -> PUBSUB
						var event data.Event
						json.Unmarshal([]byte(msg), &event)
						log.D("new message(%v): (%v %v)->(%v) %v %v (%v)", event.EvtType, event.From, event.Originated, event.To, event.MsgID, event.Body, so.Id())

						if event.EvtType == "message" {
							// if online, push into redis
							// if offline, backup received messages into QUEUE using rpush
							if event.To[0] == 'g' { // groupchat
								participantList := groupParticipants[event.To]
								for i := range participantList {
									log.D("participantList[%v]=%v", i, participantList[i])
									log.D("onlineUser[participantList[%v]]=%v", i, onlineUser[participantList[i]])
									if !onlineUser[participantList[i]] {
										log.D("PUSH to %v", participantList[i])
										pushEvent(participantList[i], &event)
									}
								}

								if event.EvtType == "message" {
									log.D("PUBLISH(%v) To: %v : Body: %v (%v)", event.EvtType, event.To, event.Body, event.MsgID)
									publishEvent(event.To, &event)
								} else if event.EvtType == "display" || event.EvtType == "delivery" {
									log.D("PUBLISH(%v) To: %v (%v)", event.EvtType, event.Originated, event.MsgID)
									publishEvent(event.Originated, &event)
								} else { // join, depart
									log.D("PUBLISH(%v) To: %v : Body: %v (%v)", event.EvtType, event.To, event.Body, event.MsgID)
									publishEvent(event.To, &event)
								}
							} else { // 1-to-1
								if !onlineUser[event.To] {
									pushEvent(event.To, &event)
								} else {
									publishEvent(event.To, &event)
								}
							}

						} else if event.EvtType == "create" {
							var participantList []string
							err = json.Unmarshal([]byte(event.Body), &participantList)
							if err != nil {
								log.E("%v", err)
							}
							log.D("create groupchat: %v with %v", event.To, participantList)

							groupParticipants[event.To] = participantList // save participant information into local memory
							setGroupInfo(event.To, participantList)       // save participant information into redis

							// subscribe the groupchat
							quitMap[event.To] = make(chan struct{})
							subscribeEvent(event.To, userEvent, quitMap[event.To])

							for i := range participantList {
								log.D("subscribe request from %v to %v (%v)", event.From, event.To, participantList[i])
								subscribeEvent := NewEvent("subscribe", event.To, user, participantList[i], "", event.Timestamp, event.Body)

								if participantList[i] != user {
									if !onlineUser[participantList[i]] {
										log.D("PUSH to %v", participantList[i])
										pushEvent(participantList[i], &subscribeEvent)
									} else {
										publishEvent(participantList[i], &subscribeEvent)
									}
								}
							}
						} else if event.EvtType == "refer" {
							var referedParticipantList []string
							err = json.Unmarshal([]byte(event.Body), &referedParticipantList)
							if err != nil {
								log.E("%v", err)
							}
							log.D("Refer: %v in %v", referedParticipantList, event.To)

							// previous participants need to know joining of new participants
							participantList := groupParticipants[event.To]
							log.D("notice to current participants that %v will join", participantList)

							for i := range participantList {
								// To-Do: curretnly I assume all users are valid but it is not true, I will add another logic for this later
								joinEvent := NewEvent("join", event.To, user, participantList[i], "", event.Timestamp, event.Body)

								if !onlineUser[participantList[i]] {
									log.D("PUSH to %v", participantList[i])
									pushEvent(participantList[i], &joinEvent)
								} else {
									publishEvent(participantList[i], &joinEvent)
								}
							}

							// subscribe request to new participants
							var updatedParticipantList []string
							for i := range referedParticipantList {
								log.D("subscribe request to %v", referedParticipantList[i])
								subscribeEvent := NewEvent("subscribe", event.To, user, referedParticipantList[i], "", event.Timestamp, event.Body)

								if !onlineUser[referedParticipantList[i]] {
									log.D("PUSH to %v", referedParticipantList[i])
									pushEvent(referedParticipantList[i], &subscribeEvent)
								} else {
									publishEvent(referedParticipantList[i], &subscribeEvent)
								}

								// add refered participants
								updatedParticipantList = append(updatedParticipantList, referedParticipantList[i])
							}
							// update participant list
							groupParticipants[event.To] = updatedParticipantList
							setGroupInfo(event.To, updatedParticipantList) // update participant list in redis

						} else if event.EvtType == "depart" {
							log.D("Depart: %v in %v", event.From, event.To)
							close(quitMap[event.To])
							delete(quitMap, event.To)

							participantList := groupParticipants[event.To]

							var newparticipantList []string
							for i := range participantList {
								log.D("notice the departure of %v to %v", event.From, participantList[i])
								departEvent := NewEvent("depart", event.To, event.From, participantList[i], "", event.Timestamp, "")

								if !onlineUser[participantList[i]] {
									log.D("PUSH to %v", participantList[i])
									pushEvent(participantList[i], &departEvent)
								} else {
									publishEvent(participantList[i], &departEvent)
								}

								if participantList[i] != event.From {
									newparticipantList = append(newparticipantList, participantList[i])
								}
							}

							groupParticipants[event.To] = newparticipantList // update local group participant list
							setGroupInfo(event.To, newparticipantList)       // update participant list in redis
						} else { // deliver, display
							// if online, push into redis
							// if offline, backup received messages into QUEUE using rpush
							if event.To[0] == 'g' { // groupchat
								if !onlineUser[event.Originated] {
									pushEvent(event.Originated, &event)
								} else {
									publishEvent(event.Originated, &event)
								}
							} else {
								if !onlineUser[event.To] {
									pushEvent(event.To, &event)
								} else {
									publishEvent(event.To, &event)
								}
							}
						}

					case <-quit:
						log.D("Close gochannel")
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
	//log.D("event: %v %v %v %v %v", event.EvtType, event.From, event.To, event.Timestamp, event.Body)

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
	//	log.D("channel: %v", channel)

	redisChan := make(chan []byte, 10)

	if err := rediscache.Subscribe(channel, redisChan, quit); err != nil {
		log.E("%s", err)
	}

	var needQuit = false

	go func() {
		select {
		case <-quit:
			log.D("Closing subscribe channel: %v", channel)
			needQuit = true
			return
		}
	}()

	go func(quit bool) {
		for {
			if quit {
				log.D("Unsubscribing : %v", channel)
				return
			}

			raw := <-redisChan
			log.D("Received event: %v", string(raw))

			var event data.Event
			errJSON := json.Unmarshal([]byte(raw), &event)
			if errJSON != nil {
				log.E("%v: %v", channel, errJSON)
			}

			userEvent <- event
		}
	}(needQuit)
}

// pushEvent is to save a message
func pushEvent(key string, event *data.Event) {
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
	log.D("event: %v %v %v %v %v %v", event.EvtType, event.From, event.To, event.MsgID, event.Timestamp, event.Body)

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
		log.D("value: %v", value.Body)
	}

	return value
}

// setGroupInfo is to save a group information
func addOnlineUserInfo(user string) {
	// load online user list
	key := "onlineUser"
	userstr, err := rediscache.GetCache(key)
	if err != nil {
		log.E("Error: %v", err)
	}
	//	log.D("raw: %v", string(userstr))

	var userList []string
	err = json.Unmarshal([]byte(userstr), &userList)
	if err != nil {
		log.E("%v: %v", key, err)
	}
	//	log.D("online users: %v", userList)

	// search the user
	index := sort.Search(len(userList), func(i int) bool {
		return user <= userList[i]
	})

	if index < len(userList) && user == userList[index] {
		log.D("%v is already on the list", user)
	} else {
		userList = append(userList, user)

		// save into redis
		raw, err := json.Marshal(userList)
		if err != nil {
			log.E("Cannot encode to Json", err)
		}
		log.D("key: %v value: %v", key, string(raw))

		_, errRedis := rediscache.SetCache(key, raw)
		if errRedis != nil {
			log.E("Error of setEvent: %v", errRedis)
		}
	}
}

// removeOnlineUserInfo is to remove an offline user from online user list
// To-Do: find better way to reduce the complexity
func removeOnlineUserInfo(user string) {
	// load online user list
	key := "onlineUser"
	userstr, err := rediscache.GetCache(key)
	if err != nil {
		log.E("Error: %v", err)
	}
	var userList []string
	err = json.Unmarshal([]byte(userstr), &userList)
	if err != nil {
		log.E("%v: %v", key, err)
	}

	// remove user
	index := sort.Search(len(userList), func(i int) bool {
		return user <= userList[i]
	})
	if index < len(userList) && user == userList[index] {
		// remover userList[i]
		userList[index] = userList[len(userList)-1]
		userList = userList[:len(userList)-1]
	} else {
		log.D("(%v) does not exist", user)
		return
	}

	log.D("user: %v, online users: %v", user, userList)

	// save into redis
	raw, err := json.Marshal(userList)
	if err != nil {
		log.E("Cannot encode to Json", err)
	}
	log.D("key: %v value: %v", key, string(raw))

	_, errRedis := rediscache.SetCache(key, raw)
	if errRedis != nil {
		log.E("Error of setEvent: %v", errRedis)
	}
}

// getOnlineUserInfo is to load onlie user list
func getOnlineUserInfo() []string {
	// load online user list
	key := "onlineUser"
	userstr, err := rediscache.GetCache(key)
	if err != nil {
		log.E("Error: %v", err)
	}
	log.D("raw: %v", string(userstr))

	if userstr != "" {
		var userList []string
		err = json.Unmarshal([]byte(userstr), &userList)
		if err != nil {
			log.E("%v", err)
		}
		return userList
	} else {
		return nil
	}
}

// setGroupInfo is to save a group information
func setGroupInfo(key string, participantList []string) {
	//	log.D("group: %v", participantList)

	raw, err := json.Marshal(participantList)
	if err != nil {
		log.E("Cannot encode to Json", err)
	}
	//	log.D("key: %v value: %v", key, string(raw))

	_, errRedis := rediscache.SetCache(key, raw)
	if errRedis != nil {
		log.E("Error of setEvent: %v", errRedis)
	}
}

// getGroupInfo is to get the information of group info
func getGroupInfo(key string) []string {
	raw, err := rediscache.GetCache(key)
	if err != nil {
		log.E("Error: %v", err)
	}
	log.D("raw: %v", string(raw))

	var participantList []string
	err = json.Unmarshal([]byte(raw), &participantList)
	if err != nil {
		log.E("%v: %v", key, err)
	}

	log.D("participants: %v", participantList) // To-Do

	return participantList
}
