# Drop all database tables
# ⚠️  WARNING: This will DELETE ALL DATA!

Write-Host "⚠️  WARNING: This will drop all database tables!" -ForegroundColor Red
go run ./scripts/drop-tables/main.go

if ($LASTEXITCODE -eq 0) {
    Write-Host "`nTables dropped successfully!" -ForegroundColor Green
} else {
    Write-Host "`nFailed to drop tables!" -ForegroundColor Red
    exit 1
}
