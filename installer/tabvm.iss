; Inno Setup script for TabVM.
; Compile with: ISCC.exe installer\tabvm.iss  (or run scripts\build-release.ps1)
; Produces a per-user installer (no admin required) that installs the agent and
; launcher and creates Desktop + Start Menu shortcuts to TabVM.exe.

#define AppName "TabVM"
#define AppVersion "0.1.1"
#define AppPublisher "TabVM"

[Setup]
AppId={{9C6F1E2A-3B4C-4D5E-9F70-TABVM0000001}
AppName={#AppName}
AppVersion={#AppVersion}
AppPublisher={#AppPublisher}
; Per-user install: no administrator privileges needed.
PrivilegesRequired=lowest
DefaultDirName={localappdata}\Programs\TabVM
DefaultGroupName=TabVM
DisableProgramGroupPage=yes
OutputDir=..\dist
OutputBaseFilename=TabVM-Setup
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
SetupIconFile=..\branding\icon\tabvm.ico
UninstallDisplayIcon={app}\tabvm.ico
; Ask Restart Manager to close apps using files we replace, but never restart
; them: the [Run] entry (or the user) relaunches the new version instead.
CloseApplications=yes
RestartApplications=no

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "Create a desktop shortcut"; GroupDescription: "Additional shortcuts:"

[Files]
; These are produced by scripts\build-release.ps1 into ..\dist\TabVM before compiling.
Source: "..\dist\TabVM\TabVM.exe";        DestDir: "{app}"; Flags: ignoreversion
Source: "..\dist\TabVM\tabvm-agent.exe";  DestDir: "{app}"; Flags: ignoreversion
Source: "..\dist\TabVM\README.txt";       DestDir: "{app}"; Flags: ignoreversion isreadme
Source: "..\dist\TabVM\tabvm.ico";        DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\TabVM";           Filename: "{app}\TabVM.exe"; IconFilename: "{app}\tabvm.ico"
Name: "{group}\Uninstall TabVM"; Filename: "{uninstallexe}"
Name: "{autodesktop}\TabVM";     Filename: "{app}\TabVM.exe"; IconFilename: "{app}\tabvm.ico"; Tasks: desktopicon

[Run]
Filename: "{app}\TabVM.exe"; Description: "Launch TabVM"; Flags: nowait postinstall skipifsilent

[Code]
// Force-kills a process by image name, ignoring failures (it may simply not be
// running). Restart Manager can miss the headless agent, so this guarantees an
// upgrade never replaces files under a live agent that would keep serving the
// old embedded web UI.
procedure KillProcess(const ExeName: String);
var
  ResultCode: Integer;
begin
  Exec('taskkill.exe', '/F /IM ' + ExeName, '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
end;

function PrepareToInstall(var NeedsRestart: Boolean): String;
begin
  KillProcess('tabvm-agent.exe');
  KillProcess('TabVM.exe');
  Result := '';
end;
