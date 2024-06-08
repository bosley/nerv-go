package nerv

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

const (
	protocolString = "http://"
)

type RemoteSubmitter struct {
	Address string
}

type RequestEventSubmission struct {
	// Auth ,,,
	EventData Event
}

type RequestSubscriberRegistration struct {
	HostAddress  string
	SubscriberId string
}

type RequestSubscription struct {
	Topic        string
	SubscriberId string
}

type RequestNewTopic struct {
	Config TopicCfg
}

type SubmissionResponse struct {
	Status string
	Body   string
}

func fmtEndpoint(address string, endpoint string) string {
	return fmt.Sprintf("%s%s%s", protocolString, address, endpoint)
}

func SubmitNewTopicRequest(address string, topicCfg *TopicCfg) (*SubmissionResponse, error) {
	out := RequestNewTopic{
		Config: *topicCfg,
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	return send(fmtEndpoint(address, endpointNewTopic), encoded)
}

func SubmitRegistrationRequest(remoteAddress string, localAddress string, receiverId string) (*SubmissionResponse, error) {
	out := RequestSubscriberRegistration{
		HostAddress:  localAddress,
		SubscriberId: receiverId,
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	return send(fmtEndpoint(remoteAddress, endpointRegister), encoded)
}

func SubmitSubscriptionRequest(address string, topic string, subscriberId string) (*SubmissionResponse, error) {
	out := RequestSubscription{
		Topic:        topic,
		SubscriberId: subscriberId,
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	return send(fmtEndpoint(address, endpointSubscribe), encoded)
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
