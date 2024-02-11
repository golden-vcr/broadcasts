package admin

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/golden-vcr/auth"
	"github.com/golden-vcr/broadcasts/internal/state"
	"github.com/golden-vcr/server-common/entry"
	"github.com/gorilla/mux"
)

type Server struct {
	w state.Writer
}

func NewServer(w state.Writer) *Server {
	return &Server{
		w: w,
	}
}

func (s *Server) RegisterRoutes(c auth.Client, r *mux.Router) {
	// Require broadcaster access for all admin routes
	r.Use(func(next http.Handler) http.Handler {
		return auth.RequireAccess(c, auth.RoleBroadcaster, next)
	})

	// POST /tape allows the broadcaster to notify the backend that we're now screening
	// a new tape
	r.Path("/tape/{id}").Methods("POST").HandlerFunc(s.handleSetTape)
	r.Path("/tape").Methods("DELETE").HandlerFunc(s.handleClearTape)
}

func (s *Server) handleSetTape(res http.ResponseWriter, req *http.Request) {
	// Figure out which tape we want to screen
	tapeIdStr, ok := mux.Vars(req)["id"]
	if !ok || tapeIdStr == "" {
		http.Error(res, "failed to parse 'id' from URL", http.StatusInternalServerError)
		return
	}
	tapeId, err := strconv.Atoi(tapeIdStr)
	if err != nil {
		http.Error(res, "tape ID must be an integer", http.StatusBadRequest)
		return
	}

	// Update the DB with our new screening, and propagate to broadcast-events
	screening, err := s.w.StartScreening(req.Context(), tapeId)
	if err != nil {
		// Return 400 if we're attempting to start a screening when our current state
		// doesn't support it; 500 for anything else
		if errors.Is(err, state.ErrNoBroadcastInProgress) || errors.Is(err, state.ErrScreeningInProgress) {
			http.Error(res, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	entry.Log(req).Info("Started screening", "screening", screening)
	res.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleClearTape(res http.ResponseWriter, req *http.Request) {
	// Update DB state and propagate to broadcast-events, only if necessary/appropriate
	err := s.w.EndCurrentScreening(req.Context())

	// If the end result is that there's no screening in progress now, return 204
	if err == nil || errors.Is(err, state.ErrNoScreeningInProgress) {
		res.WriteHeader(http.StatusNoContent)
		return
	}

	// If we couldn't update screening state because there's no broadcast in progress,
	// return 400; return 500 for anything else
	if errors.Is(err, state.ErrNoBroadcastInProgress) {
		http.Error(res, err.Error(), http.StatusBadRequest)
		return
	}
	http.Error(res, err.Error(), http.StatusInternalServerError)
}
