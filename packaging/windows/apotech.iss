; Inno Setup script for the Apotech Windows installer.
; Build with build-windows.ps1, which assembles .\payload (apotech.exe +
; bundled PostgreSQL + WinSW + scripts) and runs ISCC with /DAppVersion=...
;
; Produces ..\..\dist\ApotechSetup-<version>.exe.

#ifndef AppVersion
  #define AppVersion "0.1.0"
#endif
#define AppName "Apotech"
#define PayloadDir "payload"

[Setup]
AppId={{6F2A0E2C-APOT-4ECH-9A11-APOTECHSETUP01}}
AppName={#AppName}
AppVersion={#AppVersion}
AppPublisher=Apotech
DefaultDirName={autopf}\Apotech
DefaultGroupName=Apotech
DisableProgramGroupPage=yes
OutputDir=..\..\dist
OutputBaseFilename=ApotechSetup-{#AppVersion}
Compression=lzma2/max
SolidCompression=yes
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
; Bundled PostgreSQL + service registration require admin.
PrivilegesRequired=admin
WizardStyle=modern
UninstallDisplayName=Apotech

[Files]
; Everything assembled into payload\ by build-windows.ps1.
Source: "{#PayloadDir}\*"; DestDir: "{app}"; Flags: recursesubdirs createallsubdirs ignoreversion

[Run]
; Post-install: init DB, write config, register + start services, shortcuts.
Filename: "powershell.exe"; \
  Parameters: "-NoProfile -ExecutionPolicy Bypass -File ""{app}\scripts\setup.ps1"" -AppDir ""{app}"" -DataDir ""{commonappdata}\Apotech"" -OwnerEmail ""{code:GetOwnerEmail}"" -OwnerPassword ""{code:GetOwnerPassword}"" -BindHost ""{code:GetBindHost}"" -AppPort {code:GetAppPort} -PgPort {code:GetPgPort} -Lan {code:GetLanFlag}"; \
  StatusMsg: "Setting up database and services (this can take a minute)..."; \
  Flags: runhidden waituntilterminated

[UninstallRun]
Filename: "powershell.exe"; \
  Parameters: "-NoProfile -ExecutionPolicy Bypass -File ""{app}\scripts\uninstall.ps1"" -AppDir ""{app}"" -DataDir ""{commonappdata}\Apotech"" -PurgeData {code:GetPurgeFlag}"; \
  Flags: runhidden waituntilterminated; RunOnceId: "apotechteardown"

[Code]
var
  OwnerPage: TInputQueryWizardPage;
  PortsPage: TInputQueryWizardPage;
  TopologyPage: TInputOptionWizardPage;
  PurgeData: Boolean;

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

  { Topology }
  TopologyPage := CreateInputOptionPage(OwnerPage.ID,
    'Network access',
    'Who should reach Apotech?',
    'Choose how this computer serves the application.',
    True, False);
  TopologyPage.Add('This computer only (most secure)');
  TopologyPage.Add('Other computers on the shop network (adds a firewall rule)');
  TopologyPage.SelectedValueIndex := 0;

  { Ports }
  PortsPage := CreateInputQueryPage(TopologyPage.ID,
    'Ports',
    'Network ports',
    'Defaults are fine unless they clash with other software.');
  PortsPage.Add('Application port:', False);
  PortsPage.Add('Database port:', False);
  PortsPage.Values[0] := '8080';
  PortsPage.Values[1] := '5433';  { 5433 avoids clashing with an existing Postgres on 5432 }
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

function GetOwnerEmail(Param: String): String;    begin Result := Trim(OwnerPage.Values[0]); end;
function GetOwnerPassword(Param: String): String; begin Result := OwnerPage.Values[1]; end;
function GetAppPort(Param: String): String;       begin Result := Trim(PortsPage.Values[0]); end;
function GetPgPort(Param: String): String;        begin Result := Trim(PortsPage.Values[1]); end;

function GetBindHost(Param: String): String;
begin
  if TopologyPage.SelectedValueIndex = 1 then Result := '0.0.0.0'
  else Result := '127.0.0.1';
end;

function GetLanFlag(Param: String): String;
begin
  if TopologyPage.SelectedValueIndex = 1 then Result := '1' else Result := '0';
end;

procedure InitializeUninstallProgressForm;
begin
  PurgeData := MsgBox(
    'Remove the Apotech database and all data too?' + #13#10 +
    'Choose No to keep your data for a future reinstall.',
    mbConfirmation, MB_YESNO) = IDYES;
end;

function GetPurgeFlag(Param: String): String;
begin
  if PurgeData then Result := '1' else Result := '0';
end;
