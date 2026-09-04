package k8s

import (
	"maps"
	"reflect"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/galaxy-io/filament"
)

func TestJobFinishedRequiresTerminalCondition(t *testing.T) {
	for _, tt := range []struct {
		name string
		job  batchv1.Job
		want bool
	}{
		{name: "controller gap", job: batchv1.Job{}, want: false},
		{name: "active", job: batchv1.Job{Status: batchv1.JobStatus{Active: 1}}, want: false},
		{name: "complete", job: batchv1.Job{Status: batchv1.JobStatus{Conditions: []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue, LastTransitionTime: metav1.Now()}}}}, want: true},
		{name: "failed", job: batchv1.Job{Status: batchv1.JobStatus{Conditions: []batchv1.JobCondition{{Type: batchv1.JobFailed, Status: corev1.ConditionTrue, LastTransitionTime: metav1.Now()}}}}, want: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := jobFinished(&tt.job); got != tt.want {
				t.Fatalf("jobFinished() = %v, want %v", got, tt.want)
			}
		})
	}
}

// The Job is the entire contract between dispatch and the worker, so an unset
// map must be absent rather than empty: an empty request list is still a
// request list, and it would override whatever a namespace LimitRange supplies.
func TestWorkerResources(t *testing.T) {
	got, err := workerResources(filament.WorkerResources{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Requests != nil || got.Limits != nil {
		t.Fatalf("empty resources must map to nil lists, got %+v", got)
	}

	got, err = workerResources(filament.WorkerResources{
		Requests: map[string]string{"cpu": "500m"},
		Limits:   map[string]string{"memory": "2Gi", "nvidia.com/gpu": "1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if q := got.Requests[corev1.ResourceCPU]; q.String() != "500m" {
		t.Fatalf("cpu request = %q", q.String())
	}
	if _, ok := got.Requests[corev1.ResourceMemory]; ok {
		t.Fatal("unset memory request must be absent")
	}
	if q := got.Limits[corev1.ResourceMemory]; q.String() != "2Gi" {
		t.Fatalf("memory limit = %q", q.String())
	}
	if q := got.Limits["nvidia.com/gpu"]; q.String() != "1" {
		t.Fatalf("gpu limit = %q", q.String())
	}

	if _, err := workerResources(filament.WorkerResources{Limits: map[string]string{"cpu": "lots"}}); err == nil {
		t.Fatal("want error for an unparseable quantity")
	}
}

func TestJobAndPodLabelsMatch(t *testing.T) {
	m := &Module{cfg: Config{WorkerImage: "worker:test", WorkerSecretName: "filament-secret", JobNamePrefix: "filament"}}
	job, err := m.jobForSpec(filament.RunSpec{
		Tenant: "acme", Run: "run-1", ExecutionID: "attempt-1",
		SourceConnectionID: "source-connection", SinkConnectionID: "sink-connection",
	})
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
	if got := pod["filament.galaxy.io/execution-id"]; got != executionToken("attempt-1") {
		t.Fatalf("pod execution-id label = %q", got)
	}
	if got := pod["filament.galaxy.io/source-connection-id"]; got != "source-connection" {
		t.Fatalf("pod source connection label = %q", got)
	}
	if got := pod["filament.galaxy.io/sink-connection-id"]; got != "sink-connection" {
		t.Fatalf("pod sink connection label = %q", got)
	}
}

func TestInvalidConnectionIDUsesStableLabelToken(t *testing.T) {
	value := "connection/id/that/is/not/a/kubernetes/label/value"
	got := k8sLabelValue(value)
	if got != executionToken(value) {
		t.Fatalf("label value = %q, want stable token", got)
	}
}

func TestJobNameIsStablePerExecutionAndChangesOnResume(t *testing.T) {
	first := jobName("filament", "run-1", "attempt-1")
	redelivery := jobName("filament", "run-1", "attempt-1")
	resumed := jobName("filament", "run-1", "attempt-2")
	if first != redelivery {
		t.Fatalf("redelivery name = %q, want %q", redelivery, first)
	}
	if first == resumed {
		t.Fatalf("resumed execution reused job name %q", first)
	}
	if !isDNS1123Label(resumed) {
		t.Fatalf("job name %q is not DNS-1123", resumed)
	}
}

// jobForSpec carries the run's resolved configuration onto the container, and
// refuses the run rather than silently sizing it wrong.
func TestJobForSpecResources(t *testing.T) {
	m := &Module{cfg: Config{WorkerImage: "worker:test", WorkerSecretName: "filament-secret", JobNamePrefix: "filament"}}
	spec := filament.RunSpec{
		Tenant: "acme", Run: "run-1",
		WorkerConfiguration: filament.WorkerConfiguration{
			Resources: filament.WorkerResources{Limits: map[string]string{"cpu": "2", "memory": "8Gi"}},
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

	spec.WorkerConfiguration.Resources.Limits["memory"] = "8 gigabytes"
	if _, err := m.jobForSpec(spec); err == nil {
		t.Fatal("want error for an unparseable stored quantity")
	}
}

func TestJobForSpecRestrictedSecurity(t *testing.T) {
	m := &Module{cfg: Config{WorkerImage: "worker:test", WorkerSecretName: "filament-secret", JobNamePrefix: "filament"}}
	job, err := m.jobForSpec(filament.RunSpec{Tenant: "acme", Run: "run-1"})
	if err != nil {
		t.Fatal(err)
	}
	pod := job.Spec.Template.Spec
	if pod.AutomountServiceAccountToken == nil || *pod.AutomountServiceAccountToken {
		t.Fatal("worker pod must not mount a service account token")
	}
	if pod.SecurityContext == nil || pod.SecurityContext.RunAsNonRoot == nil || !*pod.SecurityContext.RunAsNonRoot ||
		pod.SecurityContext.SeccompProfile == nil || pod.SecurityContext.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
		t.Fatalf("pod security context = %+v, want non-root with runtime default seccomp", pod.SecurityContext)
	}
	sc := pod.Containers[0].SecurityContext
	if sc == nil || sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation ||
		sc.ReadOnlyRootFilesystem == nil || !*sc.ReadOnlyRootFilesystem ||
		sc.Capabilities == nil || !reflect.DeepEqual(sc.Capabilities.Drop, []corev1.Capability{"ALL"}) {
		t.Fatalf("container security context = %+v, want restricted profile", sc)
	}
	mounts := pod.Containers[0].VolumeMounts
	if len(mounts) != 1 || mounts[0].MountPath != "/tmp" || len(pod.Volumes) != 1 || pod.Volumes[0].EmptyDir == nil {
		t.Fatalf("worker must mount an emptyDir at /tmp for sinks that stage to disk, got mounts=%+v volumes=%+v", mounts, pod.Volumes)
	}
}

func TestJobForSpecPlacement(t *testing.T) {
	m := &Module{cfg: Config{WorkerImage: "worker:test", WorkerSecretName: "filament-secret", JobNamePrefix: "filament"}}
	spec := filament.RunSpec{Tenant: "acme", Run: "run-1"}

	job, err := m.jobForSpec(spec)
	if err != nil {
		t.Fatal(err)
	}
	if pod := job.Spec.Template.Spec; pod.NodeSelector != nil || pod.Tolerations != nil {
		t.Fatalf("unplaced run must leave placement off the Job, got selector=%v tolerations=%v", pod.NodeSelector, pod.Tolerations)
	}

	spec.WorkerConfiguration.NodeSelector = map[string]string{"tier": "workers"}
	spec.WorkerConfiguration.Tolerations = []filament.WorkerToleration{{Key: "dedicated", Operator: "Equal", Value: "filament", Effect: "NoSchedule"}}
	job, err = m.jobForSpec(spec)
	if err != nil {
		t.Fatal(err)
	}
	pod := job.Spec.Template.Spec
	if !maps.Equal(pod.NodeSelector, spec.WorkerConfiguration.NodeSelector) {
		t.Fatalf("NodeSelector = %v", pod.NodeSelector)
	}
	want := []corev1.Toleration{{Key: "dedicated", Operator: corev1.TolerationOpEqual, Value: "filament", Effect: corev1.TaintEffectNoSchedule}}
	if !reflect.DeepEqual(pod.Tolerations, want) {
		t.Fatalf("Tolerations = %+v, want %+v", pod.Tolerations, want)
	}
}
