package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/tabvm/desktop-agent/internal/hostpick"
	"github.com/tabvm/desktop-agent/internal/models"
	"github.com/tabvm/desktop-agent/internal/vbox"
)

// createJobTimeout bounds a whole background import/create, matching the longest
// VBoxManage step (appliance import) with headroom.
const createJobTimeout = 35 * time.Minute

// handlePickFile opens the native host file picker (for .ova/.iso selection) and
// returns the chosen absolute path. Authenticated and serialized like the folder
// picker so two dialogs can never race.
func (s *Server) handlePickFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.pickMu.Lock()
	defer s.pickMu.Unlock()

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
	defer cancel()

	path, err := hostpick.PickFile(ctx)
	if err != nil {
		s.logger.Error("host file picker failed", "error", err)
		http.Error(w, "Could not open the file picker.", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, models.HostFilePickResponse{
		Path:      path,
		Cancelled: path == "",
	})
}

// handleImportVm accepts an appliance import request and starts it as a
// background job, returning the job id to poll.
func (s *Server) handleImportVm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body models.VmImportRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		return
	}

	jobID := s.startCreateJob(body.Name, func(ctx context.Context) (models.VmCreateResponse, error) {
		return s.vbox.ImportAppliance(ctx, body.OvaPath, body.Name)
	})
	respondJSON(w, http.StatusAccepted, models.VmCreateJobResponse{JobID: jobID})
}

// handleCreateVm accepts an unattended create request and starts it as a
// background job, returning the job id to poll.
func (s *Server) handleCreateVm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body models.VmCreateRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		return
	}

	jobID := s.startCreateJob(body.Name, func(ctx context.Context) (models.VmCreateResponse, error) {
		return s.vbox.CreateVmUnattended(ctx, body)
	})
	respondJSON(w, http.StatusAccepted, models.VmCreateJobResponse{JobID: jobID})
}

// handleCreateVmManual accepts a manual-install create request (VM + disk +
// installer ISO attached as a DVD, no unattended setup) and starts it as a
// background job, returning the job id to poll.
func (s *Server) handleCreateVmManual(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body models.VmCreateManualRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		return
	}

	jobID := s.startCreateJob(body.Name, func(ctx context.Context) (models.VmCreateResponse, error) {
		return s.vbox.CreateVmManual(ctx, body)
	})
	respondJSON(w, http.StatusAccepted, models.VmCreateJobResponse{JobID: jobID})
}

// handleCloneVm clones a stopped source VM as a background job. It validates the
// request synchronously (source powered off, valid name, and — for a linked
// clone — an existing snapshot) so the user gets an immediate 4xx on a bad
// request, then starts the clone on the shared create-job store and returns the
// job id to poll via the create status endpoint.
func (s *Server) handleCloneVm(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body models.VmCloneRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		return
	}

	// Validate synchronously so a running source, bad name, or linked clone
	// without a snapshot is rejected now rather than surfacing as a failed job.
	if err := s.vbox.ValidateClone(r.Context(), id, body.Name, body.Linked); err != nil {
		s.handleVboxError(w, err)
		return
	}

	name := body.Name
	linked := body.Linked
	jobID := s.startCreateJob(name, func(ctx context.Context) (models.VmCreateResponse, error) {
		return s.vbox.CloneVM(ctx, id, name, linked)
	})
	respondJSON(w, http.StatusAccepted, models.VmCreateJobResponse{JobID: jobID})
}

// handleExportVm exports a stopped VM to an .ova appliance as a background job.
// It validates the request synchronously (source powered off, a valid
// destination directory, and no existing file to clobber) so the user gets an
// immediate 4xx on a bad request, then starts the export on the shared
// create-job store and returns the job id to poll via the create status
// endpoint.
func (s *Server) handleExportVm(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body models.VmExportRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		return
	}

	// Validate synchronously so a running source, an invalid directory, or a
	// clobbering target is rejected now rather than surfacing as a failed job.
	if err := s.vbox.ValidateExport(r.Context(), id, body.Directory); err != nil {
		s.handleVboxError(w, err)
		return
	}

	directory := body.Directory
	jobID := s.startCreateJob("", func(ctx context.Context) (models.VmCreateResponse, error) {
		return s.vbox.ExportVM(ctx, id, directory)
	})
	respondJSON(w, http.StatusAccepted, models.VmCreateJobResponse{JobID: jobID})
}

// handleCreateStatus returns the current state of a background create/import job.
func (s *Server) handleCreateStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	jobID := r.URL.Query().Get("job")
	s.createMu.Lock()
	job, ok := s.createJobs[jobID]
	var snapshot createJob
	if ok {
		snapshot = *job
	}
	s.createMu.Unlock()
	if !ok {
		http.Error(w, "Unknown job.", http.StatusNotFound)
		return
	}
	respondJSON(w, http.StatusOK, models.VmCreateStatusResponse{
		State:   snapshot.State,
		Message: snapshot.Message,
		VMID:    snapshot.VMID,
		Name:    snapshot.Name,
	})
}

// startCreateJob registers a running job and runs work in the background with its
// own timeout (the request context dies once the 202 response is written). It
// returns the new job id. Errors are mapped to user-safe messages.
func (s *Server) startCreateJob(name string, work func(ctx context.Context) (models.VmCreateResponse, error)) string {
	jobID := newJobID()
	s.createMu.Lock()
	s.createJobs[jobID] = &createJob{State: "running", Name: name}
	s.createMu.Unlock()

	go func() {
		// A panic anywhere in the create call chain must not kill the whole
		// agent: mark the job failed with a generic message and keep serving.
		defer func() {
			if r := recover(); r != nil {
				s.logger.Error("background create/import job panicked", "jobId", jobID, "panic", r)
				s.createMu.Lock()
				defer s.createMu.Unlock()
				if job := s.createJobs[jobID]; job != nil {
					job.State = "error"
					job.Message = "Internal server error."
				}
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), createJobTimeout)
		defer cancel()
		resp, err := work(ctx)

		s.createMu.Lock()
		defer s.createMu.Unlock()
		job := s.createJobs[jobID]
		if job == nil {
			return
		}
		if err != nil {
			job.State = "error"
			job.Message = s.jobErrorMessage(err)
			return
		}
		job.State = "done"
		job.Message = resp.Message
		job.VMID = resp.VMID
		if resp.Name != "" {
			job.Name = resp.Name
		}
	}()

	return jobID
}

// jobErrorMessage converts a service error into a user-safe message, mirroring
// handleVboxError but for the async job channel.
func (s *Server) jobErrorMessage(err error) string {
	switch e := err.(type) {
	case *vbox.ValidationError:
		return e.Message
	case *vbox.NotDiscoveredError:
		return e.Message
	case *vbox.ExecutionError:
		s.logger.Error("vbox create/import failed", "message", e.Message, "exitCode", e.ExitCode, "stderr", e.StandardError)
		return "VirtualBox operation failed."
	default:
		s.logger.Error("unexpected create/import error", "error", err)
		return "Internal server error."
	}
}

// newJobID returns a random hex identifier for a background job.
func newJobID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		// Fall back to a timestamp-derived id; collisions are extremely unlikely
		// given single-user local use.
		return "job-" + time.Now().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(b)
}
