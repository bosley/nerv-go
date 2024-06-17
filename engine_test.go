package nerv

import (
	"fmt"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"
)

type eventActivity struct {
	topic  string
	origin string
	data   int
}

type testActor struct {
	name  string
	id    int
	recvd []eventActivity
}

func (t *testActor) Id() string {
	return t.name
}

func (t *testActor) Accept(event *Event) {
	t.recvd = append(t.recvd, eventActivity{
		event.Topic,
		event.Producer,
		event.Data.(int),
	})
}

func (t *testActor) IsReady() bool {
	return true
}

func generateActors(num int) ([]*testActor, []string) {
	var testActors []*testActor
	var ids []string
	testActors = make([]*testActor, 0)
	for i := 0; i < num; i++ {
		testActors = append(
			testActors,
			&testActor{
				name:  fmt.Sprintf("device.%d", i),
				id:    i,
				recvd: make([]eventActivity, 0),
			})
		ids = append(ids, fmt.Sprintf("device.%d", i))
	}
	return testActors, ids
}

func recvdBroadcastNx(actual *testActor, event eventActivity) int {
	numOccur := 0
	for _, activity := range actual.recvd {
		if activity.data == event.data &&
			activity.origin == event.origin {
			numOccur += 1
		}
	}
	return numOccur
}

func makeTopic(name string) *TopicCfg {
	return NewTopic(name).
		UsingBroadcast().
		UsingNoSelection()
}

func TestBroadcast(t *testing.T) {

	slog.SetDefault(
		slog.New(
			slog.NewTextHandler(os.Stdout,
				&slog.HandlerOptions{
					Level: slog.LevelWarn,
				})))
	actors, actor_ids := generateActors(25)
	topics := []*TopicCfg{
		makeTopic("/user/bldg-a/floor-0/temp"),
		makeTopic("/user/bldg-a/floor-0/humitity"),
		makeTopic("/user/bldg-a/floor-1/temp"),
		makeTopic("/user/bldg-a/floor-1/humidity"),
		makeTopic("/user/bldg-a/floor-2/temp"),
		makeTopic("/user/bldg-a/floor-2/humidity"),
	}

	cbMu := new(sync.Mutex)
	cbs := make(map[string]bool)
	cbNames := make([]string, 0)

	createCb := func(id string) EventRecvr {
		cbs[id] = false
		cbNames = append(cbNames, id)
		return func(event *Event) {
			cbMu.Lock()
			defer cbMu.Unlock()
			slog.Warn("engine callback", "to", id)
			cbs[id] = true
		}
	}

	engine := NewEngine().
		WithCallbacks(
			EngineCallbacks{
				RegisterCb: createCb("registration"),
				NewTopicCb: createCb("new_topic"),
				ConsumeCb:  createCb("consumed"),
				SubmitCb:   createCb("submission"),
			}).
		// We MUST place _after_ WithCallbacks for test as
		// we expect registration callbacks for topics
		WithTopics(topics)

	for _, a := range actors {
		engine.Register(Consumer{a.Id(), a.Accept})
	}

	for _, top := range topics {
		if err := engine.SubscribeTo(top.Name, actor_ids...); err != nil {
			t.Fatalf("err:%v", err)
		}
	}

	fmt.Println("starting engine")
	if err := engine.Start(); err != nil {
		t.Fatalf("err: %v", err)
	}
	fmt.Println("[ENGINE STARTED]")
	fmt.Println("starting sends")
	numSends := 5
	events := make([]eventActivity, len(topics)*len(actors))
	for _, topic := range topics {
		for i := 0; i < numSends; i++ {
			for _, sub := range actor_ids {
				event := eventActivity{
					topic.Name,
					sub,
					i,
				}
				events = append(events, event)
				engine.Submit(sub, topic.Name, event.data)
			}
		}
	}

	fmt.Println("[SENDS COMPLETE]")
	fmt.Println("checking engine callbacks")

	time.Sleep(1 * time.Second)

	for _, cb := range cbNames {
		val, ok := cbs[cb]
		if !ok {
			t.Fatalf("Failed to get cb")
		}
		if val != true {
			t.Fatalf("Engine failed to fire a registered cb")
		}
	}

	fmt.Println("[CHECKS COMPLETE]")
	fmt.Println("stopping engine")

	if err := engine.Stop(); err != nil {
		t.Fatalf("err: %v", err)
	}
	fmt.Println("[STOP COMPLETE]")

	fmt.Println("checking actor data (may take a moment)")

	results := make([]bool, len(actors))

	var wg sync.WaitGroup

	for idx, actor := range actors {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, event := range events {
				if 0 >= len(event.origin) {
					continue
				}
				results[idx] = bool(len(topics) == recvdBroadcastNx(actor, event))
			}
		}()
	}

	wg.Wait()

	for idx, val := range results {

		if !val {
			fmt.Println("Actor", idx, ":", actors[idx].Id(), "did not receive all expected data")
			t.Fatal("ACTOR DID NOT MEET EXPECTATIONS")
		}
	}

	fmt.Println("[CHECK COMPLETE]")
	fmt.Println(len(events), "events over", len(topics), "topics processed")

	fmt.Println("[TEST COMPLETE]")
}

func TestDirectRoundRobin(t *testing.T) {

	slog.SetDefault(
		slog.New(
			slog.NewTextHandler(os.Stdout,
				&slog.HandlerOptions{
					Level: slog.LevelWarn,
				})))

	engine := NewEngine()

	recvGroup := "groupedActors"

	if err := engine.CreateTopic(
		NewTopic(recvGroup).
			UsingDirect().
			UsingRoundRobinSelection()); err != nil {
		t.Fatalf("err:%v", err)
	}

	actorA := testActor{
		name:  "A",
		id:    0,
		recvd: make([]eventActivity, 0),
	}
	engine.Register(Consumer{actorA.Id(), actorA.Accept})

	actorB := testActor{
		name:  "B",
		id:    1,
		recvd: make([]eventActivity, 0),
	}
	engine.Register(Consumer{actorB.Id(), actorB.Accept})

	actorC := testActor{
		name:  "C",
		id:    2,
		recvd: make([]eventActivity, 0),
	}
	engine.Register(Consumer{actorC.Id(), actorC.Accept})

	actorD := testActor{
		name:  "D",
		id:    3,
		recvd: make([]eventActivity, 0),
	}
	engine.Register(Consumer{actorD.Id(), actorD.Accept})

	aids := []string{
		"B", "C", "D",
	}

	if err := engine.SubscribeTo(recvGroup, aids...); err != nil {
		t.Fatalf("err:%v", err)
	}

	fmt.Println("starting engine")
	if err := engine.Start(); err != nil {
		t.Fatalf("err: %v", err)
	}

	fmt.Println("[ENGINE STARTED]")
	fmt.Println("starting sends")

	checkEventCount := func(actor testActor, expectedCount int) {
		if len(actor.recvd) != expectedCount {
			t.Fatalf("actor: %s expected to have %d events, but had %d",
				actor.Id(), expectedCount, len(actor.recvd))
		}
	}

	engine.Submit(actorA.Id(), recvGroup, 0) // B
	time.Sleep(100 * time.Millisecond)

	checkEventCount(actorB, 1)
	checkEventCount(actorC, 0)
	checkEventCount(actorD, 0)

	engine.Submit(actorA.Id(), recvGroup, 1) // C
	time.Sleep(100 * time.Millisecond)

	checkEventCount(actorB, 1)
	checkEventCount(actorC, 1)
	checkEventCount(actorD, 0)

	engine.Submit(actorA.Id(), recvGroup, 2) // D
	time.Sleep(100 * time.Millisecond)

	checkEventCount(actorB, 1)
	checkEventCount(actorC, 1)
	checkEventCount(actorD, 1)

	engine.Submit(actorA.Id(), recvGroup, 3) // B
	time.Sleep(100 * time.Millisecond)

	checkEventCount(actorB, 2)
	checkEventCount(actorC, 1)
	checkEventCount(actorD, 1)

	engine.Submit(actorA.Id(), recvGroup, 4) // C
	time.Sleep(100 * time.Millisecond)

	checkEventCount(actorB, 2)
	checkEventCount(actorC, 2)
	checkEventCount(actorD, 1)

	engine.Submit(actorA.Id(), recvGroup, 5) // D
	time.Sleep(100 * time.Millisecond)

	checkEventCount(actorB, 2)
	checkEventCount(actorC, 2)
	checkEventCount(actorD, 2)

	time.Sleep(500 * time.Millisecond)

	checkEventCount(actorB, 2)
	checkEventCount(actorC, 2)
	checkEventCount(actorD, 2)

	engine.Submit(actorA.Id(), recvGroup, 6)
	time.Sleep(100 * time.Millisecond)

	checkEventCount(actorB, 3)
	checkEventCount(actorC, 2)
	checkEventCount(actorD, 2)

	engine.Submit(actorA.Id(), recvGroup, 7)
	time.Sleep(100 * time.Millisecond)

	checkEventCount(actorB, 3)
	checkEventCount(actorC, 3)
	checkEventCount(actorD, 2)

	fmt.Println("[SENDS COMPLETE]")
	fmt.Println("stopping engine")

	if err := engine.Stop(); err != nil {
		t.Fatalf("err: %v", err)
	}
	fmt.Println("[STOP COMPLETE]")
}

func TestDirectRandom(t *testing.T) {

	const (
		numEvents = 1024 // With 3 actors receiving we want enough events
		// to ensure that he case that 1 actors gets 0 events
		// is very low
	)

	slog.SetDefault(
		slog.New(
			slog.NewTextHandler(os.Stdout,
				&slog.HandlerOptions{
					Level: slog.LevelWarn,
				})))

	engine := NewEngine()

	recvGroup := "groupedActors"

	if err := engine.CreateTopic(
		NewTopic(recvGroup).
			UsingDirect().
			UsingRandomSelection()); err != nil {
		t.Fatalf("err:%v", err)
	}

	actorA := testActor{
		name:  "A",
		id:    0,
		recvd: make([]eventActivity, 0),
	}
	engine.Register(Consumer{actorA.Id(), actorA.Accept})

	actorB := testActor{
		name:  "B",
		id:    1,
		recvd: make([]eventActivity, 0),
	}
	engine.Register(Consumer{actorB.Id(), actorB.Accept})

	actorC := testActor{
		name:  "C",
		id:    2,
		recvd: make([]eventActivity, 0),
	}
	engine.Register(Consumer{actorC.Id(), actorC.Accept})

	actorD := testActor{
		name:  "D",
		id:    3,
		recvd: make([]eventActivity, 0),
	}
	engine.Register(Consumer{actorD.Id(), actorD.Accept})

	actorIds := []string{
		actorB.Id(),
		actorC.Id(),
		actorD.Id(),
	}

	receivingActors := []*testActor{
		&actorB,
		&actorC,
		&actorD,
	}

	if err := engine.SubscribeTo(recvGroup, actorIds...); err != nil {
		t.Fatalf("err:%v", err)
	}

	fmt.Println("starting engine")
	if err := engine.Start(); err != nil {
		t.Fatalf("err: %v", err)
	}

	fmt.Println("[ENGINE STARTED]")
	fmt.Println("starting sends")

	checkEvents := func(expectedCount int) {

		sum := 0
		for _, actor := range receivingActors {
			sum += len(actor.recvd)

			if len(actor.recvd) == 0 {
				t.Fatalf("improbability: 0 of %d events directed towards 1 of %d actors in random dist",
					expectedCount,
					len(receivingActors))
			}
		}
		if sum != expectedCount {
			t.Fatalf("unexpected number of events for random distribution. expected:%d, got:%d",
				expectedCount,
				sum)
		}
	}

	for i := 0; i < numEvents; i++ {
		engine.Submit(actorA.Id(), recvGroup, i)
	}

	time.Sleep(1 * time.Second)

	checkEvents(numEvents)

	fmt.Println("[SENDS COMPLETE]")

	fmt.Println("stopping engine")

	if err := engine.Stop(); err != nil {
		t.Fatalf("err: %v", err)
	}
	fmt.Println("[STOP COMPLETE]")
}

func TestRouting(t *testing.T) {

	type tprod struct {
		prod Producer
		data int
	}
	numCases := 10

	tc := make([]tprod, numCases)

	engine := NewEngine()

	for x := 0; x < numCases; x++ {
		tc[x].data = 0

		topic := fmt.Sprintf("/route/%d", x)

		prod, err := engine.AddRoute(topic, func(c *Context) {
			tc[x].data = c.Event.Data.(int)
		})

		if err != nil {
			t.Fatalf("err:%v", err)
		}
		tc[x].prod = prod
	}

	fmt.Println("starting engine")
	if err := engine.Start(); err != nil {
		t.Fatalf("err: %v", err)
	}

	fmt.Println("[ENGINE STARTED]")
	fmt.Println("starting sends")

	for x := 0; x < numCases; x++ {
		if err := tc[x].prod(x); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	time.Sleep(100 * time.Millisecond)

	fmt.Println("stopping engine")

	if err := engine.Stop(); err != nil {
		t.Fatalf("err: %v", err)
	}
	fmt.Println("[STOP COMPLETE]")

	for x := 0; x < numCases; x++ {
		if tc[x].data != x {
			t.Fatal("route failed to update route-specific data")
		}
	}
}
