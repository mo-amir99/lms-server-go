# PowerShell build script for Windows
# Usage: .\build.ps1 [version]

param(
    [string]$Version = "dev",
    [switch]$Docker
)

$ErrorActionPreference = "Stop"

# Get git information
try {
    $GitCommit = git rev-parse --short HEAD
    if (!$Version -or $Version -eq "dev") {
        $Version = git describe --tags --always --dirty 2>$null
        if (!$Version) { $Version = "dev" }
    }
} catch {
    $GitCommit = "unknown"
}

$BuildTime = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")

Write-Host "Building LMS Server..." -ForegroundColor Green
Write-Host "Version: $Version" -ForegroundColor Cyan
Write-Host "Commit: $GitCommit" -ForegroundColor Cyan
Write-Host "Build Time: $BuildTime" -ForegroundColor Cyan

# Create bin directory if it doesn't exist
New-Item -ItemType Directory -Force -Path bin | Out-Null

# Build the binary
$env:CGO_ENABLED = "0"
$env:GOOS = "windows"
$env:GOARCH = "amd64"

$ldflags = "-w -s " +
    "-X 'github.com/mo-amir99/lms-server-go/pkg/health.Version=$Version' " +
    "-X 'github.com/mo-amir99/lms-server-go/pkg/health.GitCommit=$GitCommit' " +
    "-X 'github.com/mo-amir99/lms-server-go/pkg/health.BuildTime=$BuildTime'"

go build -ldflags="$ldflags" -o bin\lms-server.exe .\cmd\app

Write-Host "Build complete: bin\lms-server.exe" -ForegroundColor Green

# Build Docker image if requested
if ($Docker) {
    Write-Host "Building Docker image..." -ForegroundColor Green
    docker build `
        --build-arg VERSION="$Version" `
        --build-arg GIT_COMMIT="$GitCommit" `
        --build-arg BUILD_TIME="$BuildTime" `
        -t lms-server:"$Version" `
        -t lms-server:latest `
        .
    Write-Host "Docker image built: lms-server:$Version" -ForegroundColor Green
}
