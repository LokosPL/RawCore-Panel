#ifndef AppVersion
  #error AppVersion must be provided by the build script
#endif

[Setup]
AppId=RAWCRAFT-Control
AppName=RAWCRAFT Control
AppVersion={#AppVersion}
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
AppUpdatesURL=https://github.com/LokosPL/RawCore-Control/releases/latest

[Languages]
Name: "polish"; MessagesFile: "compiler:Languages\Polish.isl"
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "Utwórz skrót na pulpicie"; GroupDescription: "Skróty:"; Flags: unchecked

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
