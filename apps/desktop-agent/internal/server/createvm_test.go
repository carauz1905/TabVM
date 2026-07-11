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
)

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
