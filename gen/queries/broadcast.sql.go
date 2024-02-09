// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.25.0
// source: broadcast.sql

package queries

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

const getBroadcastData = `-- name: GetBroadcastData :many
select
    broadcast.id,
    broadcast.started_at,
    broadcast.ended_at,
    json_agg(json_build_object(
        'tape_id', screening.tape_id,
        'started_at', screening.started_at,
        'ended_at', coalesce(screening.ended_at, broadcast.ended_at)
    )) as screenings
from broadcasts.broadcast
join broadcasts.screening
    on screening.broadcast_id = broadcast.id
where broadcast.id < coalesce($1, 2147483647)
group by broadcast.id
order by broadcast.id desc
limit coalesce($2::integer, 10)
`

type GetBroadcastDataParams struct {
	BeforeBroadcastID sql.NullInt32
	Limit             sql.NullInt32
}

type GetBroadcastDataRow struct {
	ID         int32
	StartedAt  time.Time
	EndedAt    sql.NullTime
	Screenings json.RawMessage
}

func (q *Queries) GetBroadcastData(ctx context.Context, arg GetBroadcastDataParams) ([]GetBroadcastDataRow, error) {
	rows, err := q.db.QueryContext(ctx, getBroadcastData, arg.BeforeBroadcastID, arg.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []GetBroadcastDataRow
	for rows.Next() {
		var i GetBroadcastDataRow
		if err := rows.Scan(
			&i.ID,
			&i.StartedAt,
			&i.EndedAt,
			&i.Screenings,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}