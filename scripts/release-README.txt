TabVM — portable build
=======================

Control your local VirtualBox machines from the browser.

Requirements
------------
- Windows 10/11
- Oracle VirtualBox already installed (VBoxManage on the default path)

How to run
----------
1. Keep both files together in the same folder:
     - TabVM.exe          (the launcher — double-click this)
     - tabvm-agent.exe    (the local agent; started automatically)
2. Double-click TabVM.exe.
   - It starts the agent in the background (no window),
   - opens your default browser at the TabVM splash screen,
   - and shows your machines.
   Double-clicking again just opens a new browser tab.

Notes
-----
- The agent binds to 127.0.0.1:5230 only (local machine, not the network).
- A per-machine session token is generated on first run and stored under
  %APPDATA%\TabVM\session.token. The browser receives it automatically.
- Agent logs: %APPDATA%\TabVM\agent.log
- A TabVM icon appears in the notification area (system tray). Right-click it
  for "Open TabVM" or "Quit TabVM". Quit stops the agent cleanly.

Guest Additions
---------------
For clipboard sharing, shared folders, guest IPs, and dynamic resolution, the
guest needs VirtualBox Guest Additions. When a running VM is missing them, the
machine row shows an "Install Guest Additions" button that inserts the Guest
Additions disc; run the installer inside the VM to finish.
