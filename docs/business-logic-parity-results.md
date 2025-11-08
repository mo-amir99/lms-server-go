# Business Logic Parity Verification Results

**Date:** October 30, 2025

> Canonical reference: Mongo implementation when Mongo and SQL diverge.

| Domain                  | Status              | Notes                                                                                                     |
| ----------------------- | ------------------- | --------------------------------------------------------------------------------------------------------- |
| Models                  | ⚠️ Issues Found     | Field constraints diverge (e.g., announcement title length 255 vs 80); missing upload session model       |
| Controllers             | ⚠️ Major Gaps       | Announcement/user handlers missing role/group logic; upload session controller absent; stream cache stubs |
| Routes                  | ⚠️ Partial          | Upload session routes missing; some routes simplified vs Node.js                                          |
| Middleware              | ⏳ Pending          |                                                                                                           |
| Services/Utilities      | ⏳ Pending          |                                                                                                           |
| Background Jobs         | ✅ Matches/Improved | Go adds jobs absent in Node.js                                                                            |
| Auth & Security         | ⏳ Pending          |                                                                                                           |
| Config & Initialization | ⏳ Pending          |                                                                                                           |
| Real-time / Socket      | ⏳ Pending          |                                                                                                           |

Detailed findings will follow per domain.

## Models

- **Announcement:** Go model uses `varchar(255)` for `title`/`content`, whereas SQL limits to 80/400 and Mongo enforces length validation. Missing trimming logic from Node setters.
- **UserWatch:** Added in Go, matches Mongo document conceptually. ✅
- **UploadSession:** No Go model exists, but both Node implementations include session/state tracking. ❌
- **Lesson:** ✅ Struct now preloads attachments, enforces name/description bounds, trims payloads, and mirrors Node validation semantics.

## Controllers

- **Announcement Handler:** lacks group-based filtering and public-only logic present in Node (Mongo filters announcements per group). Multiple TODOs remain.
- **User Handler:** missing role-based filtering and validation (TODOs). Node versions enforce role checks, subscription scoping, and password hashing flows.
- **Package Handler:** TODO indicates it only returns active packages, while Node controllers vary behaviour based on user roles.
- **Dashboard Handler:** Stream cache placeholders return empty arrays; Node versions surface active streams from cache. Meeting stats now integrated ✅.
- **Upload Session:** Entire controller absent in Go; Node handles chunked uploads and status tracking.
- **Auth / Password flows:** Go user creation uses plaintext password field; Node applies hashing and email confirmation logic (needs review in auth service). Pending deeper dive.
- **Lesson Handler:** ✅ Routes scoped under subscriptions, attachments preload enabled, watch-limit enforcement and signed video URLs implemented; upload queue/session lifecycle still outstanding.

## Routes

- Upload session routes not registered in Go.
- Some route guards differ (Go relies on middleware; need to confirm parity). Pending deeper review.
- Lesson routes now mirror subscription-scoped structure and expose video URL endpoint; upload queue stats & session lifecycle remain pending.

## Background Jobs

- Go implementation introduces three jobs (video processing sync, storage cleanup logging, subscription expiration). Node SQL has no jobs; Mongo has cron scripts but not equivalent. ✅ Improvement.

Further sections (Middleware, Services, Auth, Config, Real-time) remain pending detailed review.
