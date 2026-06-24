We are finalizing a strict multi-service architecture.

Target architecture:

- Backend = Control-plane only
    - HTTP API
    - JWT AuthN
    - AuthZ (via user-plan service)
    - NATS publish
    - Redis read only
    - NO Postgres access

- Persistence-service = Scan data-plane
    - Owns Postgres (scan tables)
    - Owns Redis write-through for scans
    - Handles scan lifecycle events

- NEW user-plan-service = Identity & Plan data-plane
    - Owns Postgres (users, plans, cafe_wallet tables)
    - Handles user CRUD, plan lookup, plan limits
    - Exposes NATS request/reply RPC API

Scanners remain unchanged (no DB).

----------------------------------------
STEP 0 — High-level goal
----------------------------------------

Remove ALL Postgres access from backend.
Backend must not import postgres package.
Backend must not initialize DB.
Backend must not wire repositories.

----------------------------------------
STEP 1 — Create new service: user-plan-service
----------------------------------------

Create:

cmd/userplan/
internal/userplan/

Responsibilities:
- Initialize Postgres (new dedicated schema for users/plans/wallets).
- Run migrations for:
    - users
    - plans
    - cafe_wallet
- Bootstrap default plans (Free, etc.)

Implement NATS request/reply endpoints:

Subjects:

- user.get_by_email
- user.create
- user.exists_by_email
- plan.get_by_type
- plan.get_for_user
- plan.check_scan_limit
- wallet.get_by_user
- wallet.create
- wallet.update
- wallet.delete

Each handler:
- Validates input
- Uses repositories (gorm)
- Returns JSON response
- Returns structured error if needed

This service is the ONLY owner of:
- users table
- plans table
- cafe_wallet table

----------------------------------------
STEP 2 — Refactor Backend to remove Postgres
----------------------------------------

Remove from backend:

- postgres.New()
- db.Run()
- runMigrations()
- Any repository wiring in internal/app/container.go
- Any usage of gorm.DB

Delete all Postgres imports.

Backend must:

AuthN:
- Validate JWT only (no DB lookup for now).

AuthZ:
- Use NATS request/reply to user-plan-service:
    - plan.get_for_user
    - plan.check_scan_limit

User creation / signup:
- Publish request to user.create

Wallet endpoints:
- Forward to user-plan-service via NATS request/reply.

Scan endpoints:
- POST scan:
    - Check plan via NATS
    - Publish scan.requested.<kind>
    - Do NOT create any DB row

- GET scan:
    - Redis only
    - On miss: return NOT_READY
    - Do NOT query Postgres

----------------------------------------
STEP 3 — Ensure Scan lifecycle only in persistence-service
----------------------------------------

Persistence-service:
- Owns scan tables
- On scan.started:
    - Create row (if not exists)
- On scan.completed:
    - Update row
- On scan.failed:
    - Update row

Backend must not reference scan repositories.

----------------------------------------
STEP 4 — Migrations separation
----------------------------------------

Persistence-service:
- Owns scan-related migrations only.

User-plan-service:
- Owns users/plans/wallet migrations only.

Remove all migrations from backend.

----------------------------------------
STEP 5 — README update
----------------------------------------

Update README:

Backend:
- No Postgres access
- Control-plane only
- Talks to:
    - user-plan-service via NATS RPC
    - persistence-service via events
    - Redis read-only

User-plan-service:
- Owns identity and plan data
- Postgres owner for user/plan tables

Persistence-service:
- Owns scan lifecycle data
- Postgres owner for scan tables

Add architecture diagram section explaining:
- separation of control-plane and data-plane
- strict DB isolation
- future possibility of Docker network separation

----------------------------------------
CONSTRAINTS
----------------------------------------

- Do not change HTTP API shapes.
- Do not introduce hot plugins.
- Do not modify docker-compose networking yet.
- Keep services minimal and clean.
- Use existing NATS infrastructure.
- Implement request/reply using NATS with timeout and proper error handling.

----------------------------------------
END GOAL
----------------------------------------

After this refactor:

Backend:
- Zero Postgres imports.
- Zero DB initialization.
- Zero DB repositories.
- Pure control-plane.

Persistence-service:
- Single writer for scan data.

User-plan-service:
- Single writer for user/plan/wallet data.