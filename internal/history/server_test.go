package history

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/golden-vcr/broadcasts"
	"github.com/golden-vcr/broadcasts/gen/queries"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

var broadcast42EndTime = time.Date(1997, 9, 1, 14, 0, 0, 0, time.UTC)
var screening101EndTime = time.Date(1997, 9, 1, 12, 15, 0, 0, time.UTC)

func Test_handleGetHistory(t *testing.T) {
	tests := []struct {
		name       string
		beforeStr  string
		nStr       string
		q          *mockQueries
		wantStatus int
		wantBody   string
	}{
		{
			"normal usage",
			"",
			"",
			&mockQueries{
				broadcasts: []broadcasts.Broadcast{
					{
						Id:        42,
						StartedAt: time.Date(1997, 9, 1, 12, 0, 0, 0, time.UTC),
						EndedAt:   &broadcast42EndTime,
						Screenings: []broadcasts.Screening{
							{
								Id:        uuid.MustParse("bc5c85f6-fe55-4169-ae06-4b390ac13e80"),
								TapeId:    101,
								StartedAt: time.Date(1997, 9, 1, 12, 15, 0, 0, time.UTC),
								EndedAt:   &screening101EndTime,
							},
						},
					},
					{
						Id:         43,
						StartedAt:  time.Date(1997, 9, 2, 12, 0, 0, 0, time.UTC),
						EndedAt:    nil,
						Screenings: []broadcasts.Screening{},
					},
				},
			},
			http.StatusOK,
			`{"broadcasts":[{"id":43,"startedAt":"1997-09-02T12:00:00Z","endedAt":null,"screenings":[]},{"id":42,"startedAt":"1997-09-01T12:00:00Z","endedAt":"1997-09-01T14:00:00Z","screenings":[{"id":"bc5c85f6-fe55-4169-ae06-4b390ac13e80","tapeId":101,"startedAt":"1997-09-01T12:15:00Z","endedAt":"1997-09-01T12:15:00Z"}]}]}`,
		},
		{
			"restricted to broadcasts before a certain ID",
			"43",
			"",
			&mockQueries{
				broadcasts: []broadcasts.Broadcast{
					{
						Id:        42,
						StartedAt: time.Date(1997, 9, 1, 12, 0, 0, 0, time.UTC),
						EndedAt:   &broadcast42EndTime,
						Screenings: []broadcasts.Screening{
							{
								Id:        uuid.MustParse("bc5c85f6-fe55-4169-ae06-4b390ac13e80"),
								TapeId:    101,
								StartedAt: time.Date(1997, 9, 1, 12, 15, 0, 0, time.UTC),
								EndedAt:   &screening101EndTime,
							},
						},
					},
					{
						Id:         43,
						StartedAt:  time.Date(1997, 9, 2, 12, 0, 0, 0, time.UTC),
						EndedAt:    nil,
						Screenings: []broadcasts.Screening{},
					},
				},
			},
			http.StatusOK,
			`{"broadcasts":[{"id":42,"startedAt":"1997-09-01T12:00:00Z","endedAt":"1997-09-01T14:00:00Z","screenings":[{"id":"bc5c85f6-fe55-4169-ae06-4b390ac13e80","tapeId":101,"startedAt":"1997-09-01T12:15:00Z","endedAt":"1997-09-01T12:15:00Z"}]}]}`,
		},
		{
			"restricted to only 1 result",
			"",
			"1",
			&mockQueries{
				broadcasts: []broadcasts.Broadcast{
					{
						Id:        42,
						StartedAt: time.Date(1997, 9, 1, 12, 0, 0, 0, time.UTC),
						EndedAt:   &broadcast42EndTime,
						Screenings: []broadcasts.Screening{
							{
								Id:        uuid.MustParse("bc5c85f6-fe55-4169-ae06-4b390ac13e80"),
								TapeId:    101,
								StartedAt: time.Date(1997, 9, 1, 12, 15, 0, 0, time.UTC),
								EndedAt:   &screening101EndTime,
							},
						},
					},
					{
						Id:         43,
						StartedAt:  time.Date(1997, 9, 2, 12, 0, 0, 0, time.UTC),
						EndedAt:    nil,
						Screenings: []broadcasts.Screening{},
					},
				},
			},
			http.StatusOK,
			`{"broadcasts":[{"id":43,"startedAt":"1997-09-02T12:00:00Z","endedAt":null,"screenings":[]}]}`,
		},
		{
			"range with no data",
			"10",
			"",
			&mockQueries{},
			http.StatusOK,
			`{"broadcasts":[]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Run(tt.name, func(t *testing.T) {
				s := &Server{
					q: tt.q,
				}
				req := httptest.NewRequest(http.MethodGet, "/history", nil)
				q := req.URL.Query()
				if tt.beforeStr != "" {
					q.Set("before", tt.beforeStr)
				}
				if tt.nStr != "" {
					q.Set("n", tt.nStr)
				}
				req.URL.RawQuery = q.Encode()
				res := httptest.NewRecorder()
				s.handleGetHistory(res, req)

				b, err := io.ReadAll(res.Body)
				assert.NoError(t, err)
				body := strings.TrimSuffix(string(b), "\n")
				assert.Equal(t, tt.wantStatus, res.Code)
				assert.Equal(t, tt.wantBody, body)
			})
		})
	}
}

func Test_handleGetHistoryById(t *testing.T) {
	tests := []struct {
		name           string
		broadcastIdStr string
		q              *mockQueries
		wantStatus     int
		wantBody       string
	}{
		{
			"normal usage",
			"42",
			&mockQueries{
				broadcasts: []broadcasts.Broadcast{
					{
						Id:        42,
						StartedAt: time.Date(1997, 9, 1, 12, 0, 0, 0, time.UTC),
						EndedAt:   &broadcast42EndTime,
						Screenings: []broadcasts.Screening{
							{
								Id:        uuid.MustParse("bc5c85f6-fe55-4169-ae06-4b390ac13e80"),
								TapeId:    101,
								StartedAt: time.Date(1997, 9, 1, 12, 15, 0, 0, time.UTC),
								EndedAt:   &screening101EndTime,
							},
						},
					},
				},
			},
			http.StatusOK,
			`{"id":42,"startedAt":"1997-09-01T12:00:00Z","endedAt":"1997-09-01T14:00:00Z","screenings":[{"id":"bc5c85f6-fe55-4169-ae06-4b390ac13e80","tapeId":101,"startedAt":"1997-09-01T12:15:00Z","endedAt":"1997-09-01T12:15:00Z"}]}`,
		},
		{
			"normal usage: in-progress broadcast",
			"42",
			&mockQueries{
				broadcasts: []broadcasts.Broadcast{
					{
						Id:        42,
						StartedAt: time.Date(1997, 9, 1, 12, 0, 0, 0, time.UTC),
						EndedAt:   nil,
						Screenings: []broadcasts.Screening{
							{
								Id:        uuid.MustParse("bc5c85f6-fe55-4169-ae06-4b390ac13e80"),
								TapeId:    101,
								StartedAt: time.Date(1997, 9, 1, 12, 15, 0, 0, time.UTC),
								EndedAt:   nil,
							},
						},
					},
				},
			},
			http.StatusOK,
			`{"id":42,"startedAt":"1997-09-01T12:00:00Z","endedAt":null,"screenings":[{"id":"bc5c85f6-fe55-4169-ae06-4b390ac13e80","tapeId":101,"startedAt":"1997-09-01T12:15:00Z","endedAt":null}]}`,
		},
		{
			"normal usage: no screenings",
			"42",
			&mockQueries{
				broadcasts: []broadcasts.Broadcast{
					{
						Id:         42,
						StartedAt:  time.Date(1997, 9, 1, 12, 0, 0, 0, time.UTC),
						EndedAt:    nil,
						Screenings: []broadcasts.Screening{},
					},
				},
			},
			http.StatusOK,
			`{"id":42,"startedAt":"1997-09-01T12:00:00Z","endedAt":null,"screenings":[]}`,
		},
		{
			"URL parameter must be a valid broadcast ID",
			"bad-id",
			&mockQueries{},
			http.StatusBadRequest,
			"broadcast ID must be an integer",
		},
		{
			"query for nonexistent broadcast is 404",
			"45",
			&mockQueries{
				broadcasts: []broadcasts.Broadcast{
					{
						Id:        42,
						StartedAt: time.Date(1997, 9, 1, 12, 0, 0, 0, time.UTC),
						EndedAt:   &broadcast42EndTime,
						Screenings: []broadcasts.Screening{
							{
								Id:        uuid.MustParse("bc5c85f6-fe55-4169-ae06-4b390ac13e80"),
								TapeId:    101,
								StartedAt: time.Date(1997, 9, 1, 12, 15, 0, 0, time.UTC),
								EndedAt:   &screening101EndTime,
							},
						},
					},
				},
			},
			http.StatusNotFound,
			"no such broadcast",
		},
		{
			"query for nonexistent broadcast is 404 - no data",
			"42",
			&mockQueries{},
			http.StatusNotFound,
			"no such broadcast",
		},
		{
			"any other error is a 500",
			"42",
			&mockQueries{
				err: fmt.Errorf("oh no"),
			},
			http.StatusInternalServerError,
			"oh no",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Run(tt.name, func(t *testing.T) {
				s := &Server{
					q: tt.q,
				}
				req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/history/%s", tt.broadcastIdStr), nil)
				req = mux.SetURLVars(req, map[string]string{"id": tt.broadcastIdStr})
				res := httptest.NewRecorder()
				s.handleGetHistoryById(res, req)

				b, err := io.ReadAll(res.Body)
				assert.NoError(t, err)
				body := strings.TrimSuffix(string(b), "\n")
				assert.Equal(t, tt.wantStatus, res.Code)
				assert.Equal(t, tt.wantBody, body)
			})
		})
	}
}

type mockQueries struct {
	err        error
	broadcasts []broadcasts.Broadcast
}

func (m *mockQueries) GetBroadcastDataEx(ctx context.Context, arg queries.GetBroadcastDataParams) ([]broadcasts.Broadcast, error) {
	if m.err != nil {
		return nil, m.err
	}
	beforeBroadcastID := 2147483647
	if arg.BeforeBroadcastID.Valid {
		beforeBroadcastID = int(arg.BeforeBroadcastID.Int32)
	}
	limit := 10
	if arg.Limit.Valid {
		limit = int(arg.Limit.Int32)
	}

	// Sort broadcasts by ID, ascending
	sort.Slice(m.broadcasts, func(i, j int) bool { return m.broadcasts[i].Id < m.broadcasts[j].Id })

	// Iterate backwards so we encounter newer broadcasts (higher IDs) first
	results := make([]broadcasts.Broadcast, 0, limit)
	for i := len(m.broadcasts) - 1; i >= 0; i-- {
		if m.broadcasts[i].Id >= beforeBroadcastID {
			continue
		}
		results = append(results, m.broadcasts[i])
		if len(m.broadcasts) >= limit {
			break
		}
	}
	return results, nil
}
