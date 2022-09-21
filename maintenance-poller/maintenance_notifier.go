package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

const apiGroup = "/apis/mps.playfab.com/v1alpha1"

type MaintenanceNotifier interface {
	Notify(ctx context.Context) error
}

type KubernetesMaintenanceNotifier struct {
	clientset *kubernetes.Clientset
	nodeName  string
}

// NewInClusterKubernetesMaintenanceNotifier will create an in-cluster Kubernetes client.
// It will set the node name based on the NODE_NAME environment variable, therefore that must be set.
//
// For more information see [In-cluster example]
//
// [In-cluster example]: https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
func NewInClusterKubernetesMaintenanceNotifier() KubernetesMaintenanceNotifier {
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	nodeName, ok := os.LookupEnv("NODE_NAME")
	if !ok {
		panic(fmt.Errorf("NODE_NAME is not present"))
	}

	k := KubernetesMaintenanceNotifier{}
	k.clientset = clientset
	k.nodeName = nodeName
	return k
}

// NewOutOfClusterKubernetesMaintenanceNotifier will create an out-of-cluster Kubernetes client.
// It will set the node name based on the nodeName argument.
//
// For more information see [Out-of-cluster example]
//
// [Out-of-cluster example]: https://github.com/kubernetes/client-go/tree/master/examples/out-of-cluster-client-configuration
func NewOutOfClusterKubernetesMaintenanceNotifier(nodeName string) KubernetesMaintenanceNotifier {
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig-cluster", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig-cluster", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	k := KubernetesMaintenanceNotifier{}
	k.clientset = clientset
	k.nodeName = nodeName
	return k
}

// used for testing
func (n KubernetesMaintenanceNotifier) GetClientSet() *kubernetes.Clientset {
	return n.clientset
}

// Notify marks the node as unschedulable (equivalent to kubectl uncordon), so no more GameServers are scheduled.
// It also deletes any non-Active GameServers in that node so they are not allocated.
// An equal number of GameServers will be created in nodes which are not in maintenance.
func (n KubernetesMaintenanceNotifier) Notify(ctx context.Context) error {
	err := n.ToggleNodeUnschedulable(ctx, true)
	if err != nil {
		return err
	}

	var gameServers mpsv1alpha1.GameServerList
	err = n.clientset.RESTClient().Get().AbsPath(apiGroup).Resource("gameservers").Namespace("default").Do(ctx).Into(&gameServers)
	if err != nil {
		return err
	}

	for _, gs := range gameServers.Items {
		if gs.Status.NodeName == n.nodeName && gs.Status.State == mpsv1alpha1.GameServerStateStandingBy {
			err = n.clientset.RESTClient().Delete().AbsPath(apiGroup).Resource("gameservers").Namespace("default").Name(gs.Name).Do(ctx).Error()
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// ToggleNodeUnschedulable will mark a node as schedulable or unschedulable depending on the markUnschedulable flag.
//
//	ToggleNodeUnschedulable(ctx, true) = kubectl cordon
//	ToggleNodeUnschedulable(ctx, false) = kubectl uncordon
func (n KubernetesMaintenanceNotifier) ToggleNodeUnschedulable(ctx context.Context, markUnschedulable bool) error {
	payload := []patchStringValue{{
		Op:    "replace",
		Path:  "/spec/unschedulable",
		Value: markUnschedulable,
	}}
	payloadBytes, _ := json.Marshal(payload)
	_, err := n.clientset.CoreV1().Nodes().Patch(ctx, n.nodeName, types.JSONPatchType, payloadBytes, metav1.PatchOptions{})
	return err
}
