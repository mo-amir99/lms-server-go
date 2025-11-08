# Business Logic Parity Verification Plan

**Date:** October 30, 2025

## Goal

Verify that every domain (models, controllers, routes, background jobs, middleware, utilities) in the Go implementation matches the business logic of:

1. Node.js SQL implementation (`controllers/`, `models/`, `services/`)
2. Node.js Mongo implementation (`mongo-implementation/` folder)

If SQL and Mongo differ, prefer Mongo as the canonical reference.

## Methodology

1. Inventory all Go domains to review (controllers, services, models, jobs, middleware, utils).
2. For each domain:
   - Identify corresponding Node.js SQL and Mongo files.
   - Compare endpoint behaviour, validation, side effects, and data transformations.
   - Document any differences, categorize as:
     - ✅ Matches (with Go optimizations allowed)
     - ⚠️ Diverges (requires fix or justification)
     - ➕ Improvements (Go adds behaviour missing in Node)
   - Record references (file paths + line numbers when useful).
3. Summarize findings and required fixes per domain.
4. After full review, run comprehensive tests/builds.

## Domains to Review

1. Models (internal/features/\*\*/model.go, pkg/entities, etc.)
2. Controllers/Handlers (internal/features/\*\*/handler.go)
3. Routes (internal/features/\*\*/routes.go, internal/http/routes/routes.go)
4. Middleware (middlewares directory)
5. Services/Utilities (services, pkg/services, utils)
6. Background Jobs (pkg/jobs)
7. Auth & Security (middlewares/auth, security)
8. Config & Initialization (cmd/app/main.go, config)
9. Socket/Real-time behaviour (if applicable)

## Tracking

A separate summary file (`business-logic-parity-results.md`) will capture detailed comparisons and action items per domain.
