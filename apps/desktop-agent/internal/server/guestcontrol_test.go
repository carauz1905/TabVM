package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tabvm/desktop-agent/internal/models"
	"github.com/tabvm/desktop-agent/internal/vbox"
)

const guestVmID = "11111111-1111-1111-1111-111111111111"

func TestGuestRunEndpointReturnsExitCodeAndOutput(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	fake.guestRun = models.VmGuestRunResponse{
		Success:  true,
		VMID:     guestVmID,
		ExitCode: 0,
		Output:   "hello\n",
		Message:  "Command finished with exit code 0.",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/vms/"+guestVmID+"/guest/run",
		strings.NewReader(`{"exe":"/bin/echo","args":["hello"],"username":"root","password":"secret"}`))
	req.Header.Set("X-TabVM-Session-Token", "secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (body %q)", http.StatusOK, rr.Code, rr.Body.String())
	}
	if fake.lastAction != "runInGuest" || fake.lastID != guestVmID {
		t.Fatalf("expected runInGuest on %s, got %s on %s", guestVmID, fake.lastAction, fake.lastID)
	}
	if fake.lastGuestExe != "/bin/echo" || len(fake.lastGuestArgs) != 1 || fake.lastGuestArgs[0] != "hello" {
		t.Fatalf("guest run payload not forwarded: exe=%s args=%v", fake.lastGuestExe, fake.lastGuestArgs)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `"exitCode":0`) || !strings.Contains(body, "hello") {
		t.Fatalf("expected exit code and output in body, got %q", body)
	}
}

func TestGuestRunEndpointPromptsForMissingCredentials(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	fake.guestRun = models.VmGuestRunResponse{
		Success:             false,
		VMID:                guestVmID,
		CredentialsRequired: true,
		Message:             "Running a command in the guest needs the guest username and password.",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/vms/"+guestVmID+"/guest/run",
		strings.NewReader(`{"exe":"/bin/ls"}`))
	req.Header.Set("X-TabVM-Session-Token", "secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (body %q)", http.StatusOK, rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"credentialsRequired":true`) {
		t.Fatalf("expected credentialsRequired=true, got %q", rr.Body.String())
	}
}

func TestGuestRunEndpointRejectsInvalidBody(t *testing.T) {
	srv, _ := newTestServer(t, "secret")

	req := httptest.NewRequest(http.MethodPost, "/api/vms/"+guestVmID+"/guest/run",
		strings.NewReader(`{"exe":"/bin/ls","bogus":true}`))
	req.Header.Set("X-TabVM-Session-Token", "secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestGuestRunEndpointMapsValidationTo400(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	fake.guestRunErr = &vbox.ValidationError{Message: "The guest command must be an absolute path (for example /bin/ls)."}

	req := httptest.NewRequest(http.MethodPost, "/api/vms/"+guestVmID+"/guest/run",
		strings.NewReader(`{"exe":"ls","username":"root","password":"secret"}`))
	req.Header.Set("X-TabVM-Session-Token", "secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestGuestCopyFromEndpointReturnsHostPath(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	fake.guestCopyFrom = models.VmGuestCopyFromResponse{
		Success:  true,
		VMID:     guestVmID,
		HostPath: `C:\dst\report.txt`,
		Message:  `Copied "report.txt" from the guest to C:\dst\report.txt.`,
	}

	req := httptest.NewRequest(http.MethodPost, "/api/vms/"+guestVmID+"/guest/copyfrom",
		strings.NewReader(`{"guestPath":"/home/root/report.txt","directory":"C:\\dst","username":"root","password":"secret"}`))
	req.Header.Set("X-TabVM-Session-Token", "secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (body %q)", http.StatusOK, rr.Code, rr.Body.String())
	}
	if fake.lastAction != "copyFromGuest" || fake.lastID != guestVmID {
		t.Fatalf("expected copyFromGuest on %s, got %s on %s", guestVmID, fake.lastAction, fake.lastID)
	}
	if fake.lastGuestPath != "/home/root/report.txt" || fake.lastExportDir != `C:\dst` {
		t.Fatalf("copyfrom payload not forwarded: guestPath=%s dir=%s", fake.lastGuestPath, fake.lastExportDir)
	}
	if !strings.Contains(rr.Body.String(), `report.txt`) {
		t.Fatalf("expected the written host path in body, got %q", rr.Body.String())
	}
}

func TestGuestCopyFromEndpointPromptsForMissingCredentials(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	fake.guestCopyFrom = models.VmGuestCopyFromResponse{
		Success:             false,
		VMID:                guestVmID,
		CredentialsRequired: true,
		Message:             "Copying a file out of the guest needs the guest username and password.",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/vms/"+guestVmID+"/guest/copyfrom",
		strings.NewReader(`{"guestPath":"/home/root/report.txt","directory":"C:\\dst"}`))
	req.Header.Set("X-TabVM-Session-Token", "secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (body %q)", http.StatusOK, rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"credentialsRequired":true`) {
		t.Fatalf("expected credentialsRequired=true, got %q", rr.Body.String())
	}
}

func TestGuestCopyFromEndpointMapsValidationTo400(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	fake.guestCopyFromErr = &vbox.ValidationError{Message: "The destination directory must be an absolute path."}

	req := httptest.NewRequest(http.MethodPost, "/api/vms/"+guestVmID+"/guest/copyfrom",
		strings.NewReader(`{"guestPath":"/home/root/report.txt","directory":"relative","username":"root","password":"secret"}`))
	req.Header.Set("X-TabVM-Session-Token", "secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestGuestControlEndpointsConflictWhenVmLocked(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	unlock, ok := srv.tryLockVm(guestVmID)
	if !ok {
		t.Fatal("expected to acquire the VM lock")
	}
	defer unlock()

	cases := map[string]string{
		"/api/vms/" + guestVmID + "/guest/run":      `{"exe":"/bin/ls","username":"root","password":"secret"}`,
		"/api/vms/" + guestVmID + "/guest/copyfrom": `{"guestPath":"/home/root/a.txt","directory":"C:\\dst","username":"root","password":"secret"}`,
	}
	for path, body := range cases {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("X-TabVM-Session-Token", "secret")
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rr, req)

		if rr.Code != http.StatusConflict {
			t.Fatalf("expected status %d for %s, got %d", http.StatusConflict, path, rr.Code)
		}
	}
	if fake.lastAction == "runInGuest" || fake.lastAction == "copyFromGuest" {
		t.Fatal("guest control must not run while the VM is locked")
	}
}

func TestGuestControlRoutesRequireAuth(t *testing.T) {
	srv, _ := newTestServer(t, "secret")

	for _, path := range []string{"/api/vms/" + guestVmID + "/guest/run", "/api/vms/" + guestVmID + "/guest/copyfrom"} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for %s without a token, got %d", path, rr.Code)
		}
	}
}
