; Inno Setup script for the Apotech PORTABLE Windows installer (SQLite flavor).
; Produces ..\..\dist\ApotechPortableSetup-<version>.exe.
;
; Unlike the full Apotech installer (apotech.iss), this flavor:
;   - Ships only apotech-portable.exe + config + backup script (~15 MB).
;   - Bundles NO PostgreSQL — the binary embeds SQLite (modernc.org/sqlite).
;   - Registers NO Windows service — the user runs the EXE from a shortcut.
;   - Adds NO firewall rule — the server binds 127.0.0.1 only.
;   - Requires NO admin (installs per-user under {userpf}).
;
; Build via build-windows-portable.ps1.

#ifndef AppVersion
  #define AppVersion "0.1.0"
#endif
#define AppName "Apotech Portable"
#define PayloadDir "payload-portable"

[Setup]
AppId={{8B4C1E2D-APOT-4ECH-9A22-APOTECHPORT01}}
AppName={#AppName}
AppVersion={#AppVersion}
AppPublisher=Apotech
DefaultDirName={autopf}\ApotechPortable
DefaultGroupName=Apotech Portable
DisableProgramGroupPage=yes
OutputDir=..\..\dist
OutputBaseFilename=ApotechPortableSetup-{#AppVersion}
Compression=lzma2/max
SolidCompression=yes
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
; No service registration, no firewall rule — per-user install is enough.
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=dialog
WizardStyle=modern
UninstallDisplayName=Apotech Portable

[Files]
; apotech-portable.exe + config template + backup .bat + README.
Source: "{#PayloadDir}\*"; DestDir: "{app}"; Flags: recursesubdirs createallsubdirs ignoreversion

[Icons]
; Start Menu shortcut to the launcher (opens browser to http://127.0.0.1:8080).
Name: "{autoprograms}\Apotech Portable"; Filename: "{app}\apotech-portable.exe"; \
  WorkingDir: "{app}"; Comment: "Launch Apotech Portable (single-PC SQLite flavor)"
; Optional Desktop shortcut (user can untick on install).
Name: "{autodesktop}\Apotech Portable"; Filename: "{app}\apotech-portable.exe"; \
  WorkingDir: "{app}"; Tasks: desktopicon
; Backup script shortcut.
Name: "{autoprograms}\Apotech Portable\Backup database"; Filename: "{app}\apotech-portable-backup.bat"; \
  WorkingDir: "{app}"; Comment: "Snapshot the SQLite database into backups\backup_<ts>\database.db"

[Tasks]
Name: "desktopicon";     Description: "Create a &desktop icon";                       GroupDescription: "Additional shortcuts:"
Name: "runonstartup";    Description: "&Run Apotech Portable when Windows starts";    GroupDescription: "Startup:";  Flags: unchecked

[Registry]
; runonstartup task: HKCU Run key (NOT a Windows service — keeps install per-user).
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; \
  ValueType: string; ValueName: "ApotechPortable"; ValueData: """{app}\apotech-portable.exe"""; \
  Tasks: runonstartup; Flags: uninsdeletevalue

[Run]
; Post-install: write a fresh config.yaml (random JWT secret + the wizard's
; owner credentials), then launch the EXE in the background. The EXE creates
; data\apotech.db on first run and auto-opens the browser to 127.0.0.1:8080.
Filename: "powershell.exe"; \
  Parameters: "-NoProfile -ExecutionPolicy Bypass -File ""{app}\scripts\setup-portable.ps1"" -AppDir ""{app}"" -OwnerEmail ""{code:GetOwnerEmail}"" -OwnerPassword ""{code:GetOwnerPassword}"" -AppPort {code:GetAppPort}"; \
  StatusMsg: "Initializing Apotech Portable..."; \
  Flags: runhidden waituntilterminated
; Optional first-launch on Finish.
Filename: "{app}\apotech-portable.exe"; \
  Description: "Launch Apotech Portable now"; \
  Flags: postinstall nowait skipifsilent

[UninstallDelete]
; data\ + backups\ + the freshly-baked config.yaml live under {app}; the user
; chooses whether to wipe them on uninstall (see InitializeUninstallProgressForm).
Type: filesandordirs; Name: "{app}\data";    Components: ""; Tasks: ""; Languages: "";

[Code]
var
  OwnerPage: TInputQueryWizardPage;
  PortPage:  TInputQueryWizardPage;

procedure InitializeWizard;
begin
  { Owner credentials }
  OwnerPage := CreateInputQueryPage(wpSelectDir,
    'Pharmacy owner account',
    'Create the first OWNER login.',
    'You will use these to sign in the first time. Change the password after first login.');
  OwnerPage.Add('Owner email:', False);
  OwnerPage.Add('Owner password:', True);
  OwnerPage.Values[0] := 'owner@apotech.local';

  { Port — only one (no Postgres in this flavor). Default 8080. }
  PortPage := CreateInputQueryPage(OwnerPage.ID,
    'Application port',
    'Network port',
    'Default 8080 is fine unless another program already uses it.');
  PortPage.Add('Application port:', False);
  PortPage.Values[0] := '8080';
end;

function NextButtonClick(CurPageID: Integer): Boolean;
begin
  Result := True;
  if CurPageID = OwnerPage.ID then
  begin
    if Trim(OwnerPage.Values[0]) = '' then
    begin
      MsgBox('Please enter an owner email.', mbError, MB_OK); Result := False; Exit;
    end;
    if Length(OwnerPage.Values[1]) < 8 then
    begin
      MsgBox('Owner password must be at least 8 characters.', mbError, MB_OK); Result := False;
    end;
  end;
end;

function GetOwnerEmail(Param: String):    String; begin Result := Trim(OwnerPage.Values[0]); end;
function GetOwnerPassword(Param: String): String; begin Result := OwnerPage.Values[1]; end;
function GetAppPort(Param: String):       String; begin Result := Trim(PortPage.Values[0]); end;
