//go:build integration

package integration

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/memory"
	"github.com/galaxy-io/filament/internal/modules/dispatch/k8s"
	"github.com/galaxy-io/filament/module"
	"github.com/galaxy-io/filament/tests/testcontainers"
)

// TestK8sDispatchCreatesWorkerJob drives the Kubernetes dispatch backend against
// a real API server. DISPATCH_MODE=kubernetes is the default and what the chart
// ships, but nothing exercised it before this: the unit suite never builds a
// client, and every other integration test runs the in-process engine.
//
// It asserts the Job spec the control plane hands the cluster, which is the
// entire contract between dispatch and the worker.
func TestK8sDispatchCreatesWorkerJob(t *testing.T) {
	cluster := testcontainers.SharedK3s(t)
	ctx := context.Background()

	const (
		namespace = "default"
		image     = "ghcr.io/galaxy-io/filament/worker:test"
		secret    = "filament-secret"
		account   = "filament-worker"
		prefix    = "filament"
	)

	dispatcher := k8s.New(k8s.Config{
		Namespace:            namespace,
		WorkerImage:          image,
		WorkerSecretName:     secret,
		WorkerServiceAccount: account,
		JobNamePrefix:        prefix,
		Kubeconfig:           cluster.KubeconfigPath,
		BackoffLimit:         1,
	})
	if err := dispatcher.Mount(ctx, module.Deps{DataStore: memory.New()}); err != nil {
		t.Fatalf("mount dispatcher: %v", err)
	}

	spec := filament.RunSpec{
		Tenant: "acme",
		Run:    "run-dispatch-1",
		Source: filament.Ref{Connector: "postgres"},
		Sink:   filament.Ref{Connector: "stdout"},
	}
	if _, err := dispatcher.Dispatch(ctx, spec); err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	jobs, err := cluster.Clientset.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "filament.galaxy.io/run-id=" + string(spec.Run),
	})
	if err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	if len(jobs.Items) != 1 {
		t.Fatalf("got %d jobs for run %q, want 1", len(jobs.Items), spec.Run)
	}
	job := jobs.Items[0]

	if got := job.Labels["filament.galaxy.io/tenant"]; got != string(spec.Tenant) {
		t.Errorf("tenant label = %q, want %q", got, spec.Tenant)
	}
	if len(job.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("got %d containers, want 1", len(job.Spec.Template.Spec.Containers))
	}
	container := job.Spec.Template.Spec.Containers[0]
	if container.Image != image {
		t.Errorf("image = %q, want %q", container.Image, image)
	}
	if got := job.Spec.Template.Spec.ServiceAccountName; got != account {
		t.Errorf("service account = %q, want %q", got, account)
	}

	// RUN_ID is the only way the worker learns which run is its own.
	var runID string
	for _, env := range container.Env {
		if env.Name == "RUN_ID" {
			runID = env.Value
		}
	}
	if runID != string(spec.Run) {
		t.Errorf("RUN_ID = %q, want %q", runID, spec.Run)
	}

	// The worker's DSN, NATS URL and encryption key all arrive through this one
	// envFrom. If it stops being wired the worker cannot start at all.
	var envFromSecret string
	for _, src := range container.EnvFrom {
		if src.SecretRef != nil {
			envFromSecret = src.SecretRef.Name
		}
	}
	if envFromSecret != secret {
		t.Errorf("envFrom secret = %q, want %q", envFromSecret, secret)
	}
}

// TestK8sDispatchIsIdempotent covers redelivery: the bus is at-least-once, so
// the same run.requested fact can arrive twice. Creating the Job again must not
// fail the handler, or the consumer naks forever.
func TestK8sDispatchIsIdempotent(t *testing.T) {
	cluster := testcontainers.SharedK3s(t)
	ctx := context.Background()

	dispatcher := k8s.New(k8s.Config{
		Namespace:        "default",
		WorkerImage:      "ghcr.io/galaxy-io/filament/worker:test",
		WorkerSecretName: "filament-secret",
		JobNamePrefix:    "filament",
		Kubeconfig:       cluster.KubeconfigPath,
	})
	if err := dispatcher.Mount(ctx, module.Deps{DataStore: memory.New()}); err != nil {
		t.Fatalf("mount dispatcher: %v", err)
	}

	spec := filament.RunSpec{Tenant: "acme", Run: "run-dispatch-2"}
	for attempt := range 2 {
		if _, err := dispatcher.Dispatch(ctx, spec); err != nil {
			t.Fatalf("dispatch attempt %d: %v", attempt+1, err)
		}
	}

	jobs, err := cluster.Clientset.BatchV1().Jobs("default").List(ctx, metav1.ListOptions{
		LabelSelector: "filament.galaxy.io/run-id=" + string(spec.Run),
	})
	if err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	if len(jobs.Items) != 1 {
		t.Fatalf("got %d jobs after two dispatches, want 1", len(jobs.Items))
	}
}
