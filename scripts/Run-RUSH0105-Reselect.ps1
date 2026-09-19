# RUSH-010.5.1 — core-aware reselect from existing pool (NO generation)

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot\..

go build -o bin/cargoflow-boardmix.exe ./cmd/cargoflow-boardmix
if ($LASTEXITCODE -ne 0) { throw "build failed" }

.\bin\cargoflow-boardmix.exe `
  -reselect-existing output\RUSH0105_NativeCoreExpansionPilot_001 `
  -config configs\RUSH0105_NativeCoreExpansionPilot_001.json

Write-Host "Validate:"
.\bin\cargoflow-boardmix.exe -validate-batch output/RUSH0105_NativeCoreExpansionPilot_001
