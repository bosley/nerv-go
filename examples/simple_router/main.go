package main

import (
  "os"
	"fmt"
  "log/slog"
  "math/rand"
  "time"
  "sync"
	"github.com/bosley/nerv-go"
)

func MustGet[T any](t T, e error) T {
  if e != nil {
    panic(e.Error)
  }
  return t
}

func must(e error) {
  if e != nil {
    panic(e.Error())
  }
}

func main() {
	slog.SetDefault(
		slog.New(
			slog.NewTextHandler(os.Stdout,
				&slog.HandlerOptions{
					Level: slog.LevelDebug,
				})))


  engine := nerv.NewEngine()

  events := []string {
    "heat-alert",
    "movement",
    "water-alert",
    "temp-alert",
    "spontanious-combustion",
    "disturbing-silence",
  }

  writers := make(map[string]nerv.Producer)
  for _, event := range events {
    writers[event] = MustGet[nerv.Producer](engine.AddRoute(fmt.Sprintf("event.%s", event), func (c *nerv.Context) {
      fmt.Println("Received event [", event, "] [", c.Event.Data.(string), "] at", c.Event.Spawned)
    }))
  }

  must(engine.Start())

  wg := new(sync.WaitGroup)

  for _, event := range events {

    wg.Add(1)
    go func() {
      defer wg.Done()
      r := rand.Intn(10)
      time.Sleep(time.Duration(r) * time.Second)
      writers[event](fmt.Sprintf("some value sent on event for (%s)", event))
    }()
  }


  wg.Wait()

  must(engine.Stop())
}

