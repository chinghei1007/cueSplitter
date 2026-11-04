@echo off
setlocal

cd /d "%~dp0"
set "APP=bin\cue-splitter.exe"

if not exist "%APP%" (
  echo Cue Splitter has not been built yet. Building it now...

  powershell.exe -NoProfile -ExecutionPolicy Bypass -File "scripts\setup.ps1"
  if errorlevel 1 exit /b 1

  call npm --prefix frontend ci
  if errorlevel 1 exit /b 1

  call npm --prefix frontend run build
  if errorlevel 1 exit /b 1

  if not exist bin mkdir bin
  go build -o "%APP%" .\cmd\cuesplitter
  if errorlevel 1 exit /b 1
) else (
  powershell.exe -NoProfile -ExecutionPolicy Bypass -File "scripts\setup.ps1" -RuntimeOnly
  if errorlevel 1 exit /b 1
)

echo Starting Cue Splitter at http://127.0.0.1:8080
echo Press Ctrl+C to stop it.
"%APP%" %*
