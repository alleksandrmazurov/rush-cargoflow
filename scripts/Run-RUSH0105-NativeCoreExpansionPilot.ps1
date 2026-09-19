# RUSH-010.5 Native Core Expansion — heavy pilot (manual)
# Do NOT run from Cursor; expect multi-hour wall time.

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot\..

go build -o bin/cargoflow-boardmix.exe ./cmd/cargoflow-boardmix
if ($LASTEXITCODE -ne 0) { throw "build failed" }

# Optional: refresh calibration report on prior batch
# .\bin\cargoflow-boardmix.exe -calibrate-core-space output/RUSH01041_NativeFamilyCoveragePilot_001

.\bin\cargoflow-boardmix.exe `
  -config configs/RUSH0105_NativeCoreExpansionPilot_001.json `
  -resume=true

Write-Host "Validate:"
.\bin\cargoflow-boardmix.exe -validate-batch output/RUSH0105_NativeCoreExpansionPilot_001
