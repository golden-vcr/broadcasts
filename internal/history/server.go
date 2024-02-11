package history

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/golden-vcr/broadcasts"
	"github.com/golden-vcr/broadcasts/gen/queries"
	"github.com/gorilla/mux"
)

type Queries interface {
	GetBroadcastDataEx(ctx context.Context, arg queries.GetBroadcastDataParams) ([]broadcasts.Broadcast, error)
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
}

func (s *Server) handleGetHistory(res http.ResponseWriter, req *http.Request) {
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
