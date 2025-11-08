#!/bin/bash

# Build script for production deployment
# Usage: ./build.sh [version]

set -e

VERSION=${1:-$(git describe --tags --always --dirty)}
GIT_COMMIT=$(git rev-parse --short HEAD)
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

echo "Building LMS Server..."
echo "Version: ${VERSION}"
echo "Commit: ${GIT_COMMIT}"
echo "Build Time: ${BUILD_TIME}"

# Build the binary
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s \
    -X 'github.com/mo-amir99/lms-server-go/pkg/health.Version=${VERSION}' \
    -X 'github.com/mo-amir99/lms-server-go/pkg/health.GitCommit=${GIT_COMMIT}' \
    -X 'github.com/mo-amir99/lms-server-go/pkg/health.BuildTime=${BUILD_TIME}'" \
    -o bin/lms-server ./cmd/app

echo "Build complete: bin/lms-server"

# Build Docker image if requested
if [ "$2" == "docker" ]; then
    echo "Building Docker image..."
    docker build \
        --build-arg VERSION="${VERSION}" \
        --build-arg GIT_COMMIT="${GIT_COMMIT}" \
        --build-arg BUILD_TIME="${BUILD_TIME}" \
        -t lms-server:"${VERSION}" \
        -t lms-server:latest \
        .
    echo "Docker image built: lms-server:${VERSION}"
fi
