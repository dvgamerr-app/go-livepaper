Unicode True
SetCompressor /SOLID lzma

;-----------------------------------------------------------------
; Defines (APP_VERSION passed via /D from makensis CLI)
;-----------------------------------------------------------------
!ifndef APP_VERSION
  !define APP_VERSION "0.0.0"
!endif

!define APP_NAME    "Live Paper"
!define APP_EXE     "livepaper.exe"
!define INST_KEY    "Software\livepaper"
!define UNINST_KEY  "Software\Microsoft\Windows\CurrentVersion\Uninstall\livepaper"
!define RUN_KEY     "Software\Microsoft\Windows\CurrentVersion\Run"

;-----------------------------------------------------------------
; MUI setup
;-----------------------------------------------------------------
!include "MUI2.nsh"

!define MUI_ICON             "..\public\icon.ico"
!define MUI_UNICON           "..\public\icon.ico"
!define MUI_WELCOMEPAGE_TITLE  "Install ${APP_NAME} ${APP_VERSION}"
!define MUI_FINISHPAGE_RUN     "$INSTDIR\${APP_EXE}"
!define MUI_FINISHPAGE_RUN_TEXT "Launch Live Paper"

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "English"

;-----------------------------------------------------------------
; Metadata
;-----------------------------------------------------------------
Name    "${APP_NAME} ${APP_VERSION}"
OutFile "..\livepaper-setup-${APP_VERSION}.exe"
InstallDir "$LOCALAPPDATA\Programs\livepaper"
RequestExecutionLevel user

VIProductVersion "${APP_VERSION}.0"
VIAddVersionKey "ProductName"    "${APP_NAME}"
VIAddVersionKey "FileVersion"    "${APP_VERSION}"
VIAddVersionKey "ProductVersion" "${APP_VERSION}"
VIAddVersionKey "CompanyName"    "dvgamerr"
VIAddVersionKey "LegalCopyright" "dvgamerr"
VIAddVersionKey "FileDescription" "Live Paper Installer"

;-----------------------------------------------------------------
; Resolve paths relative to project root (one level up from installer/)
;-----------------------------------------------------------------
!cd ".."

;-----------------------------------------------------------------
Section "Install" SEC_MAIN
  SetOutPath "$INSTDIR"

  ; Main binary
  File "livepaper.exe"

  ; Dependency installer script
  SetOutPath "$INSTDIR\scripts"
  File "scripts\install-deps.ps1"
  SetOutPath "$INSTDIR"

  ; Install ffmpeg + mpv from the internet (non-fatal)
  DetailPrint "Installing ffmpeg and mpv..."
  nsExec::ExecToLog 'powershell.exe -NonInteractive -ExecutionPolicy Bypass \
    -File "$INSTDIR\scripts\install-deps.ps1" \
    -InstallDir "$LOCALAPPDATA\Programs\livepaper\bin"'

  ; Register auto-start on login
  WriteRegStr HKCU "${RUN_KEY}" "livepaper" '"$INSTDIR\${APP_EXE}"'

  ; Start Menu shortcuts
  CreateDirectory "$SMPROGRAMS\${APP_NAME}"
  CreateShortcut  "$SMPROGRAMS\${APP_NAME}\${APP_NAME}.lnk" "$INSTDIR\${APP_EXE}"
  CreateShortcut  "$SMPROGRAMS\${APP_NAME}\Uninstall.lnk"   "$INSTDIR\uninstall.exe"

  ; Uninstall registry
  WriteUninstaller "$INSTDIR\uninstall.exe"
  WriteRegStr  HKCU "${UNINST_KEY}" "DisplayName"     "${APP_NAME}"
  WriteRegStr  HKCU "${UNINST_KEY}" "DisplayVersion"  "${APP_VERSION}"
  WriteRegStr  HKCU "${UNINST_KEY}" "Publisher"       "dvgamerr"
  WriteRegStr  HKCU "${UNINST_KEY}" "UninstallString" '"$INSTDIR\uninstall.exe"'
  WriteRegStr  HKCU "${UNINST_KEY}" "InstallLocation" "$INSTDIR"
  WriteRegDWORD HKCU "${UNINST_KEY}" "NoModify" 1
  WriteRegDWORD HKCU "${UNINST_KEY}" "NoRepair"  1
SectionEnd

;-----------------------------------------------------------------
Section "Uninstall"
  ; Stop running instance first
  nsExec::Exec 'taskkill /IM ${APP_EXE} /F'

  Delete "$INSTDIR\${APP_EXE}"
  Delete "$INSTDIR\scripts\install-deps.ps1"
  RMDir  "$INSTDIR\scripts"
  Delete "$INSTDIR\uninstall.exe"
  RMDir  "$INSTDIR"

  DeleteRegValue HKCU "${RUN_KEY}"    "livepaper"
  DeleteRegKey   HKCU "${UNINST_KEY}"

  Delete "$SMPROGRAMS\${APP_NAME}\${APP_NAME}.lnk"
  Delete "$SMPROGRAMS\${APP_NAME}\Uninstall.lnk"
  RMDir  "$SMPROGRAMS\${APP_NAME}"
SectionEnd
