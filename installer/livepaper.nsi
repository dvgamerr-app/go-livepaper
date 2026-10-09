Unicode True
SetCompressor /SOLID lzma

;-----------------------------------------------------------------
; Defines (APP_VERSION passed via /D from makensis CLI)
;-----------------------------------------------------------------
!ifndef APP_VERSION
  !define APP_VERSION "0.0.0"
!endif
!ifndef APP_FILE_VERSION
  !define APP_FILE_VERSION "0.0.0.0"
!endif

!define APP_NAME    "Live Paper"
!define APP_EXE     "livepaper.exe"
!define APP_BIN_DIR "$INSTDIR\bin"
!define APP_EXE_PATH "$INSTDIR\bin\${APP_EXE}"
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
!define MUI_FINISHPAGE_RUN     "${APP_EXE_PATH}"
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
OutFile "bin\livepaper-setup-${APP_VERSION}.exe"
InstallDir "$LOCALAPPDATA\Programs\livepaper"
RequestExecutionLevel user

VIProductVersion "${APP_FILE_VERSION}.0.0"
VIAddVersionKey "ProductName"    "${APP_NAME}"
VIAddVersionKey "FileVersion"    "${APP_FILE_VERSION}"
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
  ; Stop any running instance before overwriting files.
  DetailPrint "Stopping running Live Paper processes..."
  nsExec::ExecToLog 'taskkill /IM ${APP_EXE} /F'
  Sleep 1000

  ; Older releases shipped an online dependency installer here; drop it on upgrade.
  RMDir /r "${APP_BIN_DIR}\scripts"
  ; Legacy wallpapers downloaded by older online builds sit as extensionless
  ; files directly in data\; keep the data\thumbnail cache sub-folder.
  Delete "${APP_BIN_DIR}\data\*.*"

  ; Explicit offline payload: the app plus the media tools bundled at build time
  ; by scripts\bundle-media-tools.ps1. makensis fails if any of them is missing,
  ; and stale build output in bin\ (scripts, data cache, setup exe) is never packaged.
  SetOutPath "${APP_BIN_DIR}"
  File "bin\${APP_EXE}"
  File "bin\ffmpeg.exe"
  File "bin\ffprobe.exe"
  File "bin\mpv.exe"
  File /r "bin\licenses"

  ; Register auto-start on login
  WriteRegStr HKCU "${RUN_KEY}" "livepaper" '"${APP_EXE_PATH}"'

  ; Start Menu shortcuts
  CreateDirectory "$SMPROGRAMS\${APP_NAME}"
  CreateShortcut  "$SMPROGRAMS\${APP_NAME}\${APP_NAME}.lnk" "${APP_EXE_PATH}"
  CreateShortcut  "$SMPROGRAMS\${APP_NAME}\Uninstall.lnk"   "$INSTDIR\uninstall.exe"
  CreateShortcut  "$DESKTOP\${APP_NAME}.lnk"                "${APP_EXE_PATH}"

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

  Delete "$INSTDIR\uninstall.exe"
  RMDir /r "${APP_BIN_DIR}"
  RMDir  "$INSTDIR"

  DeleteRegValue HKCU "${RUN_KEY}"    "livepaper"
  DeleteRegKey   HKCU "${UNINST_KEY}"

  Delete "$DESKTOP\${APP_NAME}.lnk"
  Delete "$SMPROGRAMS\${APP_NAME}\${APP_NAME}.lnk"
  Delete "$SMPROGRAMS\${APP_NAME}\Uninstall.lnk"
  RMDir  "$SMPROGRAMS\${APP_NAME}"
SectionEnd
