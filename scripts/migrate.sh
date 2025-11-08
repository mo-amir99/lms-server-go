#!/bin/bash
# Run database migrations
# This script creates/updates all database tables

echo "Running database migrations..."
go run ./scripts/migrate/main.go

if [ $? -eq 0 ]; then
    echo -e "\nMigrations completed successfully!"
else
    echo -e "\nMigrations failed!"
    exit 1
fi
