param([ValidateSet('serve','migrate','seed','purge-location','test','vet')][string]$Command = 'serve')
$ErrorActionPreference = 'Stop'
$taskProjectRoot = Split-Path -Parent $PSScriptRoot
$taskEnvPath = Join-Path $taskProjectRoot '.env'
if (Test-Path -LiteralPath $taskEnvPath) {
 Get-Content -LiteralPath $taskEnvPath | ForEach-Object {
  if ($_ -match '^([A-Z_]+)=(.*)$') { [Environment]::SetEnvironmentVariable($matches[1], $matches[2], 'Process') }
 }
}
Push-Location (Join-Path $taskProjectRoot 'backend')
try {
 if ($Command -eq 'test') {
  if (-not $env:TEST_DATABASE_URL) { $env:TEST_DATABASE_URL = $env:DATABASE_URL }
  go test ./... -count=1
 } elseif ($Command -eq 'vet') { go vet ./... }
 else { go run ./cmd/api -command $Command }
 if ($LASTEXITCODE -ne 0) { throw "Backend command failed: $Command" }
} finally { Pop-Location }
