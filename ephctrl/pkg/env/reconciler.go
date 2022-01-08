package env

import (
	"context"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type Reconciler struct{}

func NewReconciler() *Reconciler {
	return &Reconciler{}
}

func (r *Reconciler) CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error) {
	ls := metav1.SetAsLabelSelector(labels.Set{"app": "ephemerator.tilt.dev"})
	pred, err := predicate.LabelSelectorPredicate(*ls)
	if err != nil {
		return nil, err
	}
	b := ctrl.NewControllerManagedBy(mgr).
		For(&v1.ConfigMap{}, builder.WithPredicates(pred)).
		Owns(&v1.Pod{}, builder.WithPredicates(pred))

	return b, nil
}

func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	return reconcile.Result{}, nil
}
