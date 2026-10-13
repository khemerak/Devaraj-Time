-- Location evidence has a separate retention lifecycle; raw clock events remain append-only.
ALTER TABLE attendance_events ADD CONSTRAINT events_company_id_unique UNIQUE(company_id,id);
CREATE TABLE event_locations(
 event_id uuid PRIMARY KEY, company_id uuid NOT NULL REFERENCES companies,
 recorded_at timestamptz NOT NULL, latitude double precision NOT NULL,
 longitude double precision NOT NULL, accuracy_m double precision NOT NULL,
 measured_at timestamptz NOT NULL, distance_m double precision,
 FOREIGN KEY(company_id,event_id) REFERENCES attendance_events(company_id,id)
);
INSERT INTO event_locations(event_id,company_id,recorded_at,latitude,longitude,accuracy_m,measured_at,distance_m)
 SELECT id,company_id,recorded_at,latitude,longitude,accuracy_m,measured_at,distance_m FROM attendance_events WHERE latitude IS NOT NULL AND measured_at IS NOT NULL;
ALTER TABLE attendance_events DROP COLUMN latitude,DROP COLUMN longitude,DROP COLUMN accuracy_m,DROP COLUMN measured_at,DROP COLUMN distance_m;
CREATE INDEX location_expiry ON event_locations(company_id,recorded_at);
