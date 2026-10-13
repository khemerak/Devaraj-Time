CREATE TABLE platform_settings (
 singleton boolean PRIMARY KEY DEFAULT true CHECK(singleton),
 invitation_hours integer NOT NULL DEFAULT 24 CHECK(invitation_hours BETWEEN 1 AND 72),
 credential_days integer NOT NULL DEFAULT 365 CHECK(credential_days BETWEEN 7 AND 365),
 support_email text NOT NULL DEFAULT ''
);
INSERT INTO platform_settings(singleton) VALUES(true);
