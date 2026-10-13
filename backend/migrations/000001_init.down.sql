DROP TABLE IF EXISTS notifications,audit_logs,requests,attendance_events,attendance_sessions,attendance_points,workplaces,user_sessions,memberships,shifts,branches,departments,users,companies CASCADE;
DROP FUNCTION IF EXISTS protect_immutable();
DROP FUNCTION IF EXISTS protect_request_decision();
