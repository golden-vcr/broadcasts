package main

import (
	"database/sql"
	"encoding/json"
	"os"

	"github.com/codingconcepts/env"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	amqp "github.com/rabbitmq/amqp091-go"
	"golang.org/x/sync/errgroup"

	"github.com/golden-vcr/broadcasts/gen/queries"
	"github.com/golden-vcr/broadcasts/internal/state"
	etwitch "github.com/golden-vcr/schemas/twitch-events"
	"github.com/golden-vcr/server-common/db"
	"github.com/golden-vcr/server-common/entry"
	"github.com/golden-vcr/server-common/rmq"
)

type Config struct {
	DatabaseHost     string `env:"PGHOST" required:"true"`
	DatabasePort     int    `env:"PGPORT" required:"true"`
	DatabaseName     string `env:"PGDATABASE" required:"true"`
	DatabaseUser     string `env:"PGUSER" required:"true"`
	DatabasePassword string `env:"PGPASSWORD" required:"true"`
	DatabaseSslMode  string `env:"PGSSLMODE"`

	RmqHost     string `env:"RMQ_HOST" required:"true"`
	RmqPort     int    `env:"RMQ_PORT" required:"true"`
	RmqVhost    string `env:"RMQ_VHOST" required:"true"`
	RmqUser     string `env:"RMQ_USER" required:"true"`
	RmqPassword string `env:"RMQ_PASSWORD" required:"true"`
}

func main() {
	app, ctx := entry.NewApplication("broadcasts-consumer")
	defer app.Stop()

	// Parse config from environment variables
	err := godotenv.Load()
	if err != nil && !os.IsNotExist(err) {
		app.Fail("Failed to load .env file", err)
	}
	config := Config{}
	if err := env.Set(&config); err != nil {
		app.Fail("Failed to load config", err)
	}

	// Configure our database connection and initialize a Queries struct, so we can
	// check current broadcast state
	connectionString := db.FormatConnectionString(
		config.DatabaseHost,
		config.DatabasePort,
		config.DatabaseName,
		config.DatabaseUser,
		config.DatabasePassword,
		config.DatabaseSslMode,
	)
	db, err := sql.Open("postgres", connectionString)
	if err != nil {
		app.Fail("Failed to open sql.DB", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		app.Fail("Failed to connect to database", err)
	}
	q := queries.New(db)

	// Initialize an AMQP client
	amqpConn, err := amqp.Dial(rmq.FormatConnectionString(config.RmqHost, config.RmqPort, config.RmqVhost, config.RmqUser, config.RmqPassword))
	if err != nil {
		app.Fail("Failed to connect to AMQP server", err)
	}
	defer amqpConn.Close()

	// Prepare a producer that we can use to send messages to the broadcast-events
	// queue
	broadcastEventsProducer, err := rmq.NewProducer(amqpConn, "broadcast-events")
	if err != nil {
		app.Fail("Failed to initialize AMQP producer for broadcast-events", err)
	}

	// Prepare a consumer and start receiving incoming messages from the twitch-events
	// exchange: when the stream state changes, we'll automatically start/end a
	// broadcast by producing to broadcast-events
	twitchEventsConsumer, err := rmq.NewConsumer(amqpConn, "twitch-events")
	if err != nil {
		app.Fail("Failed to initialize AMQP consumer for twitch-events", err)
	}
	twitchEvents, err := twitchEventsConsumer.Recv(ctx)
	if err != nil {
		app.Fail("Failed to init recv channel on twitch-events consumer", err)
	}

	// Prepare a state.Writer interface, allowing us authoritatively modify the current
	// broadcast state in a way that propagates to the DB and the broadcast-events queue
	writer := state.NewWriter(q, broadcastEventsProducer)

	// Each time we read a message from the queue, spin up a new goroutine for that
	// message, parse it according to our twitch-events schema, then handle it
	wg, ctx := errgroup.WithContext(ctx)
	done := false
	for !done {
		select {
		case <-ctx.Done():
			app.Log().Info("Consumer context canceled; exiting main loop")
			done = true
		case d, ok := <-twitchEvents:
			if ok {
				wg.Go(func() error {
					var ev etwitch.Event
					if err := json.Unmarshal(d.Body, &ev); err != nil {
						return err
					}
					switch ev.Type {
					case etwitch.EventTypeStreamStarted:
						broadcast, err := writer.StartBroadcast(ctx)
						if err != nil {
							app.Log().Error("Failed to start broadcast", "error", err)
						} else {
							app.Log().Info("Started broadcast", "broadcast", broadcast)
						}
					case etwitch.EventTypeStreamEnded:
						err := writer.EndCurrentBroadcast(ctx)
						if err != nil {
							app.Log().Error("Failed to end broadcast", "error", err)
						} else {
							app.Log().Info("Ended broadcast")
						}
					}
					return nil
				})
			} else {
				app.Log().Info("Channel is closed; exiting main loop")
				done = true
			}
		}
	}

	if err := wg.Wait(); err != nil {
		app.Fail("Encountered an error during message handling", err)
	}
}
