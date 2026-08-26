package k8s

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type client struct {
	clientset kubernetes.Interface
}

func newClient(cfg Config) (*client, error) {
	restCfg, err := restConfig(cfg)
	if err != nil {
		return nil, err
	}
	cs, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("k8sdispatch: build clientset: %w", err)
	}
	return &client{clientset: cs}, nil
}

// restConfig resolves how to reach the API server: an explicit kubeconfig
// (out-of-cluster / local dev) takes precedence, otherwise the in-cluster
// service-account credentials mounted into the pod are used.
func restConfig(cfg Config) (*rest.Config, error) {
	if cfg.Kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", cfg.Kubeconfig)
	}
	return rest.InClusterConfig()
}

func (c *client) createJob(ctx context.Context, namespace string, j *batchv1.Job) error {
	_, err := c.clientset.BatchV1().Jobs(namespace).Create(ctx, j, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		existing, getErr := c.clientset.BatchV1().Jobs(namespace).Get(ctx, j.Name, metav1.GetOptions{})
		if getErr != nil {
			return fmt.Errorf("k8sdispatch: inspect existing job %q in %q: %w", j.Name, namespace, getErr)
		}
		for _, key := range []string{"filament.galaxy.io/run-id", "filament.galaxy.io/execution-id"} {
			if existing.Labels[key] != j.Labels[key] {
				return fmt.Errorf("k8sdispatch: job name collision %q in %q: label %q is %q, want %q", j.Name, namespace, key, existing.Labels[key], j.Labels[key])
			}
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("k8sdispatch: create job %q in %q: %w", j.Name, namespace, err)
	}
	return nil
}

func (c *client) listJobs(ctx context.Context, namespace, selector string) ([]batchv1.Job, error) {
	list, err := c.clientset.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, fmt.Errorf("k8sdispatch: list jobs in %q: %w", namespace, err)
	}
	return list.Items, nil
}
