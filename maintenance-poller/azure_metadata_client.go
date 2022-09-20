package main

import (
	"bytes"
	"encoding/json"
	"net/http"
)

type MetadataClient interface {
	GetScheduledEvents() (ScheduledEvent, error)
	ConfirmScheduledEvent(eventId string) (statusCode int, err error)
}

type AzureMetadataClient struct {
	client http.Client
}

func NewAzureMetadataClient() *AzureMetadataClient {
	c := AzureMetadataClient{}
	c.client = http.Client{}
	return &c
}

func (c AzureMetadataClient) GetScheduledEvents() (ScheduledEvent, error) {
	scheduledEvent := ScheduledEvent{}

	req, err := http.NewRequest("GET", "http://169.254.169.254/metadata/scheduledevents", nil)
	if err != nil {
		return scheduledEvent, err
	}

	q := req.URL.Query()
	q.Add("api-version", "2020-07-01")
	req.URL.RawQuery = q.Encode()

	req.Header.Add("Metadata", "true")

	res, err := c.client.Do(req)
	if err != nil {
		return scheduledEvent, err
	}

	defer res.Body.Close()

	err = json.NewDecoder(res.Body).Decode(&scheduledEvent)

	return scheduledEvent, err
}

func (c AzureMetadataClient) ConfirmScheduledEvent(eventId string) (statusCode int, err error) {
	events := ConfirmScheduledEvent{
		StartRequests: []StartRequest{{EventID: eventId}},
	}

	postBody, _ := json.Marshal(events)
	buffer := bytes.NewBuffer(postBody)

	req, err := http.NewRequest("POST", "http://169.254.169.254/metadata/scheduledevents", buffer)
	if err != nil {
		return statusCode, err
	}

	q := req.URL.Query()
	q.Add("api-version", "2020-07-01")
	req.URL.RawQuery = q.Encode()

	req.Header.Add("Metadata", "true")
	req.Header.Add("Content-Type", "application/json")

	res, err := c.client.Do(req)

	return res.StatusCode, err
}
