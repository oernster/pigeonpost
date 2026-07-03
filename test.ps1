# Runs the Go test suite with coverage and enforces the hard 100% gate on the correctness core
# (internal/domain and internal/application). Prints the full per-package report.
#
#   ./test.ps1          run tests and enforce the gate
#   ./test.ps1 -Html    also open the HTML coverage report in a browser
param(
    [switch]$Html
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $root

$coverProfile = Join-Path $env:TEMP 'pigeonpost.cov'

Write-Host 'Running tests with coverage...'
go test ./... -covermode=count -coverprofile=$coverProfile
if ($LASTEXITCODE -ne 0) {
    Write-Error 'Test run failed.'
    exit 1
}

Write-Host ''
Write-Host 'Coverage by function:'
$funcs = go tool cover -func=$coverProfile
$funcs | Write-Output

# The gate: these packages hold the correctness core and must be fully covered.
$gatedPackages = @('internal/domain', 'internal/application')
$violations = @()
foreach ($line in $funcs) {
    foreach ($pkg in $gatedPackages) {
        if ($line -match "/$pkg/" -and $line -match '([\d.]+)%\s*$') {
            if ([double]$Matches[1] -lt 100.0) {
                $violations += $line
            }
        }
    }
}

if ($violations.Count -gt 0) {
    Write-Host ''
    Write-Host 'COVERAGE GATE FAILED. These gated statements are not covered:' -ForegroundColor Red
    $violations | ForEach-Object { Write-Host "  $_" -ForegroundColor Red }
    exit 1
}

if ($Html) {
    go tool cover -html=$coverProfile
}

Write-Host ''
Write-Host 'Coverage gate passed: internal/domain and internal/application are at 100%.' -ForegroundColor Green
