package modhttp

import (
	"fmt"
	"github.com/bosley/nerv-go"
	"log/slog"
	"os"
	"testing"
	"time"
)

type testItem struct {
	sub    nerv.Consumer
	recvd  *[]*nerv.Event
	topics []string
}

const (
	testApiToken = "V2h5IGRpZCB5b3UgZGVjb2RlIHRoaXM/IFRoYXRzIHNpbGx5LCB0aGlzIGlzIGp1c3QgYSB0ZXN0IGl0ZW0uLi4uIGRvIHlvdSBnbyBhcm91bmQgZGVjb2RpbmcgcmFuZG9tIHRoaW5ncyBpbiBzdHJhbmdlcnMgY29kZSBvZnRlbj8gd2VpcmRvLi4u"
)

func TestServer(t *testing.T) {

	slog.SetDefault(
		slog.New(
			slog.NewTextHandler(os.Stdout,
				&slog.HandlerOptions{
					Level: slog.LevelDebug,
				})))

	address := "127.0.0.1:8098"
	topicName := "module.http"

	engine := nerv.NewEngine()

	mod := New(
		Config{
			Address:                  address,
			GracefulShutdownDuration: 2 * time.Second,
			AuthCb: func(req *RequestEventSubmission) bool {
				slog.Debug("http auth callback", "topic", req.Event.Topic, "prod", req.Event.Producer)
				return req.Auth.(string) == testApiToken
			}}, engine)

	topic := nerv.NewTopic(topicName).
		UsingBroadcast().
		UsingArbitrary()

	consumerARecv := false

	consumers := []nerv.Consumer{
		nerv.Consumer{
			Id: "http.receiver.a",
			Fn: func(event *nerv.Event) {
				fmt.Println("receiver got http event")
				consumerARecv = true
			},
		}}

	if err := engine.UseModule(
		mod,
		topic,
		consumers); err != nil {
		t.Fatalf("err:%v", err)
	}

	fmt.Println("starting engine")
	if err := engine.Start(); err != nil {
		t.Fatalf("err: %v", err)
	}

	fmt.Println("[ENGINE STARTED]")

	time.Sleep(1 * time.Second)

	sender := func() {
		SubmitEventWithAuth(
			address,
			&nerv.Event{
				Spawned:  time.Now(),
				Topic:    topicName,
				Producer: "http.client",
				Data:     "some simple test data",
			},
			testApiToken,
		)
	}

	sender()

	fmt.Println("stopping engine")
	if err := engine.Stop(); err != nil {
		t.Fatalf("err: %v", err)
	}

	fmt.Println("[ENGINE STOPPED]")

	if !consumerARecv {
		t.Fatal("Consumer A did not recv HTTP data")
	}
}
