package server

import (
	"context"
	"testing"
	"time"

	"github.com/tabvm/desktop-agent/internal/models"
)

// The agent runs as a long-lived tray process, and nothing ever removed a job
// entry, so one accumulated per import, create, clone and export for the whole
// life of the process.
func TestFinishedCreateJobsAreEvicted(t *testing.T) {
	srv, _ := newTestServer(t, "secret")
	now := time.Now()

	srv.createJobs["long-finished"] = &createJob{State: "done", endedAt: now.Add(-2 * createJobRetention)}
	srv.createJobs["just-finished"] = &createJob{State: "error", endedAt: now}
	srv.createJobs["still-running"] = &createJob{State: "running"}

	srv.createMu.Lock()
	srv.sweepFinishedJobsLocked(now)
	srv.createMu.Unlock()

	if _, ok := srv.createJobs["long-finished"]; ok {
		t.Error("a job finished beyond the retention window should have been evicted")
	}
	if _, ok := srv.createJobs["just-finished"]; !ok {
		t.Error("a job that just finished must stay queryable so the UI can read its result")
	}
	if _, ok := srv.createJobs["still-running"]; !ok {
		t.Error("a running job must never be evicted, however long it runs")
	}
}

// Eviction only works if terminal states are actually stamped.
func TestCompletedJobRecordsWhenItEnded(t *testing.T) {
	srv, _ := newTestServer(t, "secret")

	jobID := srv.startCreateJob("vm", func(ctx context.Context) (models.VmCreateResponse, error) {
		return models.VmCreateResponse{Success: true, Message: "done"}, nil
	})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		srv.createMu.Lock()
		job := srv.createJobs[jobID]
		state, ended := job.State, job.endedAt
		srv.createMu.Unlock()

		if state == "done" {
			if ended.IsZero() {
				t.Fatal("a finished job must record when it ended")
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("job did not finish in time")
}
