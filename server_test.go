package nerv

import (
	"log/slog"
	"os"
	"testing"
	"time"
)

type testItem struct {
	sub    Subscriber
	recvd  *[]*Event
	topics []string
}

func TestServer(t *testing.T) {

	slog.SetDefault(
		slog.New(
			slog.NewTextHandler(os.Stdout,
				&slog.HandlerOptions{
					Level: slog.LevelDebug,
				})))

	address := "127.0.0.1:8098"

	engine := NewEngine().
		WithServer(
			NervServerCfg{
				Address:                  address,
				AllowUnknownProducers:    true,
				GracefulShutdownDuration: 2 * time.Second,
			})

	if err := engine.Start(); err != nil {
		t.Fatalf("err:%v", err)
	}

	topics0 := []string{
		"topicA",
		"topicB",
		"topicC",
	}

	topics1 := []string{
		"topicX",
		"topicY",
		"topicZ",
	}

	for _, topic := range topics0 {
		if err := engine.CreateTopic(
			NewTopic(topic).
				UsingBroadcast().
				UsingNoSelection()); err != nil {
			t.Fatalf("err:%v", err)
		}
	}

	for _, topic := range topics1 {
		response, e := SubmitNewTopicRequest(address,
			NewTopic(topic).
				UsingBroadcast().
				UsingNoSelection())
		if e != nil {
			t.Fatalf("err:%v", e)
		}
		slog.Debug("http topic create", "status", response.Status)
	}

	items := []*testItem{
		buildTestItem(t, engine, address, "red", topics0),
		buildTestItem(t, engine, address, "blue", topics1),
		buildTestItem(t, engine, address, "green", topics0),
		buildTestItem(t, engine, address, "orange", topics1),
		buildTestItem(t, engine, address, "magenta", topics0),
	}

	for _, item := range items {

		slog.Debug("item", "id", item.sub.Id)
	}

	for _, topic := range topics0 {
		resp, err := SubmitEvent(address,
			&Event{
				Spawned:  time.Now(),
				Topic:    topic,
				Producer: "test-prod",
				Data:     []byte{0xff, 0xfe, 0xfd},
			},
		)
		if err != nil {
			t.Fatalf("err:%v", err)
		}
		slog.Debug("response", "status", resp.Status, "body", resp.Body)
	}

	time.Sleep(1 * time.Second)

	if len(*items[0].recvd) != 3 {
		t.Fatal("didn't get all topic0 for 0")
	}

	if len(*items[2].recvd) != 3 {
		t.Fatal("didn't get all topic0 for 2")
	}

	if len(*items[4].recvd) != 3 {
		t.Fatal("didn't get all topic0 for 4")
	}

	if err := engine.Stop(); err != nil {
		t.Fatalf("err:%v", err)
	}
}

func buildTestItem(t *testing.T, engine *Engine, address string, id string, topics []string) *testItem {
	recvd := make([]*Event, 0)
	ti := testItem{
		sub: Subscriber{
			Id: id,
			Fn: buildSubscriber(id, &recvd),
		},
		recvd:  &recvd,
		topics: topics,
	}

	// TODO: Need to find a way to hook local callback to a fwd from the server
	//       for testing. IRL it won't be a problem...
	engine.Register(ti.sub)

	// TODO: Since we can't do this easily in test without creating an infinite
	//       event loop, we will have an integration test that checks
	//       the functionality of remote receivers down the line

	//_, err := SubmitRegistrationRequest(address, address, id)
	//if err != nil {
	//  t.Fatalf("err:%v", err)
	//}

	for _, topic := range topics {
		_, err := SubmitSubscriptionRequest(address, topic, id)
		if err != nil {
			t.Fatalf("err:%v", err)
		}
	}
	return &ti
}

func buildSubscriber(id string, recvd *[]*Event) EventRecvr {
	return func(event *Event) {
		slog.Debug("event received", "by", id, "about", event.Topic, "from", event.Producer)
		*recvd = append(*recvd, event)
	}
}
