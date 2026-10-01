# GOR Sports Booking & Management System — API

Backend service for a GOR (Gelanggang Olahraga / sports hall) booking platform.
Customers browse sports and facilities, check slot availability, create bookings,
pay for them, and register for events; staff and admins manage the catalog,
pricing rules, closures, bookings, payments, events, users and audit logs.

The service is a single Go binary backed by PostgreSQL, with JWT authentication
and role-based authorization (`CUSTOMER`, `STAFF`, `ADMIN`).

## Tech stack

| Layer      | Choice                                          |
| ---------- | ----------------------------------------------- |
| Language   | Go 1.27                                         |
| HTTP       | Gin                                             |
| Database   | PostgreSQL 17 (pgx/v5 driver, embedded migrations) |
| Auth       | JWT (`golang-jwt/jwt/v5`), bcrypt password hashes |
| Validation | go-playground/validator                         |
| Container  | Multi-stage build → distroless static, nonroot  |

## Prerequisites

- Go 1.27+
- Docker 24+ with the Compose v2 plugin (`docker compose`)
- PostgreSQL 17 (only if you run the API outside Compose)

## Quick start (Docker Compose)

From the repository root:

```bash
docker compose up -d --build
```

Compose starts PostgreSQL (with a health check) and then the API, which applies
its embedded migrations and seeds reference data on first boot. Verify:

```bash
curl -s http://localhost:8080/health
# {"status":"ok","database":"up"}
```

Follow logs and shut down:

```bash
docker compose logs -f api
docker compose down          # add -v to also drop the postgres volume
```

## Local development

Create your environment file, then run the API directly:

```bash
cd backend-go
cp .env.example .env
go run ./cmd/api
```

With `MIGRATE_ON_START=true` the embedded migrations in
`internal/database/migrations/` are applied automatically at startup, and with
`SEED_ON_START=true` reference data plus the initial admin account
(`ADMIN_EMAIL` / `ADMIN_PASSWORD`) are inserted. Set either flag to `false` once
your database is managed out of band.

The `Makefile` wraps the common commands:

```bash
make help      # list targets
make run       # go run ./cmd/api
make test      # go test ./...
make test-race # go test -race ./...
make up        # docker compose up -d --build
make psql      # psql shell into the postgres service
```

## Configuration

Every variable is optional; the default shown is used when it is unset or empty.
In `APP_ENV=production` the server refuses to start unless `JWT_SECRET` is a
random value of at least 32 characters and, when seeding, `ADMIN_PASSWORD` has
been changed from its default.

| Variable                       | Default                                                                   | Description                                                          |
| ------------------------------ | ------------------------------------------------------------------------- | -------------------------------------------------------------------- |
| `APP_ENV`                      | `development`                                                             | Environment name; `production` enables hardening checks.             |
| `PORT`                         | `8080`                                                                    | HTTP listen port.                                                    |
| `LOG_LEVEL`                    | `info`                                                                    | Log verbosity: `debug`, `info`, `warn`, `error`.                     |
| `APP_TIMEZONE`                 | `Asia/Jakarta`                                                            | IANA timezone for booking/availability date math.                    |
| `DATABASE_URL`                 | `postgres://postgres:postgres@localhost:5432/gorraharja?sslmode=disable`  | PostgreSQL connection string.                                        |
| `DB_MAX_CONNS`                 | `10`                                                                      | Maximum pgx pool size.                                               |
| `DB_MIN_CONNS`                 | `1`                                                                       | Minimum idle connections held in the pool.                           |
| `JWT_SECRET`                   | `dev-insecure-jwt-secret-change-me-please-32`                             | JWT HMAC key; must be random and ≥ 32 chars in production.           |
| `JWT_ISSUER`                   | `gorraharja-api`                                                          | `iss` claim written into issued tokens.                              |
| `JWT_ACCESS_TTL`               | `15m`                                                                     | Access-token lifetime (Go duration).                                 |
| `JWT_REFRESH_TTL`              | `168h`                                                                    | Refresh-token lifetime (Go duration).                                |
| `BOOKING_PAYMENT_DEADLINE`     | `30m`                                                                     | Hold time before a PENDING booking is expired.                       |
| `BOOKING_CANCELLATION_CUTOFF`  | `0s`                                                                      | Minimum notice required to cancel; `0s` disables the cutoff.         |
| `BOOKING_MAX_ADVANCE_DAYS`     | `30`                                                                      | How many days ahead customers may book.                              |
| `EXPIRY_SWEEP_INTERVAL`        | `1m`                                                                      | Interval of the background expired-booking sweep.                    |
| `MIGRATE_ON_START`             | `true`                                                                    | Apply embedded migrations at startup.                                |
| `SEED_ON_START`                | `true`                                                                    | Seed reference data and the initial admin at startup.                |
| `ADMIN_EMAIL`                  | `admin@gorraharja.local`                                                  | Email of the seeded administrator.                                   |
| `ADMIN_PASSWORD`               | `Admin#12345`                                                             | Password of the seeded administrator; change in production.          |
| `ADMIN_NAME`                   | `GOR Administrator`                                                       | Display name of the seeded administrator.                            |
| `CORS_ORIGINS`                 | `http://localhost:5173`                                                   | Comma-separated allowed CORS origins.                                |
| `SHUTDOWN_TIMEOUT`             | `15s`                                                                     | Graceful-shutdown drain timeout.                                     |

## Project layout

```text
backend-go/
├── cmd/api/                 # main entrypoint: wiring, graceful shutdown
└── internal/
    ├── apperr/              # typed application errors → HTTP status mapping
    ├── audit/               # audit-log recording helpers
    ├── auth/                # JWT issuing/parsing, password hashing
    ├── config/              # environment loading and validation
    ├── database/            # pgx pool, embedded migrations, seed
    │   └── migrations/      # 0001_init.sql, 0002_seed.sql
    ├── health/              # /health handler (DB ping)
    ├── httpx/               # HTTP helpers: JSON responses, UUID parsing
    ├── logger/              # structured logger construction
    ├── middleware/          # request logging, recovery, auth, RBAC
    ├── model/               # domain entities, DTOs, JSON types
    ├── pricing/             # pricing-rule evaluation
    ├── repo/                # PostgreSQL queries per aggregate
    └── server/              # router assembly and route registration
```

## API overview

Base URL: `/api/v1`. All endpoints return JSON; authenticated routes expect
`Authorization: Bearer <access token>`.

### Auth

| Method  | Path                   | Description                                   |
| ------- | ---------------------- | --------------------------------------------- |
| `POST`  | `/auth/register`       | Create a customer account.                    |
| `POST`  | `/auth/login`          | Exchange credentials for an access/refresh pair. |
| `POST`  | `/auth/refresh`        | Rotate a refresh token.                       |
| `POST`  | `/auth/logout`         | Revoke the current refresh token.             |
| `GET`   | `/auth/me`             | Current user profile.                         |
| `PATCH` | `/auth/me`             | Update the current profile.                   |
| `POST`  | `/auth/me/password`    | Change the current password.                  |

### Catalog

| Method | Path                 | Description              |
| ------ | -------------------- | ------------------------ |
| `GET`  | `/sports`            | List sports.             |
| `GET`  | `/sports/:id`        | Sport detail.            |
| `GET`  | `/facilities`        | List facilities.         |
| `GET`  | `/facilities/:id`    | Facility detail.         |

### Availability

| Method | Path            | Description                                                     |
| ------ | --------------- | --------------------------------------------------------------- |
| `GET`  | `/availability` | Free slots for a facility on a date (`?facility_id=&date=YYYY-MM-DD`). |

### Bookings

| Method  | Path                     | Description                                |
| ------- | ------------------------ | ------------------------------------------ |
| `POST`  | `/bookings`              | Create a booking (held until payment deadline). |
| `GET`   | `/bookings`              | List the caller's bookings.                |
| `GET`   | `/bookings/:id`          | Booking detail.                            |
| `PATCH` | `/bookings/:id/cancel`   | Cancel a booking.                          |

### Payments

| Method | Path             | Description                    |
| ------ | ---------------- | ------------------------------ |
| `POST` | `/payments`      | Record payment for a booking.  |
| `GET`  | `/payments/:id`  | Payment detail.                |

### Events

| Method   | Path                       | Description                     |
| -------- | -------------------------- | ------------------------------- |
| `GET`    | `/events`                  | List events.                    |
| `GET`    | `/events/:id`              | Event detail.                   |
| `POST`   | `/events/:id/register`     | Register the caller for an event. |
| `DELETE` | `/events/:id/register`     | Cancel the caller's registration. |

### Admin & staff

| Method   | Path                                                | Description                          |
| -------- | --------------------------------------------------- | ------------------------------------ |
| `GET`    | `/admin/dashboard`                                  | Aggregate operational metrics.       |
| `GET`    | `/admin/sports`                                     | List sports.                         |
| `POST`   | `/admin/sports`                                     | Create a sport.                      |
| `PATCH`  | `/admin/sports/:id`                                 | Update a sport.                      |
| `DELETE` | `/admin/sports/:id`                                 | Delete a sport.                      |
| `GET`    | `/admin/facilities`                                 | List facilities.                     |
| `POST`   | `/admin/facilities`                                 | Create a facility.                   |
| `PATCH`  | `/admin/facilities/:id`                             | Update a facility.                   |
| `DELETE` | `/admin/facilities/:id`                             | Delete a facility.                   |
| `GET`    | `/admin/facilities/:id/pricing-rules`               | List pricing rules.                  |
| `POST`   | `/admin/facilities/:id/pricing-rules`               | Create a pricing rule.               |
| `PATCH`  | `/admin/facilities/:id/pricing-rules/:ruleId`       | Update a pricing rule.               |
| `DELETE` | `/admin/facilities/:id/pricing-rules/:ruleId`       | Delete a pricing rule.               |
| `GET`    | `/admin/facilities/:id/closures`                    | List facility closures.              |
| `POST`   | `/admin/facilities/:id/closures`                    | Create a closure.                    |
| `DELETE` | `/admin/facilities/:id/closures/:closureId`         | Delete a closure.                    |
| `GET`    | `/admin/bookings`                                   | List all bookings (filterable).      |
| `PATCH`  | `/admin/bookings/:id/status`                        | Transition a booking's status.       |
| `GET`    | `/admin/payments`                                   | List payments.                       |
| `PATCH`  | `/admin/payments/:id`                               | Verify or reject a payment.          |
| `GET`    | `/admin/events`                                     | List events.                         |
| `POST`   | `/admin/events`                                     | Create an event.                     |
| `PATCH`  | `/admin/events/:id`                                 | Update an event.                     |
| `DELETE` | `/admin/events/:id`                                 | Delete an event.                     |
| `GET`    | `/admin/events/:id/registrations`                   | List an event's registrations.       |
| `GET`    | `/admin/users`                                      | List users.                          |
| `GET`    | `/admin/users/:id`                                  | User detail.                         |
| `PATCH`  | `/admin/users/:id`                                  | Update a user (role/status).         |
| `DELETE` | `/admin/users/:id`                                  | Soft-delete a user.                  |
| `GET`    | `/admin/audit-logs`                                 | List audit-log entries.              |
| `GET`    | `/health`                                           | Liveness/readiness probe.            |

## Double-booking guarantee

Overlapping bookings for the same facility are prevented by the database, not by
application-level checks. The `bookings` table carries a PostgreSQL exclusion
constraint (`bookings_no_double_booking` in
`internal/database/migrations/0001_init.sql`):

```sql
ALTER TABLE bookings
    ADD CONSTRAINT bookings_no_double_booking
    EXCLUDE USING gist (
        facility_id WITH =,
        tsrange(start_at, end_at, '[)') WITH &&
    )
    WHERE (status IN ('PENDING', 'CONFIRMED', 'COMPLETED'));
```

Because the guard lives in the index, two concurrent transactions racing for the
same slot can never both commit: the loser receives a serialization-style
constraint violation, which the API maps to a `409 Conflict`. Cancelled and
expired bookings are excluded from the constraint, so a released slot becomes
bookable again immediately. This requires the `btree_gist` extension, which the
migrations enable automatically.

## Testing

```bash
cd backend-go
go test ./...          # full suite
go test -race ./...    # with the race detector
gofmt -l .             # formatting check (must print nothing)
go vet ./...
```

CI (`.github/workflows/ci.yml`) runs exactly these checks on every push and pull
request against a throwaway `postgres:17` service.
