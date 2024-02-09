-- name: GetBroadcastData :many
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
where broadcast.id < coalesce(sqlc.narg('before_broadcast_id'), 2147483647)
group by broadcast.id
order by broadcast.id desc
limit coalesce(sqlc.narg('limit')::integer, 10);