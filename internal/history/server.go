package history

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/golden-vcr/broadcasts"
	"github.com/golden-vcr/broadcasts/gen/queries"
	"github.com/gorilla/mux"
)

type Queries interface {
	GetBroadcastDataEx(ctx context.Context, arg queries.GetBroadcastDataParams) ([]broadcasts.Broadcast, error)
	GetScreeningHistory(ctx context.Context) ([]queries.GetScreeningHistoryRow, error)
}

type Server struct {
	q Queries
}

func NewServer(q *queries.Queries) *Server {
	return &Server{
		q: q,
	}
}

func (s *Server) RegisterRoutes(r *mux.Router) {
	r.Path("/history").Methods("GET").HandlerFunc(s.handleGetHistory)
	r.Path("/history/{id}").Methods("GET").HandlerFunc(s.handleGetHistoryById)
	r.Path("/screening-history").Methods("GET").HandlerFunc(s.handleGetScreeningHistory)
}

func (s *Server) handleGetHistory(res http.ResponseWriter, req *http.Request) {
	// Accept 'n' and 'before' query params to scope our request and allow pagination
	arg := queries.GetBroadcastDataParams{}
	if nStr := req.URL.Query().Get("n"); nStr != "" {
		if n, err := strconv.Atoi(nStr); err == nil && n > 0 && n <= 100 {
			arg.Limit.Valid = true
			arg.Limit.Int32 = int32(n)
		}
	}
	if beforeStr := req.URL.Query().Get("before"); beforeStr != "" {
		if before, err := strconv.Atoi(beforeStr); err == nil {
			arg.BeforeBroadcastID.Valid = true
			arg.BeforeBroadcastID.Int32 = int32(before)
		}
	}

	// Query the requested range
	rows, err := s.q.GetBroadcastDataEx(req.Context(), arg)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return a JSON object that contains our result set
	result := broadcasts.History{
		Broadcasts: rows,
	}
	if err := json.NewEncoder(res).Encode(result); err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleGetHistoryById(res http.ResponseWriter, req *http.Request) {
	// Figure out which broadcast we want to fetch by ID
	broadcastIdStr, ok := mux.Vars(req)["id"]
	if !ok || broadcastIdStr == "" {
		http.Error(res, "failed to parse 'id' from URL", http.StatusInternalServerError)
		return
	}
	broadcastId, err := strconv.Atoi(broadcastIdStr)
	if err != nil {
		http.Error(res, "broadcast ID must be an integer", http.StatusBadRequest)
		return
	}

	// Our single query returns broadcast data in descending order by ID, so we can ask
	// for a single result that appears before (broadcastId + 1) to get our desired data
	rows, err := s.q.GetBroadcastDataEx(req.Context(), queries.GetBroadcastDataParams{
		BeforeBroadcastID: sql.NullInt32{Valid: true, Int32: int32(broadcastId + 1)},
		Limit:             sql.NullInt32{Valid: true, Int32: 1},
	})
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(rows) == 0 || rows[0].Id != broadcastId {
		http.Error(res, "no such broadcast", http.StatusNotFound)
		return
	}

	// We have the requested data; return it JSON-serialized
	broadcast := rows[0]
	if err := json.NewEncoder(res).Encode(broadcast); err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleGetScreeningHistory(res http.ResponseWriter, req *http.Request) {
	rows, err := s.q.GetScreeningHistory(req.Context())
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	broadcastIdsByTapeId := make(map[string][]int)
	for _, row := range rows {
		broadcastIds := make([]int, 0, len(row.BroadcastIds))
		for _, broadcastId := range row.BroadcastIds {
			broadcastIds = append(broadcastIds, int(broadcastId))
		}
		tapeIdStr := fmt.Sprintf("%d", row.TapeID)
		broadcastIdsByTapeId[tapeIdStr] = broadcastIds
	}

	result := broadcasts.ScreeningHistory{
		BroadcastIdsByTapeId: broadcastIdsByTapeId,
	}
	if err := json.NewEncoder(res).Encode(result); err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}
