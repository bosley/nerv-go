package nerv

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log/slog"
	"net/http"
)

const (
	protocolString = "http://"
)

type RequestEventSubmission struct {
	// Auth ,,,
	EventData Event
}

type SubmissionResponse struct {
	Status string
	Body   string
}

type PingResponse struct {
	TotalPings int
	TotalFails int
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

func SubmitEvent(address string, event *Event) (*SubmissionResponse, error) {
	out := RequestEventSubmission{
		EventData: *event,
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
