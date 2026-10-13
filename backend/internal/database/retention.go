package database

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PurgeLocation deletes only expired private location evidence, never authoritative clock events.
func PurgeLocation(ctx context.Context, db *pgxpool.Pool) error {
	tx, e := db.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	rows, e := tx.Query(ctx, `WITH erased AS (DELETE FROM event_locations l USING companies c WHERE l.company_id=c.id AND l.recorded_at<now()-make_interval(days=>c.location_retention_days) RETURNING l.company_id) SELECT company_id::text,count(*) FROM erased GROUP BY company_id`)
	if e != nil {
		return e
	}
	type item struct {
		company string
		count   int
	}
	items := []item{}
	for rows.Next() {
		var v item
		if e = rows.Scan(&v.company, &v.count); e != nil {
			rows.Close()
			return e
		}
		items = append(items, v)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, v := range items {
		_, e = tx.Exec(ctx, `INSERT INTO audit_logs(company_id,action,target,reason,data,request_id) VALUES($1,'privacy.location_purged',$2,'Configured location retention deadline',jsonb_build_object('erased_count',$3::int),'retention-job')`, v.company, v.company, v.count)
		if e != nil {
			return e
		}
	}
	return tx.Commit(ctx)
}
