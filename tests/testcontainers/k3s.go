//go:build integration

package testcontainers

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	tck3s "github.com/testcontainers/testcontainers-go/modules/k3s"
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

// K3sCluster starts a k3s container (image from K3S_IMAGE in docker/.env),
// writes its kubeconfig to a temp file, builds a clientset, and registers
// cleanup. The container is privileged, so it is heavier than the other
// helpers here — prefer one cluster per test package.
func K3sCluster(t testing.TB) *K3s {
	t.Helper()
	ctx := context.Background()

	ctr, err := tck3s.Run(ctx, Image(t, "K3S_IMAGE"))
	if err != nil {
		t.Fatalf("start k3s container: %v", err)
	}
	t.Cleanup(func() {
		if ctr != nil {
			_ = ctr.Terminate(context.Background())
		}
	})

	raw, err := ctr.GetKubeConfig(ctx)
	if err != nil {
		t.Fatalf("k3s kubeconfig: %v", err)
	}
	path := filepath.Join(t.TempDir(), "kubeconfig")
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
