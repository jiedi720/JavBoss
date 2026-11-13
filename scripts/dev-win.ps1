# dev-win.ps1 — JavBoss local dev on Windows (no Docker / no NAS / no prod data)
#
# The repo's scripts/cli.sh is a bash script and does NOT run natively on Windows
# (use Git Bash or WSL for that). This PowerShell script covers the same local
# dev workflow on Windows PowerShell / PowerShell 7, keeping all runtime data in
# ./testdata so the production ./data directory, NAS and Docker volumes are never
# touched.
#
# IMPORTANT: internal/common/config.go hardcodes data/javboss.db,
# data/thumbnails and data/cover relative to the process working directory
# (cmd/server resolveBaseDir() -> os.Getwd() in development mode). The server
# does NOT read database_path from config.toml and has NO -config flag.
# Isolation is therefore achieved by running the server with ./testdata as its
# working directory, NOT by writing an override config file.
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
$DevPort  = 17654

# --- optional self-contained toolchain ---------------------------------------
# Only used when go / gcc are not already on PATH. Kept OUTSIDE the repository
# (workspace/_tools) so the checkout stays clean and no global install is needed.
$ToolsDir = Join-Path (Split-Path $RepoRoot -Parent) '_tools'
$GoBin    = Join-Path $ToolsDir 'go\bin'
$MingwBin = Join-Path $ToolsDir 'mingw64\bin'
if (Test-Path (Join-Path $GoBin 'go.exe')) { $env:PATH = "$GoBin;$env:PATH" }
if (Test-Path (Join-Path $MingwBin 'gcc.exe')) { $env:PATH = "$MingwBin;$env:PATH" }

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

# The isolated working directory is the only thing that keeps runtime data away
# from the repo's ./data and from any NAS / Docker volume. No .env, no config.toml
# override: the server has no -config flag and ignores database_path in TOML.
if (-not (Test-Path $TestData)) { New-Item -ItemType Directory -Path $TestData | Out-Null }

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
    $dbPath    = Join-Path $TestData 'data\javboss.db'
    $coverDir  = Join-Path $TestData 'data\cover'
    $thumbsDir = Join-Path $TestData 'data\thumbnails'
    Write-Host "==> backend: go run javboss/cmd/server -port $DevPort" -ForegroundColor Cyan
    Write-Host "    working dir : $TestData" -ForegroundColor Cyan
    Write-Host "    database    : $dbPath" -ForegroundColor Cyan
    Write-Host "    covers      : $coverDir" -ForegroundColor Cyan
    Write-Host "    thumbnails  : $thumbsDir" -ForegroundColor Cyan
    Write-Host "    (no NAS, no Docker, no repo ./data, no .env)" -ForegroundColor Cyan
    Push-Location $TestData
    try { go run javboss/cmd/server -port $DevPort } finally { Pop-Location }
}

function Start-Frontend {
    Write-Host "==> web: npm run dev  (proxy -> http://localhost:$DevPort)" -ForegroundColor Cyan
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
