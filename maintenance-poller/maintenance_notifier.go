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

type MaintenanceNotifier interface {
	Notify(ctx context.Context) error
}

type KubernetesMaintenanceNotifier struct {
	clientset *kubernetes.Clientset
	nodeName  string
}

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

func (n KubernetesMaintenanceNotifier) Notify(ctx context.Context) error {
	err := n.UpdateNodeIsUnschedulable(ctx, true)
	if err != nil {
		return err
	}

	var gameServers mpsv1alpha1.GameServerList
	err = n.clientset.RESTClient().Get().AbsPath("/apis/mps.playfab.com/v1alpha1").Resource("gameservers").Namespace("default").Do(ctx).Into(&gameServers)
	if err != nil {
		return err
	}

	// standBy := []mpsv1alpha1.GameServer{}
	for _, gs := range gameServers.Items {
		if gs.Status.NodeName == n.nodeName && gs.Status.State == mpsv1alpha1.GameServerStateStandingBy {
			// standBy = append(standBy, gs)

			// kubectl delete gameserver x -n default
			err = n.clientset.RESTClient().Delete().AbsPath("/apis/mps.playfab.com/v1alpha1").Resource("gameservers").Namespace("default").Name(gs.Name).Do(ctx).Error()
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// m.updateNodeIsUnschedulable(ctx, true) = kubectl cordon
// m.updateNodeIsUnschedulable(ctx, false) = kubectl uncordon
func (n KubernetesMaintenanceNotifier) UpdateNodeIsUnschedulable(ctx context.Context, value bool) error {
	payload := []patchStringValue{{
		Op:    "replace",
		Path:  "/spec/unschedulable",
		Value: value,
	}}
	payloadBytes, _ := json.Marshal(payload)
	_, err := n.clientset.CoreV1().Nodes().Patch(ctx, n.nodeName, types.JSONPatchType, payloadBytes, metav1.PatchOptions{})
	return err
}
