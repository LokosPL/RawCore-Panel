#ifndef AppVersion
  #error AppVersion must be provided by the build script
#endif

[Setup]
AppId=RAWCRAFT-Control
AppName=RAWCRAFT Control
AppVersion={#AppVersion}
AppVerName=RAWCRAFT Control {#AppVersion}
VersionInfoVersion={#AppVersion}
VersionInfoProductName=RAWCRAFT Control
VersionInfoDescription=Instalator RAWCRAFT Control
UninstallDisplayName=RAWCRAFT Control
AppPublisherURL=https://github.com/LokosPL/RawCore-Panel
AppSupportURL=https://github.com/LokosPL/RawCore-Panel/issues
AppPublisher=RAWCRAFT
DefaultDirName={localappdata}\Programs\RAWCRAFT Control
DefaultGroupName=RAWCRAFT Control
DisableDirPage=no
DisableProgramGroupPage=yes
UsePreviousAppDir=yes
PrivilegesRequired=lowest
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
MinVersion=10.0
OutputDir=..\dist
OutputBaseFilename=RAWCRAFT-Control-Setup
SetupIconFile=..\assets\app.ico
UninstallDisplayIcon={app}\RAWCRAFT-Control.exe
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
CloseApplications=yes
CloseApplicationsFilter=RAWCRAFT-Control.exe
RestartApplications=no
AppUpdatesURL=https://github.com/LokosPL/RawCore-Panel/releases/latest

[Languages]
Name: "polish"; MessagesFile: "compiler:Languages\Polish.isl"
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "Utwórz skrót na pulpicie"; GroupDescription: "Skróty:"

[Files]
Source: "..\dist\RAWCRAFT-Control.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\dist\README_PL.txt"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\dist\THIRD_PARTY_NOTICES.txt"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\RAWCRAFT Control"; Filename: "{app}\RAWCRAFT-Control.exe"; WorkingDir: "{app}"
Name: "{autodesktop}\RAWCRAFT Control"; Filename: "{app}\RAWCRAFT-Control.exe"; WorkingDir: "{app}"; Tasks: desktopicon

[Run]
Filename: "{app}\RAWCRAFT-Control.exe"; Description: "Uruchom RAWCRAFT Control"; Flags: nowait postinstall skipifsilent

[UninstallDelete]
Type: files; Name: "{app}\RAWCRAFT-Control.exe.old"
Type: files; Name: "{app}\RAWCRAFT-Control.exe.new"
Type: files; Name: "{app}\RAWCRAFT-Control.exe.*"
Type: files; Name: "{app}\.rawcraft-update-write-*"
Type: files; Name: "{app}\*.rawcraft-tmp"
Type: files; Name: "{app}\.rawcraft-write-check-*.tmp"
Type: dirifempty; Name: "{app}"

[Code]
function NextButtonClick(CurPageID: Integer): Boolean;
var
  Folder, Probe: String;
  Index: Integer;
begin
  Result := True;
  if CurPageID = wpSelectDir then
  begin
    Folder := WizardDirValue;
    Probe := '';
    try
      if not ForceDirectories(Folder) then RaiseException('Nie można utworzyć folderu.');
      Index := 0;
      repeat
        Probe := AddBackslash(Folder) + '.rawcraft-write-check-' + IntToStr(Index) + '.tmp';
        Index := Index + 1;
      until not FileExists(Probe);
      if Probe = '' then RaiseException('Brak prawa zapisu.');
      if not SaveStringToFile(Probe, 'RAWCRAFT update write test', False) then RaiseException('Brak prawa zapisu.');
    except
      MsgBox('Wybierz folder, w którym Twoje konto może zapisywać pliki. Jest to wymagane do automatycznych aktualizacji bez uruchamiania jako administrator.', mbError, MB_OK);
      Result := False;
    end;
    if Probe <> '' then DeleteFile(Probe);
  end;
end;

var
  PurgeUserData: Boolean;

{ Uninstall: stop the running panel first, then (when the user agrees or /PURGE is
  given) remove every trace: settings, saved SSH password and GitHub token,
  WebView2 data, notification history and downloaded updates.
  Silent uninstalls without /PURGE keep settings, e.g. for scripted reinstalls. }
function InitializeUninstall(): Boolean;
begin
  Result := True;
  PurgeUserData := Pos('/PURGE', UpperCase(GetCmdTail)) > 0;
  if (not PurgeUserData) and (not UninstallSilent) then
    PurgeUserData := MsgBox('Usunąć również wszystkie dane RAWCRAFT Control z tego komputera?' + #13#10 + #13#10 +
      '- ustawienia, profile VPS i lista instancji' + #13#10 +
      '- zapamiętane hasło SSH i token GitHub' + #13#10 +
      '- historia powiadomień, pamięć podręczna i pobrane aktualizacje' + #13#10 + #13#10 +
      'Tak - usuń wszystko (zalecane przy całkowitym odinstalowaniu).' + #13#10 +
      'Nie - zachowaj ustawienia na wypadek ponownej instalacji.' + #13#10 + #13#10 +
      'Pliki na VPS i serwer Minecraft nie są zmieniane.',
      mbConfirmation, MB_YESNO) = IDYES;
end;

procedure StopPanel();
var
  Code: Integer;
begin
  Exec(ExpandConstant('{sys}\taskkill.exe'), '/F /T /IM RAWCRAFT-Control.exe', '', SW_HIDE, ewWaitUntilTerminated, Code);
  Sleep(800);
end;

procedure RemoveDataDir(const Dir: String);
var
  Attempt: Integer;
begin
  for Attempt := 1 to 6 do
  begin
    if not DirExists(Dir) then
      Exit;
    DelTree(Dir, True, True, True);
    if DirExists(Dir) then
      Sleep(700);
  end;
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
begin
  if CurUninstallStep = usUninstall then
    StopPanel();
  if (CurUninstallStep = usPostUninstall) and PurgeUserData then
  begin
    RemoveDataDir(ExpandConstant('{userappdata}\RawcraftControl'));
    RemoveDataDir(ExpandConstant('{localappdata}\RawcraftControl'));
  end;
end;
