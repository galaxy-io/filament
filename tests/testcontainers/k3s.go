//go:build integration

package testcontainers

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	tck3s "github.com/testcontainers/testcontainers-go/modules/k3s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// K3s holds an ephemeral single-node Kubernetes cluster.
//
// The k8s dispatcher already reads an explicit kubeconfig path (K8S_KUBECONFIG,
// see internal/modules/dispatch/k8s/config.go), so pointing it at KubeconfigPath
// exercises the real create-a-Job path against a real API server with no
// production code changes.
type K3s struct {
	Container *tck3s.K3sContainer
	// KubeconfigPath is a temp file holding the cluster's kubeconfig, valid for
	// the lifetime of the test.
	KubeconfigPath string
	Clientset      kubernetes.Interface
}

// K3sCluster starts an exclusive k3s container (image from K3S_IMAGE in
// docker/.env), writes its kubeconfig to a temp file, builds a clientset, and
// registers cleanup. The container is privileged, so it is heavier than the
// other helpers here — prefer the suite-wide SharedK3s.
func K3sCluster(t testing.TB) *K3s {
	t.Helper()
	cluster := startK3s(t, t.TempDir())
	t.Cleanup(func() { _ = cluster.Container.Terminate(context.Background()) })
	return cluster
}

// startK3s boots the cluster with no test-scoped cleanup; shared instances
// outlive any one test and are reaped at process exit. dir holds the
// kubeconfig and must outlive the cluster's users.
func startK3s(t testing.TB, dir string) *K3s {
	t.Helper()
	ctx := context.Background()

	ctr, err := tck3s.Run(ctx, Image(t, "K3S_IMAGE"))
	if err != nil {
		t.Fatalf("start k3s container: %v", err)
	}

	raw, err := ctr.GetKubeConfig(ctx)
	if err != nil {
		t.Fatalf("k3s kubeconfig: %v", err)
	}
	path := filepath.Join(dir, "kubeconfig")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}

	cfg, err := clientcmd.RESTConfigFromKubeConfig(raw)
	if err != nil {
		t.Fatalf("parse kubeconfig: %v", err)
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		t.Fatalf("k3s clientset: %v", err)
	}

	return &K3s{Container: ctr, KubeconfigPath: path, Clientset: cs}
}

// Wipe deletes every Job (and its pods, via foreground propagation) in the
// default namespace and waits until they are gone, so job-listing probes in
// the next test see a clean slate.
func (k *K3s) Wipe(t testing.TB) {
	t.Helper()
	ctx := context.Background()
	fg := metav1.DeletePropagationForeground
	err := k.Clientset.BatchV1().Jobs("default").DeleteCollection(ctx,
		metav1.DeleteOptions{PropagationPolicy: &fg}, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("wipe jobs: %v", err)
	}
	deadline := time.Now().Add(60 * time.Second)
	for {
		jobs, err := k.Clientset.BatchV1().Jobs("default").List(ctx, metav1.ListOptions{})
		if err == nil && len(jobs.Items) == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("wipe jobs: %d still present (err %v)", len(jobs.Items), err)
		}
		time.Sleep(250 * time.Millisecond)
	}
}
