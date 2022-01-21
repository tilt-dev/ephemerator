package env

import (
	"context"
	"sync"

	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Makes sure that the gateway has host mappings for every environment.
type GatewayReconciler struct {
	cluster Cluster

	mu       sync.Mutex
	gateways map[types.NamespacedName]bool
}

func NewGatewayReconciler(cluster Cluster) *Reconciler {
	return &Reconciler{
		cluster: cluster,
	}
}

func (r *GatewayReconciler) AddToManager(mgr ctrl.Manager) error {
	adminLS := metav1.SetAsLabelSelector(labels.Set{appKey: appValue})
	adminPred, err := predicate.LabelSelectorPredicate(*adminLS)
	if err != nil {
		return err
	}

	userLS := metav1.SetAsLabelSelector(labels.Set{appKey: appValue, nameKey: nameValue})
	userPred, err := predicate.LabelSelectorPredicate(*userLS)
	if err != nil {
		return err
	}

	mapFunc := func(client.Object) []reconcile.Request {
		r.mu.Lock()
		defer r.mu.Unlock()

		reqs := []reconcile.Request{}
		for nn := range r.gateways {
			reqs = append(reqs, reconcile.Request{NamespacedName: nn})
		}
		return reqs
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1.Ingress{}, builder.WithPredicates(adminPred)).
		Watches(&source.Kind{Type: &v1.Service{}}, handler.EnqueueRequestsFromMapFunc(mapFunc), builder.WithPredicates(userPred)).
		Complete(r)
}

func (r *GatewayReconciler) client() client.Client {
	return r.cluster.GetClient()
}

func (r *GatewayReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	log := log.FromContext(ctx)
	log.Info("reconciling")

	nn := req.NamespacedName

	ing := &networkingv1.Ingress{}
	err := r.client().Get(ctx, nn, ing)
	if err != nil && !apierrors.IsNotFound(err) {
		return reconcile.Result{}, err
	}

	if apierrors.IsNotFound(err) {
		delete(r.gateways, nn)
		return reconcile.Result{}, nil
	}
	r.gateways[nn] = true

	return reconcile.Result{}, nil
}
