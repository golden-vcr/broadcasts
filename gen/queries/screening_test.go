package queries_test

import (
	"context"
	"testing"

	"github.com/golden-vcr/broadcasts/gen/queries"
	"github.com/golden-vcr/server-common/querytest"
	"github.com/stretchr/testify/assert"
)

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
