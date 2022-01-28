package env

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/tilt-dev/ephemerator/ephconfig"
	"golang.org/x/sync/errgroup"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Env struct {
	ConfigMap *v1.ConfigMap
	Pod       *v1.Pod
	Service   *v1.Service
	PodLogs   *bytes.Buffer
}

type EnvSpec struct {
	Repo   string
	Path   string
	Branch string
}

type Client struct {
	clientset *kubernetes.Clientset
	namespace string
}

func NewClient(clientset *kubernetes.Clientset, namespace string) *Client {
	return &Client{
		clientset: clientset,
		namespace: namespace,
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
		obj, err := c.clientset.CoreV1().ConfigMaps(c.namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		env.ConfigMap = obj
		return nil
	})

	g.Go(func() error {
		obj, err := c.clientset.CoreV1().Pods(c.namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
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
		obj, err := c.clientset.CoreV1().Services(c.namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		env.Service = obj
		return nil
	})

	err := g.Wait()
	if err != nil {
		return nil, err
	}

	if env.ConfigMap == nil {
		return nil, nil
	}
	return env, nil
}

// Delete the configuration for the env.
func (c *Client) DeleteEnv(ctx context.Context, name string) error {
	// Make sure we're not deleting a configmap for a non-runner.
	current, err := c.clientset.CoreV1().ConfigMaps(c.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if current.Labels[ephconfig.LabelAppKey] != ephconfig.LabelAppValueEphemerator ||
		current.Labels[ephconfig.LabelNameKey] != ephconfig.LabelNameValueEphrunner {
		// Make sure we don't overwrite a configmap for non-runners.
		return fmt.Errorf("conflict with existing env: %s", name)
	}

	return c.clientset.CoreV1().ConfigMaps(c.namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// Set the configuration for the env.
func (c *Client) SetEnvSpec(ctx context.Context, name string, spec EnvSpec) error {
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

	if current.Labels[ephconfig.LabelAppKey] != ephconfig.LabelAppValueEphemerator ||
		current.Labels[ephconfig.LabelNameKey] != ephconfig.LabelNameValueEphrunner {
		// Make sure we don't overwrite a configmap for non-runners.
		return fmt.Errorf("conflict with existing env: %s", name)
	}

	update := current.DeepCopy()
	update.Data = desired.Data
	_, err = c.clientset.CoreV1().ConfigMaps(c.namespace).Update(ctx, update, metav1.UpdateOptions{})
	return err
}
