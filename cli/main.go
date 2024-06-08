package main

import (
  "os"
  "fmt"
  "time"
  "log/slog"
  "github.com/bosley/nerv-go"
)

func main() {
  fmt.Println("cli")
	slog.SetDefault(
		slog.New(
			slog.NewTextHandler(os.Stdout,
				&slog.HandlerOptions{
					Level: slog.LevelWarn,
				})))

  engine := nerv.NewEngine().WithServer(nerv.NervServerCfg{
    Address: "127.0.0.1:8097",
    AllowUnknownProducers: true,
    GracefulShutdownDuration: 5 * time.Second,
  })

  if err := engine.Start(); err != nil {
    fmt.Println(err)
    os.Exit(1)
	}

  fmt.Println("MAKE THE CLI HERE!")

  time.Sleep(5 * time.Second)

  if err := engine.Stop(); err != nil {
    fmt.Println(err)
    os.Exit(1)
	}
}
