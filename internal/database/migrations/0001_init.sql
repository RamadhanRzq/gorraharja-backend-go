-- Initial schema for the GOR Sports Booking & Management System.
-- Requirements covered: foreign keys, unique/check constraints, indexing,
-- timestamps, soft delete and a database level double-booking guard.

CREATE EXTENSION IF NOT EXISTS btree_gist;
CREATE EXTENSION IF NOT EXISTS citext;

-- ---------------------------------------------------------------------------
-- Shared helpers
-- ---------------------------------------------------------------------------

CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- ---------------------------------------------------------------------------
-- users
-- ---------------------------------------------------------------------------

CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         CITEXT NOT NULL,
    full_name     TEXT NOT NULL CHECK (length(btrim(full_name)) > 0),
    phone         TEXT,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'CUSTOMER'
                  CHECK (role IN ('CUSTOMER', 'STAFF', 'ADMIN')),
    status        TEXT NOT NULL DEFAULT 'ACTIVE'
                  CHECK (status IN ('ACTIVE', 'SUSPENDED')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ
);

-- Email must stay unique among non-deleted accounts (soft delete friendly).
CREATE UNIQUE INDEX users_email_active_key ON users (email) WHERE deleted_at IS NULL;
CREATE INDEX users_role_idx ON users (role) WHERE deleted_at IS NULL;

CREATE TRIGGER users_set_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- refresh_tokens
-- ---------------------------------------------------------------------------

CREATE TABLE refresh_tokens (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    user_agent TEXT NOT NULL DEFAULT '',
    ip_address TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX refresh_tokens_user_idx ON refresh_tokens (user_id);
CREATE INDEX refresh_tokens_active_idx ON refresh_tokens (expires_at) WHERE revoked_at IS NULL;

-- ---------------------------------------------------------------------------
-- sports
-- ---------------------------------------------------------------------------

CREATE TABLE sports (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL CHECK (length(btrim(name)) > 0),
    slug        TEXT NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
    description TEXT NOT NULL DEFAULT '',
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX sports_active_idx ON sports (is_active);

CREATE TRIGGER sports_set_updated_at
    BEFORE UPDATE ON sports
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- facilities
-- ---------------------------------------------------------------------------

CREATE TABLE facilities (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sport_id    UUID NOT NULL REFERENCES sports (id) ON DELETE RESTRICT,
    name        TEXT NOT NULL CHECK (length(btrim(name)) > 0),
    description TEXT NOT NULL DEFAULT '',
    location    TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'ACTIVE'
                CHECK (status IN ('ACTIVE', 'MAINTENANCE', 'CLOSED')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (sport_id, name)
);

CREATE INDEX facilities_sport_idx ON facilities (sport_id);
CREATE INDEX facilities_status_idx ON facilities (status);

CREATE TRIGGER facilities_set_updated_at
    BEFORE UPDATE ON facilities
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- pricing_rules
-- ---------------------------------------------------------------------------

CREATE TABLE pricing_rules (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    facility_id    UUID NOT NULL REFERENCES facilities (id) ON DELETE CASCADE,
    day_type       TEXT NOT NULL CHECK (day_type IN ('WEEKDAY', 'WEEKEND')),
    start_time     TIME NOT NULL,
    end_time       TIME NOT NULL,
    price_per_hour BIGINT NOT NULL CHECK (price_per_hour >= 0),
    is_active      BOOLEAN NOT NULL DEFAULT TRUE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT pricing_rules_window_valid CHECK (end_time > start_time)
);

CREATE INDEX pricing_rules_facility_idx ON pricing_rules (facility_id, day_type) WHERE is_active;

CREATE TRIGGER pricing_rules_set_updated_at
    BEFORE UPDATE ON pricing_rules
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- facility_closures (maintenance / closed schedule set by admins)
-- ---------------------------------------------------------------------------

CREATE TABLE facility_closures (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    facility_id UUID NOT NULL REFERENCES facilities (id) ON DELETE CASCADE,
    start_date  DATE NOT NULL,
    end_date    DATE NOT NULL,
    start_time  TIME,
    end_time    TIME,
    reason      TEXT NOT NULL DEFAULT '',
    created_by  UUID REFERENCES users (id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT facility_closures_range_valid CHECK (end_date >= start_date),
    CONSTRAINT facility_closures_window_valid CHECK (
        (start_time IS NULL AND end_time IS NULL)
        OR (start_time IS NOT NULL AND end_time IS NOT NULL AND end_time > start_time)
    )
);

CREATE INDEX facility_closures_facility_idx ON facility_closures (facility_id, start_date, end_date);

CREATE TRIGGER facility_closures_set_updated_at
    BEFORE UPDATE ON facility_closures
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- bookings
-- ---------------------------------------------------------------------------

CREATE TABLE bookings (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    booking_code  TEXT NOT NULL UNIQUE,
    customer_id   UUID NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    facility_id   UUID NOT NULL REFERENCES facilities (id) ON DELETE RESTRICT,
    booking_date  DATE NOT NULL,
    start_time    TIME NOT NULL,
    end_time      TIME NOT NULL,
    -- Derived absolute bounds used by the exclusion constraint below.
    start_at      TIMESTAMP GENERATED ALWAYS AS (booking_date + start_time) STORED,
    end_at        TIMESTAMP GENERATED ALWAYS AS (booking_date + end_time) STORED,
    total_price   BIGINT NOT NULL CHECK (total_price >= 0),
    status        TEXT NOT NULL DEFAULT 'PENDING'
                  CHECK (status IN ('PENDING', 'CONFIRMED', 'CANCELLED', 'COMPLETED', 'EXPIRED')),
    notes         TEXT NOT NULL DEFAULT '',
    cancelled_at  TIMESTAMPTZ,
    cancelled_by  UUID REFERENCES users (id) ON DELETE SET NULL,
    cancel_reason TEXT NOT NULL DEFAULT '',
    expires_at    TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT bookings_time_order CHECK (end_time > start_time)
);

-- Double booking prevention at the database level: two active bookings for the
-- same facility can never overlap, even under concurrent transactions.
ALTER TABLE bookings
    ADD CONSTRAINT bookings_no_double_booking
    EXCLUDE USING gist (
        facility_id WITH =,
        tsrange(start_at, end_at, '[)') WITH &&
    )
    WHERE (status IN ('PENDING', 'CONFIRMED', 'COMPLETED'));

CREATE INDEX bookings_customer_idx ON bookings (customer_id, booking_date DESC);
CREATE INDEX bookings_facility_date_idx ON bookings (facility_id, booking_date);
CREATE INDEX bookings_status_idx ON bookings (status);
CREATE INDEX bookings_expiry_idx ON bookings (expires_at) WHERE status = 'PENDING';

CREATE TRIGGER bookings_set_updated_at
    BEFORE UPDATE ON bookings
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- payments
-- ---------------------------------------------------------------------------

CREATE TABLE payments (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    booking_id     UUID NOT NULL REFERENCES bookings (id) ON DELETE CASCADE,
    amount         BIGINT NOT NULL CHECK (amount >= 0),
    method         TEXT NOT NULL DEFAULT 'CASH'
                   CHECK (method IN ('CASH', 'BANK_TRANSFER', 'QRIS', 'EWALLET', 'OTHER')),
    status         TEXT NOT NULL DEFAULT 'UNPAID'
                   CHECK (status IN ('UNPAID', 'PENDING', 'PAID', 'FAILED', 'REFUNDED')),
    reference      TEXT,
    paid_at        TIMESTAMPTZ,
    verified_by    UUID REFERENCES users (id) ON DELETE SET NULL,
    failure_reason TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX payments_reference_key ON payments (reference) WHERE reference IS NOT NULL;
CREATE INDEX payments_booking_idx ON payments (booking_id, created_at DESC);
CREATE INDEX payments_status_idx ON payments (status);

CREATE TRIGGER payments_set_updated_at
    BEFORE UPDATE ON payments
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- events
-- ---------------------------------------------------------------------------

CREATE TABLE events (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title            TEXT NOT NULL CHECK (length(btrim(title)) > 0),
    description      TEXT NOT NULL DEFAULT '',
    event_type       TEXT NOT NULL DEFAULT 'OTHER',
    start_at         TIMESTAMPTZ NOT NULL,
    end_at           TIMESTAMPTZ NOT NULL,
    location         TEXT NOT NULL DEFAULT '',
    capacity         INTEGER NOT NULL CHECK (capacity > 0),
    status           TEXT NOT NULL DEFAULT 'DRAFT'
                     CHECK (status IN ('DRAFT', 'PUBLISHED', 'ONGOING', 'COMPLETED', 'CANCELLED')),
    registration_fee BIGINT NOT NULL DEFAULT 0 CHECK (registration_fee >= 0),
    created_by       UUID REFERENCES users (id) ON DELETE SET NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT events_period_valid CHECK (end_at > start_at)
);

CREATE INDEX events_status_start_idx ON events (status, start_at);
CREATE INDEX events_start_idx ON events (start_at);

CREATE TRIGGER events_set_updated_at
    BEFORE UPDATE ON events
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- event_registrations
-- ---------------------------------------------------------------------------

CREATE TABLE event_registrations (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id    UUID NOT NULL REFERENCES events (id) ON DELETE CASCADE,
    customer_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    status      TEXT NOT NULL DEFAULT 'REGISTERED'
                CHECK (status IN ('REGISTERED', 'CANCELLED')),
    notes       TEXT NOT NULL DEFAULT '',
    cancelled_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- A customer may hold at most one active registration per event.
CREATE UNIQUE INDEX event_registrations_active_key
    ON event_registrations (event_id, customer_id) WHERE status = 'REGISTERED';
CREATE INDEX event_registrations_event_idx ON event_registrations (event_id);
CREATE INDEX event_registrations_customer_idx ON event_registrations (customer_id);

CREATE TRIGGER event_registrations_set_updated_at
    BEFORE UPDATE ON event_registrations
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- audit_logs
-- ---------------------------------------------------------------------------

CREATE TABLE audit_logs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id    UUID REFERENCES users (id) ON DELETE SET NULL,
    actor_type  TEXT NOT NULL DEFAULT 'USER' CHECK (actor_type IN ('USER', 'SYSTEM')),
    action      TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id   UUID,
    metadata    JSONB NOT NULL DEFAULT '{}'::jsonb,
    ip_address  TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX audit_logs_created_idx ON audit_logs (created_at DESC);
CREATE INDEX audit_logs_entity_idx ON audit_logs (entity_type, entity_id);
CREATE INDEX audit_logs_actor_idx ON audit_logs (actor_id, created_at DESC);
