; Inno Setup script for TabVM.
; Compile with: ISCC.exe installer\tabvm.iss  (or run scripts\build-release.ps1)
; Produces a per-user installer (no admin required) that installs the agent and
; launcher and creates Desktop + Start Menu shortcuts to TabVM.exe.

#define AppName "TabVM"
#define AppVersion "0.1.0"
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
