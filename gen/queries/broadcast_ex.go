package queries

import (
	"context"
	"encoding/json"
	"time"

	"github.com/golden-vcr/broadcasts"
	"github.com/google/uuid"
)

type screeningData struct {
	ID        uuid.UUID  `json:"id"`
	TapeID    int32      `json:"tape_id"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at"`
}

func (s *screeningData) toScreening() broadcasts.Screening {
	return broadcasts.Screening{
		Id:        s.ID,
		TapeId:    int(s.TapeID),
		StartedAt: s.StartedAt,
		EndedAt:   s.EndedAt,
	}
}

func (q *Queries) GetBroadcastDataEx(ctx context.Context, arg GetBroadcastDataParams) ([]broadcasts.Broadcast, error) {
	baseRows, err := q.GetBroadcastData(ctx, arg)
	if err != nil {
		return nil, err
	}
	rows := make([]broadcasts.Broadcast, 0, len(baseRows))
	for _, baseRow := range baseRows {
		var rawScreenings []screeningData
		if err := json.Unmarshal(baseRow.Screenings, &rawScreenings); err != nil {
			return nil, err
		}
		screenings := make([]broadcasts.Screening, 0, len(rawScreenings))
		for _, rawScreening := range rawScreenings {
			screenings = append(screenings, rawScreening.toScreening())
		}

		var broadcastEndedAt *time.Time
		if baseRow.EndedAt.Valid {
			broadcastEndedAt = &baseRow.EndedAt.Time
		}

		rows = append(rows, broadcasts.Broadcast{
			Id:         int(baseRow.ID),
			StartedAt:  baseRow.StartedAt,
			EndedAt:    broadcastEndedAt,
			Screenings: screenings,
		})
	}
	return rows, nil
}
