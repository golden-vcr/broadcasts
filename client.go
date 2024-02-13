package broadcasts

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	ebroadcast "github.com/golden-vcr/schemas/broadcast-events"
	"github.com/golden-vcr/schemas/core"
	"github.com/golden-vcr/server-common/rmq"
	amqp "github.com/rabbitmq/amqp091-go"
	"golang.org/x/exp/slog"
)

// Client is a simple interface that keeps track of current broadcast state at all times
type Client interface {
	GetState() core.State
}

// NewClient initializes a broadcasts.Client that will keep track of platform-wide
// broadcast state: the client makes an initial HTTP request to the broadcasts service,
// and thereafter it consumes from the 'broadcast-events' queue in order to keep abreast
// subsequent changes in state. Calling GetState() on the resulting client (thread-safe)
// will return the current broadcast state at any time.
func NewClient(ctx context.Context, logger *slog.Logger, broadcastsUrl string, amqpConn *amqp.Connection) (Client, error) {
	// Get our current broadcast state as a starting point, so that we're fully
	// initialized without having to wait on events to arrive
	state, err := resolveInitialState(ctx, broadcastsUrl)
	if err != nil {
		return nil, err
	}

	// Prepare a client struct that encapsulates our current state
	c := &client{
		currentState: *state,
	}

	// Initialize a consumer so that whenenver broadcast state changes, we'll be
	// notified
	broadcastEventsConsumer, err := rmq.NewConsumer(amqpConn, "broadcast-events")
	if err != nil {
		return nil, fmt.Errorf("Failed to initialize AMQP consumer for broadcast-events: %w", err)
	}
	broadcastEvents, err := broadcastEventsConsumer.Recv(ctx)
	if err != nil {
		return nil, fmt.Errorf("Failed to init recv channel on broadcast-events consumer: %w", err)
	}

	// Run a goroutine that will update our client's state any time we consume an event
	// from the broadcast-events queue
	go func() {
		done := false
		for !done {
			select {
			case <-ctx.Done():
				logger.Info("Consumer context canceled; broadcast state client shutting down")
				done = true
			case d, ok := <-broadcastEvents:
				if ok {
					var ev ebroadcast.Event
					if err := json.Unmarshal(d.Body, &ev); err != nil {
						logger.Error("Failed to unmarshal event from broadcast-events; broadcast state client shutting down", "error", err)
						done = true
						break
					}
					c.handleEvent(&ev, logger)
				} else {
					logger.Info("Channel is closed; broadcast state client shutting down")
					done = true
				}
			}
		}
	}()

	return c, nil
}

// resolveInitialState makes a request to the broadcasts service in order to resolve an
// initial value for our platform-wide broadcast state
func resolveInitialState(ctx context.Context, broadcastsUrl string) (*core.State, error) {
	// Prepare a request to the broadcasts service's history API, to retrieve data for
	// the most recent broadcast
	url := broadcastsUrl + "/history"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	q.Set("n", "1")
	req.URL.RawQuery = q.Encode()

	// Make the request, and ensure that we got a valid response
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("got response %d from %s", res.StatusCode, url)
	}
	var h History
	if err := json.NewDecoder(res.Body).Decode(&h); err != nil {
		return nil, fmt.Errorf("failed to decode response body from %s: %w", url, err)
	}

	// Resolve our initial state value based on the data reported from the API
	var state core.State
	if len(h.Broadcasts) > 0 {
		if broadcast := h.Broadcasts[0]; broadcast.EndedAt == nil {
			// If our most recent broadcast is still in progress, that's our current
			// broadcast ID
			state.BroadcastId = broadcast.Id
			if len(broadcast.Screenings) > 0 {
				if screening := broadcast.Screenings[len(broadcast.Screenings)-1]; screening.EndedAt == nil {
					// If the latest screening in that broadcast is still in progress,
					// that gives us our current screening and tape IDs
					state.ScreeningId = screening.Id
					state.TapeId = screening.TapeId
				}
			}
		}
	}
	return &state, nil
}

type client struct {
	currentState core.State
	mu           sync.RWMutex
}

func (c *client) GetState() core.State {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.currentState
}

func (c *client) handleEvent(ev *ebroadcast.Event, logger *slog.Logger) {
	c.mu.Lock()
	defer c.mu.Unlock()

	prev := c.currentState
	c.currentState = ev.ToState(prev)
	logger.Info("Broadcast state changed", "broadcastEvent", ev, "prevState", prev, "newState", c.currentState)
}
