#!/bin/bash
# Create a super admin user
# This script creates a new super admin account

echo "Creating super admin user..."
go run ./scripts/create-superadmin/main.go

if [ $? -eq 0 ]; then
    echo -e "\nSuper admin created successfully!"
else
    echo -e "\nFailed to create super admin!"
    exit 1
fi
