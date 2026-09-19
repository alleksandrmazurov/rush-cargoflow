# RUSH-010.7 production pilot — manual only.
# This script performs generation and is intentionally not run by Cursor.

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot\..
$env:GOCACHE = "E:\.gocache-rush"

go build -o bin/cargoflow-boardmix.exe ./cmd/cargoflow-boardmix
if ($LASTEXITCODE -ne 0) { throw "build failed" }

.\bin\cargoflow-boardmix.exe `
  -config configs\RUSH0107_CrossRegionCausalPilot_001.json `
  -resume=true
if ($LASTEXITCODE -ne 0) { throw "pilot failed" }

.\bin\cargoflow-boardmix.exe `
  -validate-batch output\RUSH0107_CrossRegionCausalPilot_001
if ($LASTEXITCODE -ne 0) { throw "deep validation failed" }
