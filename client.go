package nerv

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
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
	HostAddress  string
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

func SubmitNewTopicRequest(address string, topic string) (*SubmissionResponse, error) {
	out := RequestNewTopic{
		Topic: topic,
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	return send(address, encoded)
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
	return send(remoteAddress, encoded)
}

func SubmitSubscriptionRequest(remoteAddress string, localAddress string, topic string, subscriberId string) (*SubmissionResponse, error) {
	out := RequestSubscription{
		HostAddress:  localAddress,
		Topic:        topic,
		SubscriberId: subscriberId,
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	return send(remoteAddress, encoded)
}

func SubmitEvent(address string, event *Event) (*SubmissionResponse, error) {
	out := RequestEventSubmission{
		EventData: *event,
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	return send(address, encoded)
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
