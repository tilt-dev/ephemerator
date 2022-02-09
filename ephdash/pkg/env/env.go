package env

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/tilt-dev/ephemerator/ephconfig"
	"golang.org/x/sync/errgroup"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	informersv1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
)

var PodGVR = v1.SchemeGroupVersion.WithResource("pods")
var ServiceGVR = v1.SchemeGroupVersion.WithResource("services")
var ConfigMapGVR = v1.SchemeGroupVersion.WithResource("configmaps")

type Env struct {
	ConfigMap *v1.ConfigMap
	Pod       *v1.Pod
	Service   *v1.Service
	PodLogs   *bytes.Buffer
}

type Client struct {
	clientset    *kubernetes.Clientset
	namespace    string
	pods         informersv1.PodInformer
	svcs         informersv1.ServiceInformer
	cms          informersv1.ConfigMapInformer
	slackWebhook string
}

func NewClient(ctx context.Context, clientset *kubernetes.Clientset, namespace string, slackWebhook string) *Client {
	options := []informers.SharedInformerOption{
		informers.WithNamespace(namespace),
	}

	factory := informers.NewSharedInformerFactoryWithOptions(clientset, time.Hour, options...)
	podInformer := factory.Core().V1().Pods()
	svcInformer := factory.Core().V1().Services()
	cmInformer := factory.Core().V1().ConfigMaps()
	go podInformer.Informer().Run(ctx.Done())
	go svcInformer.Informer().Run(ctx.Done())
	go cmInformer.Informer().Run(ctx.Done())

	return &Client{
		clientset:    clientset,
		namespace:    namespace,
		pods:         podInformer,
		svcs:         svcInformer,
		cms:          cmInformer,
		slackWebhook: slackWebhook,
	}
}

// Fetch all the objects associated with this env.
//
// The API currently assumes every user has exactly one env.
// Returns (nil, nil) if the env does not exist.
func (c *Client) GetEnv(ctx context.Context, name string) (*Env, error) {
	env := &Env{}
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		obj, err := c.cms.Lister().ConfigMaps(c.namespace).Get(name)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		if !hasRunnerLabels(obj.ObjectMeta) {
			return nil
		}
		env.ConfigMap = obj
		return nil
	})

	g.Go(func() error {
		obj, err := c.pods.Lister().Pods(c.namespace).Get(name)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		if !hasRunnerLabels(obj.ObjectMeta) {
			return nil
		}
		env.Pod = obj
		return nil
	})

	g.Go(func() error {
		req := c.clientset.CoreV1().Pods(c.namespace).GetLogs(name, &v1.PodLogOptions{Container: "tilt-upper"})
		podLogs, err := req.Stream(ctx)
		if err != nil {
			return nil // Always return nil
		}
		defer podLogs.Close()

		buf := new(bytes.Buffer)
		_, err = io.Copy(buf, podLogs)
		if err != nil {
			return nil // Always return nil
		}

		if buf.Len() != 0 {
			env.PodLogs = buf
		}
		return nil
	})

	g.Go(func() error {
		obj, err := c.svcs.Lister().Services(c.namespace).Get(name)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		if !hasRunnerLabels(obj.ObjectMeta) {
			return nil
		}
		env.Service = obj
		return nil
	})

	err := g.Wait()
	if err != nil {
		return nil, err
	}

	if env.ConfigMap == nil && env.Pod == nil {
		return nil, nil
	}
	return env, nil
}

func hasRunnerLabels(meta metav1.ObjectMeta) bool {
	return meta.Labels[ephconfig.LabelAppKey] == ephconfig.LabelAppValueEphemerator &&
		meta.Labels[ephconfig.LabelNameKey] == ephconfig.LabelNameValueEphrunner
}

type slackMessage struct {
	Text string `json:"text"`
}

// Post a slack message and ignore the result.
func (c *Client) maybePostSlackMessage(msg string) {
	if c.slackWebhook == "" {
		return
	}
	slackMsg := slackMessage{Text: msg}
	content, err := json.Marshal(slackMsg)
	if err != nil {
		return
	}

	resp, err := http.Post(c.slackWebhook, "application/json",
		bytes.NewBuffer(content))
	if err != nil {
		return
	}
	defer resp.Body.Close()
}

// Delete the configuration for the env.
func (c *Client) DeleteEnv(ctx context.Context, name string) error {
	c.maybePostSlackMessage(fmt.Sprintf("Deleting env: %s", name))

	// Make sure we're not deleting a configmap for a non-runner.
	current, err := c.clientset.CoreV1().ConfigMaps(c.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if !hasRunnerLabels(current.ObjectMeta) {
		// Make sure we don't overwrite a configmap for non-runners.
		return fmt.Errorf("conflict with existing env: %s", name)
	}

	return c.clientset.CoreV1().ConfigMaps(c.namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// Set the configuration for the env.
func (c *Client) SetEnvSpec(ctx context.Context, name string, spec ephconfig.EnvSpec) error {
	c.maybePostSlackMessage(fmt.Sprintf("Updating env %s: %+v", name, spec))

	desired := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: c.namespace,
			Labels: map[string]string{
				ephconfig.LabelAppKey:  ephconfig.LabelAppValueEphemerator,
				ephconfig.LabelNameKey: ephconfig.LabelNameValueEphrunner,
			},
		},
		Data: map[string]string{
			"repo":   spec.Repo,
			"path":   spec.Path,
			"branch": spec.Branch,
			// For now, let the controller handle expiration.
		},
	}

	// Reconcile the desired config map with the current configmap.
	current, err := c.clientset.CoreV1().ConfigMaps(c.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	if apierrors.IsNotFound(err) {
		_, err := c.clientset.CoreV1().ConfigMaps(c.namespace).Create(ctx, desired, metav1.CreateOptions{})
		return err
	}

	if !hasRunnerLabels(current.ObjectMeta) {
		// Make sure we don't overwrite a configmap for non-runners.
		return fmt.Errorf("conflict with existing env: %s", name)
	}

	update := current.DeepCopy()
	update.Data = desired.Data
	_, err = c.clientset.CoreV1().ConfigMaps(c.namespace).Update(ctx, update, metav1.UpdateOptions{})
	return err
}
