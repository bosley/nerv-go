package nerv

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
)

const (
	endpointPing     = "/ping"
	endpointSubmit   = "/submit"
	endpointPingResp = "Кто там?"
)

var ErrServerAlreadyRunning = errors.New("server already running")
var ErrServerNotRunning = errors.New("server not running")

type HttpEndpoint struct {
	wg               *sync.WaitGroup
	engine           *Engine
	server           *http.Server
	shutdownDuration time.Duration
	authCb           HttpAuthCb
}

// Within RequestEventSubmission, we optionally add Auth
// with allows users to encode their preferred auth info.
// This cb sends that back to the user to perform auth,
// then a simple T/F return dictates if the request is ok
type HttpAuthCb func(request *RequestEventSubmission) bool

type HttpEndpointCfg struct {
	Address                  string
	GracefulShutdownDuration time.Duration
	AuthCb                   HttpAuthCb
}

func HttpServer(cfg HttpEndpointCfg, engine *Engine) *HttpEndpoint {
	return &HttpEndpoint{
		wg:               nil,
		engine:           engine,
		server:           &http.Server{Addr: cfg.Address},
		shutdownDuration: cfg.GracefulShutdownDuration,
		authCb:           cfg.AuthCb,
	}
}

func (ep *HttpEndpoint) Start() error {

	slog.Info("nerv:server:start")

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

func (ep *HttpEndpoint) Stop() error {

	slog.Info("nerv:server:stop")

	if ep.wg == nil {
		return ErrServerNotRunning
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
	return nil
}

func (ep *HttpEndpoint) handlePing() func(http.ResponseWriter, *http.Request) {
	return func(writer http.ResponseWriter, req *http.Request) {

		slog.Debug("ping")

		writer.WriteHeader(200)
		writer.Write([]byte(endpointPingResp))
	}
}

func (ep *HttpEndpoint) handleSubmission() func(http.ResponseWriter, *http.Request) {
	return func(writer http.ResponseWriter, req *http.Request) {

		body, err := ioutil.ReadAll(req.Body)

		if err != nil {
			slog.Error("nerv:server:handleSubmission", "err", err.Error())
			writer.WriteHeader(400)
			return
		}

		slog.Debug("nerv:server:handleSubmission", "body", string(body))

		var reqWrapper RequestEventSubmission

		if err := json.Unmarshal(body, &reqWrapper); err != nil {
			writer.WriteHeader(400)
			return
		}

		event := reqWrapper.EventData

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

		if err := ep.engine.SubmitEvent(event); err != nil {
			slog.Warn("failed to submit to event engine", "err", err.Error())
			writer.WriteHeader(503)
			return
		}

		writer.WriteHeader(200)
		return
	}
}
