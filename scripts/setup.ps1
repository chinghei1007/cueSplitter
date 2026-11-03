param(
  [switch]$Check,
  [switch]$RuntimeOnly
)

$ErrorActionPreference = "Stop"

function Has-Command([string]$Name) {
  return $null -ne (Get-Command $Name -ErrorAction SilentlyContinue)
}

$needGo = -not $RuntimeOnly -and -not (Has-Command "go")
$needNode = -not $RuntimeOnly -and (-not (Has-Command "node") -or -not (Has-Command "npm"))
$needFFmpeg = -not (Has-Command "ffmpeg") -or -not (Has-Command "ffprobe")

function Show-Status {
  if (-not $RuntimeOnly) {
    if (Has-Command "go") { Write-Host "Go: $(go version)" } else { Write-Host "Go: missing" }
    if (Has-Command "node") { Write-Host "Node: $(node --version)" } else { Write-Host "Node: missing" }
    if (Has-Command "npm") { Write-Host "npm: $(npm --version)" } else { Write-Host "npm: missing" }
  }
  if (Has-Command "ffmpeg") { Write-Host "FFmpeg: available" } else { Write-Host "FFmpeg: missing" }
  if (Has-Command "ffprobe") { Write-Host "FFprobe: available" } else { Write-Host "FFprobe: missing" }
}

if (-not $needGo -and -not $needNode -and -not $needFFmpeg) {
  Show-Status
  Write-Host "Cue Splitter dependencies are ready."
  exit 0
}

if ($Check) {
  Show-Status
  exit 1
}

if (-not (Has-Command "winget")) {
  throw "winget is required for automatic installation on Windows. Install App Installer from Microsoft Store, then rerun this script."
}

$wingetArgs = @("install", "--exact", "--accept-package-agreements", "--accept-source-agreements", "--silent")
if ($needGo) { & winget @wingetArgs --id GoLang.Go }
if ($needNode) { & winget @wingetArgs --id OpenJS.NodeJS.LTS }
if ($needFFmpeg) { & winget @wingetArgs --id Gyan.FFmpeg }

$machinePath = [Environment]::GetEnvironmentVariable("Path", "Machine")
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
$env:Path = "$machinePath;$userPath"

if (-not $RuntimeOnly) {
  if (-not (Has-Command "go")) { throw "Go installation did not become available on PATH. Open a new PowerShell and rerun." }
  if (-not (Has-Command "node") -or -not (Has-Command "npm")) { throw "Node.js/npm did not become available on PATH. Open a new PowerShell and rerun." }
}
if (-not (Has-Command "ffmpeg") -or -not (Has-Command "ffprobe")) { throw "FFmpeg/FFprobe did not become available on PATH. Open a new PowerShell and rerun." }

Show-Status
Write-Host "Cue Splitter dependencies are ready."
