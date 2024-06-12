package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/bosley/nerv-go"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

const (
	exitCodeErr       = -1
	exitCodeForceKill = 24
)

const (
	defaultProcFileName            = ".nervd"
	defaultAddress                 = "127.0.0.1:4096"
	defaultPermitUnknownProducers  = true
	defaultGracefulShutdownTimeSec = 5
	defaultReaperDelayySec         = 5
)

const (
	appChannel  = "nerv.app.internal"
	appReaperId = "nerv.app.reaper"
)

const (
	appMsgShutdown = iota
)

type InternalMessage struct {
	Id   int
	Data interface{}
}

var eventEngine *nerv.Engine

func main() {
	slog.SetDefault(
		slog.New(
			slog.NewTextHandler(os.Stdout,
				&slog.HandlerOptions{
					Level: slog.LevelDebug,
				})))

	addrPtr := flag.String("address", defaultAddress, "Address to bind for nerv server [address:port]")
	sdtPtr := flag.Int("grace", defaultGracefulShutdownTimeSec, "Seconds to wait for shutdown of server; default: 5")
	pupPtr := flag.Bool("filter", defaultPermitUnknownProducers, "Filter unregistered producers from submitting events")
	targetPtr := flag.String("rti", defaultProcFileName, "File to store runtime information of running server")

	startPtr := flag.Bool("up", false, "Start server")
	stopPtr := flag.Bool("down", false, "Stop server (graceful)")
	cleanPtr := flag.Bool("clean", false, "Kills a server iff its running, and then wipes the rti file")
	forcePtr := flag.Bool("force", false, "Force kills nerv instance when used with -down")

	flag.Parse()

	serverCfg := nerv.NervServerCfg{
		Address:                  *addrPtr,
		AllowUnknownProducers:    *pupPtr,
		GracefulShutdownDuration: time.Duration(*sdtPtr) * time.Second,
	}

	if *stopPtr {
		if !checkIfRunning(*targetPtr) {
			fmt.Println("no server seems to be running. perhaps you forgot to specift the nerv file?")
		} else {
			doKillProc(*targetPtr, *forcePtr)
		}
	}

	if *cleanPtr {
		doClean(*targetPtr)
	}

	if *startPtr {
		doHost(serverCfg, targetPtr)
		os.Exit(0)
	}
}

func doClean(file string) {

	slog.Debug("doClean", "file", file)

	pi, err := LoadProcessInfo(file)
	if err != nil {
		if errors.Is(err, ErrNoFileAtPath) {
			slog.Debug("no file to clean :)")
			return
		}
		slog.Error(
			"Asked to clean file that wasn't able to be validated as a nerv process file")
		os.Exit(exitCodeErr)
	}

	slog.Debug("need to clean-up file. checking if listed process is reachable", "pid", pi.PID)

	// If a crash occurs, or a force kill, the file won't
	// be cleaned so the class `Running` isn't "good enough"
	if pi.IsReachable() {
		slog.Debug("process reachable, issuing kill command")
		doKillProc(file, true)
	} else {
		slog.Debug("process not reachable, cruft detected")
	}
	os.Remove(file)
	slog.Debug("cleaned")
}

func doKillProc(file string, force bool) {

	pi, err := LoadProcessInfo(file)
	if err != nil {
		slog.Error("failed to get load process info", "from", file, "err", err.Error())
		os.Exit(exitCodeErr)
	}

	proc, err := pi.GetProcessHandle()
	if err != nil {
		slog.Error("failed to get process handle", "pid", pi.PID, "err", err.Error())
		os.Exit(exitCodeErr)
	}

	sig := syscall.SIGINT
	if force {
		sig = syscall.SIGTERM
	}

	if err := proc.Signal(syscall.Signal(sig)); err != nil {
		slog.Error("failed to signal process", "pid", pi.PID, "err", err.Error())
		os.Exit(exitCodeErr)
	}
	fmt.Println("success")
}

func doHost(cfg nerv.NervServerCfg, fileName *string) {

	if checkIfRunning(*fileName) {
		slog.Error("server already running with configuration specified", "cfg file", *fileName)
		os.Exit(exitCodeErr)
	}

	procInfo := NewProcessInfo(cfg.Address)

	wg := new(sync.WaitGroup)

	LaunchServer(cfg, procInfo, wg)

	if err := WriteProcessInfo(*fileName, procInfo); err != nil {
		slog.Error("failed to write process information", "err", err.Error())
		os.Exit(exitCodeErr)
	}

	defer os.Remove(*fileName)

	// Wait until the reaper function intercepts the signal
	// and events-out to the system that we are shutting down
	wg.Wait()

}

func LaunchServer(cfg nerv.NervServerCfg, procInfo *ProcessInfo, wg *sync.WaitGroup) {

	slog.Debug("LaunchServer", "address", cfg.Address, "pid", procInfo.PID)

	eventEngine = nerv.NewEngine().WithServer(cfg)

	StartEngine()

	procInfo.Started = time.Now()

	time.Sleep(1 * time.Second)

	if !procInfo.IsReachable() {
		slog.Warn("unable to reach recently started server")
		os.Exit(exitCodeErr)
	}

	procInfo.Running = true

	slog.Debug("confirmed server started")

	if err := eventEngine.CreateTopic(
		nerv.NewTopic(appChannel).
			UsingBroadcast().
			UsingNoSelection()); err != nil {

		slog.Error("unable to create internal topic")
		os.Exit(exitCodeErr)
	}

	eventEngine.Register(
		nerv.Subscriber{
			Id: appReaperId,
			Fn: createReaperFunction(procInfo, wg),
		})

	if err := eventEngine.SubscribeTo(
		appChannel,
		appReaperId); err != nil {
		slog.Error("failed to register reaper", "err", err.Error())
		os.Exit(exitCodeErr)
	}
}

func StartEngine() {
	slog.Debug("StartEngine")
	if err := eventEngine.Start(); err != nil {
		fmt.Println(err)
		os.Exit(exitCodeErr)
	}
	slog.Debug("Started")
}

func StopEngine() {
	slog.Debug("StopEngine")
	if err := eventEngine.Stop(); err != nil {
		fmt.Println(err)
		os.Exit(exitCodeErr)
	}
	slog.Debug("Stopped")
}

func checkIfRunning(file string) bool {

	slog.Debug("check if process running based on file", "file", file)

	pi, err := LoadProcessInfo(file)
	if err != nil {
		if errors.Is(err, ErrNoFileAtPath) {
			return false
		}
		slog.Error("unable to determine if application is running. Unable to load file")
		os.Exit(exitCodeErr)
	}

	return pi.Running
}

func createReaperFunction(procInfo *ProcessInfo, wg *sync.WaitGroup) nerv.EventRecvr {

	signalChannel := make(chan os.Signal, 2)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)

	// Create a go routine that waits for a signal.
	// When that signal trips we execute the apropriate
	// killing mechanism
	wg.Add(1)
	go func() {
		defer wg.Done()

		sig := <-signalChannel
		switch sig {
		case os.Interrupt:
			shutdownServer()
		case syscall.SIGTERM:
			killServer()
		}
	}()

	// Return a function that will be invoked when an event hits the internal
	// channel. Reaper will give count-down to death by filtering all but its
	// own kill signal
	return func(event *nerv.Event) {
		if event.Producer == appReaperId {
			slog.Warn(
				"reaper received its own shutdown message",
				"seconds_remaining", event.Data.(*InternalMessage).Data.(int))
		}
	}
}

func killServer() {
	os.Exit(exitCodeForceKill)
}

func shutdownServer() {
	t := defaultReaperDelayySec
	for t != 0 {
		eventEngine.Submit(
			appReaperId,
			appChannel,
			&InternalMessage{
				Id:   appMsgShutdown,
				Data: t,
			},
		)
		t -= 1
		time.Sleep(1 * time.Second)
	}
}
