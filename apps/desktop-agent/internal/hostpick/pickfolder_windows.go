// Package hostpick opens native host-side file/folder pickers so the browser UI
// can obtain a real absolute host path. A browser cannot read absolute paths
// (the OS hides them from <input type=file>), but VirtualBox shared folders need
// one, so the path must be chosen by the host agent process instead.
package hostpick

import (
	"context"
	"fmt"
	"runtime"
	"syscall"
	"unicode/utf16"
	"unsafe"

	ole "github.com/go-ole/go-ole"
)

var (
	modole32          = syscall.NewLazyDLL("ole32.dll")
	procCoTaskMemFree = modole32.NewProc("CoTaskMemFree")
)

// IFileOpenDialog options (FILEOPENDIALOGOPTIONS) and IShellItem display forms.
const (
	fosPickFolders     = 0x00000020 // FOS_PICKFOLDERS: choose folders, not files.
	fosForceFilesystem = 0x00000040 // FOS_FORCEFILESYSTEM: only real filesystem items.
	sigdnFilesysPath   = 0x80058000 // SIGDN_FILESYSPATH: the plain "C:\..." path.
	hrCancelled        = 0x800704C7 // HRESULT_FROM_WIN32(ERROR_CANCELLED): user closed the dialog.
)

// COM vtable slot indices. IFileOpenDialog derives IFileDialog -> IModalWindow
// -> IUnknown; IShellItem derives IUnknown. These offsets are fixed by the ABI.
const (
	idxRelease            = 2  // IUnknown::Release
	idxShow               = 3  // IModalWindow::Show
	idxSetOptions         = 9  // IFileDialog::SetOptions
	idxGetOptions         = 10 // IFileDialog::GetOptions
	idxGetResult          = 20 // IFileDialog::GetResult
	idxItemGetDisplayName = 5  // IShellItem::GetDisplayName
)

var (
	clsidFileOpenDialog = ole.NewGUID("{DC1C5A9C-E88A-4dde-A5A1-60F82A20AEF7}")
	iidIFileOpenDialog  = ole.NewGUID("{D57C7288-D4AD-4768-BE02-9D969532D960}")
)

// comCall invokes vtable slot idx on a COM interface pointer, passing the
// interface itself as the implicit first argument.
func comCall(this *ole.IUnknown, idx int, args ...uintptr) uintptr {
	self := uintptr(unsafe.Pointer(this))
	vtbl := *(*uintptr)(unsafe.Pointer(self))
	method := *(*uintptr)(unsafe.Pointer(vtbl + uintptr(idx)*unsafe.Sizeof(uintptr(0))))
	ret, _, _ := syscall.SyscallN(method, append([]uintptr{self}, args...)...)
	return ret
}

// PickFolder opens the native Windows folder picker and returns the chosen
// absolute path, or "" if the user cancelled. The modal dialog runs on its own
// dedicated STA thread so it never blocks or corrupts the caller's goroutine.
func PickFolder(ctx context.Context) (string, error) {
	return pick(ctx, true)
}

// PickFile opens the native Windows file picker and returns the chosen absolute
// path, or "" if the user cancelled. No extension filter is applied here (that
// would require marshalling a COMDLG_FILTERSPEC array through the raw vtable);
// the caller validates the extension instead.
func PickFile(ctx context.Context) (string, error) {
	return pick(ctx, false)
}

// pick runs the chosen picker on a dedicated STA thread and honors ctx.
func pick(ctx context.Context, pickFolders bool) (string, error) {
	type result struct {
		path string
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		p, e := showPicker(pickFolders)
		ch <- result{p, e}
	}()

	select {
	case <-ctx.Done():
		// The dialog goroutine is abandoned; it finishes when the user acts. The
		// server serializes picker calls, so a stale dialog cannot overlap a new
		// one without the mutex still being held.
		return "", ctx.Err()
	case r := <-ch:
		return r.path, r.err
	}
}

// showPicker runs the IFileOpenDialog COM flow, in folder or file mode. It is
// panic-guarded so a bad pointer or COM fault surfaces as an error rather than
// crashing the agent.
func showPicker(pickFolders bool) (path string, err error) {
	defer func() {
		if r := recover(); r != nil {
			path, err = "", fmt.Errorf("file picker crashed: %v", r)
		}
	}()

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Shell dialogs require a single-threaded apartment. S_FALSE / already-init on
	// this fresh dedicated thread is harmless, so the init error is tolerated.
	_ = ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED)
	defer ole.CoUninitialize()

	dialog, e := ole.CreateInstance(clsidFileOpenDialog, iidIFileOpenDialog)
	if e != nil {
		return "", fmt.Errorf("create FileOpenDialog: %w", e)
	}
	defer comCall(dialog, idxRelease)

	// COM writes into these out-parameters via pointers we pass as uintptr. They
	// are heap-allocated with new() so their addresses stay stable even if the
	// goroutine stack moves between the pointer->uintptr conversion and the call.
	opts := new(uint32)
	item := new(*ole.IUnknown)
	wpath := new(uintptr)

	// Preserve existing options; add folder mode only when picking a folder.
	comCall(dialog, idxGetOptions, uintptr(unsafe.Pointer(opts)))
	*opts |= fosForceFilesystem
	if pickFolders {
		*opts |= fosPickFolders
	}
	comCall(dialog, idxSetOptions, uintptr(*opts))

	// Show is modal and pumps its own message loop until the user acts.
	hr := comCall(dialog, idxShow, 0)
	if uint32(hr) == hrCancelled {
		return "", nil
	}
	if int32(hr) < 0 {
		return "", fmt.Errorf("dialog Show failed: 0x%08x", uint32(hr))
	}

	hr = comCall(dialog, idxGetResult, uintptr(unsafe.Pointer(item)))
	if int32(hr) < 0 || *item == nil {
		return "", fmt.Errorf("dialog GetResult failed: 0x%08x", uint32(hr))
	}
	defer comCall(*item, idxRelease)

	hr = comCall(*item, idxItemGetDisplayName, uintptr(sigdnFilesysPath), uintptr(unsafe.Pointer(wpath)))
	if int32(hr) < 0 || *wpath == 0 {
		return "", fmt.Errorf("GetDisplayName failed: 0x%08x", uint32(hr))
	}
	defer procCoTaskMemFree.Call(*wpath)

	return utf16PtrToString(*wpath), nil
}

// utf16PtrToString reads a null-terminated UTF-16 string from a raw pointer.
func utf16PtrToString(p uintptr) string {
	if p == 0 {
		return ""
	}
	var buf []uint16
	for i := 0; ; i++ {
		ch := *(*uint16)(unsafe.Pointer(p + uintptr(i)*2))
		if ch == 0 {
			break
		}
		buf = append(buf, ch)
	}
	return string(utf16.Decode(buf))
}
