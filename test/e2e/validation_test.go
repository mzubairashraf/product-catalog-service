package e2e

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/spanner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"product-catalog-service/internal/app/product/domain"
	"product-catalog-service/internal/app/product/queries/get_product"
	"product-catalog-service/internal/app/product/repo"
	"product-catalog-service/internal/app/product/usecases/apply_discount"
	"product-catalog-service/internal/app/product/usecases/create_product"
	"product-catalog-service/internal/pkg/committer"
)

// TestBusinessRuleValidation tests domain business rules
// Requirements: Validate business constraints, proper error handling
func TestBusinessRuleValidation(t *testing.T) {
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

	t.Run("Cannot apply discount to inactive product", func(t *testing.T) {
		// Create inactive product
		basePrice, _ := domain.NewMoney(10000, 100)
		createResp, _ := create_product.NewInteractor(productRepo, outboxRepo, comm, clk).Execute(ctx, create_product.Request{
			Name:        "Test Product",
			Description: "Test",
			Category:    "Test",
			BasePrice:   basePrice,
		})

		// Test: Try to apply discount to inactive product
		applyDiscountUsecase := apply_discount.NewInteractor(productRepo, outboxRepo, comm, clk)
		err := applyDiscountUsecase.Execute(ctx, apply_discount.Request{
			ProductID:       createResp.ProductID,
			DiscountPercent: 20,
			StartDate:       now,
			EndDate:         now.Add(24 * time.Hour),
		})

		// Verify: Error is domain.ErrProductNotActive
		assert.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrProductNotActive)
	})

	t.Run("Cannot create product with invalid price", func(t *testing.T) {
		// Test: Try to create product with negative price
		_, err := domain.NewMoney(-10000, 100)

		// Verify: Error is returned for negative money
		assert.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrInvalidPrice)
	})

	t.Run("Cannot apply invalid discount percentage", func(t *testing.T) {
		// Test: Try to create discount with invalid percentage
		_, err := domain.NewDiscount(150, now, now.Add(24*time.Hour))

		// Verify: Error for invalid percentage
		assert.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrDiscountPercentInvalid)
	})

	t.Run("Cannot apply discount with invalid date range", func(t *testing.T) {
		// Test: Try to create discount with end before start
		_, err := domain.NewDiscount(20, now, now.Add(-24*time.Hour))

		// Verify: Error for invalid date range
		assert.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrInvalidDiscountPeriod)
	})

	t.Run("Cannot update product with empty name", func(t *testing.T) {
		// Create product
		basePrice, _ := domain.NewMoney(10000, 100)
		createResp, _ := create_product.NewInteractor(productRepo, outboxRepo, comm, clk).Execute(ctx, create_product.Request{
			Name:        "Test Product",
			Description: "Test",
			Category:    "Test",
			BasePrice:   basePrice,
		})

		// Load product and try to update with empty name
		product, _ := productRepo.FindByID(ctx, createResp.ProductID)
		err := product.Update("", "Description", "Category", now)

		// Verify: Error for empty name
		assert.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrInvalidProductName)
	})

	t.Run("Discount percentage must be between 0 and 100", func(t *testing.T) {
		// Test negative percentage
		_, err := domain.NewDiscount(-10, now, now.Add(24*time.Hour))
		assert.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrDiscountPercentInvalid)

		// Test percentage over 100
		_, err = domain.NewDiscount(101, now, now.Add(24*time.Hour))
		assert.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrDiscountPercentInvalid)

		// Test valid percentages
		_, err = domain.NewDiscount(0, now, now.Add(24*time.Hour))
		assert.NoError(t, err)

		_, err = domain.NewDiscount(100, now, now.Add(24*time.Hour))
		assert.NoError(t, err)

		_, err = domain.NewDiscount(50, now, now.Add(24*time.Hour))
		assert.NoError(t, err)
	})
}

// TestPricingCalculations tests domain service pricing logic
// Requirements: Precise decimal arithmetic, correct discount calculations
func TestPricingCalculations(t *testing.T) {
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

	testCases := []struct {
		name              string
		basePrice         int64
		discountPercent   int
		expectedEffective float64
	}{
		{"10% off 100", 10000, 10, 90.0},
		{"25% off 200", 20000, 25, 150.0},
		{"50% off 100", 10000, 50, 50.0},
		{"33% off 99.99", 9999, 33, 67.0},
		{"0% off 100", 10000, 0, 100.0},
		{"100% off 100", 10000, 100, 0.0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create and activate product
			basePrice, _ := domain.NewMoney(tc.basePrice, 100)
			createResp, _ := create_product.NewInteractor(productRepo, outboxRepo, comm, clk).Execute(ctx, create_product.Request{
				Name:        "Test Product",
				Description: "Test",
				Category:    "Test",
				BasePrice:   basePrice,
			})

			// Activate
			product, _ := productRepo.FindByID(ctx, createResp.ProductID)
			product.Activate(now)
			plan := committer.NewPlan()
			if mut := productRepo.UpdateMut(product); mut != nil {
				plan.Add(mut)
			}
			comm.Apply(ctx, plan.Mutations())

			// Apply discount
			apply_discount.NewInteractor(productRepo, outboxRepo, comm, clk).Execute(ctx, apply_discount.Request{
				ProductID:       createResp.ProductID,
				DiscountPercent: tc.discountPercent,
				StartDate:       now.Add(-1 * time.Hour),
				EndDate:         now.Add(24 * time.Hour),
			})

			// Verify effective price
			productDTO, _ := get_product.NewQuery(readModel).Execute(ctx, get_product.Request{
				ProductID: createResp.ProductID,
			})

			assert.InDelta(t, tc.expectedEffective, productDTO.EffectivePrice, 0.5,
				"Effective price calculation incorrect for %s", tc.name)
		})
	}
}

// TestConcurrentProductCreation tests concurrent operations
// Requirements: Handle concurrent updates
func TestConcurrentProductCreation(t *testing.T) {
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

	interactor := create_product.NewInteractor(productRepo, outboxRepo, comm, clk)

	// Create multiple products concurrently
	numProducts := 5
	done := make(chan error, numProducts)

	basePrice, _ := domain.NewMoney(10000, 100)

	for i := 0; i < numProducts; i++ {
		go func(index int) {
			_, err := interactor.Execute(ctx, create_product.Request{
				Name:        "Product",
				Description: "Description",
				Category:    "Category",
				BasePrice:   basePrice,
			})
			done <- err
		}(i)
	}

	// Wait for all to complete
	for i := 0; i < numProducts; i++ {
		err := <-done
		require.NoError(t, err)
	}

	// Verify all products were created
	var productCount int64
	err := client.Single().Query(ctx, spanner.Statement{
		SQL: "SELECT COUNT(*) FROM products",
	}).Do(func(r *spanner.Row) error {
		return r.Columns(&productCount)
	})
	require.NoError(t, err)
	assert.Equal(t, int64(numProducts), productCount)
}
