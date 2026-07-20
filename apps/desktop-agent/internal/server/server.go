package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/tabvm/desktop-agent/internal/config"
	"github.com/tabvm/desktop-agent/internal/console"
	"github.com/tabvm/desktop-agent/internal/hostpick"
	"github.com/tabvm/desktop-agent/internal/models"
	"github.com/tabvm/desktop-agent/internal/store"
	"github.com/tabvm/desktop-agent/internal/updatecheck"
	"github.com/tabvm/desktop-agent/internal/vbox"
	"github.com/tabvm/desktop-agent/internal/version"
	"github.com/tabvm/desktop-agent/internal/webui"
)

const sessionTokenHeader = "X-TabVM-Session-Token"

// Server is the local HTTP agent server.
type Server struct {
	cfg        *config.Agent
	vbox       vbox.Service
	store      *store.Store
	logger     *slog.Logger
	startedAt  time.Time
	mu         sync.Mutex
	devToken   string
	opMu       sync.Mutex
	ops        map[string]*sync.Mutex
	pickMu     sync.Mutex
	createMu   sync.Mutex
	createJobs map[string]*createJob
	// updateChecker performs the best-effort, cached "update available" check
	// against GitHub's public releases API. It never blocks or errors the app.
	updateChecker *updatecheck.Checker
}

// createJob tracks a background VM import/create. State is "running", "done", or
// "error"; Message carries a user-safe note or error.
type createJob struct {
	State   string
	Message string
	VMID    string
	Name    string
}

// New creates a new HTTP server for the TabVM local agent.
func New(cfg *config.Agent, vboxService vbox.Service, db *store.Store, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{
		cfg:           cfg,
		vbox:          vboxService,
		store:         db,
		logger:        logger,
		startedAt:     time.Now(),
		createJobs:    make(map[string]*createJob),
		updateChecker: updatecheck.New(version.Version, updatecheck.WithLogger(logger)),
	}
}

// Handler returns the http.Handler for the agent API.
// All /api/* routes are mounted under a single authentication handler
// so future API additions cannot accidentally skip auth.
func (s *Server) Handler() http.Handler {
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/vbox/discovery", s.handleDiscovery)
	apiMux.HandleFunc("/console/protocols", s.handleConsoleProtocols)
	apiMux.HandleFunc("/local-state/status", s.handleLocalStateStatus)
	apiMux.HandleFunc("/update-status", s.handleUpdateStatus)
	apiMux.HandleFunc("/host/pick-folder", s.handlePickFolder)
	apiMux.HandleFunc("/host/pick-file", s.handlePickFile)
	apiMux.HandleFunc("/activity", s.handleActivity)
	apiMux.HandleFunc("/vms", s.handleVms)
	// More specific create/import routes must be registered before the "/vms/"
	// subtree so they are not swallowed by the per-ID handler.
	apiMux.HandleFunc("/vms/import", s.handleImportVm)
	apiMux.HandleFunc("/vms/create", s.handleCreateVm)
	apiMux.HandleFunc("/vms/create-manual", s.handleCreateVmManual)
	apiMux.HandleFunc("/vms/create/status", s.handleCreateStatus)
	apiMux.HandleFunc("/vms/", s.handleVmByID)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.Handle("/api/", http.StripPrefix("/api", s.withAuth(apiMux)))
	mux.Handle("/", s.staticHandler())
	return mux
}

// staticHandler serves the embedded web UI for all non-API routes. Unknown
// paths fall back to index.html so client-side routes (e.g. ?console=...) load,
// and index.html is served with the session token injected so a freshly opened
// browser authenticates without any build-time secret.
func (s *Server) staticHandler() http.Handler {
	sub, err := webui.FS()
	if err != nil {
		s.logger.Error("failed to open embedded web UI", "error", err)
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "Web UI is unavailable.", http.StatusInternalServerError)
		})
	}
	fileServer := http.FileServer(http.FS(sub))
	indexTemplate, _ := fs.ReadFile(sub, "index.html")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		clean := strings.TrimPrefix(r.URL.Path, "/")
		if clean != "" && !strings.HasSuffix(r.URL.Path, "/") {
			if f, err := sub.Open(clean); err == nil {
				f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		s.serveIndex(w, indexTemplate)
	})
}

// serveIndex writes index.html with the resolved session token injected as a
// window global. The token is JSON-encoded so it is always a safe string
// literal. Injection happens at serve time, so each machine's own token is used.
func (s *Server) serveIndex(w http.ResponseWriter, template []byte) {
	tokenJSON, err := json.Marshal(s.resolveToken())
	if err != nil {
		tokenJSON = []byte(`""`)
	}
	script := "<script>window.__TABVM_SESSION_TOKEN__=" + string(tokenJSON) + ";</script></head>"
	html := strings.Replace(string(template), "</head>", script, 1)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(html))
}

// ListenAndServe starts the HTTP server on the configured address.
func (s *Server) ListenAndServe() error {
	addr := s.cfg.ListenAddress()
	s.logger.Info("starting TabVM agent", "address", addr)
	return http.ListenAndServe(addr, s.Handler())
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	respondJSON(w, http.StatusOK, models.HealthStatus{
		Status:        "healthy",
		Timestamp:     time.Now().UTC(),
		UptimeSeconds: int64(time.Since(s.startedAt).Seconds()),
		Version:       version.Version,
	})
}

// handleUpdateStatus reports whether a newer TabVM release is available. The
// checker is best-effort and cached, and always yields a safe payload, so this
// endpoint always returns 200 even when the host is offline or GitHub is
// unreachable — the local-first UI is never blocked by the check.
func (s *Server) handleUpdateStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Use a background context, not the request's: the checker caches its result
	// (including safe "no update" outcomes on failure) for hours, so a client
	// disconnecting mid-fetch must not cancel the call and poison the shared
	// cache. The checker bounds the fetch with its own timeout.
	status := s.updateChecker.Status(context.Background())
	respondJSON(w, http.StatusOK, status)
}

func (s *Server) handleDiscovery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	discovery := s.vbox.Discover(r.Context())
	respondJSON(w, http.StatusOK, discovery)
}

func (s *Server) handleConsoleProtocols(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	caps := console.Capabilities()
	protocols := make([]models.ConsoleCapability, len(caps))
	for i, c := range caps {
		protocols[i] = models.ConsoleCapability{
			ID:               c.ID,
			DisplayName:      c.DisplayName,
			CanAutoConfigure: c.CanAutoConfigure,
			Description:      c.Description,
		}
	}

	respondJSON(w, http.StatusOK, models.ConsoleProtocolsResponse{Protocols: protocols})
}

func (s *Server) handleLocalStateStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := s.store.Status()
	// Ensure the response never leaks the resolved filesystem path.
	response := map[string]any{
		"configured": status["configured"],
		"available":  status["available"],
		"schema":     status["schema"],
	}
	respondJSON(w, http.StatusOK, response)
}

// handlePickFolder opens the native host folder picker and returns the selected
// absolute path. It is authenticated (mounted under /api) so only the local UI
// can pop a host dialog, and serialized so two dialogs can never race.
func (s *Server) handlePickFolder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.pickMu.Lock()
	defer s.pickMu.Unlock()

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
	defer cancel()

	path, err := hostpick.PickFolder(ctx)
	if err != nil {
		s.logger.Error("host folder picker failed", "error", err)
		http.Error(w, "Could not open the folder picker.", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, models.HostFolderPickResponse{
		Path:      path,
		Cancelled: path == "",
	})
}

func (s *Server) handleActivity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	entries, err := s.store.ListOperations(r.Context(), 100)
	if err != nil {
		s.logger.Error("failed to read activity log", "error", err)
		http.Error(w, "Failed to read activity log.", http.StatusInternalServerError)
		return
	}

	out := make([]models.ActivityEntry, len(entries))
	for i, e := range entries {
		out[i] = models.ActivityEntry{
			VMID:       e.VMID,
			Action:     e.Action,
			Success:    e.Success,
			Message:    e.Message,
			RecordedAt: e.RecordedAt,
		}
	}
	respondJSON(w, http.StatusOK, models.ActivityResponse{Entries: out})
}

func (s *Server) handleVms(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	vms, err := s.vbox.ListVMs(r.Context())
	if err != nil {
		s.handleVboxError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, vms)
}

func (s *Server) handleVboxError(w http.ResponseWriter, err error) {
	switch e := err.(type) {
	case *vbox.ValidationError:
		http.Error(w, e.Message, http.StatusBadRequest)
	case *vbox.NotDiscoveredError:
		http.Error(w, e.Message, http.StatusServiceUnavailable)
	case *vbox.ExecutionError:
		// Log the full error server-side but only return a mapped, sanitized
		// message to avoid leaking host-sensitive paths or raw runner output.
		s.logger.Error("vbox command failed", "message", e.Message, "exitCode", e.ExitCode, "stderr", e.StandardError)
		http.Error(w, sanitizedExecMessage(e), http.StatusBadGateway)
	default:
		s.logger.Error("unexpected error", "error", err)
		http.Error(w, "Internal server error.", http.StatusInternalServerError)
	}
}

// sanitizedExecMessage maps a VBoxManage execution failure to a fixed, safe
// user-facing message. It matches known stderr signatures case-insensitively and
// never echoes host paths, tokens, or the raw runner output. The default keeps
// the exact substring "VirtualBox operation failed" so callers (and tests) can
// rely on it. The real reason is preserved separately in the operation log and
// the server-side error log.
func sanitizedExecMessage(e *vbox.ExecutionError) string {
	stderr := strings.ToLower(e.StandardError)
	contains := func(needle string) bool {
		return strings.Contains(stderr, strings.ToLower(needle))
	}

	switch {
	case contains("already locked") || contains("VBOX_E_INVALID_OBJECT_STATE"):
		return "The VM is busy or locked by another session. Wait a moment and try again."
	case contains("VERR_VMX") || contains("VT-x is not available") || contains("VERR_NEM") || contains("raw-mode is unavailable") || contains("Hyper-V"):
		return "Hardware virtualization is unavailable. Check that Hyper-V or Windows memory integrity is not blocking VirtualBox."
	case contains("VERR_NO_MEMORY") || contains("not enough memory"):
		return "Not enough host memory to start the VM."
	default:
		return fmt.Sprintf("VirtualBox operation failed (exit code %d).", e.ExitCode)
	}
}

func (s *Server) handleVmByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/vms/")
	parts := strings.SplitN(path, "/", 2)
	id := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	if !vbox.IsValidVmID(id) {
		respondJSON(w, http.StatusBadRequest, models.VmOperationResponse{
			Success: false,
			VMID:    id,
			Message: "Invalid VM identifier.",
		})
		return
	}

	switch r.Method {
	case http.MethodGet:
		switch action {
		case "status":
			s.handleVmStatus(w, r, id)
		case "console":
			s.handleVmConsoleStatus(w, r, id)
		case "telemetry":
			s.handleVmTelemetry(w, r, id)
		case "shared-folders":
			s.handleListSharedFolders(w, r, id)
		case "snapshots":
			s.handleListSnapshots(w, r, id)
		case "network":
			s.handleNetworkOptions(w, r, id)
		case "hardware":
			s.handleVmHardware(w, r, id)
		case "guest-os":
			s.handleVmGuestOS(w, r, id)
		case "serial-console":
			s.handleSerialConsoleStatus(w, r, id)
		case "storage":
			s.handleVmStorage(w, r, id)
		case "clipboard":
			s.handleGetClipboardMode(w, r, id)
		case "guest-additions":
			s.handleGuestAdditionsStatus(w, r, id)
		case "screen-stream":
			// VirtualBox COM screen capture streamed to the browser as JPEG
			// frames over a WebSocket. See screenstream.go and vmscreen.
			s.handleVmScreenStream(w, r, id)
		case "serial-stream":
			// COM1 serial port (VirtualBox host named pipe) bridged to the
			// browser terminal over a WebSocket. See serialstream.go.
			s.handleVmSerialStream(w, r, id)
		default:
			http.NotFound(w, r)
		}
	case http.MethodPost:
		switch action {
		case "start":
			s.handleVmStart(w, r, id)
		case "stop":
			s.handleVmStop(w, r, id)
		case "reset":
			s.handleVmReset(w, r, id)
		case "poweroff":
			s.handleVmPowerOff(w, r, id)
		case "console/prepare":
			s.handleVmConsolePrepare(w, r, id)
		case "console/disable":
			s.handleVmConsoleDisable(w, r, id)
		case "serial-console/enable":
			s.handleEnableSerialConsole(w, r, id)
		case "serial-console/disable":
			s.handleDisableSerialConsole(w, r, id)
		case "serial-console/enable-getty":
			s.handleEnableSerialGetty(w, r, id)
		case "shared-folders":
			s.handleAddSharedFolder(w, r, id)
		case "shared-folders/remove":
			s.handleRemoveSharedFolder(w, r, id)
		case "files":
			s.handleTransferFile(w, r, id)
		case "snapshots":
			s.handleTakeSnapshot(w, r, id)
		case "snapshots/restore":
			s.handleRestoreSnapshot(w, r, id)
		case "snapshots/delete":
			s.handleDeleteSnapshot(w, r, id)
		case "network":
			s.handleChangeNetworkMode(w, r, id)
		case "network/forwarding":
			s.handleAddPortForwarding(w, r, id)
		case "network/forwarding/delete":
			s.handleDeletePortForwarding(w, r, id)
		case "hardware":
			s.handleSetVmHardware(w, r, id)
		case "storage/resize":
			s.handleResizeDisk(w, r, id)
		case "storage/add":
			s.handleAddDisk(w, r, id)
		case "storage/detach":
			s.handleDetachDisk(w, r, id)
		case "clipboard":
			s.handleSetClipboardMode(w, r, id)
		case "guest-additions/install":
			s.handleInstallGuestAdditions(w, r, id)
		case "guest-additions/update":
			s.handleUpdateGuestAdditions(w, r, id)
		case "clone":
			s.handleCloneVm(w, r, id)
		case "export":
			s.handleExportVm(w, r, id)
		default:
			http.NotFound(w, r)
		}
	case http.MethodDelete:
		if action != "" {
			http.NotFound(w, r)
			return
		}
		s.handleVmDelete(w, r, id)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleVmStatus(w http.ResponseWriter, r *http.Request, id string) {
	status, err := s.vbox.VMStatus(r.Context(), id)
	if err != nil {
		s.handleVboxError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, status)
}

func (s *Server) handleVmStart(w http.ResponseWriter, r *http.Request, id string) {
	s.handleVmOperation(w, r, id, s.vbox.StartVM, "VM start requested.")
}

func (s *Server) handleVmStop(w http.ResponseWriter, r *http.Request, id string) {
	s.handleVmOperation(w, r, id, s.vbox.StopVM, "VM stop requested. ACPI shutdown signal sent.")
}

func (s *Server) handleVmReset(w http.ResponseWriter, r *http.Request, id string) {
	s.handleVmOperation(w, r, id, s.vbox.ResetVM, "VM reset requested.")
}

func (s *Server) handleVmPowerOff(w http.ResponseWriter, r *http.Request, id string) {
	s.handleVmOperation(w, r, id, s.vbox.ForcePowerOff, "VM power off forced.")
}

func (s *Server) handleVmConsoleStatus(w http.ResponseWriter, r *http.Request, id string) {
	status, err := s.vbox.VmConsoleStatus(r.Context(), id)
	if err != nil {
		s.handleVboxError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, status)
}

func (s *Server) handleVmTelemetry(w http.ResponseWriter, r *http.Request, id string) {
	telemetry, err := s.vbox.VmTelemetry(r.Context(), id)
	if err != nil {
		s.handleVboxError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, telemetry)
}

func (s *Server) handleListSharedFolders(w http.ResponseWriter, r *http.Request, id string) {
	folders, err := s.vbox.ListSharedFolders(r.Context(), id)
	if err != nil {
		s.handleVboxError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, folders)
}

// sharedFolderAddRequest is the JSON body for adding a shared folder.
type sharedFolderAddRequest struct {
	Name     string `json:"name"`
	HostPath string `json:"hostPath"`
}

// sharedFolderRemoveRequest is the JSON body for removing a shared folder.
type sharedFolderRemoveRequest struct {
	Name string `json:"name"`
}

func (s *Server) handleAddSharedFolder(w http.ResponseWriter, r *http.Request, id string) {
	var body sharedFolderAddRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		return
	}

	unlock, ok := s.tryLockVm(id)
	if !ok {
		respondJSON(w, http.StatusConflict, models.SharedFolderOperationResponse{
			Success: false,
			VMID:    id,
			Message: "Another lifecycle operation is already in progress for this VM.",
		})
		return
	}
	defer unlock()

	resp, err := s.vbox.AddSharedFolder(r.Context(), id, body.Name, body.HostPath)
	if err != nil {
		s.handleVboxError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleRemoveSharedFolder(w http.ResponseWriter, r *http.Request, id string) {
	var body sharedFolderRemoveRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		return
	}

	unlock, ok := s.tryLockVm(id)
	if !ok {
		respondJSON(w, http.StatusConflict, models.SharedFolderOperationResponse{
			Success: false,
			VMID:    id,
			Message: "Another lifecycle operation is already in progress for this VM.",
		})
		return
	}
	defer unlock()

	resp, err := s.vbox.RemoveSharedFolder(r.Context(), id, body.Name)
	if err != nil {
		s.handleVboxError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

// decodeJSONBody decodes a small JSON request body, rejecting unknown fields and
// oversized payloads. It writes a 400 response and returns an error when the body
// is invalid so callers can simply return.
func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 8*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		http.Error(w, "Invalid request body.", http.StatusBadRequest)
		return err
	}
	return nil
}

// maxUploadBytes caps a single drag-drop file transfer so a huge upload cannot
// exhaust host memory or disk while it is staged.
const maxUploadBytes = 256 << 20 // 256 MiB

// handleTransferFile accepts a multipart file upload dropped onto a VM and hands
// it to the transfer service, which decides between writing into a shared folder
// or copying it in via guest control. Optional username/password form fields are
// only used for the guest-control fallback.
func (s *Server) handleTransferFile(w http.ResponseWriter, r *http.Request, id string) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		http.Error(w, "Invalid or oversized upload (max 256 MB).", http.StatusBadRequest)
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "No file was uploaded.", http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Could not read the uploaded file.", http.StatusBadRequest)
		return
	}

	unlock, ok := s.tryLockVm(id)
	if !ok {
		respondJSON(w, http.StatusConflict, models.VmFileTransferResponse{
			Success: false,
			VMID:    id,
			Message: "Another operation is already in progress for this VM.",
		})
		return
	}
	defer unlock()

	resp, err := s.vbox.TransferFileToGuest(r.Context(), id, header.Filename, data, r.FormValue("username"), r.FormValue("password"))
	if err != nil {
		s.handleVboxError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleNetworkOptions(w http.ResponseWriter, r *http.Request, id string) {
	resp, err := s.vbox.NetworkOptions(r.Context(), id)
	if err != nil {
		s.handleVboxError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleChangeNetworkMode(w http.ResponseWriter, r *http.Request, id string) {
	var body models.NetworkModeRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		return
	}

	unlock, ok := s.tryLockVm(id)
	if !ok {
		respondJSON(w, http.StatusConflict, models.NetworkOperationResponse{
			Success: false,
			VMID:    id,
			Message: "Another operation is already in progress for this VM.",
		})
		return
	}
	defer unlock()

	resp, err := s.vbox.ChangeNetworkMode(r.Context(), id, body.Slot, body.Mode, body.Adapter)
	if err != nil {
		s.handleVboxError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleAddPortForwarding(w http.ResponseWriter, r *http.Request, id string) {
	var body models.PortForwardingRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		return
	}

	unlock, ok := s.tryLockVm(id)
	if !ok {
		respondJSON(w, http.StatusConflict, models.NetworkOperationResponse{
			Success: false,
			VMID:    id,
			Message: "Another operation is already in progress for this VM.",
		})
		return
	}
	defer unlock()

	resp, err := s.vbox.AddPortForwarding(r.Context(), id, body)
	if err != nil {
		s.handleVboxError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleDeletePortForwarding(w http.ResponseWriter, r *http.Request, id string) {
	var body models.PortForwardingDeleteRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		return
	}

	unlock, ok := s.tryLockVm(id)
	if !ok {
		respondJSON(w, http.StatusConflict, models.NetworkOperationResponse{
			Success: false,
			VMID:    id,
			Message: "Another operation is already in progress for this VM.",
		})
		return
	}
	defer unlock()

	resp, err := s.vbox.DeletePortForwarding(r.Context(), id, body.Slot, body.Name)
	if err != nil {
		s.handleVboxError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleListSnapshots(w http.ResponseWriter, r *http.Request, id string) {
	resp, err := s.vbox.ListSnapshots(r.Context(), id)
	if err != nil {
		s.handleVboxError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleTakeSnapshot(w http.ResponseWriter, r *http.Request, id string) {
	var body models.SnapshotTakeRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		return
	}
	s.runSnapshotMutation(w, id, func() (models.SnapshotOperationResponse, error) {
		return s.vbox.TakeSnapshot(r.Context(), id, body.Name, body.Description)
	})
}

func (s *Server) handleRestoreSnapshot(w http.ResponseWriter, r *http.Request, id string) {
	var body models.SnapshotRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		return
	}
	s.runSnapshotMutation(w, id, func() (models.SnapshotOperationResponse, error) {
		return s.vbox.RestoreSnapshot(r.Context(), id, body.UUID)
	})
}

func (s *Server) handleDeleteSnapshot(w http.ResponseWriter, r *http.Request, id string) {
	var body models.SnapshotRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		return
	}
	s.runSnapshotMutation(w, id, func() (models.SnapshotOperationResponse, error) {
		return s.vbox.DeleteSnapshot(r.Context(), id, body.UUID)
	})
}

func (s *Server) handleVmHardware(w http.ResponseWriter, r *http.Request, id string) {
	resp, err := s.vbox.VmHardware(r.Context(), id)
	if err != nil {
		s.handleVboxError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleVmGuestOS(w http.ResponseWriter, r *http.Request, id string) {
	resp, err := s.vbox.VmGuestOS(r.Context(), id)
	if err != nil {
		s.handleVboxError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleSerialConsoleStatus(w http.ResponseWriter, r *http.Request, id string) {
	resp, err := s.vbox.SerialConsoleStatus(r.Context(), id)
	if err != nil {
		s.handleVboxError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleEnableSerialConsole(w http.ResponseWriter, r *http.Request, id string) {
	unlock, ok := s.tryLockVm(id)
	if !ok {
		respondJSON(w, http.StatusConflict, models.VmOperationResponse{
			Success: false,
			VMID:    id,
			Message: "Another operation is already in progress for this VM.",
		})
		return
	}
	defer unlock()

	resp, err := s.vbox.EnableSerialConsole(r.Context(), id)
	if err != nil {
		s.handleVboxError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleDisableSerialConsole(w http.ResponseWriter, r *http.Request, id string) {
	unlock, ok := s.tryLockVm(id)
	if !ok {
		respondJSON(w, http.StatusConflict, models.VmOperationResponse{
			Success: false,
			VMID:    id,
			Message: "Another operation is already in progress for this VM.",
		})
		return
	}
	defer unlock()

	resp, err := s.vbox.DisableSerialConsole(r.Context(), id)
	if err != nil {
		s.handleVboxError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleEnableSerialGetty(w http.ResponseWriter, r *http.Request, id string) {
	var req models.SerialGettyRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		return
	}

	unlock, ok := s.tryLockVm(id)
	if !ok {
		respondJSON(w, http.StatusConflict, models.SerialGettyResponse{
			Success: false,
			VMID:    id,
			Message: "Another operation is already in progress for this VM.",
		})
		return
	}
	defer unlock()

	resp, err := s.vbox.EnableSerialGetty(r.Context(), id, req.Username, req.Password)
	if err != nil {
		s.handleVboxError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleSetVmHardware(w http.ResponseWriter, r *http.Request, id string) {
	var body models.VmHardwareRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		return
	}

	unlock, ok := s.tryLockVm(id)
	if !ok {
		respondJSON(w, http.StatusConflict, models.VmOperationResponse{
			Success: false,
			VMID:    id,
			Message: "Another operation is already in progress for this VM.",
		})
		return
	}
	defer unlock()

	resp, err := s.vbox.SetVmHardware(r.Context(), id, body.CPUs, body.MemoryMB)
	if err != nil {
		s.handleVboxError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleVmStorage(w http.ResponseWriter, r *http.Request, id string) {
	resp, err := s.vbox.VmStorage(r.Context(), id)
	if err != nil {
		s.handleVboxError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleResizeDisk(w http.ResponseWriter, r *http.Request, id string) {
	var body models.DiskResizeRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		return
	}

	unlock, ok := s.tryLockVm(id)
	if !ok {
		respondJSON(w, http.StatusConflict, models.VmOperationResponse{
			Success: false,
			VMID:    id,
			Message: "Another operation is already in progress for this VM.",
		})
		return
	}
	defer unlock()

	resp, err := s.vbox.ResizeDisk(r.Context(), id, body.UUID, body.SizeMB)
	if err != nil {
		s.handleVboxError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleAddDisk(w http.ResponseWriter, r *http.Request, id string) {
	var body models.DiskAddRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		return
	}

	unlock, ok := s.tryLockVm(id)
	if !ok {
		respondJSON(w, http.StatusConflict, models.VmOperationResponse{
			Success: false,
			VMID:    id,
			Message: "Another operation is already in progress for this VM.",
		})
		return
	}
	defer unlock()

	resp, err := s.vbox.AddDisk(r.Context(), id, body.SizeMB)
	if err != nil {
		s.handleVboxError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleDetachDisk(w http.ResponseWriter, r *http.Request, id string) {
	var body models.DiskDetachRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		return
	}

	unlock, ok := s.tryLockVm(id)
	if !ok {
		respondJSON(w, http.StatusConflict, models.VmOperationResponse{
			Success: false,
			VMID:    id,
			Message: "Another operation is already in progress for this VM.",
		})
		return
	}
	defer unlock()

	resp, err := s.vbox.DetachDisk(r.Context(), id, body.UUID, body.DeleteFile)
	if err != nil {
		s.handleVboxError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

// handleVmDelete removes a VM and its files. The vbox service refuses a live
// VM, and the per-VM lock keeps the delete from racing a start/stop/snapshot.
func (s *Server) handleVmDelete(w http.ResponseWriter, r *http.Request, id string) {
	unlock, ok := s.tryLockVm(id)
	if !ok {
		respondJSON(w, http.StatusConflict, models.VmOperationResponse{
			Success: false,
			VMID:    id,
			Message: "Another operation is already in progress for this VM.",
		})
		return
	}
	defer unlock()

	resp, err := s.vbox.DeleteVM(r.Context(), id)
	if err != nil {
		s.handleVboxError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

// runSnapshotMutation serializes a snapshot operation with the VM's other
// lifecycle operations (a snapshot restore powers the VM off, so it must not
// race a start/stop) and writes the JSON response.
func (s *Server) runSnapshotMutation(w http.ResponseWriter, id string, op func() (models.SnapshotOperationResponse, error)) {
	unlock, ok := s.tryLockVm(id)
	if !ok {
		respondJSON(w, http.StatusConflict, models.SnapshotOperationResponse{
			Success: false,
			VMID:    id,
			Message: "Another operation is already in progress for this VM.",
		})
		return
	}
	defer unlock()

	resp, err := op()
	if err != nil {
		s.handleVboxError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleGetClipboardMode(w http.ResponseWriter, r *http.Request, id string) {
	resp, err := s.vbox.GetClipboardMode(r.Context(), id)
	if err != nil {
		s.handleVboxError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

// clipboardModeRequest is the JSON body for changing the shared-clipboard mode.
type clipboardModeRequest struct {
	Mode string `json:"mode"`
}

func (s *Server) handleSetClipboardMode(w http.ResponseWriter, r *http.Request, id string) {
	var body clipboardModeRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		return
	}

	unlock, ok := s.tryLockVm(id)
	if !ok {
		respondJSON(w, http.StatusConflict, models.ClipboardModeResponse{ID: id})
		return
	}
	defer unlock()

	resp, err := s.vbox.SetClipboardMode(r.Context(), id, body.Mode)
	if err != nil {
		s.handleVboxError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleGuestAdditionsStatus(w http.ResponseWriter, r *http.Request, id string) {
	resp, err := s.vbox.GuestAdditionsStatus(r.Context(), id)
	if err != nil {
		s.handleVboxError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleInstallGuestAdditions(w http.ResponseWriter, r *http.Request, id string) {
	unlock, ok := s.tryLockVm(id)
	if !ok {
		respondJSON(w, http.StatusConflict, models.GuestAdditionsInstallResponse{
			Success: false,
			VMID:    id,
			Message: "Another lifecycle operation is already in progress for this VM.",
		})
		return
	}
	defer unlock()

	resp, err := s.vbox.InstallGuestAdditions(r.Context(), id)
	if err != nil {
		s.handleVboxError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleUpdateGuestAdditions(w http.ResponseWriter, r *http.Request, id string) {
	var req models.GuestAdditionsUpdateRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		return
	}

	unlock, ok := s.tryLockVm(id)
	if !ok {
		respondJSON(w, http.StatusConflict, models.GuestAdditionsUpdateResponse{
			Success: false,
			VMID:    id,
			Message: "Another lifecycle operation is already in progress for this VM.",
		})
		return
	}
	defer unlock()

	resp, err := s.vbox.UpdateGuestAdditions(r.Context(), id, req.Username, req.Password)
	if err != nil {
		s.handleVboxError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleVmConsolePrepare(w http.ResponseWriter, r *http.Request, id string) {
	unlock, ok := s.tryLockVm(id)
	if !ok {
		respondJSON(w, http.StatusConflict, models.VmConsoleOperationResponse{
			Success: false,
			VMID:    id,
			Message: "Another lifecycle operation is already in progress for this VM.",
		})
		return
	}
	defer unlock()

	status, err := s.vbox.PrepareVmConsole(r.Context(), id)
	if err != nil {
		s.handleVboxError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, status)
}

func (s *Server) handleVmConsoleDisable(w http.ResponseWriter, r *http.Request, id string) {
	unlock, ok := s.tryLockVm(id)
	if !ok {
		respondJSON(w, http.StatusConflict, models.VmConsoleOperationResponse{
			Success: false,
			VMID:    id,
			Message: "Another lifecycle operation is already in progress for this VM.",
		})
		return
	}
	defer unlock()

	err := s.vbox.DisableVmConsole(r.Context(), id)
	if err != nil {
		s.handleVboxError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, models.VmConsoleOperationResponse{
		Success: true,
		VMID:    id,
		Message: "VRDE console disabled.",
	})
}

func (s *Server) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := s.resolveToken()
		if token == "" {
			s.logger.Error("session token is not configured")
			http.Error(w, "Session token is not configured.", http.StatusUnauthorized)
			return
		}

		presented := r.Header.Get(sessionTokenHeader)
		if presented == "" && (isScreenStreamPath(r.URL.Path) || isSerialStreamPath(r.URL.Path)) {
			// Browsers' native WebSocket API cannot set arbitrary request
			// headers, so a WebSocket upgrade request cannot carry
			// X-TabVM-Session-Token. Fall back to a query parameter for this
			// WebSocket route only. This is still the same token, resolved
			// the same way, compared the same way -- only the transport
			// differs. See screenstream.go.
			presented = r.URL.Query().Get(screenStreamTokenQueryParam)
		}

		if presented == "" || !constantTimeEqual(presented, token) {
			http.Error(w, "Invalid or missing session token.", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) resolveToken() string {
	if s.cfg.SessionToken != "" {
		return s.cfg.SessionToken
	}

	if s.cfg.IsDevelopment() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.devToken == "" {
			bytes := make([]byte, 32)
			if _, err := rand.Read(bytes); err != nil {
				s.logger.Error("failed to generate dev token", "error", err)
				return ""
			}
			s.devToken = hex.EncodeToString(bytes)
			s.logger.Warn(
				"TABVM_AGENT_SESSION_TOKEN is not configured; using a temporary development token. Use scripts/dev-start.ps1 or set TABVM_AGENT_SESSION_TOKEN explicitly; do not rely on this fallback in production.",
			)
		}
		return s.devToken
	}

	return ""
}

func (s *Server) tryLockVm(id string) (func(), bool) {
	s.opMu.Lock()
	if s.ops == nil {
		s.ops = make(map[string]*sync.Mutex)
	}
	mu, ok := s.ops[id]
	if !ok {
		mu = &sync.Mutex{}
		s.ops[id] = mu
	}
	s.opMu.Unlock()

	if !mu.TryLock() {
		return nil, false
	}
	return func() { mu.Unlock() }, true
}

func (s *Server) handleVmOperation(w http.ResponseWriter, r *http.Request, id string, op func(context.Context, string) error, successMessage string) {
	unlock, ok := s.tryLockVm(id)
	if !ok {
		respondJSON(w, http.StatusConflict, models.VmOperationResponse{
			Success: false,
			VMID:    id,
			Message: "Another lifecycle operation is already in progress for this VM.",
		})
		return
	}
	defer unlock()

	err := op(r.Context(), id)
	if err != nil {
		s.handleVboxError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, models.VmOperationResponse{
		Success: true,
		VMID:    id,
		Message: successMessage,
	})
}

func constantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare(hashToken(a), hashToken(b)) == 1
}

func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		slog.Default().Error("failed to encode JSON response", "error", err)
	}
}
