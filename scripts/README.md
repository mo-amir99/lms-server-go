# Database Management Scripts

This directory contains scripts for managing the LMS database.

## Available Scripts

### 1. Migrate Database

Creates or updates all database tables using GORM AutoMigrate.

**PowerShell:**

```powershell
.\scripts\migrate.ps1
```

**Bash:**

```bash
./scripts/migrate.sh
```

**Direct:**

```bash
go run ./scripts/migrate/main.go
```

### 2. Create Super Admin

Creates a new super admin user account interactively.

**PowerShell:**

```powershell
.\scripts\create-superadmin.ps1
```

**Bash:**

```bash
./scripts/create-superadmin.sh
```

**Direct:**

```bash
go run ./scripts/create-superadmin/main.go
```

You will be prompted for:

- Full Name
- Email
- Password (minimum 8 characters)
- Phone (optional)

### 3. Drop All Tables

⚠️ **WARNING**: Permanently deletes all database tables and data!

**PowerShell:**

```powershell
.\scripts\drop-tables.ps1
```

**Bash:**

```bash
./scripts/drop-tables.sh
```

**Direct:**

```bash
go run ./scripts/drop-tables/main.go
```

You will be asked to confirm by typing `DROP ALL TABLES`.

## Environment Variables

All scripts use the same environment variables as the main application:

- `LMS_DB_HOST` - Database host (default: localhost)
- `LMS_DB_PORT` - Database port (default: 5432)
- `LMS_DB_USER` - Database user
- `LMS_DB_PASSWORD` - Database password
- `LMS_DB_NAME` - Database name
- `LMS_DB_SSLMODE` - SSL mode (default: disable)

Make sure your `.env` file is configured before running any scripts.

## Common Workflows

### Fresh Database Setup

```bash
# 1. Drop existing tables (if any)
.\scripts\drop-tables.ps1

# 2. Run migrations
.\scripts\migrate.ps1

# 3. Create super admin
.\scripts\create-superadmin.ps1
```

### Update Schema

```bash
# Run migrations to update existing tables
.\scripts\migrate.ps1
```

### Add New Admin

```bash
# Create additional super admin users
.\scripts\create-superadmin.ps1
```

## Notes

- **Migrations** are idempotent - safe to run multiple times
- **Drop Tables** requires explicit confirmation
- **Super Admin** emails must be unique
- All scripts connect directly to the database without starting the server
