package main

import (
	"context"
	"fmt"
	"time"
)

type Checker struct {
	client                  MetadataClient
	notifier                MaintenanceNotifier
}

func NewChecker(client MetadataClient, notifier MaintenanceNotifier) *Checker {
	checker := new(Checker)
	checker.client = client
	checker.notifier = notifier
	return checker
}

// Start will create a polling mechanism in the form of an infinite loop, with each iteration ocurring in a fixed period of time.

// For more information, see [Azure Docs].
// Most events have 5 to 15 minutes of advance notice, although in some cases advance notice might be as little as 30 seconds.
// To ensure that there is enough time to take mitigating actions, it is recommended that polling is done once per second.
//
// [Azure Docs]: https://learn.microsoft.com/en-us/azure/virtual-machines/linux/scheduled-events#polling-frequency
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

		time.Sleep(1 * time.Second)
	}
}

// Check retrieves maintenence scheduled events, takes appropriate action in case there is a new batch of events and confirms the events.
// Each batch of events is versioned with a DocumentIncarnation int value. So, in order to prevent processing the same batch
// multiple times, the lastDocumentIncarnation is compared with the new value retrieved and action is taken only if the values differ.
func (c Checker) Check(ctx context.Context, lastDocumentIncarnation int) (int, error) {
	scheduledEventsBatch, err := c.client.GetScheduledEvents()
	if err != nil {
		return lastDocumentIncarnation, err
	}

	if lastDocumentIncarnation != scheduledEventsBatch.DocumentIncarnation {
		if len(scheduledEventsBatch.Events) > 0 {
			err = c.notifier.Notify(ctx)
			if err != nil {
				return scheduledEventsBatch.DocumentIncarnation, err
			}
		}

		for _, event := range scheduledEventsBatch.Events {
			_, err := c.client.ConfirmScheduledEvent(event.EventID)
			if err != nil {
				return scheduledEventsBatch.DocumentIncarnation, err
			}
		}
	}

	return scheduledEventsBatch.DocumentIncarnation, nil
}
