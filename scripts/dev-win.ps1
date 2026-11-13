# dev-win.ps1 — JavBoss local dev on Windows (no Docker / no NAS / no prod data)
#
# The repo's scripts/cli.sh is a bash script and does NOT run natively on Windows
# (use Git Bash or WSL for that). This PowerShell script covers the same local
# dev workflow on Windows PowerShell / PowerShell 7, keeping all runtime data in
# ./testdata so the production ./data directory, NAS and Docker volumes are never
# touched.
#
# Usage examples:
#   .\scripts\dev-win.ps1 build           # go build ./... + web production build
#   .\scripts\dev-win.ps1 test            # go test (db/api/models) + web lint/format/build
#   .\scripts\dev-win.ps1 backend         # run the API server against ./testdata
#   .\scripts\dev-win.ps1 frontend        # run the Vite dev server
#   .\scripts\dev-win.ps1 all             # build + test everything (default)

$ErrorActionPreference = 'Stop'

$RepoRoot = Resolve-Path (Join-Path $PSScriptRoot '..')
$GoCache  = Join-Path $RepoRoot '.gocache'
$TestData = Join-Path $RepoRoot 'testdata'
$WebDir   = Join-Path $RepoRoot 'web'
$ConfigPath = Join-Path $TestData 'config.toml'

# --- prerequisite checks -----------------------------------------------------
function Test-Command($cmd) { try { Get-Command $cmd -ErrorAction Stop | Out-Null; return $true } catch { return $false } }

$goOk = Test-Command go
$nodeOk = Test-Command node
if (-not $goOk) { Write-Error "Go is not installed. Required: Go 1.25.1+ (see DEVELOPMENT.md)." }
if (-not $nodeOk) { Write-Error "Node.js is not installed. Required: Node.js 24 (see DEVELOPMENT.md)." }

$nodeVer = (node --version).TrimStart('v')  # e.g. 24.0.0
$nodeMajor = [int]($nodeVer.Split('.')[0])
if ($nodeMajor -lt 24) {
    Write-Warning "Node.js $nodeVer detected. DEVELOPMENT.md requires Node.js 24. Tests may still pass on 22, but CI/Docker use 24."
}

# Ensure the isolated test data directory exists and has its own config.
if (-not (Test-Path $TestData)) { New-Item -ItemType Directory -Path $TestData | Out-Null }
if (-not (Test-Path $ConfigPath)) {
    # database_path points inside testdata; thumbnails/cover dirs are derived from it.
    @"
database_path = "testdata/javboss.db"
"@ | Set-Content -Path $ConfigPath -Encoding utf8
    Write-Host "Wrote isolated config -> $ConfigPath (database_path = testdata/javboss.db)"
}

$env:GOCACHE = $GoCache
$env:CGO_ENABLED = '1'

function Invoke-BackendBuild {
    Write-Host "==> go build ./..." -ForegroundColor Cyan
    Push-Location $RepoRoot
    try { go build ./... } finally { Pop-Location }
}

function Invoke-BackendTest {
    Write-Host "==> go test ./internal/db/... ./internal/api/... ./internal/models/..." -ForegroundColor Cyan
    Push-Location $RepoRoot
    try { go test ./internal/db/... ./internal/api/... ./internal/models/... } finally { Pop-Location }
}

function Invoke-FrontendChecks {
    Write-Host "==> web: npm install + lint + format:check + build" -ForegroundColor Cyan
    Push-Location $WebDir
    try {
        npm install
        npm run lint
        npm run format:check
        npm run build
    } finally { Pop-Location }
}

function Invoke-FrontendTest {
    Write-Host "==> web: npm test (node --test)" -ForegroundColor Cyan
    Write-Warning "The full 'npm test' suite can hang on some machines (see report). Run targeted tests if it stalls:"
    Write-Host "  cd web; node --test tests/features/jav/actorApi.test.js" -ForegroundColor Yellow
    Push-Location $WebDir
    try { npm test } finally { Pop-Location }
}

function Start-Backend {
    Write-Host "==> go run ./cmd/server (config: $ConfigPath)" -ForegroundColor Cyan
    Push-Location $RepoRoot
    try { go run ./cmd/server -config $ConfigPath } finally { Pop-Location }
}

function Start-Frontend {
    Write-Host "==> web: npm run dev" -ForegroundColor Cyan
    Push-Location $WebDir
    try { npm run dev } finally { Pop-Location }
}

switch ($args[0]) {
    'build'    { Invoke-BackendBuild; Invoke-FrontendChecks }
    'test'     { Invoke-BackendTest; Invoke-FrontendChecks; Invoke-FrontendTest }
    'backend'  { Start-Backend }
    'frontend' { Start-Frontend }
    'all'      { Invoke-BackendBuild; Invoke-BackendTest; Invoke-FrontendChecks; Invoke-FrontendTest }
    default    { Invoke-BackendBuild; Invoke-BackendTest; Invoke-FrontendChecks; Invoke-FrontendTest }
}
