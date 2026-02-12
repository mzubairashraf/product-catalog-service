package e2e

import (
	"context"
	"os"
	"testing"
	"time"

	"cloud.google.com/go/spanner"
	database "cloud.google.com/go/spanner/admin/database/apiv1"
	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"product-catalog-service/internal/pkg/clock"
)

var (
	testEmulator  string
	testProjectID = "test-project"
	testInstance  = "test-instance"
	testDatabase  = "test-database"
)

func TestMain(m *testing.M) {
	testEmulator = os.Getenv("SPANNER_EMULATOR_HOST")
	if testEmulator == "" {
		testEmulator = "localhost:9010"
		os.Setenv("SPANNER_EMULATOR_HOST", testEmulator)
	}
	code := m.Run()
	os.Exit(code)
}

func setupTestDatabase(t *testing.T) (*spanner.Client, func()) {
	ctx := context.Background()

	adminClient, err := database.NewDatabaseAdminClient(ctx,
		option.WithEndpoint(testEmulator),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
		option.WithoutAuthentication(),
	)
	require.NoError(t, err)

	dbID := testDatabase + "-" + time.Now().Format("20060102150405")
	dbName := "projects/" + testProjectID + "/instances/" + testInstance + "/databases/" + dbID

	op, err := adminClient.CreateDatabase(ctx, &databasepb.CreateDatabaseRequest{
		Parent:          "projects/" + testProjectID + "/instances/" + testInstance,
		CreateStatement: "CREATE DATABASE `" + dbID + "`",
		ExtraStatements: []string{
			`CREATE TABLE products (
				product_id STRING(36) NOT NULL,
				name STRING(255) NOT NULL,
				description STRING(1024),
				category STRING(100) NOT NULL,
				base_price_numerator INT64 NOT NULL,
				base_price_denominator INT64 NOT NULL,
				discount_percent INT64,
				discount_start_date TIMESTAMP,
				discount_end_date TIMESTAMP,
				status STRING(20) NOT NULL,
				created_at TIMESTAMP NOT NULL,
				updated_at TIMESTAMP NOT NULL,
				archived_at TIMESTAMP
			) PRIMARY KEY (product_id)`,
			`CREATE TABLE outbox (
				event_id STRING(36) NOT NULL,
				event_type STRING(100) NOT NULL,
				aggregate_id STRING(36) NOT NULL,
				payload STRING(MAX) NOT NULL,
				created_at TIMESTAMP NOT NULL
			) PRIMARY KEY (event_id)`,
		},
	})
	require.NoError(t, err)

	_, err = op.Wait(ctx)
	require.NoError(t, err)

	client, err := spanner.NewClient(ctx, dbName,
		option.WithEndpoint(testEmulator),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
		option.WithoutAuthentication(),
	)
	require.NoError(t, err)

	cleanup := func() {
		client.Close()
		adminClient.DropDatabase(ctx, &databasepb.DropDatabaseRequest{Database: dbName})
		adminClient.Close()
	}

	return client, cleanup
}

type fixedClock struct {
	now time.Time
}

func (c *fixedClock) Now() time.Time {
	return c.now
}

var _ clock.Clock = (*fixedClock)(nil)

func getOutboxEvents(t *testing.T, client *spanner.Client, aggregateID string) []outboxEvent {
	ctx := context.Background()
	var events []outboxEvent

	iter := client.Single().Query(ctx, spanner.Statement{
		SQL: "SELECT event_id, event_type, aggregate_id, payload, created_at FROM outbox WHERE aggregate_id = @aggregateID ORDER BY created_at",
		Params: map[string]interface{}{
			"aggregateID": aggregateID,
		},
	})
	defer iter.Stop()

	err := iter.Do(func(r *spanner.Row) error {
		var e outboxEvent
		err := r.Columns(&e.EventID, &e.EventType, &e.AggregateID, &e.Payload, &e.CreatedAt)
		if err != nil {
			return err
		}
		events = append(events, e)
		return nil
	})
	require.NoError(t, err)

	return events
}

type outboxEvent struct {
	EventID     string
	EventType   string
	AggregateID string
	Payload     string
	CreatedAt   time.Time
}
