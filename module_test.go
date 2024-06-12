package nerv

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"sync"
	"testing"
	"time"
)

type tcpModule struct {
	server    *tcpServer
	address   string
	submitter *ModuleSubmitter
}

func newTcpModule(address string) Module {
	return &tcpModule{
		server:    nil,
		address:   address,
		submitter: nil,
	}
}

func (m *tcpModule) IndStart() error {
	var err error
	m.server, err = newTcpServer(m.address, m.submitter)
	if err != nil {
		return err
	}
	return nil
}

func (m *tcpModule) SetSubmitter(s *ModuleSubmitter) {
	if m.submitter != nil {
		return
	}
	m.submitter = s
}

func (m *tcpModule) IndShutdown() {
	m.server.stop()
}

type tcpServer struct {
	listener  net.Listener
	quit      chan interface{}
	wg        sync.WaitGroup
	submitter *ModuleSubmitter
}

func newTcpServer(addr string, submitter *ModuleSubmitter) (*tcpServer, error) {

	if submitter == nil {
		return nil, errors.New("no event submitter for tcp server. did Start() run before module registration?")
	}

	s := &tcpServer{
		quit: make(chan interface{}),
	}
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	s.submitter = submitter
	s.listener = l
	s.wg.Add(1)
	go s.serve()
	return s, nil
}

func (s *tcpServer) stop() {
	close(s.quit)
	s.listener.Close()
	s.wg.Wait()
}

func (s *tcpServer) serve() {
	defer s.wg.Done()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.quit:
				return
			default:
				slog.Error("error accepting connection", "err", err.Error())
			}
		} else {
			s.wg.Add(1)
			go func() {
				s.submitter.SubmitData(conn)
				s.wg.Done()
			}()
		}
	}
}

func makeDefaultHandler(id string, action func()) EventRecvr {
	return func(event *Event) {

		slog.Debug("recv tcp event", "id", id)

		conn := event.Data.(net.Conn)

		defer conn.Close()
		buf := make([]byte, 2048)
		conn.SetDeadline(time.Now().Add(200 * time.Millisecond))
		n, err := conn.Read(buf)
		if err != nil {
			if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
				slog.Warn("read timeout")
				return
			} else if err != io.EOF {
				slog.Error("read error", "err", err.Error())
				return
			}
		}
		if n == 0 {
			return
		}
		slog.Debug("tcp default recv", "addr", conn.RemoteAddr(), "data", string(buf[:n]))
		action()
	}
}

func TestModules(t *testing.T) {

	address := "127.0.0.1:20000"

	slog.SetDefault(
		slog.New(
			slog.NewTextHandler(os.Stdout,
				&slog.HandlerOptions{
					Level: slog.LevelDebug,
				})))

	engine := NewEngine()

	mod := newTcpModule(address)
	topic := NewTopic("module.tcp").
		UsingDirect().
		UsingRoundRobinSelection()

	consumerARecv := false
	consumerBRecv := false
	consumerCRecv := false
	consumers := []Consumer{
		Consumer{
			Id: "tcp.receiver.a",
			Fn: makeDefaultHandler("A", func() {
				consumerARecv = true
			}),
		},
		Consumer{
			Id: "tcp.receiver.b",
			Fn: makeDefaultHandler("B", func() {
				consumerBRecv = true
			}),
		},
		Consumer{
			Id: "tcp.receiver.c",
			Fn: makeDefaultHandler("C", func() {
				consumerCRecv = true
			}),
		},
	}

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

	sender := func() {
		slog.Debug("sending data...")
		conn, err := net.Dial("tcp", address)
		if err != nil {
			t.Fatalf("err:%v", err)
		}
		defer conn.Close()
		fmt.Fprintf(conn, "SOME-DATA\n")
	}

	time.Sleep(1 * time.Second)

	sender()
	sender()

	fmt.Println("stopping engine")
	if err := engine.Stop(); err != nil {
		t.Fatalf("err: %v", err)
	}

	fmt.Println("[ENGINE STOPPED]")

	// Since we roundrobin'd them we ccan expect both to be reached

	if !consumerARecv {
		t.Fatal("Consumer A did not recv TCP data")
	}

	if !consumerBRecv {
		t.Fatal("Consumer B did not recv TCP data")
	}

	if consumerCRecv {
		t.Fatal("Consumer C received TCP data despite not enough sends")
	}
}
