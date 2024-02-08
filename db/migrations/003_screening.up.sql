begin;

create table broadcasts.screening (
    id           uuid primary key,
    broadcast_id integer not null,
    tape_id      integer not null,
    started_at   timestamptz not null default now(),
    ended_at     timestamptz
);

alter table broadcasts.screening
    add constraint screening_broadcast_id_fk
    foreign key (broadcast_id) references broadcasts.broadcast (id);

comment on table broadcasts.screening is
    'Records the fact that a particular tape was played during a broadcast.';
comment on column broadcasts.screening.id is
    'Unique ID for this screening; used chiefly to associate other data with this '
    'screening.';
comment on column broadcasts.screening.broadcast_id is
    'ID of the broadcast that was live at the time the screening started.';
comment on column broadcasts.screening.tape_id is
    'ID of the tape that was screened.';
comment on column broadcasts.screening.started_at is
    'Time at which the screening started.';
comment on column broadcasts.screening.ended_at is
    'Time at which the screening ended, if it''s not stil ongoing.';

create index screening_tape_id_index on broadcasts.screening (tape_id);

commit;
