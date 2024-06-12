package nerv

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

const (
	nervTopicInternal = "nerv.internal"
)

var ErrEngineAlreadyRunning = errors.New("engine already running")
var ErrEngineNotRunning = errors.New("engine not running")
var ErrEngineUnknownTopic = errors.New("unknown topic")
var ErrEngineUnknownConsumer = errors.New("unknown consumer")
var ErrEngineDuplicateTopic = errors.New("duplicate topic")

type Engine struct {
	topics    map[string]*eventTopic
	consumers map[string]EventRecvr

	modules map[string]Module

	topicMu sync.Mutex
	subMu   sync.Mutex
	wg      sync.WaitGroup

	eventChan chan Event

	ctx    context.Context
	cancel context.CancelFunc

	running bool

	callbacks EngineCallbacks
}

type EngineCallbacks struct {
	RegisterCb EventRecvr
	NewTopicCb EventRecvr
	ConsumeCb  EventRecvr
	SubmitCb   EventRecvr
}

func NewEngine() *Engine {
	eng := &Engine{
		topics:    make(map[string]*eventTopic),
		consumers: make(map[string]EventRecvr),
		modules:   make(map[string]Module),
		eventChan: make(chan Event),
		running:   false,
		callbacks: EngineCallbacks{
			nil,
			nil,
			nil,
			nil,
		},
	}

	eng.ctx, eng.cancel = context.WithCancel(context.Background())

	eng.CreateTopic(
		NewTopic(nervTopicInternal).
			UsingBroadcast().
			UsingNoSelection())
	return eng
}

func (eng *Engine) WithTopics(topics []*TopicCfg) *Engine {
	for _, topic := range topics {
		if err := eng.CreateTopic(topic); err != nil {
			slog.Debug("failed to create bulk topic", "topic", topic.Name, "err", err.Error())
			panic("failed to bulk-create topics")
		}
		go eng.checkCallback(eng.callbacks.NewTopicCb, topic)
	}
	return eng
}

func (eng *Engine) WithCallbacks(cbs EngineCallbacks) *Engine {
	eng.callbacks = cbs
	return eng
}

func (eng *Engine) ContainsTopic(topic *string) bool {
	// no guard, just read. no-exist topics filtered on emit
	_, ok := eng.topics[*topic]
	return ok
}

func (eng *Engine) ContainsConsumer(id *string) bool {
	// no guard, just read. no-exist topics filtered on emit
	_, ok := eng.consumers[*id]
	return ok
}

func (eng *Engine) Start() error {

	slog.Debug("Start", "running", eng.running)

	if eng.running {
		return ErrEngineAlreadyRunning
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

	for name, mod := range eng.modules {
		slog.Debug("indicating start to module", "module", name)
		mod.IndStart()
	}

	return nil
}

func (eng *Engine) Stop() error {

	slog.Debug("Stop", "running", eng.running)

	if !eng.running {
		return ErrEngineNotRunning
	}

	for name, mod := range eng.modules {
		slog.Debug("indicating shutdown to module", "module", name)
		mod.IndShutdown()
	}

	close(eng.eventChan)

	eng.cancel()

	eng.wg.Wait()

	return nil
}

func (eng *Engine) checkCallback(fn EventRecvr, data interface{}) {
	if fn != nil {
		fn(&Event{
			time.Now(),
			"nerv.internal",
			"nerv.engine",
			data,
		})
	}
}

func (eng *Engine) Submit(id string, topic string, data interface{}) error {
	return eng.SubmitEvent(Event{
		time.Now(),
		topic,
		id,
		data,
	})
}

func (eng *Engine) SubmitEvent(event Event) error {

	slog.Debug("SubmitEvent", "topic", event.Topic, "producer", event.Producer)
	if !eng.running {
		return ErrEngineNotRunning
	}

	eng.eventChan <- event

	slog.Debug("SUBMITTED")

	go eng.checkCallback(eng.callbacks.SubmitCb, &event)
	return nil
}

func (eng *Engine) Register(sub Consumer) {
	slog.Debug("Register", "consumer", sub.Id)

	eng.subMu.Lock()
	defer eng.subMu.Unlock()

	eng.consumers[sub.Id] = sub.Fn

	go eng.checkCallback(eng.callbacks.RegisterCb, &sub)
	return
}

func (eng *Engine) CreateTopic(cfg *TopicCfg) error {

	slog.Debug("CreateTopic", "name", cfg.Name, "tx", cfg.DistType, "sel", cfg.SelectionType)

	eng.topicMu.Lock()
	defer eng.topicMu.Unlock()

	_, ok := eng.topics[cfg.Name]
	if ok {
		return ErrEngineDuplicateTopic
	}
	eng.topics[cfg.Name] = &eventTopic{
		distributionType: cfg.DistType,
		selectionType:    cfg.SelectionType,
		subscribed:       make([]EventRecvr, 0),
	}

	go eng.checkCallback(eng.callbacks.NewTopicCb, cfg)
	return nil
}

func (eng *Engine) DeleteTopic(topicId string) {

	slog.Debug("DeleteTopic", "name", topicId)

	eng.topicMu.Lock()
	defer eng.topicMu.Unlock()

	delete(eng.topics, topicId)
}

// Does not check for duplicate subscriptions
func (eng *Engine) SubscribeTo(topicId string, consumers ...string) error {

	slog.Debug("SubscribeTo", "topic", topicId)

	for _, s := range consumers {
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

	subscribedFn, aok := eng.consumers[subId]
	if !aok {
		return ErrEngineUnknownConsumer
	}

	topic, tok := eng.topics[topicId]
	if !tok {
		return ErrEngineUnknownTopic
	}

	slog.Info("Adding consumer", "id", subId)

	topic.subscribed = append(topic.subscribed, subscribedFn)

	eng.topics[topicId] = topic

	info := fmt.Sprintf("%s:%s", topicId, subId)
	go eng.checkCallback(eng.callbacks.ConsumeCb, &info)
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
		slog.Debug("no consumers for event topic", "topic", event.Topic, "origin", event.Producer)
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
	for _, consumer := range topic.subscribed {
		if consumer == nil {
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			consumer(event)
		}()
	}
	wg.Wait()
}

func validateId(idx int, consumers []EventRecvr) bool {
	if idx < 0 || idx >= len(consumers) {
		slog.Warn("invalid idx", "idx", idx)
		return false
	}
	if consumers[idx] == nil {
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
				slog.Warn("unable to find valid consumer for arbitrary selection")
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

func (eng *Engine) UseModule(
	mod Module,
	productionTarget *TopicCfg,
	consumers []Consumer) error {

	// Create topic that the module will publish on,
	// and ensure that the module has a unique name
	if err := eng.CreateTopic(productionTarget); err != nil {
		return err
	}

	// Setup a submitter on the module so it can post
	// data to the event bus as its [name].producer
	mod.SetSubmitter(&ModuleSubmitter{
		SubmitData: func(data interface{}) {
			eng.Submit(
				fmt.Sprintf("%s.producer", productionTarget.Name),
				productionTarget.Name,
				data)
		},
		SubmitEvent: func(event *Event) {
			eng.SubmitEvent(*event)
		},
	})

	// Register all consumers that the user gave us
	// that will receive the events of the producer
	for _, consumer := range consumers {
		eng.Register(consumer)

		if err := eng.subscribeTo(productionTarget.Name, consumer.Id); err != nil {
			return err
		}
	}

	eng.modules[productionTarget.Name] = mod
	return nil
}
