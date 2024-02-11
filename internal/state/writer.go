package state

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/golden-vcr/broadcasts"
	"github.com/golden-vcr/broadcasts/gen/queries"
	ebroadcast "github.com/golden-vcr/schemas/broadcast-events"
	"github.com/golden-vcr/server-common/rmq"
	"github.com/google/uuid"
)

var ErrBroadcastInProgress = errors.New("a broadcast is already in progress")
var ErrNoBroadcastInProgress = errors.New("no broadcast is currently in progress")
var ErrScreeningInProgress = errors.New("the desired tape is already being screened")
var ErrNoScreeningInProgress = errors.New("no tape is currently being screened")

type Writer interface {
	StartBroadcast(ctx context.Context) (*broadcasts.Broadcast, error)
	EndCurrentBroadcast(ctx context.Context) error
	StartScreening(ctx context.Context, tapeId int) (*broadcasts.Screening, error)
	EndCurrentScreening(ctx context.Context) error
}

func NewWriter(q *queries.Queries, producer rmq.Producer) Writer {
	return &writer{
		q:        q,
		producer: producer,
	}
}

type writer struct {
	q        *queries.Queries
	producer rmq.Producer
}

func (w *writer) StartBroadcast(ctx context.Context) (*broadcasts.Broadcast, error) {
	// Query the data for the most recent broadcast, if any
	rows, err := w.q.GetBroadcastDataEx(ctx, queries.GetBroadcastDataParams{
		Limit: sql.NullInt32{Valid: true, Int32: 1},
	})
	if err != nil {
		return nil, err
	}

	// If we have an existing broadcast that's still in progress, do nothing
	if len(rows) > 0 && rows[0].EndedAt == nil {
		return nil, ErrBroadcastInProgress
	}

	// Record the start of a new broadcast in the database, getting back its ID and
	// started-at timestamp
	row, err := w.q.StartBroadcast(ctx)
	if err != nil {
		return nil, err
	}

	// Produce an event to the broadcast-events queue, indicating to all downstream
	// services that we've started a new broadcast (in which we are not yet screening
	// any tapes)
	if err := w.produce(ctx, &ebroadcast.Event{
		Type: ebroadcast.EventTypeBroadcastStarted,
		Broadcast: ebroadcast.BroadcastData{
			Id:        int(row.ID),
			StartedAt: row.StartedAt,
		},
	}); err != nil {
		return nil, err
	}

	// Success; return the details of the new broadcast
	return &broadcasts.Broadcast{
		Id:         int(row.ID),
		StartedAt:  row.StartedAt,
		EndedAt:    nil,
		Screenings: []broadcasts.Screening{},
	}, nil
}

func (w *writer) EndCurrentBroadcast(ctx context.Context) error {
	// Query the data for the most recent broadcast, if any
	rows, err := w.q.GetBroadcastDataEx(ctx, queries.GetBroadcastDataParams{
		Limit: sql.NullInt32{Valid: true, Int32: 1},
	})
	if err != nil {
		return err
	}

	// If there's no in-progress broadcast to end, abort with an error
	if len(rows) == 0 || rows[0].EndedAt != nil {
		return ErrNoBroadcastInProgress
	}

	// Update the database to reflect the fact that the current broadcast has now ended
	if err := w.endBroadcast(ctx, rows[0].Id); err != nil {
		return err
	}

	// Produce an event to the broadcast-events queue, indicating to all downstream
	// services that we are no longer broadcasting or screening anything
	return w.produce(ctx, &ebroadcast.Event{
		Type: ebroadcast.EventTypeBroadcastFinished,
	})
}

func (w *writer) StartScreening(ctx context.Context, tapeId int) (*broadcasts.Screening, error) {
	// Query the data for the most recent broadcast, if any, with its list of screenings
	rows, err := w.q.GetBroadcastDataEx(ctx, queries.GetBroadcastDataParams{
		Limit: sql.NullInt32{Valid: true, Int32: 1},
	})
	if err != nil {
		return nil, err
	}

	// Require that we have a broadcast in progress in order to screen tapes
	if len(rows) == 0 || rows[0].EndedAt != nil {
		return nil, ErrNoBroadcastInProgress
	}

	// If our current broadcast has any existing screenings, check their state
	if len(rows[0].Screenings) > 0 {
		// If the most recent screening is still in progress, we need to implicitly end
		// it before starting a new one - unless this is a subsequent request to screen
		// the tape that's already being screened, in which case we want to refuse
		lastScreening := rows[0].Screenings[len(rows[0].Screenings)-1]
		if lastScreening.EndedAt == nil {
			// If ending the existing screening to create the new one would result in
			// two back-to-back screenings of the same tape, don't comply with the
			// request
			if lastScreening.TapeId == tapeId {
				return nil, ErrScreeningInProgress
			}

			// Otherwise, we need to end the current screening before we can start the
			// next one
			if err := w.endScreening(ctx, lastScreening.Id); err != nil {
				return nil, err
			}
		}
	}

	// State is clean; record a new screening in the database under the current
	// broadcast, with the requested tape ID, and get back the screening ID and
	// started-at timestamp
	screeningRow, err := w.q.StartScreening(ctx, queries.StartScreeningParams{
		BroadcastID: int32(rows[0].Id),
		TapeID:      int32(tapeId),
	})
	if err != nil {
		return nil, err
	}

	// Produce an event to the broadcast-events queue, indicating to all downstream
	// services that we're now screening a new tape
	if err := w.produce(ctx, &ebroadcast.Event{
		Type: ebroadcast.EventTypeScreeningStarted,
		Broadcast: ebroadcast.BroadcastData{
			Id:        rows[0].Id,
			StartedAt: rows[0].StartedAt,
		},
		Screening: &ebroadcast.ScreeningData{
			Id:        screeningRow.ID,
			StartedAt: screeningRow.StartedAt,
			TapeId:    tapeId,
		},
	}); err != nil {
		return nil, err
	}

	// Success; return the details of the new screening
	return &broadcasts.Screening{
		Id:        screeningRow.ID,
		TapeId:    tapeId,
		StartedAt: screeningRow.StartedAt,
		EndedAt:   nil,
	}, nil
}

func (w *writer) EndCurrentScreening(ctx context.Context) error {
	// Query the data for the most recent broadcast, if any, with its list of screenings
	rows, err := w.q.GetBroadcastDataEx(ctx, queries.GetBroadcastDataParams{
		Limit: sql.NullInt32{Valid: true, Int32: 1},
	})
	if err != nil {
		return err
	}

	// Require that we have a broadcast in progress in order to modify screening state
	if len(rows) == 0 || rows[0].EndedAt != nil {
		return ErrNoBroadcastInProgress
	}

	// Require that we have a screening in progress in order to end it
	var lastScreening *broadcasts.Screening
	if len(rows[0].Screenings) == 0 {
		lastScreening = &rows[0].Screenings[len(rows[0].Screenings)-1]
	}
	if lastScreening == nil || lastScreening.EndedAt != nil {
		return ErrNoScreeningInProgress
	}

	// Update the database to reflect the fact that this screening has ended
	if err := w.endScreening(ctx, lastScreening.Id); err != nil {
		return err
	}

	// Produce an event to the broadcast-events queue, indicating to all downstream
	// services that we're no longer screening a tape (but the broadcast remains live)
	return w.produce(ctx, &ebroadcast.Event{
		Type: ebroadcast.EventTypeScreeningFinished,
		Broadcast: ebroadcast.BroadcastData{
			Id:        rows[0].Id,
			StartedAt: rows[0].StartedAt,
		},
	})
}

func (w *writer) endBroadcast(ctx context.Context, id int) error {
	result, err := w.q.EndBroadcast(ctx, int32(id))
	if err != nil {
		return err
	}
	numRowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if numRowsAffected != int64(1) {
		return fmt.Errorf("failed to end broadcast: expected to affect 1 rows; instead affected %d", numRowsAffected)
	}
	return nil
}

func (w *writer) endScreening(ctx context.Context, id uuid.UUID) error {
	result, err := w.q.EndScreening(ctx, id)
	if err != nil {
		return err
	}
	numRowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if numRowsAffected != int64(1) {
		return fmt.Errorf("failed to end screening: expected to affect 1 rows; instead affected %d", numRowsAffected)
	}
	return nil
}

func (w *writer) produce(ctx context.Context, ev *ebroadcast.Event) error {
	data, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	return w.producer.Send(ctx, data)
}
