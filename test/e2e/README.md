# E2E Tests

This directory contains end-to-end tests that test the complete flow through the application stack, from use cases through repositories to the actual database.

## Overview

E2E tests verify:
- **Complete request/response flows** through use cases
- **Database persistence** using actual Spanner
- **Transactional consistency** (Outbox pattern)
- **Golden Mutation Pattern** implementation
- **Integration between all layers**: Domain → Application → Infrastructure

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
