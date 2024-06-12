package modhttp

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"
	"bytes"
	"fmt"
  "github.com/bosley/nerv-go"
)

const (
	protocolString = "http://"
)

type RequestEventSubmission struct {
	Auth      interface{}
	Event nerv.Event 
}

type SubmissionResponse struct {
	Status string
	Body   string
}

type PingResponse struct {
	TotalPings int
	TotalFails int
}

const (
	endpointPing     = "/ping"
	endpointSubmit   = "/submit"
	endpointPingResp = "Кто там?"
)

var ErrServerAlreadyRunning = errors.New("server already running")

type Endpoint struct {
	wg               *sync.WaitGroup
	engine           *nerv.Engine
	server           *http.Server
	shutdownDuration time.Duration
	authCb           AuthCb
  submitter        *nerv.ModuleSubmitter
}

// Within RequestEventSubmission, we optionally add Auth
// with allows users to encode their preferred auth info.
// This cb sends that back to the user to perform auth,
// then a simple T/F return dictates if the request is ok
type AuthCb func(request *RequestEventSubmission) bool

type Config struct {
	Address                  string
	GracefulShutdownDuration time.Duration
	AuthCb                   AuthCb
}

func New(cfg Config, engine *nerv.Engine) *Endpoint {
	return &Endpoint{
		wg:               nil,
		engine:           engine,
		server:           &http.Server{Addr: cfg.Address},
		shutdownDuration: cfg.GracefulShutdownDuration,
		authCb:           cfg.AuthCb,
    submitter:        nil,
	}
}

func (ep *Endpoint) SetSubmitter(s *nerv.ModuleSubmitter) {
	if ep.submitter != nil {
		return
	}
	ep.submitter = s
}

func (ep *Endpoint) IndStart() error {

	slog.Info("modhttp:server:start")

	if ep.wg != nil {
		return ErrServerAlreadyRunning
	}

	http.HandleFunc(endpointSubmit, ep.handleSubmission())
	http.HandleFunc(endpointPing, ep.handlePing())

	ep.wg = new(sync.WaitGroup)
	ep.wg.Add(1)

	go func() {
		defer ep.wg.Done()
		if err := ep.server.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("error starting http - port already in use?")
			os.Exit(1)
		}
	}()

	return nil
}

func (ep *Endpoint) IndShutdown() {

	slog.Info("modhttp:server:shutdown")

	if ep.wg == nil {
		return 
	}

	shutdownCtx, shutdownRelease := context.WithTimeout(
		context.Background(),
		ep.shutdownDuration)

	defer shutdownRelease()

	if err := ep.server.Shutdown(shutdownCtx); err != nil {
		panic(err)
	}

	ep.wg.Wait()
	ep.wg = nil
	return
}

func (ep *Endpoint) handlePing() func(http.ResponseWriter, *http.Request) {
	return func(writer http.ResponseWriter, req *http.Request) {

		slog.Debug("ping")

		writer.WriteHeader(200)
		writer.Write([]byte(endpointPingResp))
	}
}

func (ep *Endpoint) handleSubmission() func(http.ResponseWriter, *http.Request) {
	return func(writer http.ResponseWriter, req *http.Request) {

    if ep.submitter == nil {
			writer.WriteHeader(503)
    }

		body, err := ioutil.ReadAll(req.Body)

		if err != nil {
			slog.Error("modhttp:server:handleSubmission", "err", err.Error())
			writer.WriteHeader(400)
			return
		}

		slog.Debug("modhttp:server:handleSubmission", "body", string(body))

		var reqWrapper RequestEventSubmission

		if err := json.Unmarshal(body, &reqWrapper); err != nil {
			writer.WriteHeader(400)
			return
		}

		event := reqWrapper.Event

		if ep.authCb != nil {
			auth := reqWrapper.Auth
			if auth == nil {
				slog.Warn("event submission rejection - missing auth", "topic", event.Topic, "producer", event.Producer)
				writer.WriteHeader(401)
			}
			if !ep.authCb(&reqWrapper) {
				slog.Warn("event submission auth failure", "topic", event.Topic, "producer", event.Producer)
				writer.WriteHeader(401)
			}
		}

		if !ep.engine.ContainsTopic(&event.Topic) {
			writer.WriteHeader(400)
			writer.Write([]byte("unknown topic"))
			return
		}


    ep.submitter.SubmitEvent(&event)

		writer.WriteHeader(200)
		return
	}
}

func fmtEndpoint(address string, endpoint string) string {
	return fmt.Sprintf("%s%s%s", protocolString, address, endpoint)
}

func SubmitPing(address string, count int, max_failures int) PingResponse {
	pr := PingResponse{
		TotalPings: 0,
		TotalFails: 0,
	}
	for x := 0; x < count; x++ {
		slog.Debug("client:SubmitPing", "address", address, "total", count, "current", x)
		resp, err := send(fmtEndpoint(address, endpointPing), []byte{})
		pr.TotalPings += 1
		if err == nil && resp != nil && resp.Status == "200 OK" {
			slog.Debug("ping success")
		} else {
			slog.Debug("ping failure")
			pr.TotalFails += 1
			if max_failures != -1 && max_failures <= pr.TotalFails {
				slog.Debug("reached fail limit", "max", max_failures)
				return pr
			}
		}
	}
	return pr
}

// Submit an event with the optional Auth interface. Auth will be encoded into JSON
// with the rest of the message. The server, detecting Auth, will execute server-side
// callback to have the information analyzed, and conditionally, permit the event submission
func SubmitEventWithAuth(address string, event *nerv.Event, auth interface{}) (*SubmissionResponse, error) {
	out := RequestEventSubmission{
		Auth:      auth,
		Event: *event,
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	return send(fmtEndpoint(address, endpointSubmit), encoded)
}

func SubmitEvent(address string, event *nerv.Event) (*SubmissionResponse, error) {
	out := RequestEventSubmission{
		Event: *event,
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	return send(fmtEndpoint(address, endpointSubmit), encoded)
}

func send(address string, data []byte) (*SubmissionResponse, error) {

	request, err := http.NewRequest("POST", address, bytes.NewBuffer(data))

	if err != nil {
		return nil, err
	}

	request.Header.Set("Content-Type", "application/json; charset=UTF-8")

	client := &http.Client{}
	response, err := client.Do(request)

	if err != nil {
		return nil, err
	}

	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		body = []byte("{}")
	}

	return &SubmissionResponse{
			Status: response.Status,
			Body:   string(body),
		},
		nil
}
