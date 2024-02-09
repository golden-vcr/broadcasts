// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.25.0

package queries

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// Record of a broadcast that occurred (or is occurring) on the GoldenVCR Twitch channel.
type BroadcastsBroadcast struct {
	// Serial ID used to correlate other records with this broadcast.
	ID int32
	// Time at which this broadcast first started.
	StartedAt time.Time
	// Time at which the broadcast ended, if it's not still live. To account for the possibility of brief disruptions in internet service (or Twitch availability), it's possible to resume a broadcast once it's ended: a non-NULL ended_at timestamp does not definitively indicate that broadcast is done for good.
	EndedAt sql.NullTime
	// Absolute URL to a page where the recording of this broadcast can be viewed, if available.
	VodUrl sql.NullString
}

// Records the fact that a particular tape was played during a broadcast.
type BroadcastsScreening struct {
	// Unique ID for this screening; used chiefly to associate other data with this screening.
	ID uuid.UUID
	// ID of the broadcast that was live at the time the screening started.
	BroadcastID int32
	// ID of the tape that was screened.
	TapeID int32
	// Time at which the screening started.
	StartedAt time.Time
	// Time at which the screening ended, if it's not stil ongoing.
	EndedAt sql.NullTime
}