package data

// UserProfile is a structure for a person
type UserProfile struct {
	UID  string
	Name string
}

// Event is to define the event
type Event struct {
	EvtType    string
	From       string
	Originated string
	To         string
	MsgID      string
	Timestamp  int
	Body       string
}