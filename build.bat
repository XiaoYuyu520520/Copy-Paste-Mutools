@echo off
setlocal EnableExtensions

cd /d "%~dp0"

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
  echo [build] Install MSYS2 UCRT64 and add C:\msys64\ucrt64\bin to PATH.
  exit /b 1
)

if not exist "bin" mkdir "bin"

set "CGO_ENABLED=1"
set "LDFLAGS=-H windowsgui -s -w"

echo [build] building merger (desktop app)...
go build -trimpath -ldflags "%LDFLAGS%" -o "bin\merger.exe" ./cmd/merger
if errorlevel 1 (
  echo [build] ERROR: failed to build merger.
  exit /b 1
)

echo [build] building toolbox (cli)...
go build -trimpath -ldflags "-s -w" -o "bin\toolbox.exe" ./cmd/toolbox
if errorlevel 1 (
  echo [build] ERROR: failed to build toolbox.
  exit /b 1
)

echo [build] building paint (drawing board)...
go build -trimpath -ldflags "%LDFLAGS%" -o "bin\paint.exe" ./cmd/paint
if errorlevel 1 (
  echo [build] ERROR: failed to build paint.
  exit /b 1
)

echo [build] done. Output:
dir "bin"
exit /b 0
