package events

import (
	log "github.com/sirupsen/logrus"
)

var EventChannel = make(chan Event, 100)

// Receiver received an Event and processes it
type Receiver func(Event) error

// EventListener receives events on a channel and dispatches events to the various
// receivers.
type EventListener struct {
	eventChannel chan Event

	// receivers for a specific type of event
	receivers map[EventType][]Receiver
}

// NewEventListener creates a new EventListener
func NewEventLister(eventChannel chan Event) *EventListener {
	el := EventListener{}
	el.receivers = make(map[EventType][]Receiver)
	el.eventChannel = eventChannel
	return &el
}

// Listen listens to the events coming in on the event channel
func (el *EventListener) Listen() error {

	go func() {
		for e := range el.eventChannel {
			switch ev := e.(type) {
			case TootEvent:
				el.SendEventToReceivers(ev)
			case ReplyEvent:
				el.SendEventToReceivers(ev)
			case RefreshEvent:
				el.SendEventToReceivers(ev)
			default:
				log.Errorf("UNKNOWN EVENT")
			}
		}
	}()

	return nil
}

// RegisterReceiver registers a receiver for a specific event type
func (el *EventListener) RegisterReceiver(eventType EventType, receiver Receiver) error {
	el.receivers[eventType] = append(el.receivers[eventType], receiver)
	return nil
}

// SendEventToReceivers sends an event to all the receivers for that event type
func (el *EventListener) SendEventToReceivers(event Event) error {
	receivers := el.receivers[event.GetEventType()]
	for _, r := range receivers {
		r(event)
	}
	return nil
}
