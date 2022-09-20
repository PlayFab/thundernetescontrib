package main

import (
	"context"
	"fmt"
	"time"
)

type Checker struct {
	client   MetadataClient
	notifier MaintenanceNotifier
}

func NewChecker(client MetadataClient, notifier MaintenanceNotifier) *Checker {
	checker := new(Checker)
	checker.client = client
	checker.notifier = notifier
	return checker
}

func (c Checker) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			err := c.Check(ctx)

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

func (c Checker) Check(ctx context.Context) error {
	scheduledEvent, err := c.client.GetScheduledEvents()
	if err != nil {
		return err
	}
	fmt.Println(scheduledEvent.DocumentIncarnation)

	// TODO: notify only if doc incarnation has changed since last check
	if len(scheduledEvent.Events) > 0 {
		err = c.notifier.Notify(ctx)
		if err != nil {
			return err
		}
	}

	for _, event := range scheduledEvent.Events {
		statusCode, err := c.client.ConfirmScheduledEvent(event.EventID)
		fmt.Println(statusCode)
		if err != nil {
			return err
		}
	}

	return nil
}
