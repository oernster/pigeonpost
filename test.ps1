# Checks formatting, runs go vet, then runs the Go test suite with coverage and enforces the hard 100%
# gate on the correctness core (internal/domain and internal/application). Prints the full per-package
# report.
#
#   ./test.ps1          check formatting, vet, run tests and enforce the gate
#   ./test.ps1 -Html    also open the HTML coverage report in a browser
#
# Formatting and vet run BEFORE the tests, because they are the cheap checks: there is no sense waiting
# out a full coverage run to be told about a stray blank line. Each fails the script outright, so a
# green run means all three passed and not merely the last of them.
param(
    [switch]$Html
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $root

$coverProfile = Join-Path $env:TEMP 'pigeonpost.cov'

# gofmt -l names every file it would reformat and says nothing about the rest, so an empty result is
# the pass. It is run over the whole tree rather than a subset: a check narrowed until it is quiet is
# not a check. This was green from the day it was added only because the working copy had first been
# normalised to LF, which .gitattributes already declares; before that, gofmt reported all 131 CRLF
# files as needing reformatting and no scoping short of ignoring them would have made it meaningful.
Write-Host 'Checking formatting...'
$unformatted = gofmt -l .
if ($LASTEXITCODE -ne 0) {
    Write-Error 'gofmt failed to run.'
    exit 1
}
if ($unformatted) {
    Write-Host ''
    Write-Host 'FORMATTING FAILED. Run gofmt -w on these files:' -ForegroundColor Red
    $unformatted | ForEach-Object { Write-Host "  $_" -ForegroundColor Red }
    exit 1
}

# go vet catches suspect constructs the compiler accepts. It is worth its own step even though go test
# runs a handful of the same analysers: the subset go test applies is a short list (printf and a few
# others), so everything outside it, a copied lock or a malformed struct tag, reaches nothing without
# this line.
Write-Host 'Vetting...'
go vet ./...
if ($LASTEXITCODE -ne 0) {
    Write-Error 'go vet failed.'
    exit 1
}

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
