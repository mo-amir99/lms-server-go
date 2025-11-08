#!/bin/bash
# Drop all database tables
# ⚠️  WARNING: This will DELETE ALL DATA!

echo -e "\n⚠️  WARNING: This will drop all database tables!"
go run ./scripts/drop-tables/main.go

if [ $? -eq 0 ]; then
    echo -e "\nTables dropped successfully!"
else
    echo -e "\nFailed to drop tables!"
    exit 1
fi
