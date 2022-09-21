package main

import (
	"bytes"
	"encoding/json"
	"net/http"
)

type MetadataClient interface {
	// GetScheduledEvents retrieves a batch of new scheduled maintenence events
	GetScheduledEvents() (ScheduledEventsBatch, error)

	// ConfirmScheduledEvent approves a scheduled maintenence event using its eventId identifier.
	// This call indicates that the minimum notification time for an event can be shortened (when possible).
	// The event may not start immediately upon approval, in some cases requiring the approval of all the VMs
	// hosted on the node before proceeding with the event.
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

func (c AzureMetadataClient) GetScheduledEvents() (ScheduledEventsBatch, error) {
	scheduledEventsBatch := ScheduledEventsBatch{}

	req, err := http.NewRequest("GET", "http://169.254.169.254/metadata/scheduledevents", nil)
	if err != nil {
		return scheduledEventsBatch, err
	}

	q := req.URL.Query()
	q.Add("api-version", "2020-07-01")
	req.URL.RawQuery = q.Encode()

	req.Header.Add("Metadata", "true")

	res, err := c.client.Do(req)
	if err != nil {
		return scheduledEventsBatch, err
	}

	defer res.Body.Close()

	err = json.NewDecoder(res.Body).Decode(&scheduledEventsBatch)

	return scheduledEventsBatch, err
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
