package nerv

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

type Engine struct {
	topics      map[string]*eventTopic
	subscribers map[string]EventRecvr

	topicMu sync.Mutex
	subMu   sync.Mutex
	wg      sync.WaitGroup

	eventChan chan Event

	ctx    context.Context
	cancel context.CancelFunc

	server *NervServer

	running bool
}

func NewEngine() *Engine {
	eng := &Engine{
		topics:      make(map[string]*eventTopic),
		subscribers: make(map[string]EventRecvr),
		eventChan:   make(chan Event),
		server:      nil,
		running:     false,
	}

	eng.ctx, eng.cancel = context.WithCancel(context.Background())
	return eng
}

func (eng *Engine) WithServer(cfg NervServerCfg) *Engine {

	if eng.server != nil {
		return eng
	}

	eng.server = HttpServer(cfg, eng)

	if eng.running {
		if err := eng.server.Start(); err != nil {
			slog.Error("err:%v", err)
			panic("Failed to start endpoint server with running engine")
		}
	}

	return eng
}

func (eng *Engine) ContainsTopic(topic *string) bool {
	// no guard, just read. no-exist topics filtered on emit
	_, ok := eng.topics[*topic]
	return ok
}

func (eng *Engine) ContainsSubscriber(id *string) bool {
	// no guard, just read. no-exist topics filtered on emit
	_, ok := eng.subscribers[*id]
	return ok
}

func (eng *Engine) Start() error {

	slog.Debug("Start", "running", eng.running)

	if eng.running {
		return errors.New("unable to start, engine already running")
	}

	eng.wg.Add(1)

	eng.running = true

	go func() {

		defer func() {
			eng.running = false
		}()

		defer eng.wg.Done()

		for {
			select {
			case <-eng.ctx.Done():
				return
			case event := <-eng.eventChan:
				if len(event.Topic) > 0 {
					eng.emitEvent(&event)
				}
			}
		}
	}()

	if eng.server == nil {
		return nil
	}

	return eng.server.Start()
}

func (eng *Engine) Stop() error {

	slog.Debug("Stop", "running", eng.running)

	if !eng.running {
		return errors.New("unable to stop, engine not running")
	}

	close(eng.eventChan)

	eng.cancel()

	eng.wg.Wait()

	return nil
}

func (eng *Engine) Submit(id string, topic string, data interface{}) error {

	slog.Debug("Submit", "id", id, "topic", topic)
	if !eng.running {
		return errors.New("unable to submit, engine not running")
	}

	eng.eventChan <- Event{
		time.Now(),
		topic,
		id,
		data,
	}

	slog.Debug("SUBMITTED")

	return nil
}

func (eng *Engine) SubmitEvent(event Event) error {

	slog.Debug("SubmitWithTime", "topic", event.Topic, "producer", event.Producer)
	if !eng.running {
		return errors.New("unable to submit, engine not running")
	}

	eng.eventChan <- event

	slog.Debug("SUBMITTED")
	return nil
}

func (eng *Engine) Register(sub Subscriber) {
	slog.Debug("Register", "subscriber", sub.Id)
	eng.subMu.Lock()
	defer eng.subMu.Unlock()

	eng.subscribers[sub.Id] = sub.Fn
	return
}

func (eng *Engine) CreateTopic(cfg *TopicCfg) error {

	slog.Debug("CreateTopic", "name", cfg.Name, "tx", cfg.DistType, "sel", cfg.SelectionType)

	eng.topicMu.Lock()
	defer eng.topicMu.Unlock()

	_, ok := eng.topics[cfg.Name]
	if ok {
		return errors.New("Duplicate topic id")
	}
	eng.topics[cfg.Name] = &eventTopic{
		distributionType: cfg.DistType,
		selectionType:    cfg.SelectionType,
		subscribed:       make([]EventRecvr, 0),
	}
	return nil
}

func (eng *Engine) DeleteTopic(topicId string) {

	slog.Debug("DeleteTopic", "name", topicId)

	eng.topicMu.Lock()
	defer eng.topicMu.Unlock()

	delete(eng.topics, topicId)
}

// Does not check for duplicate subscriptions
func (eng *Engine) SubscribeTo(topicId string, subscribers ...string) error {

	slog.Debug("SubscribeTo", "topic", topicId)

	for _, s := range subscribers {
		if err := eng.subscribeTo(topicId, s); err != nil {
			return err
		}
	}
	return nil
}

func (eng *Engine) subscribeTo(topicId string, subId string) error {

	eng.subMu.Lock()
	defer eng.subMu.Unlock()

	eng.topicMu.Lock()
	defer eng.topicMu.Unlock()

	subscribedFn, aok := eng.subscribers[subId]
	if !aok {
		return errors.New("Uknown subscriber id")
	}

	topic, tok := eng.topics[topicId]
	if !tok {
		return errors.New("Unknown topic id")
	}

	slog.Info("Adding subscriber", "id", subId)

	topic.subscribed = append(topic.subscribed, subscribedFn)

	eng.topics[topicId] = topic

	return nil
}

func (eng *Engine) emitEvent(event *Event) {

	slog.Debug("emitEvent", "topic", event.Topic, "producer", event.Producer)

	eng.subMu.Lock()
	defer eng.subMu.Unlock()

	eng.topicMu.Lock()
	defer eng.topicMu.Unlock()

	topic, tok := eng.topics[event.Topic]
	if !tok {
		slog.Warn("unknown topic")
		return
	}

	if !topic.hasSubscriber() {
		slog.Debug("no subscribers for event topic", "topic", event.Topic, "origin", event.Producer)
		return
	}

	switch topic.distributionType {
	case distBroadcast:
		publishBroadcast(event, topic)
		return
	case distDirect:
		publishDirect(event, topic)
		return
	}

	slog.Warn("unknown distribution type", "dist", topic.distributionType)
}

func publishBroadcast(event *Event, topic *eventTopic) {
	slog.Debug("broadcast")

	var wg sync.WaitGroup
	for _, subscriber := range topic.subscribed {
		if subscriber == nil {
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			subscriber(event)
		}()
	}
	wg.Wait()
}

func validateId(idx int, subscribers []EventRecvr) bool {
	if idx < 0 || idx >= len(subscribers) {
		slog.Warn("invalid idx", "idx", idx)
		return false
	}
	if subscribers[idx] == nil {
		slog.Warn("nil idx for selection")
		return false
	}
	return true
}

func publishDirect(event *Event, topic *eventTopic) {

	switch topic.selectionType {
	case selectArbitrary:
		slog.Debug("direct", "method", "arbitrary")
		for _, s := range topic.subscribed {
			if s != nil {
				s(event)
				return
			} else {
				slog.Warn("unable to find valid subscriber for arbitrary selection")
				return
			}
		}
		return
	case selectRoundRobin:
		slog.Debug("direct", "method", "round robin")
		idx, err := topic.rrNext()
		if err != nil {
			slog.Warn(err.Error())
			return
		}
		if !validateId(idx, topic.subscribed) {
			return
		}
		topic.subscribed[idx](event)
		return
	case selectRandom:
		slog.Debug("direct", "method", "random")
		idx, err := topic.randomSubscriber()
		if err != nil {
			slog.Warn(err.Error())
			return
		}
		if !validateId(idx, topic.subscribed) {
			return
		}
		slog.Debug("dest", "idx", idx)
		topic.subscribed[idx](event)
		return
	}
	slog.Warn("invalid selection type", "selection type", topic.selectionType)
}
