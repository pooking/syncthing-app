; OspreySyncTool — Windows installer.
;
; Built by buildapp, which passes the version and the binary it just linked:
;
;     go run -tags tools ./buildapp -installer
;
; The app is portable by design, so this is a convenience rather than the only
; way to get it: dist/ also ships a zip holding the same executable plus the
; OspreySyncTool.portable marker that keeps its state beside the executable.
; Both are supported; this script exists for people who expect an entry in
; Add/Remove Programs.
;
; Two decisions are worth stating here, because they are otherwise only visible
; by reading the directives and guessing:
;
;   * Per-user, no elevation (PrivilegesRequired=lowest). The program goes to
;     %LocalAppData%\Programs\OspreySyncTool, and the app's own state stays in
;     %LocalAppData%\OspreySyncTool — the directory a non-portable run of the
;     bare executable already uses. An installed copy and an unzipped copy
;     therefore share the device identity and folder index the user already
;     has, rather than starting a second, empty installation beside it.
;
;   * The uninstaller removes the program and leaves that data directory alone.
;     Nothing below points at it, so its absence from [Files] is the whole
;     mechanism. Deleting it would throw away a device identity and a folder
;     index that the peers it syncs with still expect to hear from.
;
; Everything the wizard says is one of Inno Setup's own messages, so the whole
; installer is localized without a translation of our own living in this
; repository — with one exception, Simplified Chinese, which Inno Setup does not
; translate (see the [Languages] section below). Start-at-login is deliberately
; not offered here: the application registers and reads that registry value
; itself (Settings → Start when I sign in), and a second writer with a slightly
; different spelling of the same value is how the two would come to disagree.

#define ExeName "osprey-sync-tool.exe"

; AppId identifies this application across upgrades. It must never change: a new
; value makes Windows treat the next release as an unrelated program, leaving
; the old one installed alongside it. The braces are doubled because a leading
; "{{" is Inno's escape for a literal "{".
#define AppId "{{7F3C9A24-5B8E-4D61-9C0A-2E6D4B71A8F3}"

; The mutex the running app holds, built by Wails as "wails-app-" + UniqueId +
; "sim" (cmd/osprey-sync-tool/main.go passes UniqueId "ospreysynctool-main-window").
; Setup refuses to run while it exists, which is what keeps an install or an
; uninstall from failing halfway through on a locked executable. The app hides
; to the tray rather than exiting when its window is closed, so it is quite
; often still running when someone decides to upgrade.
#define AppRuntimeMutex "wails-app-ospreysynctool-main-windowsim"

; buildapp supplies all three. The fallbacks keep this script compilable by hand
; from the Inno Setup IDE, where no /D is passed.
#ifndef AppVersion
  #define AppVersion "v0.0.0-dev"
#endif
#ifndef SourceExe
  #define SourceExe "..\..\dist\osprey-sync-tool-windows-amd64.exe"
#endif
#ifndef OutDir
  #define OutDir "..\..\dist"
#endif
; The two version forms are separate defines on purpose: AppVersion is the app's
; own string and may read v0.0.0-dev.20261010, which is fine to show but is not
; something Windows accepts in a version resource.
#ifndef VersionInfoVersion
  #define VersionInfoVersion "0.0.0.0"
#endif

[Setup]
AppId={#AppId}
AppName=OspreySyncTool
AppVersion={#AppVersion}
AppVerName=OspreySyncTool {#AppVersion}
AppPublisher=OspreySyncTool
VersionInfoVersion={#VersionInfoVersion}
VersionInfoDescription=OspreySyncTool {#AppVersion} Setup
UninstallDisplayName=OspreySyncTool {#AppVersion}
DefaultDirName={autopf}\OspreySyncTool
DefaultGroupName=OspreySyncTool
DisableProgramGroupPage=yes
AllowNoIcons=yes
; Per-user: no UAC prompt, and no administrator needed to install.
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=dialog
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
AppMutex={#AppRuntimeMutex}
SetupMutex=OspreySyncToolSetup
OutputDir={#OutDir}
OutputBaseFilename=OspreySyncTool-{#AppVersion}-windows-amd64-setup
Compression=lzma2/max
SolidCompression=yes
WizardStyle=modern
; All three images are drawn by desktop/brand from the same geometry as the
; executable's own icon and the tray, so there is no second copy of the owl to
; keep in step with the first. They sit beside this script and are generated on
; demand: buildapp writes any that are missing before compiling.
SetupIconFile=appicon-256.ico
WizardImageFile=wizard-large.bmp
WizardSmallImageFile=wizard-small.bmp
UninstallDisplayIcon={app}\{#ExeName}

[Languages]
; Every language Inno Setup itself ships, so the wizard follows whatever
; display language Windows reports, plus one it does not ship.
#expr EmitLanguagesSection
; Simplified Chinese, vendored in languages/ beside this script: Inno Setup has
; no official Chinese translation, and its own language files are the only way
; the wizard's text can come out in Chinese. The file is a third-party
; translation used unchanged (see languages/LICENSE-ChineseSimplified.txt and
; the note at the top of this file). It is emitted here rather than written as a
; plain line above because a second [Languages] header in the script is an
; error, and EmitLanguagesSection writes the first one.
#emit "Name: chinesesimplified; MessagesFile: ""languages\ChineseSimplified.isl"""

[Tasks]
; Not ticked for the user: a desktop icon is something people notice and
; dislike having been given unasked, and the Start menu entry is always there.
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; Flags: unchecked

[Files]
Source: "{#SourceExe}"; DestDir: "{app}"; DestName: "{#ExeName}"; Flags: ignoreversion

[Icons]
Name: "{group}\OspreySyncTool"; Filename: "{app}\{#ExeName}"
Name: "{group}\{cm:UninstallProgram,OspreySyncTool}"; Filename: "{uninstallexe}"
Name: "{autodesktop}\OspreySyncTool"; Filename: "{app}\{#ExeName}"; Tasks: desktopicon

[Run]
; --minimized is deliberately not passed: the first run may still need the setup
; wizard, and a window the user never sees would leave them looking at a tray
; icon and wondering whether anything happened.
Filename: "{app}\{#ExeName}"; Description: "{cm:LaunchProgram,OspreySyncTool}"; Flags: nowait postinstall skipifsilent

[Code]
// There is no [Registry] section here on purpose: the only registry value this
// app uses is the one it writes and reads itself, for starting at sign-in, and
// the installer never writes it (see the note at the top of this file).
//
// The one thing left to do is to remove that value if the user had turned
// sign-in startup on from within the application. Leaving it would point a
// login entry at an executable that is no longer there.
//
// The value's name is shared with any portable copy of the app the user also
// runs, so this can remove an entry that a zip copy had set. That is the lesser
// of the two mistakes: a sign-in entry pointing at a deleted file is what the
// user would otherwise be left with, and the Settings switch is one click away
// in the copy they kept.
const
  RunKey = 'Software\Microsoft\Windows\CurrentVersion\Run';
  RunValueName = 'OspreySyncTool';

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
begin
  if CurUninstallStep = usPostUninstall then
    RegDeleteValue(HKCU, RunKey, RunValueName);
end;
