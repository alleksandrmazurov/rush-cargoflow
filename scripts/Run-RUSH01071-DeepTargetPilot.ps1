# RUSH-010.7.1 production pilot — manual only.
# Cursor does not run this script automatically.

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot\..
$env:GOCACHE = "E:\.gocache-rush"

go build -o bin/cargoflow-boardmix.exe ./cmd/cargoflow-boardmix
if ($LASTEXITCODE -ne 0) { throw "build failed" }

.\bin\cargoflow-boardmix.exe `
  -config configs\RUSH01071_DeepTargetPilot_001.json `
  -resume=true
if ($LASTEXITCODE -ne 0) { throw "pilot failed" }

.\bin\cargoflow-boardmix.exe `
  -validate-batch output\RUSH01071_DeepTargetPilot_001
if ($LASTEXITCODE -ne 0) { throw "batch validation failed" }
