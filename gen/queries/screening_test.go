package queries_test

import (
	"context"
	"testing"

	"github.com/golden-vcr/broadcasts/gen/queries"
	"github.com/golden-vcr/server-common/querytest"
	"github.com/stretchr/testify/assert"
)

func Test_GetScreeningHistory(t *testing.T) {
	tx := querytest.PrepareTx(t)
	q := queries.New(tx)

	// We should have no screening history initially
	rows, err := q.GetScreeningHistory(context.Background())
	assert.NoError(t, err)
	assert.Len(t, rows, 0)

	// Simulate three broadcasts
	_, err = tx.Exec(`
		INSERT INTO broadcasts.broadcast (id, started_at, ended_at) VALUES
			(1, now() - '12h'::interval, now() - '10h'::interval),
			(2, now() - '6h'::interval, now() - '4h'::interval),
			(3, now() - '2h'::interval, NULL);
	`)
	assert.NoError(t, err)

	// Simulate screenings within those broadcasts: tapes 40 and 50 in broadcast 1,
	// then tape 60 in broadcast 2, then 70 and a repeat of 40 in broadcast 3 (which is
	// ongoing)
	_, err = tx.Exec(`
		INSERT INTO broadcasts.screening (id, broadcast_id, tape_id, started_at, ended_at) VALUES
			('ddc567e6-5660-4e35-a63c-4119d7706523', 1, 40, now() - '11h30m'::interval, now() - '11h'::interval),
			('cc086e2d-dda5-473e-b039-ab61a4af38b5', 1, 50, now() - '11h'::interval, now() - '10h30m'::interval),
			('fdaa0419-6722-46d8-bcc8-cce6961381b2', 2, 60, now() - '6h'::interval, now() - '5h'::interval),
			('d5446d35-2b1b-4a62-bffe-16434c1f17b7', 3, 40, now() - '2h'::interval, now() - '1h'::interval),
			('9b776985-aa46-44ed-a42d-dfa87c5903bb', 3, 70, now() - '30m'::interval, NULL);
	`)
	assert.NoError(t, err)

	// Our screening history should now reflect our state, with entries for the 4 unique
	// tapes that we've screened
	rows, err = q.GetScreeningHistory(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, []queries.GetScreeningHistoryRow{
		{
			TapeID:       40,
			BroadcastIds: []int32{1, 3},
		},
		{
			TapeID:       50,
			BroadcastIds: []int32{1},
		},
		{
			TapeID:       60,
			BroadcastIds: []int32{2},
		},
		{
			TapeID:       70,
			BroadcastIds: []int32{3},
		},
	}, rows)
}

func Test_StartScreening(t *testing.T) {
	tx := querytest.PrepareTx(t)
	q := queries.New(tx)

	querytest.AssertCount(t, tx, 0, "SELECT COUNT(*) FROM broadcasts.broadcast")
	querytest.AssertCount(t, tx, 0, "SELECT COUNT(*) FROM broadcasts.screening")

	broadcastRow, err := q.StartBroadcast(context.Background())
	assert.NoError(t, err)

	screeningRow, err := q.StartScreening(context.Background(), queries.StartScreeningParams{
		BroadcastID: broadcastRow.ID,
		TapeID:      42,
	})
	assert.NoError(t, err)

	querytest.AssertCount(t, tx, 1, `
		SELECT COUNT(*) FROM broadcasts.screening
			WHERE id = $1
			AND broadcast_id = $2
			AND tape_id = 42
			AND ended_at IS NULL
	`, screeningRow.ID, broadcastRow.ID)
}

func Test_EndScreening(t *testing.T) {
	tx := querytest.PrepareTx(t)
	q := queries.New(tx)

	querytest.AssertCount(t, tx, 0, "SELECT COUNT(*) FROM broadcasts.screening")

	broadcastRow, err := q.StartBroadcast(context.Background())
	assert.NoError(t, err)

	screeningRow, err := q.StartScreening(context.Background(), queries.StartScreeningParams{
		BroadcastID: broadcastRow.ID,
		TapeID:      101,
	})
	assert.NoError(t, err)

	result, err := q.EndScreening(context.Background(), screeningRow.ID)
	assert.NoError(t, err)
	querytest.AssertNumRowsChanged(t, result, 1)

	querytest.AssertCount(t, tx, 1, `
		SELECT COUNT(*) FROM broadcasts.screening
			WHERE id = $1
			AND broadcast_id = $2
			AND tape_id = 101
			AND ended_at IS NOT NULL
	`, screeningRow.ID, broadcastRow.ID)
}
