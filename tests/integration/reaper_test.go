//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/memory"
	"github.com/galaxy-io/filament/eventbus/host"
	"github.com/galaxy-io/filament/eventbus/inproc"
	"github.com/galaxy-io/filament/internal/modules/dispatch/k8s"
	"github.com/galaxy-io/filament/internal/modules/reaper"
	"github.com/galaxy-io/filament/internal/modules/tracker"
	"github.com/galaxy-io/filament/module"
	"github.com/galaxy-io/filament/tests/testcontainers"
)

// TestReaperKillsStaleRuns drives the reaper against a real API server. A
// stale Running run with no worker Job is failed through the bus and folded
// terminal by the tracker; one whose Job is still unfinished is held by the
// workload probe; deleting that Job lets the next sweep kill it too.
func TestReaperKillsStaleRuns(t *testing.T) {
	cluster := testcontainers.SharedK3s(t)
	ctx := context.Background()

	const namespace = "default"
	ds := memory.New()
	bus := inproc.New()

	dispatcher := k8s.New(k8s.Config{
		Namespace:        namespace,
		WorkerImage:      "ghcr.io/galaxy-io/filament/worker:test",
		WorkerSecretName: "filament-secret",
		// The default ServiceAccount, so the Job controller can actually create
		// the pod; a pod stuck pulling a nonexistent image keeps the Job
		// unfinished, which is what holds the run.
		WorkerServiceAccount: "default",
		JobNamePrefix:        "filament",
		Kubeconfig:           cluster.KubeconfigPath,
		BackoffLimit:         1,
	})
	if err := dispatcher.Mount(ctx, module.Deps{DataStore: ds}); err != nil {
		t.Fatalf("mount dispatcher: %v", err)
	}

	dead := filament.RunID("run-reaper-dead")
	held := filament.RunID("run-reaper-held")
	for _, id := range []filament.RunID{dead, held} {
		if err := ds.SaveRun(ctx, filament.RunState{
			Run:     id,
			Tenant:  "acme",
			Status:  filament.RunRunning,
			Request: filament.RunRequest{PipelineID: "pl-reaper"},
		}); err != nil {
			t.Fatalf("seed run %s: %v", id, err)
		}
	}

	if _, err := dispatcher.Dispatch(ctx, filament.RunSpec{
		Tenant: "acme",
		Run:    held,
		Source: filament.Ref{Connector: "postgres"},
		Sink:   filament.Ref{Connector: "stdout"},
	}); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	jobName := waitJobActive(t, ctx, cluster, namespace, held)

	reap := reaper.New(
		reaper.WithInterval(250*time.Millisecond),
		reaper.WithStaleAfter(2*time.Second),
		reaper.WithWorkloadProbe(dispatcher.Workload),
	)
	mods, err := module.MountAll(ctx, module.Deps{Bus: bus, DataStore: ds}, tracker.New(), reap)
	if err != nil {
		t.Fatalf("mount: %v", err)
	}
	h := host.New(bus)
	if err := h.Run(ctx, mods...); err != nil {
		t.Fatalf("run: %v", err)
	}
	defer func() { _ = h.Close(); _ = bus.Close() }()
	sweepCtx, stopSweeps := context.WithCancel(ctx)
	defer stopSweeps()
	reap.Start(sweepCtx)

	killCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	state := waitStatus(t, killCtx, ds, dead, filament.RunFailed)
	if !strings.HasPrefix(state.Error, "reaped:") {
		t.Errorf("dead run error = %q, want reaped prefix", state.Error)
	}

	// Several more sweeps pass; the held run stays Running because its Job
	// is still unfinished.
	time.Sleep(time.Second)
	if st, err := ds.LoadRun(ctx, held); err != nil || st.Status != filament.RunRunning {
		t.Fatalf("held run: status %v err %v, want still running", st.Status, err)
	}

	fg := metav1.DeletePropagationForeground
	if err := cluster.Clientset.BatchV1().Jobs(namespace).Delete(ctx, jobName, metav1.DeleteOptions{PropagationPolicy: &fg}); err != nil {
		t.Fatalf("delete job: %v", err)
	}
	killCtx2, cancel2 := context.WithTimeout(ctx, 60*time.Second)
	defer cancel2()
	state = waitStatus(t, killCtx2, ds, held, filament.RunFailed)
	if !strings.HasPrefix(state.Error, "reaped:") {
		t.Errorf("held run error = %q, want reaped prefix", state.Error)
	}
}

// waitJobActive polls until the run's Job reports an active pod, returning the
// Job name.
func waitJobActive(t *testing.T, ctx context.Context, cluster *testcontainers.K3s, namespace string, run filament.RunID) string {
	t.Helper()
	waitCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	for {
		jobs, err := cluster.Clientset.BatchV1().Jobs(namespace).List(waitCtx, metav1.ListOptions{
			LabelSelector: "filament.galaxy.io/run-id=" + string(run),
		})
		if err == nil && len(jobs.Items) == 1 && jobs.Items[0].Status.Active > 0 {
			return jobs.Items[0].Name
		}
		select {
		case <-waitCtx.Done():
			t.Fatalf("job for run %s never reported an active pod: %v", run, err)
		case <-time.After(250 * time.Millisecond):
		}
	}
}
