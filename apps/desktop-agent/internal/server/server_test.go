package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tabvm/desktop-agent/internal/config"
	"github.com/tabvm/desktop-agent/internal/models"
	"github.com/tabvm/desktop-agent/internal/store"
	"github.com/tabvm/desktop-agent/internal/updatecheck"
	"github.com/tabvm/desktop-agent/internal/vbox"
)

type fakeVboxService struct {
	discovery         models.VirtualBoxDiscovery
	vms               models.VmListResponse
	listErr           error
	status            models.VmStatusResponse
	statusErr         error
	startErr          error
	stopErr           error
	resetErr          error
	poweroffErr       error
	consoleStatus     models.VmConsoleStatusResponse
	consoleStatusErr  error
	prepareConsole    models.VmConsoleStatusResponse
	prepareConsoleErr error
	disableConsoleErr error
	telemetry         models.VmTelemetryResponse
	telemetryErr      error
	sharedFolders     models.SharedFoldersResponse
	sharedFoldersErr  error
	sharedFolderOp    models.SharedFolderOperationResponse
	sharedFolderOpErr error
	clipboard         models.ClipboardModeResponse
	clipboardErr      error
	gaStatus          models.GuestAdditionsStatusResponse
	gaStatusErr       error
	gaInstall         models.GuestAdditionsInstallResponse
	gaInstallErr      error
	gaUpdate          models.GuestAdditionsUpdateResponse
	gaUpdateErr       error
	fileTransfer      models.VmFileTransferResponse
	fileTransferErr   error
	snapshots         models.SnapshotsResponse
	snapshotsErr      error
	snapshotOp        models.SnapshotOperationResponse
	snapshotOpErr     error
	networkOptions    models.NetworkOptionsResponse
	networkOptionsErr error
	networkOp         models.NetworkOperationResponse
	networkOpErr      error
	portForwardOp     models.NetworkOperationResponse
	portForwardErr    error
	lastForwardReq    models.PortForwardingRequest
	lastForwardSlot   int
	lastForwardName   string
	createResp        models.VmCreateResponse
	createErr         error
	deleteResp        models.VmOperationResponse
	deleteErr         error
	hardware          models.VmHardwareResponse
	hardwareErr       error
	guestOS           models.VmGuestOSResponse
	guestOSErr        error
	serialStatus      models.VmSerialConsoleResponse
	serialStatusErr   error
	serialEnableResp  models.VmOperationResponse
	serialEnableErr   error
	serialDisableResp models.VmOperationResponse
	serialDisableErr  error
	serialGettyResp   models.SerialGettyResponse
	serialGettyErr    error
	setHardwareResp   models.VmOperationResponse
	setHardwareErr    error
	lastCPUs          int
	lastMemoryMB      int
	storage           models.VmStorageResponse
	storageErr        error
	resizeResp        models.VmOperationResponse
	resizeErr         error
	lastUUID          string
	lastSizeMB        int64
	lastDeleteFile    bool
	lastName          string
	lastHostPath      string
	lastMode          string
	lastAction        string
	lastID            string
	lastManualReq     models.VmCreateManualRequest
	startBlocker      chan struct{}
	startEntered      chan struct{}
	startEnteredOnce  sync.Once
}

func (f *fakeVboxService) Discover(ctx context.Context) models.VirtualBoxDiscovery {
	return f.discovery
}

func (f *fakeVboxService) ListVMs(ctx context.Context) (models.VmListResponse, error) {
	return f.vms, f.listErr
}

func (f *fakeVboxService) VMStatus(ctx context.Context, id string) (models.VmStatusResponse, error) {
	f.lastAction = "status"
	f.lastID = id
	return f.status, f.statusErr
}

func (f *fakeVboxService) StartVM(ctx context.Context, id string) error {
	f.lastAction = "start"
	f.lastID = id
	if f.startEntered != nil {
		f.startEnteredOnce.Do(func() { close(f.startEntered) })
	}
	if f.startBlocker != nil {
		<-f.startBlocker
	}
	return f.startErr
}

func (f *fakeVboxService) StopVM(ctx context.Context, id string) error {
	f.lastAction = "stop"
	f.lastID = id
	return f.stopErr
}

func (f *fakeVboxService) ResetVM(ctx context.Context, id string) error {
	f.lastAction = "reset"
	f.lastID = id
	return f.resetErr
}

func (f *fakeVboxService) VmConsoleStatus(ctx context.Context, id string) (models.VmConsoleStatusResponse, error) {
	f.lastAction = "consoleStatus"
	f.lastID = id
	return f.consoleStatus, f.consoleStatusErr
}

func (f *fakeVboxService) PrepareVmConsole(ctx context.Context, id string) (models.VmConsoleStatusResponse, error) {
	f.lastAction = "prepareConsole"
	f.lastID = id
	return f.prepareConsole, f.prepareConsoleErr
}

func (f *fakeVboxService) DisableVmConsole(ctx context.Context, id string) error {
	f.lastAction = "disableConsole"
	f.lastID = id
	return f.disableConsoleErr
}

func (f *fakeVboxService) VmTelemetry(ctx context.Context, id string) (models.VmTelemetryResponse, error) {
	f.lastAction = "telemetry"
	f.lastID = id
	return f.telemetry, f.telemetryErr
}

func (f *fakeVboxService) ListSharedFolders(ctx context.Context, id string) (models.SharedFoldersResponse, error) {
	f.lastAction = "listSharedFolders"
	f.lastID = id
	return f.sharedFolders, f.sharedFoldersErr
}

func (f *fakeVboxService) AddSharedFolder(ctx context.Context, id, name, hostPath string) (models.SharedFolderOperationResponse, error) {
	f.lastAction = "addSharedFolder"
	f.lastID = id
	f.lastName = name
	f.lastHostPath = hostPath
	return f.sharedFolderOp, f.sharedFolderOpErr
}

func (f *fakeVboxService) RemoveSharedFolder(ctx context.Context, id, name string) (models.SharedFolderOperationResponse, error) {
	f.lastAction = "removeSharedFolder"
	f.lastID = id
	f.lastName = name
	return f.sharedFolderOp, f.sharedFolderOpErr
}

func (f *fakeVboxService) GetClipboardMode(ctx context.Context, id string) (models.ClipboardModeResponse, error) {
	f.lastAction = "getClipboard"
	f.lastID = id
	return f.clipboard, f.clipboardErr
}

func (f *fakeVboxService) SetClipboardMode(ctx context.Context, id, mode string) (models.ClipboardModeResponse, error) {
	f.lastAction = "setClipboard"
	f.lastID = id
	f.lastMode = mode
	return f.clipboard, f.clipboardErr
}

func (f *fakeVboxService) GuestAdditionsStatus(ctx context.Context, id string) (models.GuestAdditionsStatusResponse, error) {
	f.lastAction = "guestAdditionsStatus"
	f.lastID = id
	return f.gaStatus, f.gaStatusErr
}

func (f *fakeVboxService) InstallGuestAdditions(ctx context.Context, id string) (models.GuestAdditionsInstallResponse, error) {
	f.lastAction = "installGuestAdditions"
	f.lastID = id
	return f.gaInstall, f.gaInstallErr
}

func (f *fakeVboxService) UpdateGuestAdditions(ctx context.Context, id, username, password string) (models.GuestAdditionsUpdateResponse, error) {
	f.lastAction = "updateGuestAdditions"
	f.lastID = id
	return f.gaUpdate, f.gaUpdateErr
}

func (f *fakeVboxService) TransferFileToGuest(ctx context.Context, id, filename string, data []byte, username, password string) (models.VmFileTransferResponse, error) {
	f.lastAction = "transferFile"
	f.lastID = id
	return f.fileTransfer, f.fileTransferErr
}

func (f *fakeVboxService) NetworkOptions(ctx context.Context, id string) (models.NetworkOptionsResponse, error) {
	f.lastAction = "networkOptions"
	f.lastID = id
	return f.networkOptions, f.networkOptionsErr
}

func (f *fakeVboxService) ChangeNetworkMode(ctx context.Context, id string, slot int, mode, adapter string) (models.NetworkOperationResponse, error) {
	f.lastAction = "changeNetworkMode"
	f.lastID = id
	f.lastMode = mode
	return f.networkOp, f.networkOpErr
}

func (f *fakeVboxService) AddPortForwarding(ctx context.Context, id string, req models.PortForwardingRequest) (models.NetworkOperationResponse, error) {
	f.lastAction = "addPortForwarding"
	f.lastID = id
	f.lastForwardReq = req
	return f.portForwardOp, f.portForwardErr
}

func (f *fakeVboxService) DeletePortForwarding(ctx context.Context, id string, slot int, name string) (models.NetworkOperationResponse, error) {
	f.lastAction = "deletePortForwarding"
	f.lastID = id
	f.lastForwardSlot = slot
	f.lastForwardName = name
	return f.portForwardOp, f.portForwardErr
}

func (f *fakeVboxService) ListSnapshots(ctx context.Context, id string) (models.SnapshotsResponse, error) {
	f.lastAction = "listSnapshots"
	f.lastID = id
	return f.snapshots, f.snapshotsErr
}

func (f *fakeVboxService) TakeSnapshot(ctx context.Context, id, name, description string) (models.SnapshotOperationResponse, error) {
	f.lastAction = "takeSnapshot"
	f.lastID = id
	f.lastName = name
	return f.snapshotOp, f.snapshotOpErr
}

func (f *fakeVboxService) RestoreSnapshot(ctx context.Context, id, snapshotID string) (models.SnapshotOperationResponse, error) {
	f.lastAction = "restoreSnapshot"
	f.lastID = id
	return f.snapshotOp, f.snapshotOpErr
}

func (f *fakeVboxService) DeleteSnapshot(ctx context.Context, id, snapshotID string) (models.SnapshotOperationResponse, error) {
	f.lastAction = "deleteSnapshot"
	f.lastID = id
	return f.snapshotOp, f.snapshotOpErr
}

func (f *fakeVboxService) DeleteVM(ctx context.Context, id string) (models.VmOperationResponse, error) {
	f.lastAction = "deleteVm"
	f.lastID = id
	return f.deleteResp, f.deleteErr
}

func (f *fakeVboxService) VmHardware(ctx context.Context, id string) (models.VmHardwareResponse, error) {
	f.lastAction = "vmHardware"
	f.lastID = id
	return f.hardware, f.hardwareErr
}

func (f *fakeVboxService) VmGuestOS(ctx context.Context, id string) (models.VmGuestOSResponse, error) {
	f.lastAction = "vmGuestOS"
	f.lastID = id
	return f.guestOS, f.guestOSErr
}

func (f *fakeVboxService) SerialConsoleStatus(ctx context.Context, id string) (models.VmSerialConsoleResponse, error) {
	f.lastAction = "serialConsoleStatus"
	f.lastID = id
	return f.serialStatus, f.serialStatusErr
}

func (f *fakeVboxService) EnableSerialConsole(ctx context.Context, id string) (models.VmOperationResponse, error) {
	f.lastAction = "enableSerialConsole"
	f.lastID = id
	return f.serialEnableResp, f.serialEnableErr
}

func (f *fakeVboxService) DisableSerialConsole(ctx context.Context, id string) (models.VmOperationResponse, error) {
	f.lastAction = "disableSerialConsole"
	f.lastID = id
	return f.serialDisableResp, f.serialDisableErr
}

func (f *fakeVboxService) EnableSerialGetty(ctx context.Context, id, username, password string) (models.SerialGettyResponse, error) {
	f.lastAction = "enableSerialGetty"
	f.lastID = id
	return f.serialGettyResp, f.serialGettyErr
}

func (f *fakeVboxService) SetVmHardware(ctx context.Context, id string, cpus, memoryMB int) (models.VmOperationResponse, error) {
	f.lastAction = "setVmHardware"
	f.lastID = id
	f.lastCPUs = cpus
	f.lastMemoryMB = memoryMB
	return f.setHardwareResp, f.setHardwareErr
}

func (f *fakeVboxService) VmStorage(ctx context.Context, id string) (models.VmStorageResponse, error) {
	f.lastAction = "vmStorage"
	f.lastID = id
	return f.storage, f.storageErr
}

func (f *fakeVboxService) ResizeDisk(ctx context.Context, id, uuid string, sizeMB int64) (models.VmOperationResponse, error) {
	f.lastAction = "resizeDisk"
	f.lastID = id
	f.lastUUID = uuid
	f.lastSizeMB = sizeMB
	return f.resizeResp, f.resizeErr
}

func (f *fakeVboxService) AddDisk(ctx context.Context, id string, sizeMB int64) (models.VmOperationResponse, error) {
	f.lastAction = "addDisk"
	f.lastID = id
	f.lastSizeMB = sizeMB
	return f.resizeResp, f.resizeErr
}

func (f *fakeVboxService) DetachDisk(ctx context.Context, id, uuid string, deleteFile bool) (models.VmOperationResponse, error) {
	f.lastAction = "detachDisk"
	f.lastID = id
	f.lastUUID = uuid
	f.lastDeleteFile = deleteFile
	return f.resizeResp, f.resizeErr
}

func (f *fakeVboxService) ImportAppliance(ctx context.Context, ovaPath, name string) (models.VmCreateResponse, error) {
	f.lastAction = "importAppliance"
	return f.createResp, f.createErr
}

func (f *fakeVboxService) CreateVmUnattended(ctx context.Context, req models.VmCreateRequest) (models.VmCreateResponse, error) {
	f.lastAction = "createVmUnattended"
	return f.createResp, f.createErr
}

func (f *fakeVboxService) CreateVmManual(ctx context.Context, req models.VmCreateManualRequest) (models.VmCreateResponse, error) {
	f.lastAction = "createVmManual"
	f.lastManualReq = req
	return f.createResp, f.createErr
}

func newTestServer(t *testing.T, token string) (*Server, *fakeVboxService) {
	t.Helper()
	cfg := &config.Agent{
		BindAddress:  "127.0.0.1",
		BindPort:     5230,
		SessionToken: token,
		Environment:  "Development",
	}
	fake := &fakeVboxService{}
	srv := New(cfg, fake, nil, nil)
	return srv, fake
}

func TestHealthEndpointIsPublic(t *testing.T) {
	srv, _ := newTestServer(t, "secret")
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, `"status":"healthy"`) {
		t.Fatalf("expected healthy status in body, got %q", body)
	}
}

func TestHealthEndpointReportsUptime(t *testing.T) {
	srv, _ := newTestServer(t, "secret")
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if !strings.Contains(rr.Body.String(), `"uptimeSeconds":`) {
		t.Fatalf("expected uptimeSeconds in health body, got %q", rr.Body.String())
	}
}

func TestApiEndpointRequiresToken(t *testing.T) {
	srv, _ := newTestServer(t, "secret")
	req := httptest.NewRequest(http.MethodGet, "/api/vms", nil)
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestApiEndpointAcceptsValidToken(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	fake.vms = models.VmListResponse{VMs: []models.VmInfo{{ID: "11111111-1111-1111-1111-111111111111", Name: "VM", State: "listed"}}}

	req := httptest.NewRequest(http.MethodGet, "/api/vms", nil)
	req.Header.Set("X-TabVM-Session-Token", "secret")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, `"vms"`) {
		t.Fatalf("expected VMs in body, got %q", body)
	}
}

func TestUpdateStatusEndpointReturnsStatus(t *testing.T) {
	srv, _ := newTestServer(t, "secret")
	// Seed the checker's cache so the endpoint never touches the network.
	srv.updateChecker = updatecheck.New("0.1.2", updatecheck.WithSeededStatus(models.UpdateStatus{
		Current:         "0.1.2",
		Latest:          "0.1.3",
		UpdateAvailable: true,
		ReleaseURL:      "https://github.com/carauz1905/TabVM/releases/tag/v0.1.3",
	}, time.Now()))

	req := httptest.NewRequest(http.MethodGet, "/api/update-status", nil)
	req.Header.Set("X-TabVM-Session-Token", "secret")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (body %q)", http.StatusOK, rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, `"updateAvailable":true`) {
		t.Fatalf("expected updateAvailable=true in body, got %q", body)
	}
	if !strings.Contains(body, `"latest":"0.1.3"`) {
		t.Fatalf("expected latest version in body, got %q", body)
	}
	if !strings.Contains(body, `"releaseUrl":"https://github.com/carauz1905/TabVM/releases/tag/v0.1.3"`) {
		t.Fatalf("expected release URL in body, got %q", body)
	}
}

func TestApiEndpointRejectsInvalidToken(t *testing.T) {
	srv, _ := newTestServer(t, "secret")
	req := httptest.NewRequest(http.MethodGet, "/api/vms", nil)
	req.Header.Set("X-TabVM-Session-Token", "wrong")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestVmGuestOSEndpoint(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	id := "11111111-1111-1111-1111-111111111111"
	fake.guestOS = models.VmGuestOSResponse{ID: id, OSType: "Ubuntu_64", Family: "linux", TerminalCapable: true}

	req := httptest.NewRequest(http.MethodGet, "/api/vms/"+id+"/guest-os", nil)
	req.Header.Set("X-TabVM-Session-Token", "secret")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (body %q)", http.StatusOK, rr.Code, rr.Body.String())
	}
	if fake.lastAction != "vmGuestOS" || fake.lastID != id {
		t.Fatalf("expected vmGuestOS on %s, got %s on %s", id, fake.lastAction, fake.lastID)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `"terminalCapable":true`) || !strings.Contains(body, `"family":"linux"`) {
		t.Fatalf("expected guest OS fields in body, got %q", body)
	}
}

func TestSerialConsoleStatusEndpoint(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	id := "11111111-1111-1111-1111-111111111111"
	fake.serialStatus = models.VmSerialConsoleResponse{ID: id, Enabled: true, TerminalCapable: true, Running: true, Editable: false}

	req := httptest.NewRequest(http.MethodGet, "/api/vms/"+id+"/serial-console", nil)
	req.Header.Set("X-TabVM-Session-Token", "secret")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (body %q)", http.StatusOK, rr.Code, rr.Body.String())
	}
	if fake.lastAction != "serialConsoleStatus" || fake.lastID != id {
		t.Fatalf("expected serialConsoleStatus on %s, got %s on %s", id, fake.lastAction, fake.lastID)
	}
	if !strings.Contains(rr.Body.String(), `"terminalCapable":true`) {
		t.Fatalf("expected serial console fields in body, got %q", rr.Body.String())
	}
}

func TestEnableSerialConsoleEndpoint(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	id := "11111111-1111-1111-1111-111111111111"
	fake.serialEnableResp = models.VmOperationResponse{Success: true, VMID: id, Message: "Serial terminal enabled. Start the VM to use it."}

	req := httptest.NewRequest(http.MethodPost, "/api/vms/"+id+"/serial-console/enable", nil)
	req.Header.Set("X-TabVM-Session-Token", "secret")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (body %q)", http.StatusOK, rr.Code, rr.Body.String())
	}
	if fake.lastAction != "enableSerialConsole" || fake.lastID != id {
		t.Fatalf("expected enableSerialConsole on %s, got %s on %s", id, fake.lastAction, fake.lastID)
	}
}

func TestEnableSerialGettyEndpoint(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	id := "11111111-1111-1111-1111-111111111111"
	fake.serialGettyResp = models.SerialGettyResponse{Success: true, VMID: id, Message: "Serial login enabled. Open the terminal to connect."}

	body := strings.NewReader(`{"username":"root","password":"secret"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/vms/"+id+"/serial-console/enable-getty", body)
	req.Header.Set("X-TabVM-Session-Token", "secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (body %q)", http.StatusOK, rr.Code, rr.Body.String())
	}
	if fake.lastAction != "enableSerialGetty" || fake.lastID != id {
		t.Fatalf("expected enableSerialGetty on %s, got %s on %s", id, fake.lastAction, fake.lastID)
	}
}

func TestDeleteVmEndpointDeletesVm(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	id := "11111111-1111-1111-1111-111111111111"
	fake.deleteResp = models.VmOperationResponse{Success: true, VMID: id, Message: "VM deleted."}

	req := httptest.NewRequest(http.MethodDelete, "/api/vms/"+id, nil)
	req.Header.Set("X-TabVM-Session-Token", "secret")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (body %q)", http.StatusOK, rr.Code, rr.Body.String())
	}
	if fake.lastAction != "deleteVm" || fake.lastID != id {
		t.Fatalf("expected deleteVm on %s, got %s on %s", id, fake.lastAction, fake.lastID)
	}
	if !strings.Contains(rr.Body.String(), `"success":true`) {
		t.Fatalf("expected success response, got %q", rr.Body.String())
	}
}

func TestDeleteVmEndpointRejectsRunningVm(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	id := "11111111-1111-1111-1111-111111111111"
	fake.deleteErr = &vbox.ValidationError{Message: "The VM is running. Power it off before deleting it."}

	req := httptest.NewRequest(http.MethodDelete, "/api/vms/"+id, nil)
	req.Header.Set("X-TabVM-Session-Token", "secret")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestDeleteVmEndpointRejectsSubpath(t *testing.T) {
	srv, _ := newTestServer(t, "secret")
	id := "11111111-1111-1111-1111-111111111111"

	req := httptest.NewRequest(http.MethodDelete, "/api/vms/"+id+"/snapshots", nil)
	req.Header.Set("X-TabVM-Session-Token", "secret")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestVmHardwareEndpointReturnsConfig(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	id := "11111111-1111-1111-1111-111111111111"
	fake.hardware = models.VmHardwareResponse{ID: id, CPUs: 2, MemoryMB: 2048, HostCPUs: 8, HostMemoryMB: 16384, Editable: true}

	req := httptest.NewRequest(http.MethodGet, "/api/vms/"+id+"/hardware", nil)
	req.Header.Set("X-TabVM-Session-Token", "secret")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (body %q)", http.StatusOK, rr.Code, rr.Body.String())
	}
	if fake.lastAction != "vmHardware" || fake.lastID != id {
		t.Fatalf("expected vmHardware on %s, got %s on %s", id, fake.lastAction, fake.lastID)
	}
	if !strings.Contains(rr.Body.String(), `"hostCpus":8`) {
		t.Fatalf("expected host limits in body, got %q", rr.Body.String())
	}
}

func TestSetVmHardwareEndpointAppliesChange(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	id := "11111111-1111-1111-1111-111111111111"
	fake.setHardwareResp = models.VmOperationResponse{Success: true, VMID: id, Message: "Hardware updated."}

	req := httptest.NewRequest(http.MethodPost, "/api/vms/"+id+"/hardware", strings.NewReader(`{"cpus":4,"memoryMb":4096}`))
	req.Header.Set("X-TabVM-Session-Token", "secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (body %q)", http.StatusOK, rr.Code, rr.Body.String())
	}
	if fake.lastAction != "setVmHardware" || fake.lastCPUs != 4 || fake.lastMemoryMB != 4096 {
		t.Fatalf("unexpected call: action=%s cpus=%d mem=%d", fake.lastAction, fake.lastCPUs, fake.lastMemoryMB)
	}
}

func TestSetVmHardwareEndpointRejectsRunningVm(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	id := "11111111-1111-1111-1111-111111111111"
	fake.setHardwareErr = &vbox.ValidationError{Message: "The VM is running. Power it off before changing vCPU or memory."}

	req := httptest.NewRequest(http.MethodPost, "/api/vms/"+id+"/hardware", strings.NewReader(`{"cpus":4,"memoryMb":4096}`))
	req.Header.Set("X-TabVM-Session-Token", "secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestVmStorageEndpointReturnsDisks(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	id := "11111111-1111-1111-1111-111111111111"
	fake.storage = models.VmStorageResponse{
		ID:       id,
		Editable: true,
		Disks:    []models.DiskInfo{{UUID: "ca9ba73f-d0d3-4184-86f1-7206a952bc10", Name: "disk1.vdi", Format: "VDI", CapacityMB: 10240, Resizable: true}},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/vms/"+id+"/storage", nil)
	req.Header.Set("X-TabVM-Session-Token", "secret")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (body %q)", http.StatusOK, rr.Code, rr.Body.String())
	}
	if fake.lastAction != "vmStorage" || fake.lastID != id {
		t.Fatalf("expected vmStorage on %s, got %s on %s", id, fake.lastAction, fake.lastID)
	}
	if !strings.Contains(rr.Body.String(), `"format":"VDI"`) {
		t.Fatalf("expected disk in body, got %q", rr.Body.String())
	}
}

func TestResizeDiskEndpointAppliesChange(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	id := "11111111-1111-1111-1111-111111111111"
	fake.resizeResp = models.VmOperationResponse{Success: true, VMID: id, Message: "Disk resized."}

	req := httptest.NewRequest(http.MethodPost, "/api/vms/"+id+"/storage/resize",
		strings.NewReader(`{"uuid":"ca9ba73f-d0d3-4184-86f1-7206a952bc10","sizeMb":20480}`))
	req.Header.Set("X-TabVM-Session-Token", "secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (body %q)", http.StatusOK, rr.Code, rr.Body.String())
	}
	if fake.lastAction != "resizeDisk" || fake.lastUUID != "ca9ba73f-d0d3-4184-86f1-7206a952bc10" || fake.lastSizeMB != 20480 {
		t.Fatalf("unexpected call: action=%s uuid=%s size=%d", fake.lastAction, fake.lastUUID, fake.lastSizeMB)
	}
}

func TestResizeDiskEndpointRejectsShrink(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	id := "11111111-1111-1111-1111-111111111111"
	fake.resizeErr = &vbox.ValidationError{Message: "Disks can only grow."}

	req := httptest.NewRequest(http.MethodPost, "/api/vms/"+id+"/storage/resize",
		strings.NewReader(`{"uuid":"ca9ba73f-d0d3-4184-86f1-7206a952bc10","sizeMb":100}`))
	req.Header.Set("X-TabVM-Session-Token", "secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestAddDiskEndpointCreatesDisk(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	id := "11111111-1111-1111-1111-111111111111"
	fake.resizeResp = models.VmOperationResponse{Success: true, VMID: id, Message: "Added a disk."}

	req := httptest.NewRequest(http.MethodPost, "/api/vms/"+id+"/storage/add",
		strings.NewReader(`{"sizeMb":5120}`))
	req.Header.Set("X-TabVM-Session-Token", "secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (body %q)", http.StatusOK, rr.Code, rr.Body.String())
	}
	if fake.lastAction != "addDisk" || fake.lastSizeMB != 5120 {
		t.Fatalf("unexpected call: action=%s size=%d", fake.lastAction, fake.lastSizeMB)
	}
}

func TestDetachDiskEndpointDetachesAndDeletes(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	id := "11111111-1111-1111-1111-111111111111"
	fake.resizeResp = models.VmOperationResponse{Success: true, VMID: id, Message: "Disk detached."}

	req := httptest.NewRequest(http.MethodPost, "/api/vms/"+id+"/storage/detach",
		strings.NewReader(`{"uuid":"ca9ba73f-d0d3-4184-86f1-7206a952bc10","deleteFile":true}`))
	req.Header.Set("X-TabVM-Session-Token", "secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (body %q)", http.StatusOK, rr.Code, rr.Body.String())
	}
	if fake.lastAction != "detachDisk" || fake.lastUUID != "ca9ba73f-d0d3-4184-86f1-7206a952bc10" || !fake.lastDeleteFile {
		t.Fatalf("unexpected call: action=%s uuid=%s delete=%v", fake.lastAction, fake.lastUUID, fake.lastDeleteFile)
	}
}

func TestAddPortForwardingEndpointAppliesRule(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	id := "11111111-1111-1111-1111-111111111111"
	fake.portForwardOp = models.NetworkOperationResponse{Success: true, VMID: id, Message: "Forwarding 127.0.0.1:2222 -> guest:22 added on adapter 1."}

	req := httptest.NewRequest(http.MethodPost, "/api/vms/"+id+"/network/forwarding",
		strings.NewReader(`{"slot":1,"name":"ssh","protocol":"tcp","hostIp":"","hostPort":2222,"guestIp":"","guestPort":22}`))
	req.Header.Set("X-TabVM-Session-Token", "secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (body %q)", http.StatusOK, rr.Code, rr.Body.String())
	}
	if fake.lastAction != "addPortForwarding" || fake.lastID != id {
		t.Fatalf("expected addPortForwarding on %s, got %s on %s", id, fake.lastAction, fake.lastID)
	}
	want := models.PortForwardingRequest{Slot: 1, Name: "ssh", Protocol: "tcp", HostPort: 2222, GuestPort: 22}
	if fake.lastForwardReq != want {
		t.Fatalf("unexpected forwarding request: %+v", fake.lastForwardReq)
	}
	if !strings.Contains(rr.Body.String(), `"success":true`) {
		t.Fatalf("expected success response, got %q", rr.Body.String())
	}
}

func TestAddPortForwardingEndpointRejectsInvalidRule(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	id := "11111111-1111-1111-1111-111111111111"
	fake.portForwardErr = &vbox.ValidationError{Message: "Adapter 1 must be in NAT mode to add a port-forwarding rule."}

	req := httptest.NewRequest(http.MethodPost, "/api/vms/"+id+"/network/forwarding",
		strings.NewReader(`{"slot":1,"name":"ssh","protocol":"tcp","hostPort":2222,"guestPort":22}`))
	req.Header.Set("X-TabVM-Session-Token", "secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestDeletePortForwardingEndpointRemovesRule(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	id := "11111111-1111-1111-1111-111111111111"
	fake.portForwardOp = models.NetworkOperationResponse{Success: true, VMID: id, Message: `Forwarding rule "ssh" removed from adapter 1.`}

	req := httptest.NewRequest(http.MethodPost, "/api/vms/"+id+"/network/forwarding/delete",
		strings.NewReader(`{"slot":1,"name":"ssh"}`))
	req.Header.Set("X-TabVM-Session-Token", "secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (body %q)", http.StatusOK, rr.Code, rr.Body.String())
	}
	if fake.lastAction != "deletePortForwarding" || fake.lastForwardSlot != 1 || fake.lastForwardName != "ssh" {
		t.Fatalf("unexpected call: action=%s slot=%d name=%s", fake.lastAction, fake.lastForwardSlot, fake.lastForwardName)
	}
}

func TestCreateManualEndpointStartsJobAndDispatches(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	fake.createResp = models.VmCreateResponse{
		Success: true,
		VMID:    "11111111-1111-1111-1111-111111111111",
		Name:    "alpine",
		Message: `"alpine" created. Start it and install the OS from the attached ISO.`,
	}

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

	// The work runs on a background goroutine; poll the status endpoint until
	// the job resolves.
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
		if status.State == "done" {
			if !strings.Contains(status.Message, "install the OS from the attached ISO") {
				t.Fatalf("unexpected job message: %q", status.Message)
			}
			break
		}
		if status.State == "error" {
			t.Fatalf("job failed: %q", status.Message)
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for the manual create job")
		}
		time.Sleep(10 * time.Millisecond)
	}

	if fake.lastAction != "createVmManual" {
		t.Fatalf("expected createVmManual dispatch, got %q", fake.lastAction)
	}
	want := models.VmCreateManualRequest{
		Name: "alpine", OsType: "Linux_64", IsoPath: `C:\iso\alpine.iso`, MemoryMB: 2048, Cpus: 2, DiskGB: 20,
	}
	if fake.lastManualReq != want {
		t.Fatalf("unexpected request: %+v", fake.lastManualReq)
	}
}

func TestConsoleProtocolsEndpointReturnsCapabilities(t *testing.T) {
	srv, _ := newTestServer(t, "secret")

	req := httptest.NewRequest(http.MethodGet, "/api/console/protocols", nil)
	req.Header.Set("X-TabVM-Session-Token", "secret")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, `"id":"rdp"`) {
		t.Fatalf("expected rdp protocol in body, got %q", body)
	}
	if !strings.Contains(body, `"id":"vnc"`) {
		t.Fatalf("expected vnc protocol in body, got %q", body)
	}
	if !strings.Contains(body, `"id":"ssh"`) {
		t.Fatalf("expected ssh protocol in body, got %q", body)
	}
	if !strings.Contains(body, `"canAutoConfigure":true`) {
		t.Fatalf("expected at least one auto-configurable protocol, got %q", body)
	}
}

func TestConsoleProtocolsEndpointRequiresAuth(t *testing.T) {
	srv, _ := newTestServer(t, "secret")

	req := httptest.NewRequest(http.MethodGet, "/api/console/protocols", nil)
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestDiscoveryEndpointProtected(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	fake.discovery = models.VirtualBoxDiscovery{Found: true, Version: "7.0.14r161095"}

	req := httptest.NewRequest(http.MethodGet, "/api/vbox/discovery", nil)
	req.Header.Set("X-TabVM-Session-Token", "secret")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, `"found":true`) {
		t.Fatalf("expected found=true in body, got %q", body)
	}
	if strings.Contains(body, "vBoxManagePath") {
		t.Fatalf("discovery response leaked the resolved executable path: %q", body)
	}
}

func TestAllApiRoutesRequireAuth(t *testing.T) {
	srv, _ := newTestServer(t, "secret")

	routes := []string{
		"/api/vms",
		"/api/vbox/discovery",
		"/api/console/protocols",
		"/api/update-status",
		"/api/vms/11111111-1111-1111-1111-111111111111/status",
	}
	for _, route := range routes {
		t.Run(route, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, route, nil)
			rr := httptest.NewRecorder()

			srv.Handler().ServeHTTP(rr, req)

			if rr.Code != http.StatusUnauthorized {
				t.Fatalf("expected status %d for %s, got %d", http.StatusUnauthorized, route, rr.Code)
			}
		})
	}
}

func TestVmLifecycleRoutesRequireAuth(t *testing.T) {
	srv, _ := newTestServer(t, "secret")

	routes := []string{
		"/api/vms/11111111-1111-1111-1111-111111111111/start",
		"/api/vms/11111111-1111-1111-1111-111111111111/stop",
		"/api/vms/11111111-1111-1111-1111-111111111111/reset",
		"/api/vms/11111111-1111-1111-1111-111111111111/network/forwarding",
		"/api/vms/11111111-1111-1111-1111-111111111111/network/forwarding/delete",
	}
	for _, route := range routes {
		t.Run(route, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, route, nil)
			rr := httptest.NewRecorder()

			srv.Handler().ServeHTTP(rr, req)

			if rr.Code != http.StatusUnauthorized {
				t.Fatalf("expected status %d for %s, got %d", http.StatusUnauthorized, route, rr.Code)
			}
		})
	}
}

func TestUnknownApiRouteIsUnauthorized(t *testing.T) {
	srv, _ := newTestServer(t, "secret")

	req := httptest.NewRequest(http.MethodGet, "/api/not-a-route", nil)
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestTokenCompareUsesFixedSizeHash(t *testing.T) {
	srv, fake := newTestServer(t, "a")
	fake.vms = models.VmListResponse{VMs: []models.VmInfo{{ID: "11111111-1111-1111-1111-111111111111", Name: "VM", State: "listed"}}}

	req := httptest.NewRequest(http.MethodGet, "/api/vms", nil)
	req.Header.Set("X-TabVM-Session-Token", "a")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestVmStatusEndpointReturnsStatus(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	fake.status = models.VmStatusResponse{ID: "11111111-1111-1111-1111-111111111111", State: "running"}

	req := httptest.NewRequest(http.MethodGet, "/api/vms/11111111-1111-1111-1111-111111111111/status", nil)
	req.Header.Set("X-TabVM-Session-Token", "secret")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, `"state":"running"`) {
		t.Fatalf("expected running state in body, got %q", body)
	}
}

func TestVmStartEndpointReturnsOperationResult(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	fake.status = models.VmStatusResponse{ID: "11111111-1111-1111-1111-111111111111", State: "running"}

	req := httptest.NewRequest(http.MethodPost, "/api/vms/11111111-1111-1111-1111-111111111111/start", nil)
	req.Header.Set("X-TabVM-Session-Token", "secret")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	if fake.lastAction != "start" || fake.lastID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("expected start action for 11111111-1111-1111-1111-111111111111, got %s/%s", fake.lastAction, fake.lastID)
	}

	body := rr.Body.String()
	if !strings.Contains(body, `"success":true`) {
		t.Fatalf("expected success=true in body, got %q", body)
	}
}

func TestVmStopEndpointReturnsOperationResult(t *testing.T) {
	srv, fake := newTestServer(t, "secret")

	req := httptest.NewRequest(http.MethodPost, "/api/vms/11111111-1111-1111-1111-111111111111/stop", nil)
	req.Header.Set("X-TabVM-Session-Token", "secret")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	if fake.lastAction != "stop" || fake.lastID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("expected stop action for 11111111-1111-1111-1111-111111111111, got %s/%s", fake.lastAction, fake.lastID)
	}
}

func TestVmResetEndpointReturnsOperationResult(t *testing.T) {
	srv, fake := newTestServer(t, "secret")

	req := httptest.NewRequest(http.MethodPost, "/api/vms/11111111-1111-1111-1111-111111111111/reset", nil)
	req.Header.Set("X-TabVM-Session-Token", "secret")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	if fake.lastAction != "reset" || fake.lastID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("expected reset action for 11111111-1111-1111-1111-111111111111, got %s/%s", fake.lastAction, fake.lastID)
	}
}

func TestVmOperationRejectsInvalidID(t *testing.T) {
	srv, _ := newTestServer(t, "secret")

	req := httptest.NewRequest(http.MethodPost, "/api/vms/bad;id/start", nil)
	req.Header.Set("X-TabVM-Session-Token", "secret")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestVmStatusRejectsInvalidID(t *testing.T) {
	srv, _ := newTestServer(t, "secret")

	req := httptest.NewRequest(http.MethodGet, "/api/vms/bad;id/status", nil)
	req.Header.Set("X-TabVM-Session-Token", "secret")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestVmStartReturnsConflictForConcurrentSameVMOperation(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	fake.startBlocker = make(chan struct{})
	fake.startEntered = make(chan struct{})

	var firstWg sync.WaitGroup
	firstWg.Add(1)
	firstCode := 0

	go func() {
		defer firstWg.Done()
		req := httptest.NewRequest(http.MethodPost, "/api/vms/11111111-1111-1111-1111-111111111111/start", nil)
		req.Header.Set("X-TabVM-Session-Token", "secret")
		rr := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rr, req)
		firstCode = rr.Code
	}()

	// Wait until the first request has acquired the VM operation lock.
	<-fake.startEntered

	// The second request must be issued while the first request still holds the
	// lock, so it receives 409 Conflict deterministically.
	req2 := httptest.NewRequest(http.MethodPost, "/api/vms/11111111-1111-1111-1111-111111111111/start", nil)
	req2.Header.Set("X-TabVM-Session-Token", "secret")
	rr2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusConflict {
		t.Fatalf("expected second request to receive 409 Conflict, got %d", rr2.Code)
	}

	close(fake.startBlocker)
	firstWg.Wait()

	if firstCode != http.StatusOK {
		t.Fatalf("expected first request to succeed, got %d", firstCode)
	}
}

func TestVmOperationSanitizesExecutionError(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	fake.startErr = &vbox.ExecutionError{
		ExitCode:      1,
		StandardError: "VBoxManage: could not find executable at C:\\Secret\\VBoxManage.exe",
		Message:       "VBoxManage failed while starting VM: exec error",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/vms/11111111-1111-1111-1111-111111111111/start", nil)
	req.Header.Set("X-TabVM-Session-Token", "secret")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Fatalf("expected status %d, got %d", http.StatusBadGateway, rr.Code)
	}

	body := rr.Body.String()
	if strings.Contains(body, "VBoxManage") {
		t.Fatalf("response leaked raw VBoxManage detail: %q", body)
	}
	if strings.Contains(body, "C:\\Secret") {
		t.Fatalf("response leaked resolved executable path: %q", body)
	}
	if !strings.Contains(body, "VirtualBox operation failed") {
		t.Fatalf("expected sanitized error message, got %q", body)
	}
}

// TestSanitizedExecMessage covers Part B2: known VBoxManage stderr signatures
// map to fixed, actionable messages, and anything else falls back to a generic
// message that never leaks host paths or the raw "VBoxManage" detail.
func TestSanitizedExecMessage(t *testing.T) {
	cases := []struct {
		name        string
		err         *vbox.ExecutionError
		wantContain string
		wantAbsent  []string
	}{
		{
			name:        "already locked maps to busy message",
			err:         &vbox.ExecutionError{ExitCode: 1, StandardError: "VBoxManage: error: The machine 'kali' is already locked by a session"},
			wantContain: "busy or locked",
		},
		{
			name:        "invalid object state maps to busy message",
			err:         &vbox.ExecutionError{ExitCode: 1, StandardError: "error: code VBOX_E_INVALID_OBJECT_STATE (0x80bb0007)"},
			wantContain: "busy or locked",
		},
		{
			name:        "VT-x unavailable maps to virtualization message",
			err:         &vbox.ExecutionError{ExitCode: 1, StandardError: "VERR_VMX_NO_VMX: VT-x is not available"},
			wantContain: "Hardware virtualization is unavailable",
		},
		{
			name:        "Hyper-V maps to virtualization message",
			err:         &vbox.ExecutionError{ExitCode: 1, StandardError: "VERR_NEM_NOT_AVAILABLE (Hyper-V is active)"},
			wantContain: "Hardware virtualization is unavailable",
		},
		{
			name:        "no memory maps to memory message",
			err:         &vbox.ExecutionError{ExitCode: 1, StandardError: "VERR_NO_MEMORY: not enough memory to complete the operation"},
			wantContain: "Not enough host memory",
		},
		{
			name:        "unknown stderr falls back without leaking",
			err:         &vbox.ExecutionError{ExitCode: 7, StandardError: "VBoxManage: could not find executable at C:\\Secret\\VBoxManage.exe"},
			wantContain: "VirtualBox operation failed",
			wantAbsent:  []string{"VBoxManage", "C:\\Secret"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizedExecMessage(tc.err)
			if !strings.Contains(got, tc.wantContain) {
				t.Fatalf("expected %q to contain %q", got, tc.wantContain)
			}
			for _, absent := range tc.wantAbsent {
				if strings.Contains(got, absent) {
					t.Fatalf("expected %q to not contain %q", got, absent)
				}
			}
		})
	}
}

func TestVmConsoleStatusEndpointReturnsStatus(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	fake.consoleStatus = models.VmConsoleStatusResponse{
		ID:      "11111111-1111-1111-1111-111111111111",
		Enabled: true,
		Address: "127.0.0.1",
		Port:    5432,
		Ready:   true,
		Target:  "127.0.0.1:5432",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/vms/11111111-1111-1111-1111-111111111111/console", nil)
	req.Header.Set("X-TabVM-Session-Token", "secret")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, `"ready":true`) {
		t.Fatalf("expected ready=true in body, got %q", body)
	}
	if !strings.Contains(body, `"target":"127.0.0.1:5432"`) {
		t.Fatalf("expected target in body, got %q", body)
	}
}

func TestVmConsolePrepareEndpointReturnsStatus(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	fake.prepareConsole = models.VmConsoleStatusResponse{
		ID:      "11111111-1111-1111-1111-111111111111",
		Enabled: true,
		Address: "127.0.0.1",
		Port:    5432,
		Ready:   true,
		Target:  "127.0.0.1:5432",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/vms/11111111-1111-1111-1111-111111111111/console/prepare", nil)
	req.Header.Set("X-TabVM-Session-Token", "secret")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	if fake.lastAction != "prepareConsole" || fake.lastID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("expected prepareConsole action for 11111111-1111-1111-1111-111111111111, got %s/%s", fake.lastAction, fake.lastID)
	}

	body := rr.Body.String()
	if !strings.Contains(body, `"ready":true`) {
		t.Fatalf("expected ready=true in body, got %q", body)
	}
}

func TestVmConsoleDisableEndpointReturnsOperationResult(t *testing.T) {
	srv, _ := newTestServer(t, "secret")

	req := httptest.NewRequest(http.MethodPost, "/api/vms/11111111-1111-1111-1111-111111111111/console/disable", nil)
	req.Header.Set("X-TabVM-Session-Token", "secret")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, `"success":true`) {
		t.Fatalf("expected success=true in body, got %q", body)
	}
}

func TestVmConsoleRoutesRequireAuth(t *testing.T) {
	srv, _ := newTestServer(t, "secret")

	routes := []struct {
		method string
		route  string
	}{
		{http.MethodGet, "/api/vms/11111111-1111-1111-1111-111111111111/console"},
		{http.MethodPost, "/api/vms/11111111-1111-1111-1111-111111111111/console/prepare"},
		{http.MethodPost, "/api/vms/11111111-1111-1111-1111-111111111111/console/disable"},
	}
	for _, r := range routes {
		t.Run(r.route, func(t *testing.T) {
			req := httptest.NewRequest(r.method, r.route, nil)
			rr := httptest.NewRecorder()

			srv.Handler().ServeHTTP(rr, req)

			if rr.Code != http.StatusUnauthorized {
				t.Fatalf("expected status %d for %s, got %d", http.StatusUnauthorized, r.route, rr.Code)
			}
		})
	}
}

func TestVmConsolePrepareConflictsWithConcurrentLifecycleOperation(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	fake.startBlocker = make(chan struct{})
	fake.startEntered = make(chan struct{})

	var startWg sync.WaitGroup
	startWg.Add(1)
	startCode := 0

	go func() {
		defer startWg.Done()
		req := httptest.NewRequest(http.MethodPost, "/api/vms/11111111-1111-1111-1111-111111111111/start", nil)
		req.Header.Set("X-TabVM-Session-Token", "secret")
		rr := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rr, req)
		startCode = rr.Code
	}()

	<-fake.startEntered

	req := httptest.NewRequest(http.MethodPost, "/api/vms/11111111-1111-1111-1111-111111111111/console/prepare", nil)
	req.Header.Set("X-TabVM-Session-Token", "secret")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected console prepare to receive 409 Conflict while lifecycle operation holds the VM lock, got %d", rr.Code)
	}

	close(fake.startBlocker)
	startWg.Wait()

	if startCode != http.StatusOK {
		t.Fatalf("expected lifecycle operation to succeed, got %d", startCode)
	}
}

func TestVmOperationsOnDifferentVMsAreNotBlocked(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	fake.status = models.VmStatusResponse{ID: "11111111-1111-1111-1111-111111111111", State: "running"}

	var wg sync.WaitGroup
	codes := make(map[int]int)
	var mu sync.Mutex

	for _, id := range []string{"11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222"} {
		wg.Add(1)
		go func(vmID string) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/api/vms/"+vmID+"/start", nil)
			req.Header.Set("X-TabVM-Session-Token", "secret")
			rr := httptest.NewRecorder()
			srv.Handler().ServeHTTP(rr, req)
			mu.Lock()
			codes[rr.Code]++
			mu.Unlock()
		}(id)
	}

	wg.Wait()

	if codes[http.StatusOK] != 2 {
		t.Fatalf("expected both VM start operations to succeed, got codes: %+v", codes)
	}
}

func TestVmConsolePrepareRejectsInvalidID(t *testing.T) {
	srv, _ := newTestServer(t, "secret")

	req := httptest.NewRequest(http.MethodPost, "/api/vms/bad;id/console/prepare", nil)
	req.Header.Set("X-TabVM-Session-Token", "secret")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestVmConsolePrepareSanitizesExecutionError(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	fake.prepareConsoleErr = &vbox.ExecutionError{
		ExitCode:      1,
		StandardError: "VBoxManage: could not find executable at C:\\Secret\\VBoxManage.exe",
		Message:       "VBoxManage failed while preparing console: exec error",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/vms/11111111-1111-1111-1111-111111111111/console/prepare", nil)
	req.Header.Set("X-TabVM-Session-Token", "secret")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Fatalf("expected status %d, got %d", http.StatusBadGateway, rr.Code)
	}

	body := rr.Body.String()
	if strings.Contains(body, "VBoxManage") {
		t.Fatalf("response leaked raw VBoxManage detail: %q", body)
	}
	if strings.Contains(body, "C:\\Secret") {
		t.Fatalf("response leaked resolved executable path: %q", body)
	}
}

func TestLocalStateStatusEndpointRequiresAuth(t *testing.T) {
	srv, _ := newTestServer(t, "secret")

	req := httptest.NewRequest(http.MethodGet, "/api/local-state/status", nil)
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestLocalStateStatusEndpointDoesNotExposePath(t *testing.T) {
	srv, _ := newTestServer(t, "secret")

	req := httptest.NewRequest(http.MethodGet, "/api/local-state/status", nil)
	req.Header.Set("X-TabVM-Session-Token", "secret")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, `"configured"`) {
		t.Fatalf("expected configured field in body, got %q", body)
	}
	if strings.Contains(body, "tabvm.db") {
		t.Fatalf("local state status leaked database path: %q", body)
	}
}

func TestVmTelemetryEndpointReturnsInterfaces(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	fake.telemetry = models.VmTelemetryResponse{
		ID:             "11111111-1111-1111-1111-111111111111",
		CPUCount:       4,
		RAMMB:          8192,
		GuestAdditions: true,
		Networks: []models.NetworkInterface{
			{Slot: 1, Mode: "bridged", MAC: "0800271122AA", IPv4: []string{"192.168.1.42"}},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/vms/11111111-1111-1111-1111-111111111111/telemetry", nil)
	req.Header.Set("X-TabVM-Session-Token", "secret")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `"192.168.1.42"`) {
		t.Fatalf("expected guest IPv4 in body, got %q", body)
	}
	if !strings.Contains(body, `"guestAdditions":true`) {
		t.Fatalf("expected guestAdditions flag in body, got %q", body)
	}
	if fake.lastAction != "telemetry" {
		t.Fatalf("expected telemetry action, got %s", fake.lastAction)
	}
}

func TestActivityEndpointReturnsRecordedOperations(t *testing.T) {
	db, err := store.OpenInMemory(context.Background())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()
	if err := db.LogOperation(context.Background(), "vm-1", "start", true, "started"); err != nil {
		t.Fatalf("log: %v", err)
	}

	cfg := &config.Agent{BindAddress: "127.0.0.1", BindPort: 5230, SessionToken: "secret", Environment: "Development"}
	srv := New(cfg, &fakeVboxService{}, db, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/activity", nil)
	req.Header.Set("X-TabVM-Session-Token", "secret")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `"action":"start"`) || !strings.Contains(body, `"success":true`) {
		t.Fatalf("expected logged operation in body, got %q", body)
	}
}

func TestActivityEndpointRequiresAuth(t *testing.T) {
	srv, _ := newTestServer(t, "secret")

	req := httptest.NewRequest(http.MethodGet, "/api/activity", nil)
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestVmTelemetryEndpointRequiresAuth(t *testing.T) {
	srv, _ := newTestServer(t, "secret")

	req := httptest.NewRequest(http.MethodGet, "/api/vms/11111111-1111-1111-1111-111111111111/telemetry", nil)
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestListSharedFoldersEndpoint(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	fake.sharedFolders = models.SharedFoldersResponse{
		Folders: []models.SharedFolder{
			{Name: "labshare", HostPath: `C:\labs\share`, Transient: false},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/vms/11111111-1111-1111-1111-111111111111/shared-folders", nil)
	req.Header.Set("X-TabVM-Session-Token", "secret")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `"labshare"`) {
		t.Fatalf("expected shared folder in body, got %q", rr.Body.String())
	}
	if fake.lastAction != "listSharedFolders" {
		t.Fatalf("expected listSharedFolders action, got %s", fake.lastAction)
	}
}

func TestAddSharedFolderEndpointPassesNameAndPath(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	fake.sharedFolderOp = models.SharedFolderOperationResponse{Success: true, VMID: "11111111-1111-1111-1111-111111111111", Message: "added"}

	body := strings.NewReader(`{"name":"labshare","hostPath":"C:\\labs\\share"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/vms/11111111-1111-1111-1111-111111111111/shared-folders", body)
	req.Header.Set("X-TabVM-Session-Token", "secret")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (%s)", http.StatusOK, rr.Code, rr.Body.String())
	}
	if fake.lastAction != "addSharedFolder" || fake.lastName != "labshare" || fake.lastHostPath != `C:\labs\share` {
		t.Fatalf("unexpected forwarded args: action=%s name=%q path=%q", fake.lastAction, fake.lastName, fake.lastHostPath)
	}
}

func TestAddSharedFolderEndpointRejectsUnknownFields(t *testing.T) {
	srv, _ := newTestServer(t, "secret")

	body := strings.NewReader(`{"name":"labshare","hostPath":"C:\\labs","evil":true}`)
	req := httptest.NewRequest(http.MethodPost, "/api/vms/11111111-1111-1111-1111-111111111111/shared-folders", body)
	req.Header.Set("X-TabVM-Session-Token", "secret")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d for unknown field, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestAddSharedFolderEndpointMapsValidationErrorTo400(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	fake.sharedFolderOpErr = &vbox.ValidationError{Message: "Host path must be a directory."}

	body := strings.NewReader(`{"name":"labshare","hostPath":"C:\\labs\\file.txt"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/vms/11111111-1111-1111-1111-111111111111/shared-folders", body)
	req.Header.Set("X-TabVM-Session-Token", "secret")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestRemoveSharedFolderEndpoint(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	fake.sharedFolderOp = models.SharedFolderOperationResponse{Success: true, VMID: "11111111-1111-1111-1111-111111111111", Message: "removed"}

	body := strings.NewReader(`{"name":"labshare"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/vms/11111111-1111-1111-1111-111111111111/shared-folders/remove", body)
	req.Header.Set("X-TabVM-Session-Token", "secret")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (%s)", http.StatusOK, rr.Code, rr.Body.String())
	}
	if fake.lastAction != "removeSharedFolder" || fake.lastName != "labshare" {
		t.Fatalf("unexpected forwarded args: action=%s name=%q", fake.lastAction, fake.lastName)
	}
}

func TestSharedFoldersEndpointRequiresAuth(t *testing.T) {
	srv, _ := newTestServer(t, "secret")

	req := httptest.NewRequest(http.MethodGet, "/api/vms/11111111-1111-1111-1111-111111111111/shared-folders", nil)
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestGetClipboardModeEndpoint(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	fake.clipboard = models.ClipboardModeResponse{ID: "11111111-1111-1111-1111-111111111111", Mode: "bidirectional"}

	req := httptest.NewRequest(http.MethodGet, "/api/vms/11111111-1111-1111-1111-111111111111/clipboard", nil)
	req.Header.Set("X-TabVM-Session-Token", "secret")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `"bidirectional"`) {
		t.Fatalf("expected mode in body, got %q", rr.Body.String())
	}
	if fake.lastAction != "getClipboard" {
		t.Fatalf("expected getClipboard action, got %s", fake.lastAction)
	}
}

func TestSetClipboardModeEndpointForwardsMode(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	fake.clipboard = models.ClipboardModeResponse{ID: "11111111-1111-1111-1111-111111111111", Mode: "bidirectional"}

	body := strings.NewReader(`{"mode":"bidirectional"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/vms/11111111-1111-1111-1111-111111111111/clipboard", body)
	req.Header.Set("X-TabVM-Session-Token", "secret")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d (%s)", http.StatusOK, rr.Code, rr.Body.String())
	}
	if fake.lastAction != "setClipboard" || fake.lastMode != "bidirectional" {
		t.Fatalf("unexpected forwarded args: action=%s mode=%q", fake.lastAction, fake.lastMode)
	}
}

func TestSetClipboardModeEndpointMapsValidationErrorTo400(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	fake.clipboardErr = &vbox.ValidationError{Message: "Clipboard mode must be one of: ..."}

	body := strings.NewReader(`{"mode":"nonsense"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/vms/11111111-1111-1111-1111-111111111111/clipboard", body)
	req.Header.Set("X-TabVM-Session-Token", "secret")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

// The host folder picker opens a native modal dialog, so it is only exercised
// through its guards here: a non-POST method and a missing token must both be
// rejected before PickFolder is ever called (no dialog can pop in a test).

func TestPickFolderRejectsNonPost(t *testing.T) {
	srv, _ := newTestServer(t, "secret")
	req := httptest.NewRequest(http.MethodGet, "/api/host/pick-folder", nil)
	req.Header.Set("X-TabVM-Session-Token", "secret")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected %d, got %d", http.StatusMethodNotAllowed, rr.Code)
	}
}

func TestPickFolderRequiresToken(t *testing.T) {
	srv, _ := newTestServer(t, "secret")
	req := httptest.NewRequest(http.MethodPost, "/api/host/pick-folder", nil)
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func (f *fakeVboxService) ForcePowerOff(ctx context.Context, id string) error {
	f.lastAction = "poweroff"
	f.lastID = id
	return f.poweroffErr
}

func TestVmPowerOffEndpointReturnsOperationResult(t *testing.T) {
	srv, fake := newTestServer(t, "secret")

	req := httptest.NewRequest(http.MethodPost, "/api/vms/11111111-1111-1111-1111-111111111111/poweroff", nil)
	req.Header.Set("X-TabVM-Session-Token", "secret")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	if fake.lastAction != "poweroff" || fake.lastID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("expected poweroff action for 11111111-1111-1111-1111-111111111111, got %s/%s", fake.lastAction, fake.lastID)
	}

	body := rr.Body.String()
	if !strings.Contains(body, `"success":true`) {
		t.Fatalf("expected success=true in body, got %q", body)
	}
}

func TestVmPowerOffRouteRequiresAuth(t *testing.T) {
	srv, _ := newTestServer(t, "secret")

	req := httptest.NewRequest(http.MethodPost, "/api/vms/11111111-1111-1111-1111-111111111111/poweroff", nil)
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

// compile-time check that fakeVboxService implements vbox.Service.
var _ vbox.Service = (*fakeVboxService)(nil)
