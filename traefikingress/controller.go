package main

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	traefikv1alpha1 "github.com/traefik/traefik/v2/pkg/provider/kubernetes/crd/traefik/v1alpha1"
)

var (
	ownerKey = ".metadata.controller"
	apiGVStr = mpsv1alpha1.GroupVersion.String()
)

// GameServerReconciler reconciles a GameServer object
type GameServerReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

func (r *GameServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var gs mpsv1alpha1.GameServer
	if err := r.Get(ctx, req.NamespacedName, &gs); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Unable to fetch GameServer - skipping")
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch GameServer")
		return ctrl.Result{}, err
	}

	// check if the corresponding service exists
	var svc corev1.Service
	if err := r.Get(ctx, client.ObjectKey{Namespace: gs.Namespace, Name: gs.Name}, &svc); err != nil {
		if apierrors.IsNotFound(err) {
			err = r.createService(ctx, &gs)
			if apierrors.IsConflict(err) {
				log.Info("Service already exists - skipping")
				return ctrl.Result{}, nil
			}
			if err != nil {
				log.Error(err, "unable to create service")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, err
	}

	// check if the K8s Endpoint exists
	// if not, IngressRoute will be created but traefik will log "endpoints not found"
	var se corev1.Endpoints
	if err := r.Get(ctx, client.ObjectKey{Namespace: gs.Namespace, Name: gs.Name}, &se); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{Requeue: true, RequeueAfter: time.Second}, nil
		}
		return ctrl.Result{}, err
	}

	// if Subsets is zero, traefik will log "subset not found"
	if len(se.Subsets) == 0 {
		return ctrl.Result{Requeue: true, RequeueAfter: time.Second}, nil
	}

	// check if the corresponding Traefik ingress route exists
	var ig traefikv1alpha1.IngressRoute
	if err := r.Get(ctx, client.ObjectKey{Namespace: gs.Namespace, Name: gs.Name}, &ig); err != nil {
		if apierrors.IsNotFound(err) {
			err = r.createIngressRoute(ctx, &gs, &svc)
			if apierrors.IsConflict(err) {
				log.Info("IngressRoute already exists - skipping")
				return ctrl.Result{}, nil
			}
			if err != nil {
				log.Error(err, "unable to create ingress route")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GameServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &corev1.Pod{}, ownerKey, func(rawObj client.Object) []string {
		// grab the Pod object, extract the owner...
		pod := rawObj.(*corev1.Pod)
		owner := metav1.GetControllerOf(pod)
		if owner == nil {
			return nil
		}
		// ...make sure it's a GameServer...
		if owner.APIVersion != apiGVStr || owner.Kind != "GameServer" {
			return nil
		}

		// ...and if so, return it
		return []string{owner.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&mpsv1alpha1.GameServer{}).
		Complete(r)
}

func getPortDetails(ctx context.Context, gs *mpsv1alpha1.GameServer, pte mpsv1alpha1.PortToExpose) *corev1.ServicePort {
	log := log.FromContext(ctx)
	for _, container := range gs.Spec.Template.Spec.Containers {
		if pte.ContainerName == container.Name {
			for _, port := range container.Ports {
				if port.Protocol == corev1.ProtocolUDP {
					log.Info("UDP ports are not supported - skipping")
					continue
				}
				if port.Name == pte.PortName {
					return &corev1.ServicePort{
						Name:     pte.PortName,
						Port:     port.ContainerPort,
						Protocol: port.Protocol,
					}
				}
			}
		}
	}
	return nil
}

func (r *GameServerReconciler) createService(ctx context.Context, gs *mpsv1alpha1.GameServer) error {
	portsForService := []corev1.ServicePort{}
	for _, pte := range gs.Spec.PortsToExpose {
		pd := getPortDetails(ctx, gs, pte)
		if pd != nil {
			portsForService = append(portsForService, *pd)
		}
	}

	svc := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gs.Name,
			Namespace: gs.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(gs, schema.GroupVersionKind{
					Group:   mpsv1alpha1.GroupVersion.Group,
					Version: mpsv1alpha1.GroupVersion.Version,
					Kind:    "GameServer",
				}),
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"OwningGameServer": gs.Name,
			},
			Ports: portsForService,
		},
	}

	if err := r.Create(ctx, &svc); err != nil {
		return err
	}

	return nil
}

func (r *GameServerReconciler) createIngressRoute(ctx context.Context, gs *mpsv1alpha1.GameServer, svc *corev1.Service) error {
	portsForTraefikService := []traefikv1alpha1.Service{}
	for _, port := range svc.Spec.Ports {
		portsForTraefikService = append(portsForTraefikService, traefikv1alpha1.Service{
			LoadBalancerSpec: traefikv1alpha1.LoadBalancerSpec{
				Name: gs.Name,
				Port: intstr.FromInt(int(port.Port)),
			},
		})
	}

	entryPoints := []string{}
	if envNonTlsEntryPoint != "" {
		entryPoints = append(entryPoints, envNonTlsEntryPoint)
	}
	if envTlsEntryPoint != "" {
		entryPoints = append(entryPoints, envTlsEntryPoint)
	}

	ig := traefikv1alpha1.IngressRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gs.Name,
			Namespace: gs.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(gs, schema.GroupVersionKind{
					Group:   mpsv1alpha1.GroupVersion.Group,
					Version: mpsv1alpha1.GroupVersion.Version,
					Kind:    "GameServer",
				}),
			},
		},
		Spec: traefikv1alpha1.IngressRouteSpec{
			EntryPoints: entryPoints,
			Routes: []traefikv1alpha1.Route{
				{
					Kind:  "Rule",
					Match: fmt.Sprintf("Host(`%s`) && PathPrefix(`/%s`)", envDnsName, gs.Name),
					Middlewares: []traefikv1alpha1.MiddlewareRef{
						{
							Name:      envMiddlewareName,
							Namespace: envMiddlewareNamespace,
						},
					},
					Services: portsForTraefikService,
				},
			},
		},
	}

	if err := r.Create(ctx, &ig); err != nil {
		return err
	}

	return nil
}
