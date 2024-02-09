package queries

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type GetBroadcastDataRowEx struct {
	ID         int32
	StartedAt  time.Time
	EndedAt    sql.NullTime
	Screenings []ScreeningData
}

type ScreeningData struct {
	ID        uuid.UUID  `json:"id"`
	TapeID    int32      `json:"tape_id"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at"`
}

func (q *Queries) GetBroadcastDataEx(ctx context.Context, arg GetBroadcastDataParams) ([]GetBroadcastDataRowEx, error) {
	baseRows, err := q.GetBroadcastData(ctx, arg)
	if err != nil {
		return nil, err
	}
	rows := make([]GetBroadcastDataRowEx, 0, len(baseRows))
	for _, baseRow := range baseRows {
		var screenings []ScreeningData
		if err := json.Unmarshal(baseRow.Screenings, &screenings); err != nil {
			return nil, err
		}
		rows = append(rows, GetBroadcastDataRowEx{
			ID:         baseRow.ID,
			StartedAt:  baseRow.StartedAt,
			EndedAt:    baseRow.EndedAt,
			Screenings: screenings,
		})
	}
	return rows, nil
}
