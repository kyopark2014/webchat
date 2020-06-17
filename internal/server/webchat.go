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

	userMap := make(map[string]string)  // for session debuging, usermap to memorize the pair of socket id and user id
	onlineUser := make(map[string]bool) // it is a local cache and similar with regisration

	// once the server is booted , load online user list
	// userList := getOnlineUserInfo()
	userList := getOnlineUserList()
	for i := range userList {
		onlineUser[userList[i]] = true
	}

	server.On("connection", func(so socketio.Socket) {
		log.D("connected... %v", so.Id())

		newMessages := make(chan string, 100)

		so.On("chat", func(msg string) {
			newMessages <- msg
		})

		quit := make(chan struct{})               // 1-to-1
		quitMap := make(map[string]chan struct{}) // group

		groupList := make(map[string]int) // 0: deactivated 1:activated in order to prevent duplicated subscription

		so.On("disconnection", func() {
			user := userMap[so.Id()]
			log.D("disconnected... %v (%v)", so.Id(), user)

			// removeOnlineUserInfo(user)
			setOffline(user)

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

		// PUBSUB->IM
		userEvent := make(chan data.Event, 1000)

		so.On("join", func(user string) {
			log.D("Join...%v (%v)", user, so.Id())

			onlineUser[user] = true
			// addOnlineUserInfo(user)
			setOnline(user)

			for s, u := range userMap {
				log.D("session: %v, uid: %v", s, u)
				if u == user { // delete duplicated session
					delete(userMap, s)

					log.D("%v is trying to join again!", user)
				}
			}

			userMap[so.Id()] = user
			log.D("successfully subscribe was requested: (%v)->(%v)", user, user)

			subscribeEvent(user, userEvent, quit, user)

			subState := loadSubState(user)
			for i := range subState {
				log.D("%v subState: %v %v", user, i, subState[i])
				if groupList[subState[i]] != 1 {
					groupList[subState[i]] = 1

					log.D("SUBSCRIBE was requested: (%v)->(%v)", user, subState[i])
					subscribeEvent(subState[i], userEvent, quitMap[subState[i]], user)
				} else {
					log.E("It should not exist: %v", subState[i])
				}
			}

			// restore undelivered message when the user is offline
			log.D("Check S&F messages in redis")
			getEventList(user, userEvent)

			go func() {
				for { // received event
					select {
					case event := <-userEvent: // PUBSUB->IM->client
						if event.From[0] == 'g' {
							log.D("..> event(%v): (%v)->(%v) %v %v (%v)", event.EvtType, event.Originated, event.To, event.MsgID, event.Body, so.Id())
						} else {
							log.D("..> event(%v): (%v)->(%v) %v %v (%v)", event.EvtType, event.From, event.To, event.MsgID, event.Body, so.Id())
						}

						if event.EvtType == "subscribe" { // if the event type of received event is "subscribe", do "SUBSCRIBE"
							quitMap[event.From] = make(chan struct{})

							if groupList[event.From] != 1 {
								log.D("successfully SUBSCRIBE was requested: from (%v) toward (%v)", user, event.From)
								groupList[event.From] = 1

								subscribeEvent(event.From, userEvent, quitMap[event.From], user)

							} else {
								log.D("the subcribe was failed by duplication: (%v)->(%v)", user, event.From)
							}

							// NOTIFY
							participantList := loadParticipantList(event.From)
							raw, err := json.Marshal(participantList)
							if err != nil {
								log.E("Cannot encode to Json", err)
							}

							log.D("<-- %v: (%v)->(client) all participants: %v", "notify", event.Originated, string(raw))
							NotifyEvent := NewEvent("notify", event.From, event.Originated, user, "", event.Timestamp, string(raw))
							so.Emit("chat", NotifyEvent)

						} else if event.EvtType == "depart" {
							log.D("<-- %v: (%v)->(client) left user: %v from %v", "depart", event.Originated, user, event.From)
							so.Emit("chat", event)
						} else if event.EvtType == "join" {
							log.D("<-- %v: (%v)->(client) add participants: %v", "join", event.Originated, event.Body)

							so.Emit("chat", event)
						} else { // --> web client (socket.io)
							if event.Originated != user {
								log.D("<-- %v: (%v)->(client) %v", event.EvtType, event.Originated, event.MsgID)
								so.Emit("chat", event)
							}
						}

					case msg := <-newMessages: // client->IM->PUBSUB
						var event data.Event
						json.Unmarshal([]byte(msg), &event)
						if event.From[0] == 'g' {
							log.D("--> event(%v): (%v)->(%v) %v %v (%v)", event.EvtType, event.Originated, event.To, event.MsgID, event.Body, so.Id())
						} else {
							log.D("--> event(%v): (%v)->(%v) %v %v (%v)", event.EvtType, event.From, event.To, event.MsgID, event.Body, so.Id())
						}

						if event.EvtType == "message" {
							// if online, push into redis
							// if offline, backup received messages into QUEUE using rpush
							if event.To[0] == 'g' { // groupchat
								participantList := loadParticipantList(event.From)

								// To-Do: need to check sender is a valid user based on participant list
								if len(participantList) > 0 {
									for i := range participantList {
										if !onlineUser[participantList[i]] { // offline
											log.D("%v OFFLINE: PUSH (%v)->(%v) : %v (%v)", participantList[i], user, participantList[i], event.Body, event.MsgID)
											pushEvent(participantList[i], &event)
										}
									}

									log.D("<-- message: %v (%v)->(%v) (%v)", event.Body, event.Originated, event.To, event.MsgID)
									publishEvent(event.To, &event)
								} else { // if the server doesn't have participant list of the groupchat, save the event and then restart the groupchat
									log.D("PUSH (%v)->(%v) : %v", user, event.From, "store:"+event.From)
									pushEvent("store:"+event.From, &event)

									log.D("<-- RESTART to the client since no participant list in the server.")
									RestartEvent := NewEvent("restart", event.From, event.Originated, user, "", event.Timestamp, "")
									so.Emit("chat", RestartEvent)
								}

							} else { // 1-to-1
								if !onlineUser[event.To] {
									pushEvent(event.To, &event)
								} else {
									publishEvent(event.To, &event)
								}
							}

						} else if event.EvtType == "create" {
							log.I("Create groupchat: %v", event.From)

							var participantList []string
							if err = json.Unmarshal([]byte(event.Body), &participantList); err != nil {
								log.E("%v", err)
							}

							storeParticipantList(event.From, participantList) // save participant information into redis

							saveSubState(event.From, participantList) // save subscriber's state

							// subscribe the groupchat
							groupList[event.From] = 1 // locally managed group list to prevent duplicated subscribep
							quitMap[event.To] = make(chan struct{})

							log.D("do SUBSCRIBE itselt: (%v)->(%v)", user, event.From)
							subscribeEvent(event.From, userEvent, quitMap[event.From], user)

							for i := range participantList {
								if participantList[i] != user {
									log.D("request SUBSCRIBE: (%v)->(%v) for (%v)", user, participantList[i], event.From)
									subReqEvent := NewEvent("subscribe", event.From, user, participantList[i], "", event.Timestamp, event.Body)

									// update online hashmap
									onlineUser[participantList[i]] = checkOnline(participantList[i])

									if !onlineUser[participantList[i]] { // offline
										log.D("participantList[i] offline: PUSH the event (%v)->(%v)", user, participantList[i])
										pushEvent(participantList[i], &subReqEvent)
									} else { // online
										publishEvent(participantList[i], &subReqEvent)
										log.D("participantList[i] online: PUBLISH the event (%v)->(%v)", user, participantList[i])
									}
								}
							}
						} else if event.EvtType == "rejoin" {
							participantList := loadParticipantList(event.From)
							if len(participantList) == 0 { // if the server doesn't have participant list, use the client has
								err = json.Unmarshal([]byte(event.Body), &participantList)
								if err != nil {
									log.E("%v", err)
								}

								log.D("new participant list: %v for %v", participantList, event.From)
								storeParticipantList(event.From, participantList) // update participant list in redis

								log.D("Check S&F messages: %v", "store:"+event.From)
								fowardEventList("store:"+event.From, newMessages) // send the stored messages which was not sent since there is no participant list
							}

							if groupList[event.From] != 1 { // If the uses doesn't subscribe it yet, subscribe it
								groupList[event.From] = 1
								quitMap[event.From] = make(chan struct{})

								log.D("do REJOIN itself: (%v)->(%v)", event.Originated, event.To)
								subscribeEvent(event.From, userEvent, quitMap[event.From], user) // subscribe the groupchat

								// if the groupchat is deactivated, active it
								if checkGroupActivated(event.From) == false {
									for i := range participantList {
										if participantList[i] != user {
											log.D("request SUBSCRIBE: (%v)->(%v) for (%v)", user, participantList[i], event.To)
											subRqEvent := NewEvent("subscribe", event.To, user, participantList[i], "", event.Timestamp, event.Body)

											// update online hashmap
											onlineUser[participantList[i]] = checkOnline(participantList[i])

											if !onlineUser[participantList[i]] {
												log.D("offline: PUSH the event (%v)->(%v)", user, participantList[i])
												pushEvent(participantList[i], &subRqEvent)
											} else {
												publishEvent(participantList[i], &subRqEvent)
												log.D("online: PUBLISH the event (%v)->(%v)", user, participantList[i])
											}
										}
									}
								}

							} else {
								log.D("Rejoin was not requested by duplication: (%v)->(%v)", event.Originated, event.To)
							}

							// load particpant list and forward to the client
							raw, err := json.Marshal(participantList)
							if err != nil {
								log.E("Cannot encode to Json", err)
							}

							log.D("<-- NOTIFY from: %v To: %v Participants:%v", event.From, user, string(raw))
							NotifyEvent := NewEvent("notify", event.From, event.Originated, user, "", event.Timestamp, string(raw))
							so.Emit("chat", NotifyEvent)

						} else if event.EvtType == "refer" {
							var referedParticipantList []string
							err = json.Unmarshal([]byte(event.Body), &referedParticipantList)
							if err != nil {
								log.E("%v", err)
							}
							log.D("(REFER) request (%v)->(%v) with %v", user, event.From, referedParticipantList)

							saveSubState(event.From, referedParticipantList) // add refered subscriber's state

							// update the partcipant list
							var updatedParticipantList []string

							// current participants should know the joining of new participants
							participantList := loadParticipantList(event.From)
							for i := range participantList {
								// To-Do: curretnly I assume all users are valid but it is not true, I will add another logic for this later
								joinEvent := NewEvent("join", event.From, user, participantList[i], "", event.Timestamp, event.Body)

								// update online list
								onlineUser[participantList[i]] = checkOnline(participantList[i])

								if !onlineUser[participantList[i]] {
									log.D("PUSH (%v)->(%v)", user, participantList[i])
									pushEvent(participantList[i], &joinEvent)
								} else {
									publishEvent(participantList[i], &joinEvent)
								}

								updatedParticipantList = append(updatedParticipantList, participantList[i])
							}

							// subscribe request to new participants
							for i := range referedParticipantList {
								log.D("subscribe request: (%v)->(%v)", user, referedParticipantList[i])
								subRqEvent := NewEvent("subscribe", event.From, user, referedParticipantList[i], "", event.Timestamp, event.Body)

								if !onlineUser[referedParticipantList[i]] {
									log.D("PUSH (%v)->(%v)", user, referedParticipantList[i])
									pushEvent(referedParticipantList[i], &subRqEvent)
								} else {
									publishEvent(referedParticipantList[i], &subRqEvent)
								}

								// add refered participants
								updatedParticipantList = append(updatedParticipantList, referedParticipantList[i])
							}

							storeParticipantList(event.From, updatedParticipantList) // update participant list in redis

						} else if event.EvtType == "depart" {
							groupList[event.From] = 0 // deactivate
							log.D("Depart request: (%v)->(%v)", event.Originated, event.From)
							close(quitMap[event.From])
							delete(quitMap, event.From)

							removeSubState(event.Originated, event.From) // update substate

							participantList := loadParticipantList(event.From)

							var newparticipantList []string
							for i := range participantList {
								log.D("notice the departure of %v to %v in %v", event.Originated, participantList[i], event.From)
								departEvent := NewEvent("depart", event.From, user, participantList[i], "", event.Timestamp, "")

								// update online hashmap
								onlineUser[participantList[i]] = checkOnline(participantList[i])

								if !onlineUser[participantList[i]] {
									log.D("PUSH (%v)->(%v)", participantList[i])
									pushEvent(participantList[i], &departEvent)
								} else {
									publishEvent(participantList[i], &departEvent)
								}

								if participantList[i] != event.From {
									newparticipantList = append(newparticipantList, participantList[i])
								}
							}

							storeParticipantList(event.From, newparticipantList) // update participant list in redis

						} else { // deliver, display
							// if online, push into redis
							// if offline, backup received messages into QUEUE using rpush
							if event.To[0] == 'g' { // groupchat
								if !onlineUser[event.Originated] {
									pushEvent(event.Originated, &event)
									log.D("<.. push %v", event.MsgID)
								} else {
									publishEvent(event.Originated, &event)
									log.D("<.. publish %v", event.MsgID)
								}
							} else {
								if !onlineUser[event.To] {
									pushEvent(event.To, &event)
									log.D("<.. push %v", event.MsgID)
								} else {
									publishEvent(event.To, &event)
									log.D("<.. publish %v", event.MsgID)
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
func NewEvent(evtType string, from string, originated string, to string, msgID string, timestamp int, body string) data.Event {
	return data.Event{evtType, from, originated, to, msgID, timestamp, body}
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

func subscribeEvent(channel string, userEvent chan data.Event, quit chan struct{}, user string) {
	//	log.D("channel: %v", channel)

	redisChan := make(chan []byte, 1000)

	if err := rediscache.Subscribe(channel, redisChan, quit); err != nil {
		log.E("%s", err)
	}

	var needQuit = false

	go func() {
		select {
		case <-quit:
			log.D("Closing subscribe channel: %v by %v", channel, user)
			needQuit = true

			return
		}
	}()

	go func(quit bool) {
		for {
			if quit {
				log.D("Unsubscribing : %v (%v)", channel, user)
				return
			}

			raw := <-redisChan
			//	log.D("--> Event of (%v): %v", user, string(raw))

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
	//	log.D("<-- pushed message: key: %v value: %v", key, string(raw))

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

// fowardEventList is to get event list
func fowardEventList(key string, e chan string) {
	raw, err := rediscache.GetList(key)
	if err != nil {
		log.E("Error: %v", err)
	}

	if err = rediscache.Del(key); err != nil {
		log.E("Fail to delete: key: %v errMsg: %v", key, err)
	}

	for index := range raw {
		log.D("raw[%v] : %v", index, string(raw[index]))

		e <- raw[index]
	}
}

// pushEvent is to save a message
func saveSubState(groupID string, participantList []string) {
	raw, err := json.Marshal(groupID)
	if err != nil {
		log.E("Cannot encode to Json", err)
	}
	//	log.D("<-- pushed message: key: %v value: %v", key, string(raw))

	for i := range participantList {
		key := "substate:" + participantList[i]
		_, errRedis := rediscache.PushList(key, raw)
		if errRedis != nil {
			log.E("Error of pushEvent: %v", errRedis)
		}
	}
}

// getEventList is to get event list
func loadSubState(uid string) []string {
	key := "substate:" + uid
	raw, err := rediscache.GetList(key)
	if err != nil {
		log.E("Error: %v", err)
	}

	var list []string
	for i := range raw {
		var groupID string
		err = json.Unmarshal([]byte(raw[i]), &groupID)

		list = append(list, groupID)

		if err != nil {
			log.E("%v: %v", key, err)
		}
	}

	return list
}

func removeSubState(uid string, groupID string) {
	key := "substate:" + uid
	log.D("substate(remove): %v of %v (%v)", groupID, uid, key)

	if err := rediscache.DelList(key, groupID); err != nil {
		log.E("%v", err)
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

	_, errRedis := rediscache.SetCache(key, raw, 259200)
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

func checkOnline(user string) bool {
	status, err := rediscache.GetCache("on:" + user)
	if err != nil {
		log.E("Error: %v", err)
	}

	// log.D("status: %v", status)

	if status == "1" {
		return true
	} else {
		return false
	}
}

func setOnline(user string) {
	key := ("on:" + user)

	// save into redis
	raw, err := json.Marshal(1)
	if err != nil {
		log.E("Cannot encode to Json", err)
	}
	log.D("key: %v value: %v", key, string(raw))

	_, errRedis := rediscache.SetCache(key, raw, 3600)
	if errRedis != nil {
		log.E("Error of setEvent: %v", errRedis)
	}
}

func setOffline(user string) {
	key := ("on:" + user)

	// save into redis
	raw, err := json.Marshal(0)
	if err != nil {
		log.E("Cannot encode to Json", err)
	}
	log.D("key: %v value: %v", key, string(raw))

	_, errRedis := rediscache.SetCache(key, raw, 3600)
	if errRedis != nil {
		log.E("Error of setEvent: %v", errRedis)
	}
}

// addOnlineUserInfo is to save a group information
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

		_, errRedis := rediscache.SetCache(key, raw, 259200)
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

	// log.D("users: %v", userList)

	sort.Slice(userList, func(i, j int) bool {
		return userList[i] < userList[j]
	})
	// remove user
	index := sort.Search(len(userList), func(i int) bool {
		return user <= userList[i]
	})

	// log.D("users: %v", userList)

	// log.D("user: %v, index: %v, userList[%v]:%v lenth of userList:%v", user, index, index, userList[index], len(userList))
	if (index < len(userList)) && (user == userList[index]) {
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
	//	log.D("key: %v value: %v", key, string(raw))

	_, errRedis := rediscache.SetCache(key, raw, 259200)
	if errRedis != nil {
		log.E("Error of setEvent: %v", errRedis)
	}
}

// getOnlineUserInfo is to load onlie user list
/*
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
} */

// getOnlineUserInfo is to load onlie user list
func getOnlineUserList() []string {
	var onlineList []string
	values := rediscache.GetPrefixValues("on:")
	for i := range values {
		if checkOnline(values[i]) {
			onlineList = append(onlineList, values[i][2:len(values[i])])
		}
	}

	for i := range onlineList {
		log.D("%v %v", i, onlineList[i])
	}

	return onlineList
}

func checkGroupActivated(gid string) bool {
	// load online user list
	key := gid
	raw, err := rediscache.GetCache(key)
	if err != nil {
		log.E("Error: %v", err)
		return false
	}
	log.D("%v: %v", gid, raw)

	if raw == "[]" || raw == "" {
		return false
	} else {
		return true
	}
}

// storeParticipantList is to save the list of group participants
func storeParticipantList(groupID string, participantList []string) {
	//	log.D("group: %v", participantList)

	key := "participants:" + groupID

	raw, err := json.Marshal(participantList)
	if err != nil {
		log.E("Cannot encode to Json", err)
	}

	_, errRedis := rediscache.SetCache(key, raw, 0)
	if errRedis != nil {
		log.E("Error of setEvent: %v", errRedis)
	}
}

// loadParticipantList is to get the list of group participants
func loadParticipantList(groupID string) []string {
	key := "participants:" + groupID

	raw, err := rediscache.GetCache(key)
	if err != nil {
		log.E("Error: %v", err)
	}

	var participantList []string
	err = json.Unmarshal([]byte(raw), &participantList)
	if err != nil {
		log.E("%v: %v", key, err)
	}

	log.D("participants: %v", participantList) // To-Do

	return participantList
}
