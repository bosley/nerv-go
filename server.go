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

type NervServer struct {
	wg               *sync.WaitGroup
	engine           *Engine
	server           *http.Server
	shutdownDuration time.Duration
}

type NervServerCfg struct {
	Address                  string
	GracefulShutdownDuration time.Duration
}

func HttpServer(cfg NervServerCfg, engine *Engine) *NervServer {
	return &NervServer{
		wg:               nil,
		engine:           engine,
		server:           &http.Server{Addr: cfg.Address},
		shutdownDuration: cfg.GracefulShutdownDuration,
	}
}

func (nrvs *NervServer) Start() error {

	slog.Info("nerv:server:start")

	if nrvs.wg != nil {
		return ErrServerAlreadyRunning
	}

	http.HandleFunc(endpointSubmit, nrvs.handleSubmission())
	http.HandleFunc(endpointPing, nrvs.handlePing())

	nrvs.wg = new(sync.WaitGroup)
	nrvs.wg.Add(1)

	go func() {
		defer nrvs.wg.Done()
		if err := nrvs.server.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("error starting http - port already in use?")
			os.Exit(1)
		}
	}()

	return nil
}

func (nrvs *NervServer) Stop() error {

	slog.Info("nerv:server:stop")

	if nrvs.wg == nil {
		return ErrServerNotRunning
	}

	shutdownCtx, shutdownRelease := context.WithTimeout(
		context.Background(),
		nrvs.shutdownDuration)

	defer shutdownRelease()

	if err := nrvs.server.Shutdown(shutdownCtx); err != nil {
		panic(err)
	}

	nrvs.wg.Wait()
	nrvs.wg = nil
	return nil
}

func (nrvs *NervServer) handlePing() func(http.ResponseWriter, *http.Request) {
	return func(writer http.ResponseWriter, req *http.Request) {

		slog.Debug("ping")

		writer.WriteHeader(200)
		writer.Write([]byte(endpointPingResp))
	}
}

func (nrvs *NervServer) handleSubmission() func(http.ResponseWriter, *http.Request) {
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

		if !nrvs.engine.ContainsTopic(&event.Topic) {
			writer.WriteHeader(400)
			writer.Write([]byte("unknown topic"))
			return
		}

		if err := nrvs.engine.SubmitEvent(event); err != nil {
			slog.Warn("failed to submit to event engine", "err", err.Error())
			writer.WriteHeader(503)
			return
		}

		writer.WriteHeader(200)
		return
	}
}
