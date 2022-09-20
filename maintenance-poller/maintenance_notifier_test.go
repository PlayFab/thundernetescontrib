package main

import (
	"context"
	"testing"
)

func TestNotifier(t *testing.T) {
	t.Run("notifier", func(t *testing.T) {
		node := "aks-agentpool-33482676-vmss000000"

		notifier := NewOutOfClusterKubernetesMaintenanceNotifier(node)

		err := notifier.Notify(context.TODO())

		if err != nil {
			t.Fatalf("not expected")
		}

		// clean
		notifier.UpdateNodeIsUnschedulable(context.TODO(), false)
	})
}
