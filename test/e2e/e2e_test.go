// Package e2e drives the service through its public gRPC API against a real
// Spanner (the emulator), using the same wiring as the server binary and the
// schema from migrations/. Nothing below the API is faked.
//
// The tests are skipped unless SPANNER_EMULATOR_HOST is set:
//
//	docker compose up -d
//	SPANNER_EMULATOR_HOST=localhost:9010 go test ./test/e2e/...
package e2e

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/spanner"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/mzubairashraf/product-catalog-service/internal/bootstrap"
	"github.com/mzubairashraf/product-catalog-service/internal/pkg/clock"
	"github.com/mzubairashraf/product-catalog-service/internal/schema"
	"github.com/mzubairashraf/product-catalog-service/internal/transport/grpcapi"
	productpb "github.com/mzubairashraf/product-catalog-service/proto/product/v1"
)

const (
	project    = "test-project"
	instanceID = "test-instance"
)

type suite struct {
	api productpb.ProductServiceClient
	db  *spanner.Client
}

// setup creates a fresh database from the migration file and serves the API
// on top of it. Each test gets its own database, so tests are independent.
func setup(t *testing.T) *suite {
	t.Helper()
	if os.Getenv("SPANNER_EMULATOR_HOST") == "" {
		t.Skip("SPANNER_EMULATOR_HOST is not set; start the emulator with `docker compose up -d`")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)

	if err := schema.EnsureEmulatorInstance(ctx, project, instanceID); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../../migrations/001_initial_schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	dbID := fmt.Sprintf("e2e-%d", time.Now().UnixNano()%1e15)
	parent := "projects/" + project + "/instances/" + instanceID
	if _, err := schema.CreateDatabase(ctx, project, instanceID, dbID, schema.Statements(string(raw))); err != nil {
		t.Fatal(err)
	}

	dbName := parent + "/databases/" + dbID
	db, err := spanner.NewClient(ctx, dbName)
	if err != nil {
		t.Fatal(err)
	}

	// Only errors are logged, so a failing test shows the storage error that
	// the API deliberately hides from callers.
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer(grpc.ChainUnaryInterceptor(grpcapi.Recovery(logger)))
	productpb.RegisterProductServiceServer(server, grpcapi.NewServer(bootstrap.NewApplication(db, clock.NewSystemClock()), logger))
	go func() { _ = server.Serve(listener) }()

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
		server.Stop()
		db.Close()
		_ = schema.DropDatabase(context.Background(), dbName)
	})
	return &suite{api: productpb.NewProductServiceClient(conn), db: db}
}

type outboxRow struct {
	Type, Status, Payload string
}

// outbox returns the events stored for an aggregate, oldest first.
func (s *suite) outbox(t *testing.T, aggregateID string) []outboxRow {
	t.Helper()
	iter := s.db.Single().Query(context.Background(), spanner.Statement{
		SQL:    `SELECT event_type, status, TO_JSON_STRING(payload) FROM outbox_events WHERE aggregate_id = @id ORDER BY created_at`,
		Params: map[string]any{"id": aggregateID},
	})
	defer iter.Stop()
	var rows []outboxRow
	for {
		row, err := iter.Next()
		if err == iterator.Done {
			return rows
		}
		if err != nil {
			t.Fatalf("reading outbox: %v", err)
		}
		var r outboxRow
		if err := row.Columns(&r.Type, &r.Status, &r.Payload); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, r)
	}
}

func (s *suite) create(t *testing.T, name, category string, numerator, denominator int64) string {
	t.Helper()
	reply, err := s.api.CreateProduct(context.Background(), &productpb.CreateProductRequest{
		Name: name, Description: name + " description", Category: category, PriceNumerator: numerator, PriceDenominator: denominator,
	})
	if err != nil {
		t.Fatalf("CreateProduct(%s): %v", name, err)
	}
	return reply.GetProductId()
}

func (s *suite) get(t *testing.T, id string) *productpb.Product {
	t.Helper()
	reply, err := s.api.GetProduct(context.Background(), &productpb.GetProductRequest{ProductId: id})
	if err != nil {
		t.Fatalf("GetProduct: %v", err)
	}
	return reply.GetProduct()
}

func TestProductLifecycle(t *testing.T) {
	s := setup(t)
	ctx := context.Background()

	id := s.create(t, "Mechanical Keyboard", "Electronics", 1999, 100)
	p := s.get(t, id)
	if p.GetStatus() != "inactive" || p.GetName() != "Mechanical Keyboard" || p.GetBasePrice().GetNumerator() != 1999 ||
		p.GetBasePrice().GetDenominator() != 100 || p.GetEffectivePrice().GetNumerator() != 1999 || p.GetDiscount() != nil {
		t.Fatalf("new product = %v", p)
	}

	// A discount cannot be applied until the product is active.
	discount := &productpb.ApplyDiscountRequest{ProductId: id, DiscountPercent: 15,
		StartDateUnix: time.Now().Add(-time.Hour).Unix(), EndDateUnix: time.Now().Add(time.Hour).Unix()}
	if _, err := s.api.ApplyDiscount(ctx, discount); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("discount on an inactive product: %v, want FailedPrecondition", err)
	}

	if _, err := s.api.ActivateProduct(ctx, &productpb.ActivateProductRequest{ProductId: id}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.api.UpdateProduct(ctx, &productpb.UpdateProductRequest{ProductId: id, Name: "Mechanical Keyboard v2", Description: "Hot-swappable", Category: "Electronics"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.api.ApplyDiscount(ctx, discount); err != nil {
		t.Fatal(err)
	}

	// 19.99 less 15% is 16.9915 exactly, i.e. 33983/2000. A float pipeline
	// would not return this fraction.
	p = s.get(t, id)
	if p.GetStatus() != "active" || p.GetName() != "Mechanical Keyboard v2" || p.GetDescription() != "Hot-swappable" ||
		p.GetDiscount().GetPercentage() != 15 || !p.GetDiscount().GetIsActive() ||
		p.GetEffectivePrice().GetNumerator() != 33983 || p.GetEffectivePrice().GetDenominator() != 2000 {
		t.Fatalf("discounted product = %v", p)
	}

	if _, err := s.api.ApplyDiscount(ctx, discount); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("second discount: %v, want FailedPrecondition", err)
	}
	if _, err := s.api.RemoveDiscount(ctx, &productpb.RemoveDiscountRequest{ProductId: id}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.api.DeactivateProduct(ctx, &productpb.DeactivateProductRequest{ProductId: id}); err != nil {
		t.Fatal(err)
	}
	p = s.get(t, id)
	if p.GetStatus() != "inactive" || p.GetDiscount() != nil || p.GetEffectivePrice().GetNumerator() != 1999 {
		t.Fatalf("final product = %v", p)
	}

	// Every successful command, and no failed one, left exactly one event.
	events := s.outbox(t, id)
	var types []string
	for _, e := range events {
		types = append(types, e.Type)
		if e.Status != "pending" || !strings.HasPrefix(e.Payload, "{") {
			t.Errorf("outbox row = %+v", e)
		}
	}
	want := []string{"product.created", "product.activated", "product.updated", "discount.applied", "discount.removed", "product.deactivated"}
	if !slices.Equal(types, want) {
		t.Errorf("outbox events = %v\nwant           %v", types, want)
	}
}

func TestListProducts(t *testing.T) {
	s := setup(t)
	ctx := context.Background()

	var ids []string
	for i := 1; i <= 5; i++ {
		ids = append(ids, s.create(t, fmt.Sprintf("Book %d", i), "Books", int64(1000*i), 100))
	}
	toy := s.create(t, "Robot", "Toys", 2500, 100)
	for _, id := range append(ids[:3:3], toy) { // activate three books and the toy
		if _, err := s.api.ActivateProduct(ctx, &productpb.ActivateProductRequest{ProductId: id}); err != nil {
			t.Fatal(err)
		}
	}

	list := func(req *productpb.ListProductsRequest) *productpb.ListProductsReply {
		t.Helper()
		reply, err := s.api.ListProducts(ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		return reply
	}

	if got := list(&productpb.ListProductsRequest{}); len(got.GetProducts()) != 6 || got.GetHasMore() {
		t.Errorf("all: %d products, has_more=%v; want 6, false", len(got.GetProducts()), got.GetHasMore())
	}
	if got := list(&productpb.ListProductsRequest{Category: "Books"}); len(got.GetProducts()) != 5 {
		t.Errorf("books: %d, want 5", len(got.GetProducts()))
	}
	if got := list(&productpb.ListProductsRequest{Category: "Books", Status: "active"}); len(got.GetProducts()) != 3 {
		t.Errorf("active books: %d, want 3", len(got.GetProducts()))
	}
	if got := list(&productpb.ListProductsRequest{Status: "active"}); len(got.GetProducts()) != 4 {
		t.Errorf("active: %d, want 4", len(got.GetProducts()))
	}

	// Page through the five books two at a time; no gaps, no repeats.
	seen := map[string]bool{}
	for offset, wantLen, wantMore := int32(0), []int{2, 2, 1}, []bool{true, true, false}; offset < 6; offset += 2 {
		page := list(&productpb.ListProductsRequest{Category: "Books", Limit: 2, Offset: offset})
		i := offset / 2
		if len(page.GetProducts()) != wantLen[i] || page.GetHasMore() != wantMore[i] {
			t.Errorf("page at offset %d: %d products, has_more=%v; want %d, %v", offset, len(page.GetProducts()), page.GetHasMore(), wantLen[i], wantMore[i])
		}
		for _, p := range page.GetProducts() {
			if seen[p.GetProductId()] {
				t.Errorf("product %s appeared on two pages", p.GetProductId())
			}
			seen[p.GetProductId()] = true
		}
	}
	if len(seen) != 5 {
		t.Errorf("paging returned %d distinct books, want 5", len(seen))
	}
}

func TestErrorsLeaveNoTrace(t *testing.T) {
	s := setup(t)
	ctx := context.Background()
	const missing = "99999999-9999-4999-8999-999999999999"

	if _, err := s.api.GetProduct(ctx, &productpb.GetProductRequest{ProductId: missing}); status.Code(err) != codes.NotFound {
		t.Errorf("get missing: %v, want NotFound", err)
	}
	if _, err := s.api.ActivateProduct(ctx, &productpb.ActivateProductRequest{ProductId: missing}); status.Code(err) != codes.NotFound {
		t.Errorf("activate missing: %v, want NotFound", err)
	}
	if _, err := s.api.CreateProduct(ctx, &productpb.CreateProductRequest{Name: "", Category: "Books", PriceNumerator: 1, PriceDenominator: 1}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("create without a name: %v, want InvalidArgument", err)
	}

	id := s.create(t, "Lamp", "Home", 4500, 100)
	if _, err := s.api.DeactivateProduct(ctx, &productpb.DeactivateProductRequest{ProductId: id}); status.Code(err) != codes.FailedPrecondition {
		t.Errorf("deactivate an inactive product: %v, want FailedPrecondition", err)
	}
	// The rejected command must not have written an event or changed the row.
	if events := s.outbox(t, id); len(events) != 1 || events[0].Type != "product.created" {
		t.Errorf("outbox after a rejected command = %+v, want only product.created", events)
	}
	if p := s.get(t, id); p.GetStatus() != "inactive" || p.GetCreatedAtUnix() != p.GetUpdatedAtUnix() {
		t.Errorf("product changed after a rejected command: %v", p)
	}
}
