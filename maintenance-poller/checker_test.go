package main

import (
	"context"
	"testing"
)

type MockMetadataClient struct {
	mockedScheduledEvent ScheduledEventsBatch
}

func (c MockMetadataClient) GetScheduledEvents() (ScheduledEventsBatch, error) {
	return c.mockedScheduledEvent, nil
}

func (c MockMetadataClient) ConfirmScheduledEvent(eventId string) (statusCode int, err error) {
	return 200, nil
}

type MockMaintenanceNotifier struct{}

func (n MockMaintenanceNotifier) Notify(ctx context.Context) error {
	return nil
}

func Test(t *testing.T) {
	t.Run("abc", func(t *testing.T) {
		client := MockMetadataClient{}
		notifier := MockMaintenanceNotifier{}
		checker := NewChecker(client, notifier)

		_, err := checker.Check(context.TODO(), -1)

		if err != nil {
			t.Fatalf("not expected")
		}
	})
}
