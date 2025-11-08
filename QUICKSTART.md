# Quick Start Guide - Go Backend

## Database Setup

The Go backend now uses separate scripts for database management instead of automatic migrations on startup.

### 1. Run Migrations

Create/update all database tables:

**Windows (PowerShell):**

```powershell
.\scripts\migrate.ps1
```

**Linux/Mac (Bash):**

```bash
chmod +x scripts/*.sh
./scripts/migrate.sh
```

### 2. Create Super Admin

Create your first admin account:

**Windows:**

```powershell
.\scripts\create-superadmin.ps1
```

**Linux/Mac:**

```bash
./scripts/create-superadmin.sh
```

### 3. Start the Server

**Development:**

```powershell
go run ./cmd/app .
```

**Production:**

```powershell
go build -o lms-server ./cmd/app
./lms-server
```

## Environment Configuration

Make sure your `.env` file is configured with database credentials:

```env
# Database
LMS_DB_HOST=localhost
LMS_DB_PORT=5432
LMS_DB_USER=postgres
LMS_DB_PASSWORD=your_password
LMS_DB_NAME=lms
LMS_DB_SSLMODE=disable

# Migrations (default: false - run separately)
LMS_DB_RUN_MIGRATIONS=false

# Other required configs...
```

## Available Scripts

| Script                                           | Purpose                       | Warning                    |
| ------------------------------------------------ | ----------------------------- | -------------------------- |
| `migrate.ps1` / `migrate.sh`                     | Create/update database tables | Safe to run multiple times |
| `create-superadmin.ps1` / `create-superadmin.sh` | Create admin user             | Email must be unique       |
| `drop-tables.ps1` / `drop-tables.sh`             | Delete all tables             | ⚠️ DESTROYS ALL DATA       |

See `scripts/README.md` for detailed documentation.

## Common Issues

### "table already exists" error

- This is normal if tables exist - GORM AutoMigrate is idempotent
- To start fresh: run `drop-tables` then `migrate`

### Slow query warnings

- These are from GORM introspecting schema metadata
- Normal on first run or after schema changes
- Will be faster on subsequent runs

### Migration fails

- Check database connection in `.env`
- Ensure PostgreSQL is running
- Verify credentials are correct

## Migration from Node.js

The Go backend has full parity with the Node.js implementation. Key differences:

1. **Migrations**: Now run separately via scripts (not automatic on startup)
2. **Attachment Uploads**: Use signed URLs for direct Bunny Storage uploads
3. **Lesson Videos**: Use signed URLs for direct Bunny Stream uploads
4. **No Queue**: Bull queue replaced with direct uploads

See `docs/frontend_migration_go.md` for complete migration guide.

## Development Workflow

1. **Fresh Start:**

   ```powershell
   .\scripts\drop-tables.ps1
   .\scripts\migrate.ps1
   .\scripts\create-superadmin.ps1
   go run ./cmd/app .
   ```

2. **Schema Updates:**

   ```powershell
   # Update model files
   .\scripts\migrate.ps1  # Apply changes
   go run ./cmd/app .
   ```

3. **Add Admin:**
   ```powershell
   .\scripts\create-superadmin.ps1
   ```

## Testing

```powershell
# Run all tests
go test ./...

# Run specific package tests
go test ./internal/features/user

# With coverage
go test -cover ./...
```

## Production Deployment

1. Build the binary:

   ```bash
   go build -o lms-server ./cmd/app
   ```

2. Run migrations on production database:

   ```bash
   ./scripts/migrate.sh
   ```

3. Create production admin:

   ```bash
   ./scripts/create-superadmin.sh
   ```

4. Start server:
   ```bash
   ./lms-server
   ```

## Support

For issues or questions:

- Check `scripts/README.md` for script documentation
- Check `docs/go_parity_todo.md` for feature status
- Check `docs/frontend_migration_go.md` for API changes
