# E2E Tests

This directory contains end-to-end tests that test the complete flow through the application stack, from use cases through repositories to the actual database.

## Overview

E2E tests verify the complete application stack by testing use cases directly against a real Spanner database (emulator). These tests follow the exact pattern specified in requirements.md.

### Test Coverage - All Main Flows from Requirements

#### ✅ Product Creation Flow (`TestProductCreationFlow`)
**Requirements**: Create products with name, description, base price, and category
- Setup: Real Spanner connection (emulator), real repos, real usecases
- Test: Create product via create_product use case
- Verify:
  - Product created successfully with all fields
  - Query returns correct data (name, description, category, price, status)
  - Product starts in `inactive` status
  - Outbox event created: `product.created`
  - Event stored atomically with product

#### ✅ Product Update Flow (`TestProductUpdateFlow`)
**Requirements**: Update product details (name, description, category)
- Test: Create product → Update details via update_product use case
- Verify:
  - All fields updated correctly
  - Change tracking working (only modified fields updated)
  - Outbox events: `product.created` + `product.updated`
  - Both events persisted

#### ✅ Discount Application Flow (`TestDiscountApplicationFlow`)
**Requirements**: Apply percentage-based discounts with start/end dates, precise decimal arithmetic
- Test: Create product → Activate → Apply 20% discount
- Verify:
  - Discount applied successfully with correct percentage
  - Effective price calculated correctly (base price - discount)
  - Discount stored with start/end dates
  - Outbox event: `discount.applied`
  - Pricing uses precise decimal arithmetic

#### ✅ Product Activation/Deactivation Flow (`TestProductActivationDeactivationFlow`)
**Requirements**: Activate/Deactivate products
- Test: Create product → Activate → Deactivate
- Verify:
  - Product starts as `inactive`
  - Activation changes status to `active`
  - Deactivation changes status back to `inactive`
  - Outbox events: `product.created`, `product.activated`, `product.deactivated`
  - All state transitions captured in events

#### ✅ Business Rule Validation (`TestBusinessRuleValidation`)
**Requirements**: Proper domain validation and error handling
- Test: Multiple validation scenarios
- Verify:
  - ❌ Cannot apply discount to inactive product (`ErrProductNotActive`)
  - ❌ Cannot create product with negative price (`ErrInvalidMoneyValue`)
  - ❌ Cannot apply discount > 100% (`ErrInvalidDiscountPercentage`)
  - ❌ Cannot apply discount with end before start (`ErrInvalidDiscountPeriod`)
  - ❌ Cannot update product with empty name (`ErrInvalidProductName`)
  - All domain errors properly captured and returned

#### ✅ Product Query Flow (`TestListProductsQuery`)
**Requirements**: Get product by ID with effective price, list active products with pagination, filter by category
- Test: Create multiple products → Query via list_products
- Verify:
  - All products returned correctly
  - Total count accurate
  - Pagination parameters work
  - Read model bypasses domain for optimization (CQRS)

#### ✅ Transactional Outbox (`TestTransactionalOutbox`)
**Requirements**: Publish events reliably using transactional outbox pattern
- Test: Create product and verify atomic consistency
- Verify:
  - Product and outbox event both exist (atomic transaction)
  - No product without event or vice versa
  - Golden Mutation Pattern: mutations collected and applied atomically
  - Transaction commits or rolls back as a unit

### Architecture Patterns Verified

These E2E tests validate all critical architectural patterns:

1. **Domain Layer Purity**: Domain logic executed without infrastructure dependencies
2. **CQRS Pattern**: Commands through aggregates, queries through read models
3. **Golden Mutation Pattern**: Repositories return mutations, committer applies atomically
4. **Transactional Outbox**: Events stored atomically with state changes
5. **Change Tracking**: Only modified fields generate database updates
6. **Repository Pattern**: Domain entities mapped to database models correctly

## Running E2E Tests

### Prerequisites

1. **Docker** installed and running
2. **Spanner Emulator** available

### Quick Start

```bash
# Start Spanner emulator
docker compose up -d spanner-emulator

# Run E2E tests
export SPANNER_EMULATOR_HOST=localhost:9010
go test -v ./test/e2e/...

# Or use make target
make test-e2e
```

### Using Make Targets

The project includes convenient make targets for E2E testing:

```bash
# Run E2E tests with automatic emulator management
make test-e2e

# Run with coverage report
make test-e2e-coverage

# Run specific test
make test-e2e-single TEST=TestE2E_CreateProduct

# Run all tests (unit + E2E)
make test-all
```

## Test Structure

### Current Implementation

The E2E test framework is set up with:

- **Test Database Setup**: Automatic Spanner database creation per test
- **Schema Management**: DDL statements for products and outbox tables
- **Cleanup**: Automatic database teardown after each test
- **Clock Mocking**: Fixed time for deterministic tests

### Recommended Test Scenarios

When writing E2E tests, cover these scenarios:

#### 1. Product Lifecycle Tests
```go
- Create product → Verify persistence
- Create → Update → Verify changes
- Create → Activate → Apply discount → Verify state
- Complete lifecycle: Create → Activate → Update → Discount → Deactivate
```

#### 2. Transactional Tests
```go
- Verify outbox events are created atomically with state changes
- Test rollback scenarios
- Verify mutation plan execution
```

#### 3. Query Tests
```go
- Get product by ID
- List products with filtering
- List with pagination
- Verify read model consistency
```

#### 4. Concurrent Operations
```go
- Multiple products created concurrently
- Concurrent updates to same product
- Race condition handling
```

## Writing E2E Tests

### Basic Test Template

```go
func TestE2E_YourScenario(t *testing.T) {
    // 1. Setup test database
    client, cleanup := setupTestDatabase(t)
    defer cleanup()

    ctx := context.Background()
    now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

    // 2. Setup dependencies
    productRepo := repo.NewProductRepository(client)
    outboxRepo := repo.NewOutboxRepository(client)
    committer := committer.NewSpannerCommitter(client)
    clock := &fixedClock{now: now}

    // 3. Execute use case
    interactor := create_product.NewInteractor(
        productRepo, outboxRepo, committer, clock)
    
    basePrice, _ := domain.NewMoney(10000, 100)
    resp, err := interactor.Execute(ctx, create_product.Request{
        Name:        "Test Product",
        Description: "Test",
        Category:    "Test",
        BasePrice:   basePrice,
    })

    // 4. Verify results
    require.NoError(t, err)
    require.NotNil(t, resp)
    
    // 5. Verify database state
    product, err := productRepo.FindByID(ctx, resp.ProductID)
    require.NoError(t, err)
    require.Equal(t, "Test Product", product.Name())
}
```

### Helper: Fixed Clock

```go
type fixedClock struct {
    now time.Time
}

func (c *fixedClock) Now() time.Time {
    return c.now
}
```

### Database Queries

To verify state directly in Spanner:

```go
var count int64
err := client.Single().Query(ctx, spanner.Statement{
    SQL: "SELECT COUNT(*) FROM products WHERE category = @category",
    Params: map[string]interface{}{
        "category": "Electronics",
    },
}).Do(func(r *spanner.Row) error {
    return r.Columns(&count)
})
```

## Architecture Verified

E2E tests validate these architectural patterns:

### 1. Golden Mutation Pattern
- Use cases build mutation plans
- Repositories return mutations (not execute them)
- Committer applies all mutations atomically

### 2. Transactional Outbox
- Domain events captured during aggregate operations
- Events persisted atomically with state changes
- No events lost or partially saved

### 3. CQRS
- Commands go through use cases
- Queries use read models
- Proper separation maintained

### 4. Change Tracking
- Only modified fields generate mutations
- Efficient database updates
- Audit trail through domain events

### 5. Domain Isolation
- Domain logic independent of infrastructure
- Repository abstraction working correctly
- No database leakage into domain

## Troubleshooting

### Emulator Not Running

```bash
Error: connection refused to localhost:9010

Solution:
docker compose up -d spanner-emulator
sleep 3  # Wait for emulator to be ready
```

### Port Already in Use

```bash
Error: address already in use

Solution:
docker compose down
docker compose up -d spanner-emulator
```

### Database Creation Fails

```bash
Error: database already exists

Solution:
# Tests automatically create unique database names with timestamps
# If issue persists, restart emulator:
docker compose restart spanner-emulator
```

### Test Timeout

```bash
Solution:
go test -v ./test/e2e/... -timeout 10m
```

## Performance Considerations

- Each test creates a fresh database (isolation)
- Database creation takes 1-2 seconds
- Total test suite should complete in < 1 minute
- Use `t.Parallel()` for independent tests to speed up execution

## CI/CD Integration

For GitHub Actions or similar:

```yaml
- name: Start Spanner Emulator
  run: docker compose up -d spanner-emulator

- name: Wait for Emulator
  run: sleep 5

- name: Run E2E Tests
  run: make test-e2e
  env:
    SPANNER_EMULATOR_HOST: localhost:9010
```

## Future Enhancements

Potential additions to E2E test suite:

1. **gRPC Layer Tests**: Full request/response through gRPC handlers
2. **Error Scenarios**: Network failures, constraint violations
3. **Performance Tests**: Load testing with many products
4. **Migration Tests**: Schema evolution scenarios
5. **Multi-tenant Tests**: If applicable to your use case

## Resources

- [Spanner Emulator Docs](https://cloud.google.com/spanner/docs/emulator)
- [Golden Mutation Pattern](../../internal/pkg/committer/README.md)
- [Test Coverage Report](../../coverage.html)
