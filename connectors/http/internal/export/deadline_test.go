package export

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/galaxy-io/filament/connectors/http/manifest"
	"github.com/galaxy-io/filament/connectors/http/template"
)

func deadlineFixture(t *testing.T, phases ...string) (*Runtime, manifest.Resource, RunState) {
	t.Helper()
	c := &Runtime{}
	res := manifest.Resource{Name: "items", Parent: &manifest.ParentRef{Concurrency: 1}, Export: &manifest.ExportSpec{
		ParentKey: []string{"id"},
		Start:     manifest.ExportStart{ExportRequest: manifest.ExportRequest{Method: "POST", Path: "/exports"}, Capture: map[string]string{"id": "$.id"}},
		Wait: manifest.ExportWait{
			Type: "job", TimeoutSeconds: 30, Request: &manifest.ExportRequest{Method: "GET", Path: "/status"},
			State: &manifest.ExportJobState{Path: "$.status", Ready: []string{"ready"}, Pending: []string{"pending"}},
		},
	}}
	state := c.emptyExportRun(res, false)
	parents := make([]map[string]string, len(phases))
	for i := range phases {
		parents[i] = map[string]string{"id": fmt.Sprint(i)}
	}
	if err := c.initializeExportParents(res, &state, parents); err != nil {
		t.Fatal(err)
	}
	for i, key := range state.Selected {
		entry := state.Parents[key]
		entry.Committed = "committed"
		entry.Job.Phase = phases[i]
		entry.Job.Deadline = time.Now().Add(-time.Hour)
		entry.Job.Seed, entry.Job.Overlap = "seed", 60
		entry.Job.Captures = map[string]string{"id": "saved-job"}
		entry.Job.Candidate = "candidate"
		state.Parents[key] = entry
	}
	return c, res, state
}

func TestPlanRenewsOnlyExpiredReplayDeadlines(t *testing.T) {
	c, res, state := deadlineFixture(t, "waiting", "done", "creating", "waiting")
	state.Incremental = true
	state.Parents[state.Selected[3]].Job.Deadline = time.Now().Add(time.Hour)
	cp, err := Checkpoint(res.Name, state)
	if err != nil {
		t.Fatal(err)
	}
	planned, err := c.PlanIncremental(res, cp)
	if err != nil {
		t.Fatal(err)
	}
	next, err := c.PlanState(res, planned, true)
	if err != nil {
		t.Fatal(err)
	}
	for key, before := range state.Parents {
		after := next.Parents[key]
		if before.Job.Phase != "creating" && before.Job.Deadline.Before(time.Now()) {
			if !after.Job.Deadline.After(time.Now()) {
				t.Fatal("expired replay deadline was not renewed")
			}
		} else if !after.Job.Deadline.Equal(before.Job.Deadline) {
			t.Fatal("creation or unexpired deadline changed")
		}
		job := *after.Job
		job.Deadline = before.Job.Deadline
		after.Job = &job
		if !reflect.DeepEqual(before, after) {
			t.Fatal("recovery inputs or committed progress changed")
		}
	}
}

type deadlineControl struct{ starts, polls int }

var errDeadlinePollReached = errors.New("saved job reached polling")

func (c *deadlineControl) Start(context.Context, manifest.Resource, template.Scope) ([]byte, error) {
	c.starts++
	return nil, errors.New("unexpected creation")
}

func (c *deadlineControl) Poll(ctx context.Context, _ string, _ manifest.ExportRequest, scope template.Scope) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if scope.Job["id"] != "saved-job" {
		return nil, errors.New("lost saved job identity")
	}
	c.polls++
	return nil, errDeadlinePollReached
}

func TestDirectResumeRenewsExpiredReplayDeadline(t *testing.T) {
	for _, phase := range []string{"waiting", "done", "creating"} {
		t.Run(phase, func(t *testing.T) {
			c, res, state := deadlineFixture(t, phase)
			var err error
			c.ResumeToken, err = state.token()
			if err != nil {
				t.Fatal(err)
			}
			control := &deadlineControl{}
			c.Control = control
			err = c.Run(t.Context(), res, rejectingSink{}, nil)
			if phase == "creating" {
				if err == nil || !strings.Contains(err.Error(), "reconcile") || control.starts != 0 || control.polls != 0 {
					t.Fatalf("ambiguous creation retried: %v", err)
				}
				return
			}
			if !errors.Is(err, errDeadlinePollReached) || control.starts != 0 || control.polls != 1 {
				t.Fatalf("saved job did not reach polling: starts=%d polls=%d err=%v", control.starts, control.polls, err)
			}
		})
	}
}
