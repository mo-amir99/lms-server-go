# Run database migrations
# This script creates/updates all database tables

Write-Host "Running database migrations..." -ForegroundColor Cyan
go run ./scripts/migrate/main.go

if ($LASTEXITCODE -eq 0) {
    Write-Host "`nMigrations completed successfully!" -ForegroundColor Green
} else {
    Write-Host "`nMigrations failed!" -ForegroundColor Red
    exit 1
}
