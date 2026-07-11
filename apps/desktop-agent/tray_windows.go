//go:build windows

package main

import (
	_ "embed"
	"log/slog"
	"os/exec"
	"syscall"

	"github.com/getlantern/systray"
)

//go:embed tray.ico
var trayIcon []byte

const (
	trayOpenURL    = "http://127.0.0.1:5230/?splash=1"
	createNoWindow = 0x08000000
)

// runTray shows a notification-area icon so the background agent can be opened
// or stopped without Task Manager. It blocks on the tray message loop and
// returns true once the user chooses Quit, signalling the caller to exit.
func runTray(logger *slog.Logger) bool {
	systray.Run(func() {
		systray.SetIcon(trayIcon)
		systray.SetTitle("TabVM")
		systray.SetTooltip("TabVM — local VirtualBox control")

		mOpen := systray.AddMenuItem("Open TabVM", "Open TabVM in your browser")
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("Quit TabVM", "Stop the TabVM agent")

		go func() {
			for {
				select {
				case <-mOpen.ClickedCh:
					openDefaultBrowser(trayOpenURL)
				case <-mQuit.ClickedCh:
					systray.Quit()
					return
				}
			}
		}()
	}, func() {
		logger.Info("TabVM tray exited; stopping agent")
	})
	return true
}

func openDefaultBrowser(url string) {
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	_ = cmd.Start()
}
