package env

import (
	"context"
	"fmt"
	"sort"
	"sync"

	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/equality"
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

func NewGatewayReconciler(cluster Cluster) *GatewayReconciler {
	return &GatewayReconciler{
		cluster:  cluster,
		gateways: make(map[types.NamespacedName]bool),
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

// Make sure the rules match
func (r *GatewayReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	log := log.FromContext(ctx)
	log.Info("reconciling ingress")

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

	svcs, err := r.services(ctx)
	if err != nil {
		return reconcile.Result{}, err
	}

	rules := r.desiredRules(svcs)

	if !equality.Semantic.DeepEqual(ing.Spec.Rules, rules) {
		update := ing.DeepCopy()
		update.Spec.Rules = rules
		err := r.client().Update(ctx, update)
		if err != nil {
			return reconcile.Result{}, err
		}
		log.Info(fmt.Sprintf("updated gateway with %d hosts for %d services", len(rules), len(svcs)))
	}

	return reconcile.Result{}, nil
}

// Convert the services into a set of ingress rules.
func (r *GatewayReconciler) desiredRules(svcs []v1.Service) []networkingv1.IngressRule {
	rules := []networkingv1.IngressRule{}
	prefix := networkingv1.PathTypePrefix
	for _, svc := range svcs {
		for _, port := range svc.Spec.Ports {
			subdomain := fmt.Sprintf("%d", port.Port)
			if port.Port == 10350 {
				subdomain = "tilt"
			}

			rules = append(rules, networkingv1.IngressRule{
				Host: fmt.Sprintf("%s.%s.localhost", subdomain, svc.Name),
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{
							{
								Path:     "/",
								PathType: &prefix,
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: svc.Name,
										Port: networkingv1.ServiceBackendPort{
											Number: port.Port,
										},
									},
								},
							},
						},
					},
				},
			})
		}
	}
	return rules
}

// Fetch all the services that need ingress host names assigned.
//
// The set is guaranteed to be stable.
func (r *GatewayReconciler) services(ctx context.Context) ([]v1.Service, error) {
	userLS := labels.SelectorFromSet(labels.Set{appKey: appValue, nameKey: nameValue})
	continueToken := ""

	result := []v1.Service{}
	for {
		var list v1.ServiceList
		err := r.client().List(ctx, &list, &client.ListOptions{
			Continue:      continueToken,
			LabelSelector: userLS,
		})
		if err != nil {
			return nil, err
		}

		result = append(result, list.Items...)

		if list.Continue == "" {
			sort.Slice(result, func(i, j int) bool {
				return result[i].Name < result[j].Name
			})

			return result, nil
		}

		continueToken = list.Continue
	}
}
