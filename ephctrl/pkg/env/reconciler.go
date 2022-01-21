package env

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	appKey          = "app.kubernetes.io/part-of"
	appValue        = "ephemerator.tilt.dev"
	nameKey         = "app.kubernetes.io/name"
	nameValue       = "ephrunner"
	ephOwnerNameKey = "ephemerator.tilt.dev/owner-name"
	configKey       = "ephemerator.tilt.dev/configmap"
)

type Cluster interface {
	GetClient() client.Client
	GetConfig() *rest.Config
	GetScheme() *runtime.Scheme
}

type Reconciler struct {
	cluster Cluster
}

func NewReconciler(cluster Cluster) *Reconciler {
	return &Reconciler{
		cluster: cluster,
	}
}

func (r *Reconciler) AddToManager(mgr ctrl.Manager) error {
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

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.ConfigMap{}, builder.WithPredicates(adminPred)).
		Owns(&v1.Pod{}, builder.WithPredicates(userPred)).
		Owns(&v1.Service{}, builder.WithPredicates(userPred)).
		Complete(r)
}

func (r *Reconciler) client() client.Client {
	return r.cluster.GetClient()
}

func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := log.FromContext(ctx)
	log.Info("reconciling")

	nn := req.NamespacedName

	cm := &v1.ConfigMap{}
	err := r.client().Get(ctx, nn, cm)
	if err != nil && !apierrors.IsNotFound(err) {
		return reconcile.Result{}, err
	}

	pod := &v1.Pod{}
	err = r.client().Get(ctx, nn, pod)
	if err != nil && !apierrors.IsNotFound(err) {
		return reconcile.Result{}, err
	}

	service := &v1.Service{}
	err = r.client().Get(ctx, nn, service)
	if err != nil && !apierrors.IsNotFound(err) {
		return reconcile.Result{}, err
	}

	if pod.Name != "" && pod.Labels[appKey] != appValue {
		// If the labels don't match, bail out.
		return reconcile.Result{}, fmt.Errorf("Cannot touch conficting pod")
	}

	if service.Name != "" && service.Labels[appKey] != appValue {
		// If the labels don't match, bail out.
		return reconcile.Result{}, fmt.Errorf("Cannot touch conficting service")
	}

	pod, err = r.maybeDeletePod(ctx, pod, cm)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("deleting pod: %v", err)
	}

	needsCreate := pod.Name == "" && cm.Name != ""
	if needsCreate {
		pod, err = r.createPod(ctx, cm)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("creating pod: %v", err)
		}
	}

	service, err = r.maybeDeleteService(ctx, service, cm)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("deleting service: %v", err)
	}

	needsService := service.Name == "" && cm.Name != "" &&
		pod.Name != "" && pod.Status.Phase == v1.PodRunning
	if needsService {
		err = r.createService(ctx, cm)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("creating servoce: %v", err)
		}
	}

	// When the pod is running,

	_ = pod

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
	log := log.FromContext(ctx)
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
			{
				Name: "dind-socket",
				VolumeSource: v1.VolumeSource{
					EmptyDir: &v1.EmptyDirVolumeSource{},
				},
			},
		},
		Containers: []v1.Container{
			{
				Name:  "dind",
				Image: os.Getenv("DIND_IMAGE"),
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
					{
						Name:      "dind-socket",
						MountPath: "/run",
					},
				},
			},
			{
				Name:  "tilt-upper",
				Image: os.Getenv("TILT_UPPER_IMAGE"),
				Env: []v1.EnvVar{
					{
						Name:  "TILT_UPPER_REPO",
						Value: cm.Data["repo"],
					},
					{
						Name:  "TILT_UPPER_PATH",
						Value: cm.Data["path"],
					},
					{
						Name:  "TILT_UPPER_BRANCH",
						Value: cm.Data["branch"],
					},
				},
				ReadinessProbe: &v1.Probe{
					ProbeHandler: v1.ProbeHandler{
						Exec: &v1.ExecAction{
							Command: []string{"python3", "tilt-healthcheck.py"},
						},
					},
					TimeoutSeconds: 2,
					PeriodSeconds:  5,
				},
				VolumeMounts: []v1.VolumeMount{
					{
						Name:      "dind-socket",
						MountPath: "/run",
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
				appKey:          appValue,
				nameKey:         nameValue,
				ephOwnerNameKey: cm.Name,
			},
			Annotations: map[string]string{
				configKey: configAnnoValue,
			},
		},
		Spec: spec,
	}

	err = ctrl.SetControllerReference(cm, pod, r.cluster.GetScheme())
	if err != nil {
		return nil, err
	}

	log.Info("creating pod")
	return pod, r.client().Create(ctx, pod)
}

// Determine if there's any mismatch between the pod and its owner config,
// deleting if necessary.
func (r *Reconciler) maybeDeletePod(ctx context.Context, pod *v1.Pod, owner *v1.ConfigMap) (*v1.Pod, error) {
	log := log.FromContext(ctx)
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
	return client.IgnoreNotFound(r.client().Delete(ctx, pod))
}

/*
// Once the pod is healthy, `tilt get uiresources` should give us a list of
// endpoints that need port-forwarding.
func (r *Reconciler) determinePorts(ctx context.Context, pod *v1.Pod) ([]int, error) {
	cmd := []string{"tilt", "get", "uiresources", "-o", "json"}
	restClient, err := rest.RESTClientFor(r.cluster.GetConfig())
	if err != nil {
		return nil, err
	}
	req := restClient.Post().
		Resource("pods").
		Namespace(pod.Namespace).
		Name(pod.Name).
		SubResource("exec").
		Param("container", "tilt-upper")
	req.VersionedParams(&v1.PodExecOptions{
		Container: "tilt-upper",
		Command:   cmd,
		Stdin:     false,
		Stdout:    true,
		Stderr:    false,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(r.cluster.GetConfig(), "POST", req.URL())
	if err != nil {
		return nil, err
	}

	stdout := bytes.NewBuffer(nil)

	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  bytes.NewBuffer(nil),
		Stdout: stdout,
		Stderr: bytes.NewBuffer(nil),
	})
	if err != nil {
		return nil, err
	}

	decoder := json.NewDecoder(stdout)
	_ = decoder
	return nil, nil
}
*/

func (r *Reconciler) createService(ctx context.Context, cm *v1.ConfigMap) error {
	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cm.Name,
			Namespace: cm.Namespace,
			Labels: map[string]string{
				appKey:  appValue,
				nameKey: nameValue,
			},
		},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{
				appKey:          appValue,
				nameKey:         nameValue,
				ephOwnerNameKey: cm.Name,
			},
			Ports: []v1.ServicePort{
				{
					Name:     "tcp-10350",
					Protocol: "TCP",
					Port:     10350,
				},
			},
		},
	}

	err := ctrl.SetControllerReference(cm, svc, r.cluster.GetScheme())
	if err != nil {
		return err
	}

	log.FromContext(ctx).Info("creating service")
	return r.client().Create(ctx, svc)
}

// Determine if there's any mismatch between the service and its owner config,
// deleting if necessary.
func (r *Reconciler) maybeDeleteService(ctx context.Context, service *v1.Service, owner *v1.ConfigMap) (*v1.Service, error) {
	log := log.FromContext(ctx)
	needsDelete := false
	if service.Name != "" && owner.Name == "" {
		// If the configmap has been deleted, and the pod has not been, delete the pod.
		log.Info("deleting service because configmap was deleted")
		needsDelete = true
	}

	if needsDelete {
		err := client.IgnoreNotFound(r.client().Delete(ctx, service))
		if err != nil {
			return nil, err
		}
		service = &v1.Service{}
	}
	return service, nil
}
