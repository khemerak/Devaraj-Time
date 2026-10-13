ALTER TABLE attendance_events ADD COLUMN latitude double precision, ADD COLUMN longitude double precision, ADD COLUMN accuracy_m double precision, ADD COLUMN measured_at timestamptz, ADD COLUMN distance_m double precision;
-- Erased evidence is intentionally not recreated by rollback.
DROP TABLE event_locations;
ALTER TABLE attendance_events DROP CONSTRAINT events_company_id_unique;
