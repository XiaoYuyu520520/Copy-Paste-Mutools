@echo off
setlocal EnableExtensions

cd /d "%~dp0"

set "APP_ID=prompt-merger-native"
set "APP_NAME=merger-native"
set "APP_VERSION=0.0.1"
set "FYNE_CROSS_VERSION=v1.6.3"
set "ICON_PATH=ico/icon.png"

echo [multibuild] project: %CD%
echo [multibuild] target: windows/amd64, linux/amd64, linux/arm64

where go >nul 2>nul
if errorlevel 1 (
  echo [multibuild] ERROR: go.exe was not found in PATH.
  exit /b 1
)

where docker >nul 2>nul
if errorlevel 1 (
  echo [multibuild] ERROR: Docker was not found in PATH.
  echo [multibuild] Install Docker Desktop and start it before running this script.
  exit /b 1
)

docker info >nul 2>nul
if errorlevel 1 (
  echo [multibuild] ERROR: Docker is installed but its engine is not running.
  echo [multibuild] Start Docker Desktop and run this script again.
  exit /b 1
)

echo [multibuild] building Windows amd64...
go run github.com/fyne-io/fyne-cross@%FYNE_CROSS_VERSION% windows ^
  -arch=amd64 -name "%APP_NAME%" -app-id "%APP_ID%" -app-version "%APP_VERSION%" ^
  -icon "%ICON_PATH%" -release
if errorlevel 1 exit /b 1

echo [multibuild] building Linux amd64 and arm64...
go run github.com/fyne-io/fyne-cross@%FYNE_CROSS_VERSION% linux ^
  -arch=amd64,arm64 -name "%APP_NAME%" -app-id "%APP_ID%" -app-version "%APP_VERSION%" ^
  -icon "%ICON_PATH%" -release
if errorlevel 1 exit /b 1

echo [multibuild] done. Output is under fyne-cross\dist.
echo [multibuild] macOS packages are produced by .github\workflows\build-multiplatform.yml
echo [multibuild] because non-macOS hosts require an Apple SDK and Xcode license.
exit /b 0
