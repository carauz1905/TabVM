package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tabvm/desktop-agent/internal/models"
	"github.com/tabvm/desktop-agent/internal/vbox"
)

func TestExportEndpointStartsJobAndSucceeds(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	fake.createResp = models.VmCreateResponse{
		Success: true,
		VMID:    cloneSourceID,
		Message: `Exported to C:\out\lab-vm.ova`,
	}

	req := httptest.NewRequest(http.MethodPost, "/api/vms/"+cloneSourceID+"/export",
		strings.NewReader(`{"directory":"C:\\out"}`))
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
	if fake.lastAction != "exportVm" || fake.lastExportDir != `C:\out` {
		t.Fatalf("export payload not forwarded: action=%s dir=%s", fake.lastAction, fake.lastExportDir)
	}
	// The written .ova path flows back to the client through the status message.
	if !strings.Contains(status.Message, "lab-vm.ova") {
		t.Fatalf("expected the written path in the status message, got %q", status.Message)
	}
}

func TestExportEndpointRejectsRunningSource(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	// A running source is rejected by ValidateExport with a ValidationError, which
	// the server maps to a 400 before any job is started.
	fake.exportValidateErr = &vbox.ValidationError{Message: "The VM is running. Power it off before exporting it."}

	req := httptest.NewRequest(http.MethodPost, "/api/vms/"+cloneSourceID+"/export",
		strings.NewReader(`{"directory":"C:\\out"}`))
	req.Header.Set("X-TabVM-Session-Token", "secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d (body %q)", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
	if fake.lastAction == "exportVm" {
		t.Fatal("ExportVM must not run when validation fails")
	}
}

func TestExportEndpointRejectsInvalidDirectory(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	// A relative/invalid directory is rejected by ValidateExport with a
	// ValidationError, which the server maps to a 400 before any job is started.
	fake.exportValidateErr = &vbox.ValidationError{Message: "The destination directory must be an absolute path."}

	req := httptest.NewRequest(http.MethodPost, "/api/vms/"+cloneSourceID+"/export",
		strings.NewReader(`{"directory":"relative/dir"}`))
	req.Header.Set("X-TabVM-Session-Token", "secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d (body %q)", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
	if fake.lastAction == "exportVm" {
		t.Fatal("ExportVM must not run when validation fails")
	}
}

func TestExportEndpointRejectsInvalidBody(t *testing.T) {
	srv, _ := newTestServer(t, "secret")

	req := httptest.NewRequest(http.MethodPost, "/api/vms/"+cloneSourceID+"/export",
		strings.NewReader(`{"directory":"C:\\out","bogus":true}`))
	req.Header.Set("X-TabVM-Session-Token", "secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d (body %q)", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestExportEndpointConflictsWhenVmLocked(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	// Hold the per-VM lifecycle lock so a concurrent export must refuse to start.
	unlock, ok := srv.tryLockVm(cloneSourceID)
	if !ok {
		t.Fatal("expected to acquire the VM lock")
	}
	defer unlock()

	req := httptest.NewRequest(http.MethodPost, "/api/vms/"+cloneSourceID+"/export",
		strings.NewReader(`{"directory":"C:\\out"}`))
	req.Header.Set("X-TabVM-Session-Token", "secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d (body %q)", http.StatusConflict, rr.Code, rr.Body.String())
	}
	if fake.lastAction == "exportVm" {
		t.Fatal("ExportVM must not run while the VM is locked")
	}
}

func TestExportRouteRequiresAuth(t *testing.T) {
	srv, _ := newTestServer(t, "secret")

	req := httptest.NewRequest(http.MethodPost, "/api/vms/"+cloneSourceID+"/export",
		strings.NewReader(`{"directory":"C:\\out"}`))
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}
