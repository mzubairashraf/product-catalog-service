package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/spanner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"product-catalog-service/internal/app/product/domain"
	"product-catalog-service/internal/app/product/queries/get_product"
	"product-catalog-service/internal/app/product/queries/list_products"
	"product-catalog-service/internal/app/product/repo"
	"product-catalog-service/internal/app/product/usecases/activate_product"
	"product-catalog-service/internal/app/product/usecases/apply_discount"
	"product-catalog-service/internal/app/product/usecases/create_product"
	"product-catalog-service/internal/app/product/usecases/deactivate_product"
	"product-catalog-service/internal/app/product/usecases/update_product"
	"product-catalog-service/internal/pkg/committer"
)

// TestProductCreationFlow tests the complete product creation flow
// Requirements: Product creation, event publishing, query consistency
func TestProductCreationFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	client, cleanup := setupTestDatabase(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	// Setup: Real Spanner connection (emulator), real repos, real usecases
	productRepo := repo.NewProductRepository(client)
	outboxRepo := repo.NewOutboxRepository()
	comm := committer.NewSpannerCommitter(client)
	clk := &fixedClock{now: now}
	readModel := repo.NewProductReadModel(client)

	createUsecase := create_product.NewInteractor(productRepo, outboxRepo, comm, clk)
	getQuery := get_product.NewQuery(readModel)

	// Test: Create product
	basePrice, err := domain.NewMoney(29900, 100)
	require.NoError(t, err)

	resp, err := createUsecase.Execute(ctx, create_product.Request{
		Name:        "Test Product",
		Description: "Test Description",
		Category:    "Electronics",
		BasePrice:   basePrice,
	})

	// Verify: Product created successfully
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotEmpty(t, resp.ProductID)

	// Verify: Query returns correct data
	product, err := getQuery.Execute(ctx, get_product.Request{
		ProductID: resp.ProductID,
	})
	require.NoError(t, err)
	assert.Equal(t, "Test Product", product.Name)
	assert.Equal(t, "Test Description", product.Description)
	assert.Equal(t, "Electronics", product.Category)
	assert.InDelta(t, 299.0, product.BasePrice, 0.01)
	assert.Equal(t, "inactive", product.Status)

	// Verify: Outbox event was created
	events := getOutboxEvents(t, client, resp.ProductID)
	require.Len(t, events, 1, "Should have exactly 1 outbox event for product creation")
	assert.True(t, strings.Contains(events[0].EventType, "created"), "Event should be product.created")
	assert.Equal(t, resp.ProductID, events[0].AggregateID)
}

// TestProductUpdateFlow tests product update with change tracking
// Requirements: Update product details, event publishing
func TestProductUpdateFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test")
	}

	client, cleanup := setupTestDatabase(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	// Setup
	productRepo := repo.NewProductRepository(client)
	outboxRepo := repo.NewOutboxRepository()
	comm := committer.NewSpannerCommitter(client)
	clk := &fixedClock{now: now}
	readModel := repo.NewProductReadModel(client)

	// Create initial product
	basePrice, _ := domain.NewMoney(10000, 100)
	createResp, err := create_product.NewInteractor(productRepo, outboxRepo, comm, clk).Execute(ctx, create_product.Request{
		Name:        "Original Name",
		Description: "Original Description",
		Category:    "Original Category",
		BasePrice:   basePrice,
	})
	require.NoError(t, err)

	// Update product
	updateUsecase := update_product.NewInteractor(productRepo, outboxRepo, comm, clk)
	err = updateUsecase.Execute(ctx, update_product.Request{
		ProductID:   createResp.ProductID,
		Name:        "Updated Name",
		Description: "Updated Description",
		Category:    "Updated Category",
	})
	require.NoError(t, err)

	// Verify: Updated data is persisted
	product, err := get_product.NewQuery(readModel).Execute(ctx, get_product.Request{
		ProductID: createResp.ProductID,
	})
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", product.Name)
	assert.Equal(t, "Updated Description", product.Description)
	assert.Equal(t, "Updated Category", product.Category)

	// Verify: Both creation and update events exist
	events := getOutboxEvents(t, client, createResp.ProductID)
	require.Len(t, events, 2, "Should have 2 events: created + updated")
	assert.True(t, strings.Contains(events[0].EventType, "created"))
	assert.True(t, strings.Contains(events[1].EventType, "updated"))
}

// TestDiscountApplicationFlow tests discount application with price calculation
// Requirements: Apply discounts, pricing calculations, event publishing
func TestDiscountApplicationFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test")
	}

	client, cleanup := setupTestDatabase(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	// Setup
	productRepo := repo.NewProductRepository(client)
	outboxRepo := repo.NewOutboxRepository()
	comm := committer.NewSpannerCommitter(client)
	clk := &fixedClock{now: now}
	readModel := repo.NewProductReadModel(client)

	// Create and activate product
	basePrice, _ := domain.NewMoney(10000, 100)
	createResp, _ := create_product.NewInteractor(productRepo, outboxRepo, comm, clk).Execute(ctx, create_product.Request{
		Name:        "Test Product",
		Description: "Test",
		Category:    "Test",
		BasePrice:   basePrice,
	})

	// Activate product (required before applying discount)
	product, _ := productRepo.FindByID(ctx, createResp.ProductID)
	err := product.Activate(now)
	require.NoError(t, err)
	plan := committer.NewPlan()
	if mut := productRepo.UpdateMut(product); mut != nil {
		plan.Add(mut)
	}
	comm.Apply(ctx, plan.Mutations())

	// Test: Apply discount
	applyDiscountUsecase := apply_discount.NewInteractor(productRepo, outboxRepo, comm, clk)
	err = applyDiscountUsecase.Execute(ctx, apply_discount.Request{
		ProductID:       createResp.ProductID,
		DiscountPercent: 20,
		StartDate:       now.Add(-1 * time.Hour),
		EndDate:         now.Add(24 * time.Hour),
	})
	require.NoError(t, err)

	// Verify: Effective price is calculated correctly
	productDTO, err := get_product.NewQuery(readModel).Execute(ctx, get_product.Request{
		ProductID: createResp.ProductID,
	})
	require.NoError(t, err)
	assert.NotNil(t, productDTO.Discount)
	assert.Equal(t, 20, productDTO.Discount.Percentage)
	assert.InDelta(t, 80.0, productDTO.EffectivePrice, 0.01, "Effective price should be 100 - 20% = 80")

	// Verify: Discount data persisted in database
	var discountPercent spanner.NullInt64
	err = client.Single().Query(ctx, spanner.Statement{
		SQL: "SELECT discount_percent FROM products WHERE product_id = @id",
		Params: map[string]interface{}{
			"id": createResp.ProductID,
		},
	}).Do(func(r *spanner.Row) error {
		return r.Columns(&discountPercent)
	})
	require.NoError(t, err)
	require.True(t, discountPercent.Valid)
	assert.Equal(t, int64(20), discountPercent.Int64)

	// Verify: Discount applied event was created
	events := getOutboxEvents(t, client, createResp.ProductID)
	hasDiscountEvent := false
	for _, e := range events {
		if strings.Contains(e.EventType, "discount") && strings.Contains(e.EventType, "applied") {
			hasDiscountEvent = true
			break
		}
	}
	assert.True(t, hasDiscountEvent, "Should have discount.applied event")
}

// TestProductActivationDeactivationFlow tests product status changes
// Requirements: Activate/Deactivate products, event publishing
func TestProductActivationDeactivationFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test")
	}

	client, cleanup := setupTestDatabase(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	// Setup
	productRepo := repo.NewProductRepository(client)
	outboxRepo := repo.NewOutboxRepository()
	comm := committer.NewSpannerCommitter(client)
	clk := &fixedClock{now: now}
	readModel := repo.NewProductReadModel(client)

	// Create product (starts as inactive)
	basePrice, _ := domain.NewMoney(10000, 100)
	createResp, err := create_product.NewInteractor(productRepo, outboxRepo, comm, clk).Execute(ctx, create_product.Request{
		Name:        "Test Product",
		Description: "Test",
		Category:    "Test",
		BasePrice:   basePrice,
	})
	require.NoError(t, err)

	// Verify initial state: inactive
	product, err := get_product.NewQuery(readModel).Execute(ctx, get_product.Request{
		ProductID: createResp.ProductID,
	})
	require.NoError(t, err)
	assert.Equal(t, "inactive", product.Status)

	// Test: Activate product
	activateUsecase := activate_product.NewInteractor(productRepo, outboxRepo, comm, clk)
	err = activateUsecase.Execute(ctx, activate_product.Request{
		ProductID: createResp.ProductID,
	})
	require.NoError(t, err)

	// Verify: Product is now active
	product, err = get_product.NewQuery(readModel).Execute(ctx, get_product.Request{
		ProductID: createResp.ProductID,
	})
	require.NoError(t, err)
	assert.Equal(t, "active", product.Status)

	// Test: Deactivate product
	deactivateUsecase := deactivate_product.NewInteractor(productRepo, outboxRepo, comm, clk)
	err = deactivateUsecase.Execute(ctx, deactivate_product.Request{
		ProductID: createResp.ProductID,
	})
	require.NoError(t, err)

	// Verify: Product is inactive again
	product, err = get_product.NewQuery(readModel).Execute(ctx, get_product.Request{
		ProductID: createResp.ProductID,
	})
	require.NoError(t, err)
	assert.Equal(t, "inactive", product.Status)

	// Verify: All events captured (created, activated, deactivated)
	events := getOutboxEvents(t, client, createResp.ProductID)
	require.GreaterOrEqual(t, len(events), 3, "Should have at least 3 events")

	eventTypes := make([]string, len(events))
	for i, e := range events {
		eventTypes[i] = e.EventType
	}

	hasCreated := false
	hasActivated := false
	hasDeactivated := false

	for _, et := range eventTypes {
		if strings.Contains(et, "created") {
			hasCreated = true
		}
		if strings.Contains(et, "activated") {
			hasActivated = true
		}
		if strings.Contains(et, "deactivated") {
			hasDeactivated = true
		}
	}

	assert.True(t, hasCreated, "Should have product.created event")
	assert.True(t, hasActivated, "Should have product.activated event")
	assert.True(t, hasDeactivated, "Should have product.deactivated event")
}

// TestListProductsQuery tests product listing with pagination
// Requirements: List active products with pagination, filter by category
func TestListProductsQuery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test")
	}

	client, cleanup := setupTestDatabase(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	// Setup
	productRepo := repo.NewProductRepository(client)
	outboxRepo := repo.NewOutboxRepository()
	comm := committer.NewSpannerCommitter(client)
	clk := &fixedClock{now: now}
	readModel := repo.NewProductReadModel(client)

	// Create multiple products
	createUsecase := create_product.NewInteractor(productRepo, outboxRepo, comm, clk)
	basePrice, _ := domain.NewMoney(10000, 100)

	for i := 0; i < 5; i++ {
		_, err := createUsecase.Execute(ctx, create_product.Request{
			Name:        "Product",
			Description: "Test",
			Category:    "Electronics",
			BasePrice:   basePrice,
		})
		require.NoError(t, err)
	}

	// Test: List all products
	listQuery := list_products.NewQuery(readModel)
	result, err := listQuery.Execute(ctx, list_products.Request{
		Limit: 10,
	})

	// Verify: All products returned
	require.NoError(t, err)
	assert.Len(t, result.Products, 5)
	assert.Equal(t, int64(5), result.TotalCount)
}

// TestTransactionalOutbox tests transactional outbox pattern
// Requirements: Publish events reliably using transactional outbox pattern
func TestTransactionalOutbox(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test")
	}

	client, cleanup := setupTestDatabase(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	// Setup
	productRepo := repo.NewProductRepository(client)
	outboxRepo := repo.NewOutboxRepository()
	comm := committer.NewSpannerCommitter(client)
	clk := &fixedClock{now: now}

	// Create product
	basePrice, _ := domain.NewMoney(10000, 100)
	createResp, _ := create_product.NewInteractor(productRepo, outboxRepo, comm, clk).Execute(ctx, create_product.Request{
		Name:        "Test Product",
		Description: "Test",
		Category:    "Test",
		BasePrice:   basePrice,
	})

	// Verify: Product and outbox event both exist (atomic transaction)
	var productExists, outboxExists bool

	// Check product
	err := client.Single().Query(ctx, spanner.Statement{
		SQL: "SELECT TRUE FROM products WHERE product_id = @id",
		Params: map[string]interface{}{
			"id": createResp.ProductID,
		},
	}).Do(func(r *spanner.Row) error {
		productExists = true
		return nil
	})
	require.NoError(t, err)

	// Check outbox
	err = client.Single().Query(ctx, spanner.Statement{
		SQL: "SELECT TRUE FROM outbox WHERE aggregate_id = @id",
		Params: map[string]interface{}{
			"id": createResp.ProductID,
		},
	}).Do(func(r *spanner.Row) error {
		outboxExists = true
		return nil
	})
	require.NoError(t, err)

	// Both must exist (atomic transaction)
	assert.True(t, productExists, "Product must exist")
	assert.True(t, outboxExists, "Outbox event must exist")
	t.Log("✓ Transactional outbox pattern: Both product and event persisted atomically")
}
