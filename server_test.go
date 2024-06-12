package nerv

import (
	"log/slog"
	"os"
	"testing"
	"time"
)

type testItem struct {
	sub    Consumer
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
		if err := engine.CreateTopic(
			NewTopic(topic).
				UsingBroadcast().
				UsingNoSelection()); err != nil {
			t.Fatalf("err:%v", err)
		}
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
    time.Sleep(10 * time.Millisecond)
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
		sub: Consumer{
			Id: id,
			Fn: buildConsumer(id, &recvd),
		},
		recvd:  &recvd,
		topics: topics,
	}

	engine.Register(ti.sub)

	for _, top := range topics {
		if err := engine.SubscribeTo(top, id); err != nil {
			t.Fatalf("err:%v", err)
		}
	}
	return &ti
}

func buildConsumer(id string, recvd *[]*Event) EventRecvr {
	return func(event *Event) {
		slog.Debug("event received", "by", id, "about", event.Topic, "from", event.Producer)
		*recvd = append(*recvd, event)
	}
}
