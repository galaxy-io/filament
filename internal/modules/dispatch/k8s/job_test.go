package k8s

import (
	"maps"
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/galaxy-io/filament"
)

// The Job is the entire contract between dispatch and the worker, so an unset
// quantity must be absent rather than zero: a zero CPU request is a request,
// and it would override whatever a namespace LimitRange supplies.
func TestWorkerResources(t *testing.T) {
	t.Run("unset resources produce no requirements", func(t *testing.T) {
		got, err := workerResources(filament.WorkerResources{})
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Requests) != 0 || len(got.Limits) != 0 {
			t.Fatalf("got %+v, want empty", got)
		}
	})

	t.Run("partial resources set only what was named", func(t *testing.T) {
		got, err := workerResources(filament.WorkerResources{CPURequest: "500m", MemoryLimit: "4Gi"})
		if err != nil {
			t.Fatal(err)
		}
		if q, ok := got.Requests[corev1.ResourceCPU]; !ok || q.String() != "500m" {
			t.Fatalf("cpu request = %v (present %v), want 500m", q.String(), ok)
		}
		if _, ok := got.Requests[corev1.ResourceMemory]; ok {
			t.Fatal("memory request should be absent")
		}
		if q, ok := got.Limits[corev1.ResourceMemory]; !ok || q.String() != "4Gi" {
			t.Fatalf("memory limit = %v (present %v), want 4Gi", q.String(), ok)
		}
		if _, ok := got.Limits[corev1.ResourceCPU]; ok {
			t.Fatal("cpu limit should be absent")
		}
	})

	t.Run("unparseable quantity is refused", func(t *testing.T) {
		if _, err := workerResources(filament.WorkerResources{CPURequest: "half a core"}); err == nil {
			t.Fatal("want error for an unparseable quantity")
		}
	})
}

// A selector that finds the Job must find its pods, so the two label sets have
// to stay identical — they drifted apart when they were written out separately.
func TestJobAndPodLabelsMatch(t *testing.T) {
	m := &Module{cfg: Config{WorkerImage: "worker:test", WorkerSecretName: "filament-secret", JobNamePrefix: "filament"}}
	job, err := m.jobForSpec(filament.RunSpec{Tenant: "acme", Run: "run-1"})
	if err != nil {
		t.Fatal(err)
	}
	pod := job.Spec.Template.Labels
	if !maps.Equal(job.Labels, pod) {
		t.Fatalf("job labels %v != pod labels %v", job.Labels, pod)
	}
	if got := pod["filament.galaxy.io/tenant"]; got != "acme" {
		t.Fatalf("pod tenant label = %q, want acme", got)
	}
	if got := pod["filament.galaxy.io/run-id"]; got != "run-1" {
		t.Fatalf("pod run-id label = %q, want run-1", got)
	}
}

// jobForSpec carries the run's resolved configuration onto the container, and
// refuses the run rather than silently sizing it wrong.
func TestJobForSpecResources(t *testing.T) {
	m := &Module{cfg: Config{WorkerImage: "worker:test", WorkerSecretName: "filament-secret", JobNamePrefix: "filament"}}
	spec := filament.RunSpec{
		Tenant: "acme", Run: "run-1",
		WorkerConfiguration: filament.WorkerConfiguration{
			Resources: filament.WorkerResources{CPULimit: "2", MemoryLimit: "8Gi"},
		},
	}
	job, err := m.jobForSpec(spec)
	if err != nil {
		t.Fatal(err)
	}
	limits := job.Spec.Template.Spec.Containers[0].Resources.Limits
	if q := limits[corev1.ResourceCPU]; q.String() != "2" {
		t.Fatalf("cpu limit = %q, want 2", q.String())
	}
	if q := limits[corev1.ResourceMemory]; q.String() != "8Gi" {
		t.Fatalf("memory limit = %q, want 8Gi", q.String())
	}

	spec.WorkerConfiguration.Resources.MemoryLimit = "8 gigabytes"
	if _, err := m.jobForSpec(spec); err == nil {
		t.Fatal("want error for an unparseable stored quantity")
	}
}
