package nerv

import (
	"errors"
	"math/rand/v2"
	"sync"
)

const (
	distBroadcast = iota
	distDirect
)

const (
	selectArbitrary = iota
	selectRoundRobin
	selectRandom
)

var ErrTopicNoSubscriberFound = errors.New("no subscriber found")

type eventTopic struct {
	distributionType int
	selectionType    int
	subscribed       []EventRecvr
	rrIdx            int
	rrMu             sync.Mutex
}

type TopicCfg struct {
	Name          string
	DistType      int
	SelectionType int
}

func NewTopic(name string) *TopicCfg {
	return &TopicCfg{
		Name:          name,
		DistType:      distBroadcast,
		SelectionType: selectArbitrary,
	}
}

func (t *TopicCfg) UsingBroadcast() *TopicCfg {
	t.DistType = distBroadcast
	return t
}

func (t *TopicCfg) UsingDirect() *TopicCfg {
	t.DistType = distDirect
	return t
}

func (t *TopicCfg) UsingNoSelection() *TopicCfg {
	return t.UsingArbitrary()
}

func (t *TopicCfg) UsingRoundRobinSelection() *TopicCfg {
	t.SelectionType = selectRoundRobin
	return t
}

func (t *TopicCfg) UsingRandomSelection() *TopicCfg {
	t.SelectionType = selectRandom
	return t
}

func (t *TopicCfg) UsingArbitrary() *TopicCfg {
	t.SelectionType = selectArbitrary
	return t
}

func (t *eventTopic) hasSubscriber() bool {
	for _, s := range t.subscribed {
		if s != nil {
			return true
		}
	}
	return false
}

func (t *eventTopic) randomSubscriber() (int, error) {

	var potentials []int

	for i, s := range t.subscribed {
		if s != nil {
			potentials = append(potentials, i)
		}
	}

	if len(potentials) == 0 {
		return -1, ErrTopicNoSubscriberFound
	}

	return potentials[rand.IntN(len(potentials))], nil
}

func (t *eventTopic) rrNext() (int, error) {

	t.rrMu.Lock()
	defer t.rrMu.Unlock()

	if t.rrIdx >= len(t.subscribed) {
		t.rrIdx = 0
	}

	checked := 1
	for {
		if t.subscribed[t.rrIdx] != nil {
			break
		}

		t.rrIdx += 1

		if t.rrIdx >= len(t.subscribed) {
			t.rrIdx = 0
		}

		checked += 1
		if checked > len(t.subscribed) {
			return -1, ErrTopicNoSubscriberFound
		}
	}

	selected := t.rrIdx
	t.rrIdx += 1
	return selected, nil
}
