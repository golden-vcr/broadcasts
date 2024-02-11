package main

import (
	"database/sql"
	"os"

	"github.com/codingconcepts/env"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/golden-vcr/auth"
	"github.com/golden-vcr/broadcasts/gen/queries"
	"github.com/golden-vcr/broadcasts/internal/admin"
	"github.com/golden-vcr/broadcasts/internal/state"
	"github.com/golden-vcr/server-common/db"
	"github.com/golden-vcr/server-common/entry"
	"github.com/golden-vcr/server-common/rmq"
)

type Config struct {
	BindAddr   string `env:"BIND_ADDR"`
	ListenPort uint16 `env:"LISTEN_PORT" default:"5007"`

	AuthURL string `env:"AUTH_URL" default:"http://localhost:5002"`

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
	app, ctx := entry.NewApplication("broadcasts")
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
	// view and modify current broadcast state
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

	// Initialize an auth client so we can require broadcaster-level access in order to
	// call the admin-only API
	authClient, err := auth.NewClient(ctx, config.AuthURL)
	if err != nil {
		app.Fail("Failed to initialize auth client", err)
	}

	// Prepare a producer that we can use to send messages to the broadcast-events
	// queue
	broadcastEventsProducer, err := rmq.NewProducer(amqpConn, "broadcast-events")
	if err != nil {
		app.Fail("Failed to initialize AMQP producer for broadcast-events", err)
	}

	// Prepare a state.Writer interface, allowing us authoritatively modify the current
	// broadcast state in a way that propagates to the DB and the broadcast-events queue
	writer := state.NewWriter(q, broadcastEventsProducer)

	// Start setting up our HTTP handlers, using gorilla/mux for routing
	r := mux.NewRouter()

	// We can call the broadcaster-only admin API to directly modify broadcast state
	{
		adminServer := admin.NewServer(writer)
		adminServer.RegisterRoutes(authClient, r)
	}

	// Handle incoming HTTP connections until our top-level context is canceled, at
	// which point shut down cleanly
	entry.RunServer(ctx, app.Log(), r, config.BindAddr, config.ListenPort)
}
