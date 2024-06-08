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
	endpointSubmit    = "/submit"
	endpointRegister  = "/register"
	endpointNewTopic  = "/new_topic"
	endpointSubscribe = "/subscribe"
)

var ErrServerAlreadyRunning = errors.New("server already running")
var ErrServerNotRunning = errors.New("server not running")

type NervServer struct {
	wg               *sync.WaitGroup
	engine           *Engine
	server           *http.Server
	shutdownDuration time.Duration

	allowUnknownProducers bool
}

type NervServerCfg struct {
	Address                  string
	AllowUnknownProducers    bool
	GracefulShutdownDuration time.Duration
}

func HttpServer(cfg NervServerCfg, engine *Engine) *NervServer {
	return &NervServer{
		wg:                    nil,
		engine:                engine,
		server:                &http.Server{Addr: cfg.Address},
		allowUnknownProducers: cfg.AllowUnknownProducers,
		shutdownDuration:      cfg.GracefulShutdownDuration,
	}
}

func (nrvs *NervServer) Start() error {

	slog.Info("nerv:server:start")

	if nrvs.wg != nil {
		return ErrServerAlreadyRunning
	}

	// TODO: We should "wrap" the endpoints conditionally with
	//       an auth message later to ensure that registrations/subs/topics
	//       can be "world facing" and not a risk.
	/*

		      if nrvs.requireAuth {

			      http.HandleFunc(
		          endpointSubscribe, nrvs.handleAuthBefore(nrvs.handleSubscribe))

		          \---- Then when req comes in handleAuthBefore will auth,
		                then, conditionally go to handle subscribe
		      }

	*/

	http.HandleFunc("/", nrvs.handleMain())
	http.HandleFunc(endpointSubmit, nrvs.handleSubmission())
	http.HandleFunc(endpointRegister, nrvs.handleRegistration())
	http.HandleFunc(endpointNewTopic, nrvs.handleNewTopic())
	http.HandleFunc(endpointSubscribe, nrvs.handleSubscribe())

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

func (nrvs *NervServer) handleMain() func(http.ResponseWriter, *http.Request) {
	return func(writer http.ResponseWriter, req *http.Request) {
		writer.WriteHeader(418)
		writer.Write([]byte("i'm a teapot"))
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

		if !nrvs.allowUnknownProducers {
			if !nrvs.engine.ContainsSubscriber(&event.Producer) {
				writer.WriteHeader(400)
				writer.Write([]byte("unknown producer"))
				return
			}
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

func (nrvs *NervServer) handleRegistration() func(http.ResponseWriter, *http.Request) {
	return func(writer http.ResponseWriter, req *http.Request) {

		body, err := ioutil.ReadAll(req.Body)

		if err != nil {
			slog.Error("nerv:server:handleRegistration", "err", err.Error())
			writer.WriteHeader(400)
			return
		}

		slog.Debug("nerv:server:handleRegistration", "body", string(body))

		var reqWrapper RequestSubscriberRegistration

		if err := json.Unmarshal(body, &reqWrapper); err != nil {
			writer.WriteHeader(400)
			return
		}

		nrvs.engine.Register(
			setupSubscriptionFwd(
				reqWrapper.HostAddress,
				reqWrapper.SubscriberId))

		writer.WriteHeader(200)
	}
}

func (nrvs *NervServer) handleNewTopic() func(http.ResponseWriter, *http.Request) {
	return func(writer http.ResponseWriter, req *http.Request) {

		body, err := ioutil.ReadAll(req.Body)

		if err != nil {
			slog.Error("nerv:server:handleNewTopic", "err", err.Error())
			writer.WriteHeader(400)
			return
		}

		slog.Debug("nerv:server:handleNewTopic", "body", string(body))

		var reqWrapper RequestNewTopic

		if err := json.Unmarshal(body, &reqWrapper); err != nil {
			writer.WriteHeader(400)
			return
		}

		err = nrvs.engine.CreateTopic(&reqWrapper.Config)
		if err != nil {
			writer.WriteHeader(409)
			if errors.Is(err, ErrEngineDuplicateTopic) {
				writer.Write([]byte("duplicate topic"))
			}
			return
		}
	}
}

func (nrvs *NervServer) handleSubscribe() func(http.ResponseWriter, *http.Request) {
	return func(writer http.ResponseWriter, req *http.Request) {
		writer.WriteHeader(418)
		writer.Write([]byte("i'm a teapot"))
	}
}

// Create a subscriber with an anonymous function that utilizes
// the http client's SubmitEvent to forward any events that
// the subscriber receives on the local event bus
func setupSubscriptionFwd(address string, subscriber string) Subscriber {
	return Subscriber{
		Id: subscriber,
		Fn: func(event *Event) {

			// Don't send them back messages that they sent us
			if event.Producer == subscriber {
				return
			}
			SubmitEvent(address, event)
		},
	}
}
