package admin

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golden-vcr/broadcasts"
	"github.com/golden-vcr/broadcasts/internal/state"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

func Test_Server_handleSetTape(t *testing.T) {
	tests := []struct {
		name       string
		tapeIdStr  string
		w          *mockWriter
		wantStatus int
		wantBody   string
	}{
		{
			"normal usage",
			"42",
			&mockWriter{},
			http.StatusNoContent,
			"",
		},
		{
			"URL parameter must be a valid tape ID",
			"bad-id",
			&mockWriter{},
			http.StatusBadRequest,
			"tape ID must be an integer",
		},
		{
			"changing tape without an active broadcast is a 400",
			"42",
			&mockWriter{
				err: state.ErrNoBroadcastInProgress,
			},
			http.StatusBadRequest,
			"no broadcast is currently in progress",
		},
		{
			"screening a tape that's already being screened is a 400",
			"42",
			&mockWriter{
				err: state.ErrScreeningInProgress,
			},
			http.StatusBadRequest,
			"the desired tape is already being screened",
		},
		{
			"any other error is a 500",
			"42",
			&mockWriter{
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
					w: tt.w,
				}
				req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/tape/%s", tt.tapeIdStr), nil)
				req = mux.SetURLVars(req, map[string]string{"id": tt.tapeIdStr})
				res := httptest.NewRecorder()
				s.handleSetTape(res, req)

				b, err := io.ReadAll(res.Body)
				assert.NoError(t, err)
				body := strings.TrimSuffix(string(b), "\n")
				assert.Equal(t, tt.wantStatus, res.Code)
				assert.Equal(t, tt.wantBody, body)
			})
		})
	}
}

func Test_Server_handleClearTape(t *testing.T) {
	tests := []struct {
		name       string
		w          *mockWriter
		wantStatus int
		wantBody   string
	}{
		{
			"normal usage",
			&mockWriter{},
			http.StatusNoContent,
			"",
		},
		{
			"clearing tape is still successful if nothing was being screened",
			&mockWriter{
				err: state.ErrNoScreeningInProgress,
			},
			http.StatusNoContent,
			"",
		},
		{
			"clearing tape without an active broadcast is a 400",
			&mockWriter{
				err: state.ErrNoBroadcastInProgress,
			},
			http.StatusBadRequest,
			"no broadcast is currently in progress",
		},
		{
			"any other error is a 500",
			&mockWriter{
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
					w: tt.w,
				}
				req := httptest.NewRequest(http.MethodDelete, "/admin/tape", nil)
				res := httptest.NewRecorder()
				s.handleClearTape(res, req)

				b, err := io.ReadAll(res.Body)
				assert.NoError(t, err)
				body := strings.TrimSuffix(string(b), "\n")
				assert.Equal(t, tt.wantStatus, res.Code)
				assert.Equal(t, tt.wantBody, body)
			})
		})
	}
}

type mockWriter struct {
	err error
}

func (m *mockWriter) StartBroadcast(ctx context.Context) (*broadcasts.Broadcast, error) {
	return nil, fmt.Errorf("not mocked")
}

func (m *mockWriter) EndCurrentBroadcast(ctx context.Context) error {
	return fmt.Errorf("not mocked")
}

func (m *mockWriter) StartScreening(ctx context.Context, tapeId int) (*broadcasts.Screening, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &broadcasts.Screening{
		Id:        uuid.MustParse("df0d9c53-8a7a-4788-9d20-dc718cf4a7b3"),
		TapeId:    tapeId,
		StartedAt: time.Date(1997, 9, 1, 12, 0, 0, 0, time.UTC),
		EndedAt:   nil,
	}, nil
}

func (m *mockWriter) EndCurrentScreening(ctx context.Context) error {
	return m.err
}
