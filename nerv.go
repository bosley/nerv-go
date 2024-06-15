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

// The receiver of events, potentially subscribed
// to multiple topics
type Consumer struct {
	Id string
	Fn EventRecvr
}

// Interface used in nerv engine to manage modules
// loaded in by the user
type Module interface {
	GetName() string
	RecvModulePane(pane *ModulePane)
	Start() error
	Shutdown()
}

// Interface for module to take action without
// access to engine object
type ModulePane struct {

	// Subscribe a set of consumers to a topic. If register is TRUE, then the
	// engine will be momentarily locked to ensure that the consumer is registered
	// as subscription requires a registered consumer. It is safe to always pass TRUE
	// but it map cause performance overhead if its called a lot as such
	SubscribeTo func(topic string, consumers []Consumer, register bool) error

	// The submitter functions
	Submitter ModuleSubmitter

	// Retrieve whatever meta-data the users stored
	// with the module
	GetModuleMeta func(moduleName string) interface{}
}

// Submission functions leveraged by modules
//
//	SubmitToTopic - Permits module to submit raw data as an event onto
//	                its associated topics as its own producer, where consumers
//	                registered to that function will recieve it
//	SubmitEvent   - Place an event onto the bus from the module that may or may
//	                not go to consumers of the module. This function is useful
//	                for fowarding events through a module without obfuscating
//	                the original event
type ModuleSubmitter struct {
	SubmitTo    func(topic string, data interface{})
	SubmitEvent EventRecvr
}

// A handler interface made to permit handing off submission access to
// structs and functions without handing over entire access to the
// primary eventing engine and without having to make a whole module
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
