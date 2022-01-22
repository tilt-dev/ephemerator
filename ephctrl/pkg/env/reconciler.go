package env

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"
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
	cluster   Cluster
	clientset *kubernetes.Clientset
}

func NewReconciler(cluster Cluster) (*Reconciler, error) {
	clientset, err := kubernetes.NewForConfig(cluster.GetConfig())
	if err != nil {
		return nil, err
	}

	return &Reconciler{
		cluster:   cluster,
		clientset: clientset,
	}, nil
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

	desiredSvc, err := r.desiredService(ctx, cm, pod)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("connecting service: %v", err)
	}

	err = r.maybeUpdateService(ctx, service, desiredSvc)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("reconciling service: %v", err)
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

// Once the pod is healthy, `tilt get uiresources` should give us a list of
// endpoints that need port-forwarding.
func (r *Reconciler) determinePorts(ctx context.Context, pod *v1.Pod) ([]int32, error) {
	cmd := []string{"tilt", "get", "uiresources", "-o", "json"}
	log.FromContext(ctx).Info(fmt.Sprintf("running in pod: %s", cmd))
	req := r.clientset.CoreV1().RESTClient().Post().
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
		Stderr:    true,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(r.cluster.GetConfig(), "POST", req.URL())
	if err != nil {
		return nil, err
	}

	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)

	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: stdout,
		Stderr: stderr,
	})
	if err != nil {
		return nil, err
	}

	decoder := json.NewDecoder(stdout)
	var uiResourceList v1alpha1.UIResourceList
	err = decoder.Decode(&uiResourceList)
	if err != nil {
		return nil, err
	}

	// Default tilt port.
	ports := []int32{10350}
	for _, uiResource := range uiResourceList.Items {
		for _, link := range uiResource.Status.EndpointLinks {
			var port int32
			_, err := fmt.Sscanf(link.URL, "http://0.0.0.0:%d/", &port)
			if err != nil || port == 0 {
				continue
			}
			ports = append(ports, port)
		}
	}

	sort.Slice(ports, func(i, j int) bool { return ports[i] < ports[j] })
	return ports, nil
}

func (r *Reconciler) desiredService(ctx context.Context, cm *v1.ConfigMap, pod *v1.Pod) (*v1.Service, error) {
	if cm.Name == "" || pod.Name == "" || pod.Status.Phase != v1.PodRunning {
		return nil, nil
	}

	ports, err := r.determinePorts(ctx, pod)
	if err != nil {
		return nil, err
	}

	servicePorts := []v1.ServicePort{}
	for _, p := range ports {
		servicePorts = append(servicePorts, v1.ServicePort{
			Name:     fmt.Sprintf("tcp-%d", p),
			Protocol: "TCP",
			Port:     p,
		})
	}

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
			Ports: servicePorts,
		},
	}

	err = ctrl.SetControllerReference(cm, svc, r.cluster.GetScheme())
	if err != nil {
		return nil, err
	}
	return svc, nil
}

// Reconcile the desired service spec with the current service.
func (r *Reconciler) maybeUpdateService(ctx context.Context, current, desired *v1.Service) error {
	currentMissing := current == nil || current.Name == ""
	desiredMissing := desired == nil || desired.Name == ""
	if currentMissing && desiredMissing {
		return nil
	}

	if desiredMissing {
		log.FromContext(ctx).Info("deleting service")
		return client.IgnoreNotFound(r.client().Delete(ctx, current))
	}

	if currentMissing {
		log.FromContext(ctx).Info("creating service")
		return r.client().Create(ctx, desired)
	}

	if equality.Semantic.DeepEqual(desired.Spec.Ports, current.Spec.Ports) {
		return nil
	}

	log.FromContext(ctx).Info("updating service")
	update := current.DeepCopy()
	update.Spec.Ports = desired.Spec.Ports
	return r.client().Update(ctx, update)
}
