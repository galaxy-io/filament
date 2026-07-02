package k8sdispatch

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
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("k8sdispatch: create job %q in %q: %w", j.Name, namespace, err)
	}
	return nil
}
