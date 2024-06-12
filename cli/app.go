package main

import (
	"encoding/json"
	"errors"
	"github.com/bosley/nerv-go/modhttp"
	"io/ioutil"
	"log/slog"
	"os"
	"time"
)

var ErrNoFileAtPath = errors.New("no file at path")
var ErrFailedParse = errors.New("failed to parse json")

type ProcessInfo struct {
	PID     int
	Started time.Time
	Running bool
	Address string
}

func NewProcessInfo(address string) *ProcessInfo {
	return &ProcessInfo{
		PID:     os.Getpid(),
		Running: false,
		Address: address,
	}
}

func LoadProcessInfo(path string) (*ProcessInfo, error) {

	slog.Debug("app:LoadProcessInfo", "path", path)

	fileData, err := os.Open(path)
	if err != nil {
		return nil, ErrNoFileAtPath
	}

	defer fileData.Close()

	byteValue, _ := ioutil.ReadAll(fileData)

	var pi ProcessInfo
	if err := json.Unmarshal([]byte(byteValue), &pi); err != nil {
		return nil, ErrFailedParse
	}

	return &pi, nil
}

func WriteProcessInfo(path string, pi *ProcessInfo) error {

	slog.Debug("app:WriteProcessInfo", "path", path)

	raw, err := json.Marshal(*pi)

	if err != nil {
		return err
	}

	if err := os.WriteFile(path, raw, 0644); err != nil {
		return err
	}

	return nil
}

func (p *ProcessInfo) GetProcessHandle() (*os.Process, error) {

	slog.Debug("app:ProcessInfo:GetProcessHandle", "pid", p.PID)

	return os.FindProcess(p.PID)
}

func (p *ProcessInfo) IsRunning() bool {
	_, e := p.GetProcessHandle()
	if e == nil {
		return true
	}
	return false
}

func (p *ProcessInfo) IsReachable() bool {
	return modhttp.SubmitPing(p.Address, 1, -1).TotalFails == 0
}
