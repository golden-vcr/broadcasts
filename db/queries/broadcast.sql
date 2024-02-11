-- name: GetBroadcastData :many
select
    broadcast.id,
    broadcast.started_at,
    broadcast.ended_at,
    coalesce(
        json_agg(json_build_object(
            'tape_id', screening.tape_id,
            'started_at', screening.started_at,
            'ended_at', coalesce(screening.ended_at, broadcast.ended_at)
        )) filter (where screening.id is not null),
        '[]'::json
     )::json as screenings
from broadcasts.broadcast
left join broadcasts.screening
    on screening.broadcast_id = broadcast.id
where broadcast.id < coalesce(sqlc.narg('before_broadcast_id'), 2147483647)
group by broadcast.id
order by broadcast.id desc
limit coalesce(sqlc.narg('limit')::integer, 10);

-- name: StartBroadcast :one
insert into broadcasts.broadcast (started_at)
values (now())
returning broadcast.id, broadcast.started_at;

-- name: ResumeBroadcast :execresult
update broadcasts.broadcast set ended_at = null
where broadcast.id = sqlc.arg('broadcast_id')
    and broadcast.ended_at is not null;

-- name: EndBroadcast :execresult
update broadcasts.broadcast set ended_at = now()
where broadcast.id = sqlc.arg('broadcast_id')
    and broadcast.ended_at is null;
