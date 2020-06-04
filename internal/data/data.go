package data

// UserProfile is a structure for a person
type UserProfile struct {
	UID   string
	Name  string
	Email string
	Age   int
}

// Message is the data structure of messages
type Message struct {
	EvtType   string
	From      string
	To        string
	MsgID     string
	Timestamp int
	Text      string
}

// Event is to define the event
type Event struct {
	EvtType   string
	From      string
	To        string
	MsgID     string
	Timestamp int
	Text      string
}
