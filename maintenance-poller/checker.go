package main

import (
	"context"
	"fmt"
	"time"
)

type Checker struct {
	client   MetadataClient
	notifier MaintenanceNotifier
	lastDocumentIncarnation int
}

func NewChecker(client MetadataClient, notifier MaintenanceNotifier) *Checker {
	checker := new(Checker)
	checker.client = client
	checker.notifier = notifier
	checker.lastDocumentIncarnation = -1
	return checker
}

func (c Checker) Start(ctx context.Context) {
	lastDocumentIncarnation := -1
	var err error
	for {
		select {
		case <-ctx.Done():
			return
		default:
			lastDocumentIncarnation, err = c.Check(ctx, lastDocumentIncarnation)

			if err != nil {
				fmt.Println(err.Error())
			}
		}

		// https://learn.microsoft.com/en-us/azure/virtual-machines/linux/scheduled-events#polling-frequency
		// Most events have 5 to 15 minutes of advance notice, although in some cases advance notice might be as little as 30 seconds.
		// To ensure that you have as much time as possible to take mitigating actions, we recommend that you poll the service once per second.
		time.Sleep(1 * time.Second)
	}
}

func (c Checker) Check(ctx context.Context, lastDocumentIncarnation int) (int, error) {
	scheduledEvent, err := c.client.GetScheduledEvents()
	if err != nil {
		return lastDocumentIncarnation, err
	}

	if (lastDocumentIncarnation != scheduledEvent.DocumentIncarnation) {
		if len(scheduledEvent.Events) > 0 {
			err = c.notifier.Notify(ctx)
			if err != nil {
				return scheduledEvent.DocumentIncarnation, err
			}
		}
	
		for _, event := range scheduledEvent.Events {
			_, err := c.client.ConfirmScheduledEvent(event.EventID)
			if err != nil {
				return scheduledEvent.DocumentIncarnation, err
			}
		}
	}

	return scheduledEvent.DocumentIncarnation, nil
}
