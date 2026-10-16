@echo off
setlocal

cd /d "%~dp0"

set "TARGET=merger-native.exe"
set "TEMP_TARGET=merger-native.exe.build"

echo [build] project: %CD%
echo [build] checking required tools...

where go >nul 2>nul
if errorlevel 1 (
  echo [build] ERROR: go.exe was not found in PATH.
  exit /b 1
)

where gcc >nul 2>nul
if errorlevel 1 (
  echo [build] ERROR: gcc.exe was not found in PATH. Fyne requires CGO/MinGW.
  exit /b 1
)

tasklist /FI "IMAGENAME eq %TARGET%" /FO CSV /NH 2>nul | find /I "%TARGET%" >nul
if not errorlevel 1 (
  echo [build] ERROR: %TARGET% is running. Close it before rebuilding.
  exit /b 1
)

if exist "%TEMP_TARGET%" del /q "%TEMP_TARGET%"

echo [build] go version:
go version

echo [build] compiling...
set "CGO_ENABLED=1"
go build -ldflags "-H windowsgui" -o "%TEMP_TARGET%" .
if errorlevel 1 (
  echo [build] ERROR: go build failed.
  if exist "%TEMP_TARGET%" del /q "%TEMP_TARGET%"
  exit /b 1
)

move /Y "%TEMP_TARGET%" "%TARGET%" >nul
if errorlevel 1 (
  echo [build] ERROR: could not replace %TARGET%.
  exit /b 1
)

echo [build] done:
dir "%TARGET%"
exit /b 0
