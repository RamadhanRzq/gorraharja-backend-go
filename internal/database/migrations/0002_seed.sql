-- Initial catalogue data. Sports and facilities stay configurable through the
-- admin API; these rows only bootstrap a fresh installation.

INSERT INTO sports (id, name, slug, description, is_active) VALUES
    ('11111111-1111-4111-8111-111111111111', 'Futsal', 'futsal',
     'Lapangan futsal indoor dengan rumput sintetis', TRUE),
    ('22222222-2222-4222-8222-222222222222', 'Volleyball', 'volleyball',
     'Lapangan bola voli indoor', TRUE)
ON CONFLICT (id) DO NOTHING;

INSERT INTO facilities (id, sport_id, name, description, location, status) VALUES
    ('a1111111-1111-4111-8111-111111111111', '11111111-1111-4111-8111-111111111111',
     'Futsal A', 'Lapangan futsal utama', 'GOR Lantai 1', 'ACTIVE'),
    ('a1111111-1111-4111-8111-111111111112', '11111111-1111-4111-8111-111111111111',
     'Futsal B', 'Lapangan futsal kedua', 'GOR Lantai 1', 'ACTIVE'),
    ('b2222222-2222-4222-8222-222222222221', '22222222-2222-4222-8222-222222222222',
     'Volleyball Indoor A', 'Lapangan voli indoor utama', 'GOR Lantai 2', 'ACTIVE'),
    ('b2222222-2222-4222-8222-222222222222', '22222222-2222-4222-8222-222222222222',
     'Volleyball Indoor B', 'Lapangan voli indoor kedua', 'GOR Lantai 2', 'ACTIVE')
ON CONFLICT (id) DO NOTHING;

-- Weekday 08:00-16:00 Rp100.000, 16:00-23:00 Rp150.000.
-- Weekend 08:00-16:00 Rp125.000, 16:00-23:00 Rp175.000.
INSERT INTO pricing_rules (facility_id, day_type, start_time, end_time, price_per_hour)
SELECT f.id, r.day_type, r.start_time, r.end_time, r.price_per_hour
FROM facilities f
CROSS JOIN (VALUES
    ('WEEKDAY', TIME '08:00', TIME '16:00', 100000::BIGINT),
    ('WEEKDAY', TIME '16:00', TIME '23:00', 150000::BIGINT),
    ('WEEKEND', TIME '08:00', TIME '16:00', 125000::BIGINT),
    ('WEEKEND', TIME '16:00', TIME '23:00', 175000::BIGINT)
) AS r (day_type, start_time, end_time, price_per_hour)
WHERE NOT EXISTS (
    SELECT 1 FROM pricing_rules p WHERE p.facility_id = f.id
);
