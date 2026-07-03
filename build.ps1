# Builds PigeonPost: the application executable and the bespoke Wails setup installer.
#
#   ./build.ps1                 build the app exe and the installer
#   ./build.ps1 -SkipInstaller  build only the app exe
#
# Outputs:
#   build/bin/PigeonPost.exe            the application
#   dist-installer/PigeonPostSetup.exe  the setup program (embeds the app as its payload)
param(
    [switch]$SkipInstaller
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $root

$version = (Get-Content (Join-Path $root 'VERSION')).Trim()
Write-Host "Building PigeonPost $version"

Write-Host 'Generating icons from pigeonpost.png...'
go run ./tools/genicons

Write-Host 'Building application (wails)...'
wails build

if ($SkipInstaller) {
    Write-Host 'Done: build/bin/PigeonPost.exe'
    exit 0
}

Write-Host 'Packaging application as installer payload...'
$payload = Join-Path $root 'installer/payload.zip'
if (Test-Path $payload) { Remove-Item $payload }
Compress-Archive -Path (Join-Path $root 'build/bin/*') -DestinationPath $payload

Write-Host 'Building installer (wails)...'
Push-Location (Join-Path $root 'installer')
try {
    wails build -ldflags "-X main.appVersion=$version"
} finally {
    Pop-Location
}

Write-Host 'Collecting installer...'
$distDir = Join-Path $root 'dist-installer'
New-Item -ItemType Directory -Force -Path $distDir | Out-Null
Copy-Item (Join-Path $root 'installer/build/bin/PigeonPostSetup.exe') (Join-Path $distDir 'PigeonPostSetup.exe') -Force

# Restore the tiny empty-zip placeholder so `go build ./...` keeps working without a full build.
$placeholder = New-Object byte[] 22
$placeholder[0] = 0x50; $placeholder[1] = 0x4B; $placeholder[2] = 0x05; $placeholder[3] = 0x06
[System.IO.File]::WriteAllBytes($payload, $placeholder)

Write-Host "Done: dist-installer/PigeonPostSetup.exe ($version)"
