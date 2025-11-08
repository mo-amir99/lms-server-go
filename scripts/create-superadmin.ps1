# Create a super admin user
# This script creates a new super admin account

Write-Host "Creating super admin user..." -ForegroundColor Cyan
go run ./scripts/create-superadmin/main.go

if ($LASTEXITCODE -eq 0) {
    Write-Host "`nSuper admin created successfully!" -ForegroundColor Green
} else {
    Write-Host "`nFailed to create super admin!" -ForegroundColor Red
    exit 1
}
