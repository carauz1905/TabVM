// Package vmscreen captures a running VirtualBox VM's screen directly through
// the VirtualBox COM API (VBoxSVC), with no dependency on VRDE/RDP.
//
// It replaces the abandoned IronRDP-over-VRDE browser-console spike. See the
// architecture memory "Pivot browser console: IronRDP+VRDE -> VirtualBox COM
// screenshot-stream" for the rationale and the measured latency data
// (TakeScreenShotToArray ~15-20ms/frame at 1280x720 => 46-67fps).
//
// COM affinity: all COM interaction happens on a single dedicated OS thread
// (the run loop below), which owns the apartment and holds a shared session
// lock on the VM for the capturer's lifetime. Callers interact only through
// the channel-based Grab/Close methods, which are safe to call from any
// goroutine.
package vmscreen

import (
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"syscall"
	"time"
	"unsafe"

	ole "github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

// bitmapFormatRGBA is the VirtualBox BitmapFormat 'RGBA' fourcc. The returned
// byte stream is R,G,B,A order, which matches Go's image.RGBA pixel layout
// directly (no channel swap needed).
const bitmapFormatRGBA = 0x41424752

// lockTypeShared is VirtualBox LockType::Shared, used to attach to a VM that
// is already running (started elsewhere) without taking exclusive control.
const lockTypeShared = 1

var (
	oleaut32              = syscall.NewLazyDLL("oleaut32.dll")
	procSafeArrayAccess   = oleaut32.NewProc("SafeArrayAccessData")
	procSafeArrayUnaccess = oleaut32.NewProc("SafeArrayUnaccessData")
)

// Frame is a single captured screen image in RGBA order.
type Frame struct {
	Pix    []byte
	Width  int
	Height int
}

type grabResult struct {
	frame Frame
	err   error
}

// input command kinds sent to the COM thread.
const (
	inputScancodes = 1
	inputMouseAbs  = 2
)

type inputCmd struct {
	kind        int
	scancodes   []int
	x, y        int
	dz, dw      int
	buttonState int
	resp        chan error
}

// Capturer owns a COM apartment + shared session lock on one VM and serves
// frame grabs sequentially on its dedicated thread.
type Capturer struct {
	grabCh  chan chan grabResult
	inputCh chan inputCmd
	closeCh chan struct{}
	doneCh  chan struct{}

	width  int
	height int
}

// New starts a capturer for the given VM. vboxManagePath is used once, up
// front, to read the guest's true resolution from a screenshot PNG header
// (VBoxManage captures at native resolution, avoiding VirtualBox-side
// scaling). It returns an error if COM setup or the session lock fails (for
// example, if the VM is not running).
func New(vboxManagePath, vmID string) (*Capturer, error) {
	w, h, err := detectResolution(vboxManagePath, vmID)
	if err != nil {
		return nil, fmt.Errorf("detect guest resolution: %w", err)
	}

	c := &Capturer{
		grabCh:  make(chan chan grabResult),
		inputCh: make(chan inputCmd),
		closeCh: make(chan struct{}),
		doneCh:  make(chan struct{}),
		width:   w,
		height:  h,
	}

	ready := make(chan error, 1)
	go c.run(vmID, ready)
	if err := <-ready; err != nil {
		return nil, err
	}
	return c, nil
}

// Size returns the capture resolution.
func (c *Capturer) Size() (int, int) { return c.width, c.height }

// Grab returns the current screen frame. Safe to call from any goroutine;
// grabs are serialized onto the COM thread.
func (c *Capturer) Grab() (Frame, error) {
	respCh := make(chan grabResult, 1)
	select {
	case c.grabCh <- respCh:
		res := <-respCh
		return res.frame, res.err
	case <-c.doneCh:
		return Frame{}, fmt.Errorf("capturer closed")
	}
}

// Close releases the session lock and tears down the COM apartment.
func (c *Capturer) Close() {
	select {
	case <-c.doneCh:
		return
	default:
	}
	close(c.closeCh)
	<-c.doneCh
}

func (c *Capturer) run(vmID string, ready chan error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// S_FALSE (already initialized on this thread) is reported as an error by
	// go-ole but is harmless; the capturer thread is dedicated, so a fresh
	// apartment is expected.
	_ = ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED)
	defer ole.CoUninitialize()

	vboxUnk, err := oleutil.CreateObject("VirtualBox.VirtualBox")
	if err != nil {
		ready <- fmt.Errorf("create VirtualBox COM object: %w", err)
		return
	}
	vbox, err := vboxUnk.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		ready <- fmt.Errorf("query IDispatch on VirtualBox: %w", err)
		return
	}
	defer vbox.Release()

	machineV, err := oleutil.CallMethod(vbox, "FindMachine", vmID)
	if err != nil {
		ready <- fmt.Errorf("FindMachine %q: %w", vmID, err)
		return
	}
	machine := machineV.ToIDispatch()
	defer machine.Release()

	sessionUnk, err := oleutil.CreateObject("VirtualBox.Session")
	if err != nil {
		ready <- fmt.Errorf("create Session COM object: %w", err)
		return
	}
	session, err := sessionUnk.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		ready <- fmt.Errorf("query IDispatch on Session: %w", err)
		return
	}
	defer session.Release()

	if _, err := oleutil.CallMethod(machine, "LockMachine", session, lockTypeShared); err != nil {
		ready <- fmt.Errorf("LockMachine (is the VM running?): %w", err)
		return
	}
	defer oleutil.CallMethod(session, "UnlockMachine")

	consoleV, err := oleutil.GetProperty(session, "Console")
	if err != nil {
		ready <- fmt.Errorf("get Console: %w", err)
		return
	}
	console := consoleV.ToIDispatch()
	defer console.Release()

	displayV, err := oleutil.GetProperty(console, "Display")
	if err != nil {
		ready <- fmt.Errorf("get Display: %w", err)
		return
	}
	display := displayV.ToIDispatch()
	defer display.Release()

	keyboardV, err := oleutil.GetProperty(console, "Keyboard")
	if err != nil {
		ready <- fmt.Errorf("get Keyboard: %w", err)
		return
	}
	keyboard := keyboardV.ToIDispatch()
	defer keyboard.Release()

	mouseV, err := oleutil.GetProperty(console, "Mouse")
	if err != nil {
		ready <- fmt.Errorf("get Mouse: %w", err)
		return
	}
	mouse := mouseV.ToIDispatch()
	defer mouse.Release()

	ready <- nil
	c.loop(display, keyboard, mouse)
}

func (c *Capturer) loop(display, keyboard, mouse *ole.IDispatch) {
	defer close(c.doneCh)
	n := c.width * c.height * 4
	// Force the guest to redraw its framebuffer periodically. On a headless or
	// detached VM the framebuffer can stop updating until a real viewer attaches
	// — opening the VirtualBox window "unfreezes" it. InvalidateAndUpdate is the
	// programmatic equivalent of that attach-and-redraw, so the stream stays live
	// (e.g. across the login->desktop transition) without needing the GUI.
	redraw := time.NewTicker(time.Second)
	defer redraw.Stop()
	oleutil.CallMethod(display, "InvalidateAndUpdate") // nudge so the first frames are current
	for {
		select {
		case respCh := <-c.grabCh:
			frame, err := grab(display, c.width, c.height, n)
			respCh <- grabResult{frame: frame, err: err}
		case cmd := <-c.inputCh:
			cmd.resp <- applyInput(keyboard, mouse, cmd)
		case <-redraw.C:
			oleutil.CallMethod(display, "InvalidateAndUpdate")
		case <-c.closeCh:
			return
		}
	}
}

func applyInput(keyboard, mouse *ole.IDispatch, cmd inputCmd) error {
	switch cmd.kind {
	case inputScancodes:
		for _, sc := range cmd.scancodes {
			if _, err := oleutil.CallMethod(keyboard, "PutScancode", sc); err != nil {
				return fmt.Errorf("PutScancode: %w", err)
			}
		}
		return nil
	case inputMouseAbs:
		// PutMouseEventAbsolute(x, y, dz, dw, buttonState). Coordinates are
		// 1-based within the guest resolution; the caller clamps them.
		if _, err := oleutil.CallMethod(mouse, "PutMouseEventAbsolute",
			cmd.x, cmd.y, cmd.dz, cmd.dw, cmd.buttonState); err != nil {
			return fmt.Errorf("PutMouseEventAbsolute: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unknown input kind %d", cmd.kind)
	}
}

// SendScancodes injects a sequence of PC/AT set-1 scancodes (make/break,
// including 0xE0-prefixed extended keys as separate entries). Safe from any
// goroutine.
func (c *Capturer) SendScancodes(scancodes []int) error {
	resp := make(chan error, 1)
	select {
	case c.inputCh <- inputCmd{kind: inputScancodes, scancodes: scancodes, resp: resp}:
		return <-resp
	case <-c.doneCh:
		return fmt.Errorf("capturer closed")
	}
}

// SendMouseAbsolute injects an absolute mouse event. x,y are 1-based guest
// pixel coordinates (clamped to the capture resolution); buttonState is a
// bitmask (1=left, 2=right, 4=middle); dz is vertical wheel, dw horizontal.
// Absolute positioning requires Guest Additions in the guest.
func (c *Capturer) SendMouseAbsolute(x, y, buttonState, dz, dw int) error {
	if x < 1 {
		x = 1
	}
	if y < 1 {
		y = 1
	}
	if x > c.width {
		x = c.width
	}
	if y > c.height {
		y = c.height
	}
	resp := make(chan error, 1)
	select {
	case c.inputCh <- inputCmd{kind: inputMouseAbs, x: x, y: y, dz: dz, dw: dw, buttonState: buttonState, resp: resp}:
		return <-resp
	case <-c.doneCh:
		return fmt.Errorf("capturer closed")
	}
}

func grab(display *ole.IDispatch, w, h, n int) (Frame, error) {
	res, err := oleutil.CallMethod(display, "TakeScreenShotToArray", 0, w, h, bitmapFormatRGBA)
	if err != nil {
		return Frame{}, fmt.Errorf("TakeScreenShotToArray: %w", err)
	}
	// The returned VARIANT owns a freshly allocated SAFEARRAY (~w*h*4 bytes).
	// It MUST be cleared every frame or the guest's screen buffer leaks at the
	// full frame rate and TakeScreenShotToArray soon fails with an
	// out-of-memory error. bulkBytes copies the pixels out before this runs.
	defer res.Clear()

	sac := res.ToArray()
	if sac == nil {
		return Frame{}, fmt.Errorf("TakeScreenShotToArray returned no array")
	}
	pix := bulkBytes(sac, n)
	if pix == nil {
		return Frame{}, fmt.Errorf("SafeArrayAccessData failed")
	}
	return Frame{Pix: pix, Width: w, Height: h}, nil
}

// bulkBytes copies a SAFEARRAY(UI1) into a Go slice with a single memcpy via
// SafeArrayAccessData. go-ole's SafeArrayConversion.ToByteArray copies element
// by element (~80x slower for multi-MB frames), which is far too slow for a
// live stream.
func bulkBytes(sac *ole.SafeArrayConversion, n int) []byte {
	var pv uintptr
	hr, _, _ := procSafeArrayAccess.Call(
		uintptr(unsafe.Pointer(sac.Array)),
		uintptr(unsafe.Pointer(&pv)),
	)
	if hr != 0 || pv == 0 {
		return nil
	}
	out := make([]byte, n)
	copy(out, unsafe.Slice((*byte)(unsafe.Pointer(pv)), n))
	procSafeArrayUnaccess.Call(uintptr(unsafe.Pointer(sac.Array)))
	return out
}

// DetectResolution returns the guest's current native screen resolution. It is
// used to notice when the guest changes resolution while a stream is live so the
// capturer can be re-created at the new size.
func DetectResolution(vboxManagePath, vmID string) (int, int, error) {
	return detectResolution(vboxManagePath, vmID)
}

// SetVideoModeHint asks the guest OS to switch its display to width x height.
// This only has an effect when Guest Additions is running in the guest (the
// additions video driver honors the hint); without them it is a silent no-op,
// which is exactly the desired fallback behavior. Used to make the guest match
// the console viewport so the stream fills it with no letterboxing.
func SetVideoModeHint(vboxManagePath, vmID string, width, height int) error {
	if width <= 0 || height <= 0 {
		return fmt.Errorf("invalid video mode %dx%d", width, height)
	}
	cmd := exec.Command(vboxManagePath, "controlvm", vmID, "setvideomodehint",
		strconv.Itoa(width), strconv.Itoa(height), "32")
	// Hidden window: the agent has no console, so this VBoxManage child must not
	// flash its own (matches detectResolution).
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("VBoxManage setvideomodehint: %w (%s)", err, string(out))
	}
	return nil
}

// detectResolution reads the VM's true screen resolution from the IHDR of a
// PNG captured by VBoxManage (which renders at native resolution). This avoids
// go-ole VT_BYREF marshaling, which does not reliably support the out
// parameters of IDisplay::GetScreenResolution.
func detectResolution(vboxManagePath, vmID string) (int, int, error) {
	tmp, err := os.CreateTemp("", "tabvm-screen-*.png")
	if err != nil {
		return 0, 0, err
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(tmpPath)

	cmd := exec.Command(vboxManagePath, "controlvm", vmID, "screenshotpng", tmpPath)
	// CREATE_NO_WINDOW: the agent runs without a console, so a console child like
	// VBoxManage must not pop its own window (it would flash on every capture).
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	if out, err := cmd.CombinedOutput(); err != nil {
		return 0, 0, fmt.Errorf("VBoxManage screenshotpng: %w (%s)", err, string(out))
	}

	header := make([]byte, 24)
	f, err := os.Open(tmpPath)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	if _, err := f.Read(header); err != nil {
		return 0, 0, err
	}
	// PNG signature (8) + IHDR length/type (8) + width(4) + height(4).
	if len(header) < 24 || header[0] != 0x89 || header[1] != 'P' {
		return 0, 0, fmt.Errorf("not a PNG screenshot")
	}
	w := int(binary.BigEndian.Uint32(header[16:20]))
	h := int(binary.BigEndian.Uint32(header[20:24]))
	if w <= 0 || h <= 0 {
		return 0, 0, fmt.Errorf("invalid resolution %dx%d", w, h)
	}
	return w, h, nil
}
