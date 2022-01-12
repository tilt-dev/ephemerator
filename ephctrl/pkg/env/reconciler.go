package env

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	appKey    = "app.kubernetes.io/part-of"
	appValue  = "ephemerator.tilt.dev"
	nameKey   = "app.kubernetes.io/name"
	nameValue = "cluster-bootstrapper"
	ownerKey  = ".metadata.controller"
	apiGVStr  = "v1"
	configKey = "ephemerator.tilt.dev/configmap"
)

type Reconciler struct {
	client client.Client
}

func NewReconciler(client client.Client) *Reconciler {
	return &Reconciler{client: client}
}

func (r *Reconciler) CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error) {
	adminLS := metav1.SetAsLabelSelector(labels.Set{appKey: appValue})
	adminPred, err := predicate.LabelSelectorPredicate(*adminLS)
	if err != nil {
		return nil, err
	}

	userLS := metav1.SetAsLabelSelector(labels.Set{appKey: appValue, nameKey: nameValue})
	userPred, err := predicate.LabelSelectorPredicate(*userLS)
	if err != nil {
		return nil, err
	}

	// Adapted from
	// https://book.kubebuilder.io/cronjob-tutorial/controller-implementation.html
	// Index all pods that are owned by the ephemerator.
	err = mgr.GetFieldIndexer().IndexField(context.Background(),
		&v1.Pod{}, ownerKey, func(rawObj client.Object) []string {
			pod := rawObj.(*v1.Pod)
			if pod.Labels[appKey] != appValue || pod.Labels[nameKey] != nameValue {
				return nil
			}

			owner := metav1.GetControllerOf(pod)
			if owner == nil {
				return nil
			}
			if owner.APIVersion != apiGVStr || owner.Kind != "ConfigMap" {
				return nil
			}
			return []string{owner.Name}
		})
	if err != nil {
		return nil, err
	}

	b := ctrl.NewControllerManagedBy(mgr).
		For(&v1.ConfigMap{}, builder.WithPredicates(adminPred)).
		Owns(&v1.Pod{}, builder.WithPredicates(userPred))

	return b, nil
}

func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := log.FromContext(ctx)
	log.Info("reconciling")

	nn := req.NamespacedName

	cm := &v1.ConfigMap{}
	err := r.client.Get(ctx, nn, cm)
	if err != nil && !apierrors.IsNotFound(err) {
		return reconcile.Result{}, err
	}

	pod := &v1.Pod{}
	err = r.client.Get(ctx, nn, pod)
	if err != nil && !apierrors.IsNotFound(err) {
		return reconcile.Result{}, err
	}

	if pod.Name != "" && pod.Labels[appKey] != appValue {
		// If the labels don't match, bail out.
		return reconcile.Result{}, fmt.Errorf("Cannot touch conficting pod")
	}

	pod, err = r.maybeDeletePod(ctx, log, pod, cm)
	if err != nil {
		return reconcile.Result{}, err
	}

	needsCreate := pod.Name == ""
	if needsCreate {
		pod, err = r.createPod(ctx, cm)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func (r *Reconciler) createAnnotation(cm *v1.ConfigMap) (string, error) {
	if cm.Name == "" {
		return "", nil
	}

	buf := bytes.NewBuffer(nil)
	encoder := json.NewEncoder(buf)
	err := encoder.Encode(cm.Data)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// Create the pod with the parameters specified
// in the given configmap.
func (r *Reconciler) createPod(ctx context.Context, cm *v1.ConfigMap) (*v1.Pod, error) {
	configAnnoValue, err := r.createAnnotation(cm)
	if err != nil {
		return nil, fmt.Errorf("serializing configmap: %v", err)
	}

	// Credits:
	// https://radu-matei.com/blog/kubernetes-e2e-kind-brigade/
	// https://github.com/kubernetes-sigs/kind/issues/303
	// for instructions on how to set up kind-in-kubernetes
	privileged := true
	hostPathDirectory := v1.HostPathDirectory
	spec := v1.PodSpec{
		DNSPolicy: "None",
		DNSConfig: &v1.PodDNSConfig{
			Nameservers: []string{"1.1.1.1", "1.0.0.1"},
		},
		Volumes: []v1.Volume{
			{
				Name: "modules",
				VolumeSource: v1.VolumeSource{
					HostPath: &v1.HostPathVolumeSource{Path: "/lib/modules", Type: &hostPathDirectory},
				},
			},
			{
				Name: "cgroup",
				VolumeSource: v1.VolumeSource{
					HostPath: &v1.HostPathVolumeSource{Path: "/sys/fs/cgroup", Type: &hostPathDirectory},
				},
			},
			{
				Name: "dind-storage",
				VolumeSource: v1.VolumeSource{
					EmptyDir: &v1.EmptyDirVolumeSource{},
				},
			},
		},
		Containers: []v1.Container{
			{
				Name:  "cluster-bootstrapper",
				Image: os.Getenv("CLUSTER_BOOTSTRAPPER_IMAGE"),
				SecurityContext: &v1.SecurityContext{
					Privileged: &privileged,
				},
				VolumeMounts: []v1.VolumeMount{
					{
						MountPath: "/lib/modules",
						Name:      "modules",
						ReadOnly:  true,
					},
					{
						MountPath: "/sys/fs/cgroup",
						Name:      "cgroup",
					},
					{
						Name:      "dind-storage",
						MountPath: "/var/lib/docker",
					},
				},
			},
		},
	}

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cm.Name,
			Namespace: cm.Namespace,
			Labels: map[string]string{
				appKey:  appValue,
				nameKey: nameValue,
			},
			Annotations: map[string]string{
				configKey: configAnnoValue,
			},
		},
		Spec: spec,
	}

	return pod, r.client.Create(ctx, pod)
}

// Determine if there's any mismatch between the pod and its owner config,
// deleting if necessary.
func (r *Reconciler) maybeDeletePod(ctx context.Context, log logr.Logger, pod *v1.Pod, owner *v1.ConfigMap) (*v1.Pod, error) {
	needsDelete := false
	if pod.Name != "" && owner.Name == "" {
		// If the configmap has been deleted, and the pod has not been, delete the pod.
		log.Info("deleting pod because configmap was deleted")
		needsDelete = true
	}

	if !needsDelete && pod.Name != "" {
		configAnnoValue, err := r.createAnnotation(owner)
		if err != nil {
			return nil, fmt.Errorf("serializing configmap: %v", err)
		}

		if pod.Annotations[configKey] != configAnnoValue {
			log.Info("deleting pod because configmap changed")
			needsDelete = true
		}
	}

	if needsDelete {
		err := r.deletePod(ctx, pod)
		if err != nil {
			return nil, err
		}
		pod = &v1.Pod{}
	}
	return pod, nil
}

// TODO(nick): This isn't quite right. We probably need to do bookkeeping to make
// sure the pod is getting cleaned up correctly (since it's talking directly
// to the docker socket).
func (r *Reconciler) deletePod(ctx context.Context, pod *v1.Pod) error {
	return r.client.Delete(ctx, pod)
}
