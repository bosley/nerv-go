package nerv

import (
	"time"
)

type EventRecvr func(event *Event)

type DataRecvr func(data interface{})

// Event structure that is pushed through the event engine and delivered
// to the subscriber(s) of topics
type Event struct {
	Spawned  time.Time   `json:spawned`
	Topic    string      `json:topic`
	Producer string      `json:producer`
	Data     interface{} `json:data`
}

// The receiver of events, potentially subscribed
// to multiple topics
type Consumer struct {
	Id string
	Fn EventRecvr
}

// Interface used in nerv engine to manage modules
// loaded in by the user
type Module interface {
	Start() error
	Shutdown()
	SetSubmitter(fn *ModuleSubmitter)
}

// Submission functions leveraged by modules
//
//	SubmitData -  Permits module to submit raw data as an event onto
//	              its associated topic as its own producer, where consumers
//	              registered to that function will recieve it
//	SubmitEvent - Place an event onto the bus from the module that may or may
//	              not go to consumers of the module. This function is useful
//	              for fowarding events through a module without obfuscating
//	              the original event
type ModuleSubmitter struct {
	SubmitData  DataRecvr
	SubmitEvent EventRecvr
}

// A handler interface made to permit handing off submission access to
// structs and functions without handing over entire access to the
// primary eventing engine
type SubmissionHandler interface {
	Submit(topic string, data interface{}) error
	SubmitAs(id string, topic string, data interface{}) error
}

func NewSubmissionHandler(engine *Engine, defaultSubmitter string) SubmissionHandler {
	return &submissionHandler{
		defaultSubmitter,
		engine,
	}
}

type submissionHandler struct {
	defaultSubmitter string
	engine           *Engine
}

func (sh *submissionHandler) Submit(topic string, data interface{}) error {
	return sh.engine.Submit(sh.defaultSubmitter, topic, data)
}

func (sh *submissionHandler) SubmitAs(id string, topic string, data interface{}) error {
	return sh.engine.Submit(id, topic, data)
}
