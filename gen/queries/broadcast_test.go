package queries_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/golden-vcr/broadcasts/gen/queries"
	"github.com/golden-vcr/server-common/querytest"
	"github.com/stretchr/testify/assert"
)

func Test_GetBroadcastData(t *testing.T) {
	tx := querytest.PrepareTx(t)
	q := queries.New(tx)

	// Simulate a completed broadcast (1) and an in-progress broadcast (2)
	_, err := tx.Exec(`
		INSERT INTO broadcasts.broadcast (id, started_at, ended_at, vod_url) VALUES
			(1, now() - '12h'::interval, now() - '10h'::interval, 'https://vods.com/1'),
			(2, now() - '2h'::interval, NULL, NULL);
	`)
	assert.NoError(t, err)

	// If we query for data with no screenings present, we should still get results
	rows, err := q.GetBroadcastDataEx(context.Background(), queries.GetBroadcastDataParams{})
	assert.NoError(t, err)
	assert.Len(t, rows, 2)
	assert.Equal(t, 2, rows[0].Id)
	assert.Len(t, rows[0].Screenings, 0)
	assert.Equal(t, 1, rows[1].Id)
	assert.Len(t, rows[1].Screenings, 0)

	// Add a few completed screenings to broadcast 1, and add an in-progress screening
	// for broadcast 2
	_, err = tx.Exec(`
		INSERT INTO broadcasts.screening (id, broadcast_id, tape_id, started_at, ended_at) VALUES
			('6c2c94e3-db0c-4367-8ce7-e86f98ac03d0', 1, 40, now() - '11h30m'::interval, now() - '11h'::interval),
			('638a6e4b-4225-4aba-8893-b1c5cbad4e21', 1, 50, now() - '11h'::interval, now() - '10h30m'::interval),
			('df38802e-cbc0-46a8-b98b-8584e5222335', 1, 60, now() - '6h'::interval, now() - '5h'::interval),
			('23c6d0dd-e376-49f8-b600-1e7c460cc094', 2, 70, now() - '30m'::interval, NULL);
	`)
	assert.NoError(t, err)

	// If we query for recent broadcast data with no params, we should get data for
	// both, in descending order (most recent first)
	rows, err = q.GetBroadcastDataEx(context.Background(), queries.GetBroadcastDataParams{})
	assert.NoError(t, err)
	assert.Len(t, rows, 2)
	assert.Equal(t, 2, rows[0].Id)
	assert.Len(t, rows[0].Screenings, 1)
	assert.Equal(t, 1, rows[1].Id)
	assert.Len(t, rows[1].Screenings, 3)

	// Our most recent broadcast (at index 0) should still be in progress, and its one
	// and only screening should be in progress as well
	assert.Nil(t, rows[0].EndedAt)
	assert.Nil(t, rows[0].Screenings[0].EndedAt)

	// Our prior broadcast should be finished
	assert.NotNil(t, rows[1].EndedAt)
	assert.NotNil(t, rows[1].Screenings[0].EndedAt)
	assert.NotNil(t, rows[1].Screenings[1].EndedAt)
	assert.NotNil(t, rows[1].Screenings[2].EndedAt)

	// Asking for only 1 result should give us data for the most recent broadcast only
	rows, err = q.GetBroadcastDataEx(context.Background(), queries.GetBroadcastDataParams{
		Limit: sql.NullInt32{Valid: true, Int32: 1},
	})
	assert.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Equal(t, 2, rows[0].Id)

	// Asking for broadcasts prior to broadcast 2 should give us broadcast 1 only
	rows, err = q.GetBroadcastDataEx(context.Background(), queries.GetBroadcastDataParams{
		BeforeBroadcastID: sql.NullInt32{Valid: true, Int32: 2},
	})
	assert.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Equal(t, 1, rows[0].Id)
}

func Test_StartBroadcast(t *testing.T) {
	tx := querytest.PrepareTx(t)
	q := queries.New(tx)

	querytest.AssertCount(t, tx, 0, "SELECT COUNT(*) FROM broadcasts.broadcast")

	row, err := q.StartBroadcast(context.Background())
	assert.NoError(t, err)

	querytest.AssertCount(t, tx, 1, `
		SELECT COUNT(*) FROM broadcasts.broadcast
			WHERE id = $1
			AND ended_at IS NULL
	`, row.ID)
}

func Test_ResumeBroadcast(t *testing.T) {
	tx := querytest.PrepareTx(t)
	q := queries.New(tx)

	querytest.AssertCount(t, tx, 0, "SELECT COUNT(*) FROM broadcasts.broadcast")

	row, err := q.StartBroadcast(context.Background())
	assert.NoError(t, err)
	result, err := q.EndBroadcast(context.Background(), row.ID)
	assert.NoError(t, err)
	querytest.AssertNumRowsChanged(t, result, 1)
	result, err = q.ResumeBroadcast(context.Background(), row.ID)
	assert.NoError(t, err)
	querytest.AssertNumRowsChanged(t, result, 1)

	querytest.AssertCount(t, tx, 1, `
		SELECT COUNT(*) FROM broadcasts.broadcast
			WHERE id = $1
			AND ended_at IS NULL
	`, row.ID)
}

func Test_EndBroadcast(t *testing.T) {
	tx := querytest.PrepareTx(t)
	q := queries.New(tx)

	querytest.AssertCount(t, tx, 0, "SELECT COUNT(*) FROM broadcasts.broadcast")

	row, err := q.StartBroadcast(context.Background())
	assert.NoError(t, err)
	result, err := q.EndBroadcast(context.Background(), row.ID)
	assert.NoError(t, err)
	querytest.AssertNumRowsChanged(t, result, 1)

	querytest.AssertCount(t, tx, 1, `
		SELECT COUNT(*) FROM broadcasts.broadcast
			WHERE id = $1
			AND ended_at IS NOT NULL
	`, row.ID)
}
