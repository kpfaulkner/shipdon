package events

// EventType...   send toot, refresh, reply etc.
type EventType int

type RefreshType int

const (
	SEND_TOOT EventType = iota
	REPLY
	REFRESH_MESSAGES

	TIMELINE_REFRESH RefreshType = iota
	USER_REFRESH
	HASHTAG_REFRESH
	NOTIFICATION_REFRESH
	THREAD_REFRESH
	LIST_REFRESH
	HOME_REFRESH
)

// Event means something's happened in the UI that needs to go back to the main app for
// action.
type Event interface {
	GetEventType() EventType
}

type EventBase struct {
	EType EventType
}

func (eb EventBase) GetEventType() EventType {
	return eb.EType
}

// TootEvent...  dummy
type TootEvent struct {
	EventBase
	Message string
}

func NewTootEvent(message string) TootEvent {
	te := TootEvent{EventBase{EType: SEND_TOOT}, message}
	return te
}

type RefreshEvent struct {
	EventBase

	// Refresh user, home, hashtag, notification etc.
	RefreshType RefreshType

	// GetOlder means instead of getting latest, get earlier ones (ie the user has scrolled down)
	GetOlder bool

	// used for queries against Mastodon. This can also be home, notification but in the case of
	// lists, it might be !42 where 42 is the list ID, or #hashtag...  or userID (int64... how to identify that its for a user?)
	TimelineID string

	// number of status' to get
	Count int

	// clear existing entries for this timeline (instead of adding to existing)
	ClearExisting bool
}

// Create RefreshEvent for a specific timeline and with optional SinceID/MaxIDs
func NewRefreshEvent(timelineID string, clearExisting bool, refreshType RefreshType) RefreshEvent {
	te := RefreshEvent{EventBase: EventBase{EType: REFRESH_MESSAGES}, TimelineID: timelineID, RefreshType: refreshType, ClearExisting: clearExisting}
	return te
}

func NewRefreshAllEvent(clearExisting bool, refreshType RefreshType) RefreshEvent {
	te := RefreshEvent{EventBase: EventBase{EType: REFRESH_MESSAGES}, TimelineID: "home", RefreshType: refreshType, ClearExisting: clearExisting}
	return te
}

func NewHomeRefreshEvent(clearExisting bool, refreshType RefreshType) RefreshEvent {
	te := RefreshEvent{EventBase: EventBase{EType: REFRESH_MESSAGES}, TimelineID: "home", RefreshType: refreshType, ClearExisting: clearExisting}
	return te
}

func NewGetOlderRefreshEvents(timelineID string, refreshType RefreshType) RefreshEvent {
	te := RefreshEvent{EventBase: EventBase{EType: REFRESH_MESSAGES}, TimelineID: timelineID, RefreshType: refreshType, ClearExisting: false, GetOlder: true}
	return te
}

// ReplyEvent...  dummy
type ReplyEvent struct {
	EventBase
	Message string
	ReplyID string
}

func NewReplyEvent(message string, replyID string) ReplyEvent {
	re := ReplyEvent{EventBase{EType: REPLY}, message, replyID}
	return re
}

// UGLY UGLY Global FireEvent function... used to populate event onto channel
func FireEvent(ev Event) error {

	EventChannel <- ev
	return nil
}
