package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tabvm/desktop-agent/internal/config"
	"github.com/tabvm/desktop-agent/internal/models"
	"github.com/tabvm/desktop-agent/internal/vbox"
)

const cloneSourceID = "11111111-1111-1111-1111-111111111111"

// pollJobUntil polls the shared create-status endpoint until the job leaves the
// running state or the deadline passes, returning the final status.
func pollJobUntil(t *testing.T, srv *Server, jobID string) models.VmCreateStatusResponse {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		sreq := httptest.NewRequest(http.MethodGet, "/api/vms/create/status?job="+jobID, nil)
		sreq.Header.Set("X-TabVM-Session-Token", "secret")
		srr := httptest.NewRecorder()
		srv.Handler().ServeHTTP(srr, sreq)
		if srr.Code != http.StatusOK {
			t.Fatalf("status poll failed: %d (body %q)", srr.Code, srr.Body.String())
		}
		var status models.VmCreateStatusResponse
		if err := json.Unmarshal(srr.Body.Bytes(), &status); err != nil {
			t.Fatalf("bad status body %q: %v", srr.Body.String(), err)
		}
		if status.State != "running" {
			return status
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for the clone job to resolve")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestCloneEndpointStartsJobAndSucceeds(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	fake.createResp = models.VmCreateResponse{
		Success: true,
		VMID:    "22222222-2222-2222-2222-222222222222",
		Name:    "lab-clone",
		Message: "Full clone \"lab-clone\" created and registered.",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/vms/"+cloneSourceID+"/clone",
		strings.NewReader(`{"name":"lab-clone","linked":false}`))
	req.Header.Set("X-TabVM-Session-Token", "secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d (body %q)", http.StatusAccepted, rr.Code, rr.Body.String())
	}
	var job models.VmCreateJobResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &job); err != nil || job.JobID == "" {
		t.Fatalf("expected a job id, got %q (err %v)", rr.Body.String(), err)
	}

	status := pollJobUntil(t, srv, job.JobID)
	if status.State != "done" {
		t.Fatalf("expected job done, got %q (%q)", status.State, status.Message)
	}
	if fake.lastAction != "cloneVm" || fake.lastCloneSourceID != cloneSourceID || fake.lastCloneName != "lab-clone" || fake.lastCloneLinked {
		t.Fatalf("clone payload not forwarded: action=%s src=%s name=%s linked=%v",
			fake.lastAction, fake.lastCloneSourceID, fake.lastCloneName, fake.lastCloneLinked)
	}
}

func TestCloneEndpointForwardsLinkedFlag(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	fake.createResp = models.VmCreateResponse{Success: true, VMID: "22222222-2222-2222-2222-222222222222", Name: "lab-clone"}

	req := httptest.NewRequest(http.MethodPost, "/api/vms/"+cloneSourceID+"/clone",
		strings.NewReader(`{"name":"lab-clone","linked":true}`))
	req.Header.Set("X-TabVM-Session-Token", "secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d (body %q)", http.StatusAccepted, rr.Code, rr.Body.String())
	}
	var job models.VmCreateJobResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &job)
	_ = pollJobUntil(t, srv, job.JobID)
	if !fake.lastCloneLinked {
		t.Fatal("expected the linked flag to be forwarded to CloneVM")
	}
}

func TestCloneEndpointRejectsRunningSource(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	// A running source is rejected by ValidateClone with a ValidationError, which
	// the server maps to a 400 before any job is started.
	fake.cloneValidateErr = &vbox.ValidationError{Message: "The VM is running. Power it off before cloning it."}

	req := httptest.NewRequest(http.MethodPost, "/api/vms/"+cloneSourceID+"/clone",
		strings.NewReader(`{"name":"lab-clone","linked":false}`))
	req.Header.Set("X-TabVM-Session-Token", "secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d (body %q)", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
	if fake.lastAction == "cloneVm" {
		t.Fatal("CloneVM must not run when validation fails")
	}
}

func TestCloneEndpointRejectsInvalidBody(t *testing.T) {
	srv, _ := newTestServer(t, "secret")

	req := httptest.NewRequest(http.MethodPost, "/api/vms/"+cloneSourceID+"/clone",
		strings.NewReader(`{"name":"lab-clone","bogus":true}`))
	req.Header.Set("X-TabVM-Session-Token", "secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d (body %q)", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestCloneJobRecordsFailure(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	// Validation passes, but the clone itself fails inside the background job.
	fake.createErr = &vbox.ExecutionError{ExitCode: 1, Message: "clone failed"}

	req := httptest.NewRequest(http.MethodPost, "/api/vms/"+cloneSourceID+"/clone",
		strings.NewReader(`{"name":"lab-clone","linked":false}`))
	req.Header.Set("X-TabVM-Session-Token", "secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d (body %q)", http.StatusAccepted, rr.Code, rr.Body.String())
	}
	var job models.VmCreateJobResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &job)
	status := pollJobUntil(t, srv, job.JobID)
	if status.State != "error" {
		t.Fatalf("expected job error, got %q", status.State)
	}
	// The execution failure is mapped to a safe, generic message.
	if status.Message != "VirtualBox operation failed." {
		t.Fatalf("expected a sanitized failure message, got %q", status.Message)
	}
}

func TestCloneRouteRequiresAuth(t *testing.T) {
	srv, _ := newTestServer(t, "secret")

	req := httptest.NewRequest(http.MethodPost, "/api/vms/"+cloneSourceID+"/clone",
		strings.NewReader(`{"name":"lab-clone","linked":false}`))
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

// panickingVboxService wraps the shared fake and panics on a manual create, to
// prove a background create job survives a panic in the service layer instead
// of killing the whole agent process.
type panickingVboxService struct {
	fakeVboxService
}

func (p *panickingVboxService) CreateVmManual(ctx context.Context, req models.VmCreateManualRequest) (models.VmCreateResponse, error) {
	panic("boom: internal service defect")
}

func TestCreateJobRecoversFromServicePanic(t *testing.T) {
	cfg := &config.Agent{
		BindAddress:  "127.0.0.1",
		BindPort:     5230,
		SessionToken: "secret",
		Environment:  "Development",
	}
	srv := New(cfg, &panickingVboxService{}, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/vms/create-manual",
		strings.NewReader(`{"name":"alpine","osType":"Linux_64","isoPath":"C:\\iso\\alpine.iso","memoryMb":2048,"cpus":2,"diskGb":20}`))
	req.Header.Set("X-TabVM-Session-Token", "secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d (body %q)", http.StatusAccepted, rr.Code, rr.Body.String())
	}
	var job models.VmCreateJobResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &job); err != nil || job.JobID == "" {
		t.Fatalf("expected a job id, got %q (err %v)", rr.Body.String(), err)
	}

	// The panic happens on the background goroutine; the job must end in the
	// error state with a generic message and the process must survive.
	deadline := time.Now().Add(3 * time.Second)
	for {
		sreq := httptest.NewRequest(http.MethodGet, "/api/vms/create/status?job="+job.JobID, nil)
		sreq.Header.Set("X-TabVM-Session-Token", "secret")
		srr := httptest.NewRecorder()
		srv.Handler().ServeHTTP(srr, sreq)
		if srr.Code != http.StatusOK {
			t.Fatalf("status poll failed: %d (body %q)", srr.Code, srr.Body.String())
		}
		var status models.VmCreateStatusResponse
		if err := json.Unmarshal(srr.Body.Bytes(), &status); err != nil {
			t.Fatalf("bad status body %q: %v", srr.Body.String(), err)
		}
		if status.State == "error" {
			// The message must be generic: internals (the panic value) must
			// never leak to the client.
			if status.Message != "Internal server error." {
				t.Fatalf("expected a generic error message, got %q", status.Message)
			}
			break
		}
		if status.State == "done" {
			t.Fatal("job unexpectedly succeeded")
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for the panicked job to resolve")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
