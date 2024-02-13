package broadcasts

import (
	"time"

	"github.com/google/uuid"
)

type History struct {
	Broadcasts []Broadcast `json:"broadcasts"`
}

type ScreeningHistory struct {
	BroadcastIdsByTapeId map[string][]int `json:"broadcastIdsByTapeId"`
}

type Broadcast struct {
	Id         int         `json:"id"`
	StartedAt  time.Time   `json:"startedAt"`
	EndedAt    *time.Time  `json:"endedAt"`
	Screenings []Screening `json:"screenings"`
}

type Screening struct {
	Id        uuid.UUID  `json:"id"`
	TapeId    int        `json:"tapeId"`
	StartedAt time.Time  `json:"startedAt"`
	EndedAt   *time.Time `json:"endedAt"`
}
