-- name: GetScreeningHistory :many
select
    screening.tape_id,
    array_agg(
        distinct screening.broadcast_id
        order by screening.broadcast_id
    )::integer[] as broadcast_ids
from broadcasts.screening
group by screening.tape_id
order by screening.tape_id;

-- name: StartScreening :one
insert into broadcasts.screening (
    id,
    broadcast_id,
    tape_id,
    started_at
) values (
    gen_random_uuid(),
    sqlc.arg('broadcast_id'),
    sqlc.arg('tape_id'),
    now()
)
returning screening.id, screening.started_at;

-- name: EndScreening :execresult
update broadcasts.screening set ended_at = now()
where screening.id = sqlc.arg('screening_id')
    and screening.ended_at is null;
