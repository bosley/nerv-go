package nerv

import (
	"time"
)

type Event struct {
	Spawned  time.Time   `json:spawned`
	Topic    string      `json:topic`
	Producer string      `json:producer`
	Data     interface{} `json:data`
}

type EventRecvr func(event *Event)
type DataRecvr func(data interface{})

type Consumer struct {
	Id string
	Fn EventRecvr
}

type Module interface {
  IndStart() error
  IndShutdown()
  SetSubmitterFn(fn DataRecvr)
}

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
