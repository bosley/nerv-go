package nerv

import (
	"time"
)

// Something that receives a nerv event
type EventRecvr func(event *Event)

// Event structure that is pushed through the event engine and delivered
// to the subscriber(s) of topics
type Event struct {
	Spawned  time.Time   `json:spawned`
	Topic    string      `json:topic`
	Producer string      `json:producer`
	Data     interface{} `json:data`
}

// Generalized "producer" that can be set
// to publish to an event system in different ways
// (remotely, locally, to multople topics, etc)
type Producer func(data interface{}) error

// The receiver of events, potentially subscribed
// to multiple topics
type Consumer struct {
	Id string
	Fn EventRecvr
}

// Context hands the event that has occurred along with
// a producer to publish back onto the engine. Since there is no
// connection directly to the sender, the state of the conversation
// must be saved to track state over time if so desired.
type Context struct {
	Event *Event
}

// A route is just a context receiver that can be handed around
// when a "Route" is added to an engine. These "routes" are a
// simple abstraction over the topic/producer/consumer module
// that makes it simple to leverage the eventing system in an
// application for smaller taskes without requiring the implementation
// of an entire module
type Route func(c *Context)
