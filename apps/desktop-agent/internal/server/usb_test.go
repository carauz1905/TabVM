package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tabvm/desktop-agent/internal/models"
	"github.com/tabvm/desktop-agent/internal/vbox"
)

func TestUsbEndpointListsDevices(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	id := "11111111-1111-1111-1111-111111111111"
	fake.usbResp = models.VmUsbResponse{
		Devices: []models.UsbDevice{
			{UUID: "2b7e1a10-1234-4abc-8def-0123456789ab", VendorID: "0x0781", ProductID: "0x5567", Manufacturer: "SanDisk", Product: "Cruzer Blade", State: "Available"},
		},
		ExtensionPackInstalled: true,
		USBControllerEnabled:   true,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/vms/"+id+"/usb", nil)
	req.Header.Set("X-TabVM-Session-Token", "secret")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (body %q)", http.StatusOK, rr.Code, rr.Body.String())
	}
	if fake.lastAction != "vmUsb" || fake.lastID != id {
		t.Fatalf("expected vmUsb on %s, got %s on %s", id, fake.lastAction, fake.lastID)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Cruzer Blade") || !strings.Contains(body, `"extensionPackInstalled":true`) {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestUsbAttachEndpointCapturesDevice(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	id := "11111111-1111-1111-1111-111111111111"
	uuid := "2b7e1a10-1234-4abc-8def-0123456789ab"
	fake.usbOp = models.UsbOperationResponse{Success: true, VMID: id, Message: "USB device attached to the VM."}

	req := httptest.NewRequest(http.MethodPost, "/api/vms/"+id+"/usb/attach",
		strings.NewReader(`{"deviceUuid":"`+uuid+`"}`))
	req.Header.Set("X-TabVM-Session-Token", "secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (body %q)", http.StatusOK, rr.Code, rr.Body.String())
	}
	if fake.lastAction != "attachUsb" || fake.lastID != id || fake.lastUsbDevice != uuid {
		t.Fatalf("expected attachUsb(%s,%s), got %s(%s,%s)", id, uuid, fake.lastAction, fake.lastID, fake.lastUsbDevice)
	}
	if !strings.Contains(rr.Body.String(), `"success":true`) {
		t.Fatalf("expected success response, got %q", rr.Body.String())
	}
}

func TestUsbDetachEndpointReleasesDevice(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	id := "11111111-1111-1111-1111-111111111111"
	uuid := "2b7e1a10-1234-4abc-8def-0123456789ab"
	fake.usbOp = models.UsbOperationResponse{Success: true, VMID: id, Message: "USB device detached from the VM."}

	req := httptest.NewRequest(http.MethodPost, "/api/vms/"+id+"/usb/detach",
		strings.NewReader(`{"deviceUuid":"`+uuid+`"}`))
	req.Header.Set("X-TabVM-Session-Token", "secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (body %q)", http.StatusOK, rr.Code, rr.Body.String())
	}
	if fake.lastAction != "detachUsb" || fake.lastUsbDevice != uuid {
		t.Fatalf("expected detachUsb with %s, got %s with %s", uuid, fake.lastAction, fake.lastUsbDevice)
	}
}

func TestUsbAttachEndpointMapsValidationTo400(t *testing.T) {
	srv, fake := newTestServer(t, "secret")
	id := "11111111-1111-1111-1111-111111111111"
	fake.usbOpErr = &vbox.ValidationError{Message: "The VM must be running to attach or detach USB devices."}

	req := httptest.NewRequest(http.MethodPost, "/api/vms/"+id+"/usb/attach",
		strings.NewReader(`{"deviceUuid":"2b7e1a10-1234-4abc-8def-0123456789ab"}`))
	req.Header.Set("X-TabVM-Session-Token", "secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestUsbAttachEndpointRejectsInvalidBody(t *testing.T) {
	srv, _ := newTestServer(t, "secret")
	id := "11111111-1111-1111-1111-111111111111"

	req := httptest.NewRequest(http.MethodPost, "/api/vms/"+id+"/usb/attach",
		strings.NewReader(`{"deviceUuid":`))
	req.Header.Set("X-TabVM-Session-Token", "secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d for malformed body, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestUsbEndpointsRequireAuth(t *testing.T) {
	srv, _ := newTestServer(t, "secret")
	id := "11111111-1111-1111-1111-111111111111"

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/vms/" + id + "/usb"},
		{http.MethodPost, "/api/vms/" + id + "/usb/attach"},
		{http.MethodPost, "/api/vms/" + id + "/usb/detach"},
	}
	for _, c := range cases {
		req := httptest.NewRequest(c.method, c.path, strings.NewReader(`{"deviceUuid":"2b7e1a10-1234-4abc-8def-0123456789ab"}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		srv.Handler().ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for %s %s without a token, got %d", c.method, c.path, rr.Code)
		}
	}
}
