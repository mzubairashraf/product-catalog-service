# Test Task: Product Catalog Service (Middle Golang Engineer)

## Overview

You are tasked with implementing a simplified **Product Catalog Service** using the same architecture, patterns, and technology stack used in our production systems. This service will manage products and their pricing with proper domain-driven design and clean architecture principles.

**Time Estimate:** 8-12 hours
**Submission:** GitHub repository with README, working code, and tests

---

## Business Requirements

The service must handle:

1. **Product Management**
   - Create products with name, description, base price, and category
   - Update product details (name, description, category)
   - Activate/Deactivate products
   - Archive products (soft delete)

2. **Pricing Rules**
   - Apply percentage-based discounts to products
   - Discounts have start/end dates
   - Only one active discount per product at a time
   - Pricing calculations must use precise decimal arithmetic

3. **Product Queries**
   - Get product by ID with current effective price
   - List active products with pagination
   - Filter products by category

4. **Event Publishing**
   - Publish events when products are created, updated, or pricing changes
   - Events must be published reliably using transactional outbox pattern

---

## Required Technology Stack

### Core Stack
- **Language:** Go 1.21+
- **Database:** Google Cloud Spanner (use emulator for local development)
- **Transport:** gRPC with Protocol Buffers
- **Transaction Management:** `github.com/Vektor-AI/commitplan` with Spanner driver
- **Decimal Precision:** `math/big` for money calculations
- **Testing:** Standard Go testing with testify assertions

### Google Cloud Dependencies
```go
import (
    "cloud.google.com/go/spanner"
    "github.com/Vektor-AI/commitplan"
    "github.com/Vektor-AI/commitplan/drivers/spanner"
)
```

### Expected Project Structure
```
product-catalog-service/
├── cmd/
│   └── server/
│       └── main.go                    # Service entry point
├── internal/
│   ├── app/
│   │   └── product/
│   │       ├── domain/
│   │       │   ├── product.go         # Product aggregate
│   │       │   ├── discount.go        # Discount value object
│   │       │   ├── money.go           # Money value object
│   │       │   ├── domain_events.go   # Domain events
│   │       │   ├── domain_errors.go   # Domain errors
│   │       │   └── services/
│   │       │       └── pricing_calculator.go  # Domain service
│   │       ├── usecases/
│   │       │   ├── create_product/
│   │       │   │   └── interactor.go
│   │       │   ├── update_product/
│   │       │   │   └── interactor.go
│   │       │   ├── apply_discount/
│   │       │   │   └── interactor.go
│   │       │   └── activate_product/
│   │       │       └── interactor.go
│   │       ├── queries/
│   │       │   ├── get_product/
│   │       │   │   ├── query.go
│   │       │   │   └── dto.go
│   │       │   └── list_products/
│   │       │       ├── query.go
│   │       │       └── dto.go
│   │       ├── contracts/
│   │       │   ├── product_repo.go    # Repository interface
│   │       │   └── read_model.go      # Read model interface
│   │       └── repo/
│   │           └── product_repo.go    # Spanner implementation
│   ├── models/
│   │   ├── m_product/
│   │   │   ├── data.go               # Product DB model
│   │   │   └── fields.go             # Field constants
│   │   └── m_outbox/
│   │       ├── data.go               # Outbox DB model
│   │       └── fields.go
│   ├── transport/
│   │   └── grpc/
│   │       └── product/
│   │           ├── handler.go        # gRPC server
│   │           ├── create.go         # Create handler
│   │           ├── update.go         # Update handler
│   │           ├── get.go            # Get handler
│   │           ├── list.go           # List handler
│   │           ├── mappers.go        # Proto <-> Domain mapping
│   │           └── errors.go         # Error mapping
│   ├── services/
│   │   └── options.go                # DI container
│   └── pkg/
│       ├── committer/
│       │   └── plan.go               # Typed CommitPlan wrapper
│       └── clock/
│           └── clock.go              # Time abstraction
├── proto/
│   └── product/
│       └── v1/
│           └── product_service.proto  # gRPC API definition
├── migrations/
│   └── 001_initial_schema.sql        # Spanner DDL
├── tests/
│   └── e2e/
│       └── product_test.go           # E2E tests
├── docker-compose.yml                # Spanner emulator setup
├── go.mod
├── go.sum
└── README.md
```

---

## Architecture Requirements

### 1. Domain Layer Purity (CRITICAL)

Your domain layer must follow these rules:

✅ **MUST:**
- Pure Go business logic only
- Use `*big.Rat` for all money calculations
- Define domain errors as sentinel values
- Implement proper aggregate with encapsulation
- Use change tracking for dirty fields
- Capture domain events as intents (simple structs)

❌ **MUST NOT:**
- Import `context.Context` in domain
- Import database libraries (Spanner, SQL, etc.)
- Import proto definitions
- Import external frameworks
- Contain any infrastructure concerns

**Example Domain Aggregate:**
```go
package domain

type Product struct {
    id           string
    name         string
    description  string
    category     string
    basePrice    *Money
    discount     *Discount
    status       ProductStatus
    changes      *ChangeTracker
    events       []DomainEvent
}

// Business method (pure logic)
func (p *Product) ApplyDiscount(discount *Discount, now time.Time) error {
    if p.status != ProductStatusActive {
        return ErrProductNotActive
    }

    if !discount.IsValidAt(now) {
        return ErrInvalidDiscountPeriod
    }

    p.discount = discount
    p.changes.MarkDirty(FieldDiscount)
    p.events = append(p.events, &DiscountAppliedEvent{...})
    return nil
}
```

### 2. CQRS Pattern

**Commands (Write Operations):**
- MUST go through domain aggregate
- Use CommitPlan for atomic transactions
- Return `error` only (or minimal reply with created IDs)
- Never bypass domain logic

**Queries (Read Operations):**
- MAY bypass domain for optimization
- Use read models and DTOs
- Direct database access via read model interface
- No mutations or side effects

### 3. The Golden Mutation Pattern

Every write operation must follow this exact pattern:

```go
func (it *CreateProductInteractor) Execute(ctx context.Context, req Request) (string, error) {
    // 1. Create or load aggregate
    product := domain.NewProduct(req.Name, req.Description, req.Category, basePrice, it.clock.Now())

    // 2. Domain validation (already done in constructor/methods)

    // 3. Build commit plan
    plan := commitplan.NewPlan()

    // 4. Get mutations from repository (repo returns, doesn't apply)
    if mut := it.repo.InsertMut(product); mut != nil {
        plan.Add(mut)
    }

    // 5. Add outbox events
    for _, event := range product.DomainEvents() {
        outboxMut := it.outboxRepo.InsertMut(enrichEvent(event))
        if outboxMut != nil {
            plan.Add(outboxMut)
        }
    }

    // 6. Apply plan (usecase applies, NOT handler!)
    if err := it.committer.Apply(ctx, plan); err != nil {
        return "", err
    }

    return product.ID(), nil
}
```

### 4. Repository Pattern

Repositories must:
- Return mutations, NEVER apply them
- Read change tracker to build targeted updates
- Map domain entities to database models
- Use generated model facades for type safety

```go
func (r *ProductRepo) UpdateMut(p *domain.Product) *spanner.Mutation {
    updates := make(map[string]interface{})

    if p.Changes().Dirty(domain.FieldName) {
        updates[m_product.Name] = p.Name()
    }

    if p.Changes().Dirty(domain.FieldDiscount) {
        if d := p.Discount(); d != nil {
            updates[m_product.DiscountPercent] = d.Percentage()
            updates[m_product.DiscountStartDate] = d.StartDate()
            updates[m_product.DiscountEndDate] = d.EndDate()
        }
    }

    if len(updates) == 0 {
        return nil // No changes
    }

    updates[m_product.UpdatedAt] = time.Now()
    return r.model.UpdateMut(p.ID(), updates)
}
```

### 5. gRPC Handler Pattern

Handlers are thin and follow this structure:

```go
func (h *ProductHandler) CreateProduct(ctx context.Context, req *pb.CreateProductRequest) (*pb.CreateProductReply, error) {
    // 1. Validate proto request
    if err := validateCreateRequest(req); err != nil {
        return nil, status.Error(codes.InvalidArgument, err.Error())
    }

    // 2. Map proto to application request
    appReq := mapToCreateProductRequest(req)

    // 3. Call usecase (usecase applies plan internally)
    productID, err := h.commands.CreateProduct.Execute(ctx, appReq)
    if err != nil {
        return nil, mapDomainErrorToGRPC(err)
    }

    // 4. Return response
    return &pb.CreateProductReply{
        ProductId: productID,
    }, nil
}
```

### 6. Transactional Outbox Pattern

- Domain captures simple event intents
- Usecases enrich events with metadata (company_id, user_id, etc.)
- Events stored in outbox table within same transaction
- Use `m_outbox` model with fields: id, event_type, payload, status, created_at

---

## Database Schema

Create the following Spanner tables:

```sql
CREATE TABLE products (
    product_id STRING(36) NOT NULL,
    name STRING(255) NOT NULL,
    description STRING(MAX),
    category STRING(100) NOT NULL,
    base_price_numerator INT64 NOT NULL,
    base_price_denominator INT64 NOT NULL,
    discount_percent NUMERIC,
    discount_start_date TIMESTAMP,
    discount_end_date TIMESTAMP,
    status STRING(20) NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    archived_at TIMESTAMP,
) PRIMARY KEY (product_id);

CREATE TABLE outbox_events (
    event_id STRING(36) NOT NULL,
    event_type STRING(100) NOT NULL,
    aggregate_id STRING(36) NOT NULL,
    payload JSON NOT NULL,
    status STRING(20) NOT NULL,
    created_at TIMESTAMP NOT NULL,
    processed_at TIMESTAMP,
) PRIMARY KEY (event_id);

CREATE INDEX idx_outbox_status ON outbox_events(status, created_at);
CREATE INDEX idx_products_category ON products(category, status);
```

---

## Required API Endpoints (gRPC)

Define the following in your proto file:

```protobuf
service ProductService {
    // Commands
    rpc CreateProduct(CreateProductRequest) returns (CreateProductReply);
    rpc UpdateProduct(UpdateProductRequest) returns (UpdateProductReply);
    rpc ActivateProduct(ActivateProductRequest) returns (ActivateProductReply);
    rpc DeactivateProduct(DeactivateProductRequest) returns (DeactivateProductReply);
    rpc ApplyDiscount(ApplyDiscountRequest) returns (ApplyDiscountReply);
    rpc RemoveDiscount(RemoveDiscountRequest) returns (RemoveDiscountReply);

    // Queries
    rpc GetProduct(GetProductRequest) returns (GetProductReply);
    rpc ListProducts(ListProductsRequest) returns (ListProductsReply);
}
```

---

## Required Domain Events

Your domain should capture these events:

1. `ProductCreatedEvent` - When product is created
2. `ProductUpdatedEvent` - When product details change
3. `ProductActivatedEvent` - When product is activated
4. `ProductDeactivatedEvent` - When product is deactivated
5. `DiscountAppliedEvent` - When discount is added
6. `DiscountRemovedEvent` - When discount is removed

---

## Testing Requirements

### E2E Tests (Required)

Write E2E tests that test usecases directly (no gRPC calls needed):

```go
func TestProductCreationFlow(t *testing.T) {
    // Setup: Real Spanner connection (emulator), real repos, real usecases

    // Test: Create product
    productID, err := createProductUsecase.Execute(ctx, request)
    require.NoError(t, err)

    // Verify: Query returns correct data
    product, err := getProductQuery.Execute(ctx, productID)
    require.NoError(t, err)
    assert.Equal(t, "Test Product", product.Name)

    // Verify: Outbox event was created
    events := getOutboxEvents(t, db, productID)
    require.Len(t, events, 1)
    assert.Equal(t, "product.created", events[0].EventType)
}

func TestDiscountApplicationFlow(t *testing.T) {
    // Setup & create product

    // Test: Apply discount
    err := applyDiscountUsecase.Execute(ctx, ApplyDiscountRequest{...})
    require.NoError(t, err)

    // Verify: Effective price is calculated correctly
    product, _ := getProductQuery.Execute(ctx, productID)
    assert.Equal(t, expectedDiscountedPrice, product.EffectivePrice)
}

func TestBusinessRuleValidation(t *testing.T) {
    // Test: Cannot apply discount to inactive product
    err := applyDiscountUsecase.Execute(ctx, request)
    assert.ErrorIs(t, err, domain.ErrProductNotActive)
}
```

Required test scenarios:
- Product creation flow
- Product update flow
- Discount application with price calculation
- Product activation/deactivation
- Business rule validation (errors)
- Concurrent updates (optimistic locking if implemented)
- Outbox event creation

### Unit Tests (Recommended)

Test domain logic in isolation:
- Money calculations
- Discount validation
- Pricing calculator domain service
- State machine transitions

---

## Evaluation Criteria

You will be evaluated on:

### Architecture & Design (35%)
- ✅ Clean separation of domain/application/infrastructure layers
- ✅ Domain purity (no external dependencies in domain)
- ✅ Proper aggregate boundaries and encapsulation
- ✅ CQRS separation (commands vs queries)
- ✅ Repository pattern implementation

### Pattern Implementation (30%)
- ✅ Golden Mutation Pattern correctly applied
- ✅ CommitPlan usage (atomic transactions)
- ✅ Repository returns mutations (doesn't apply)
- ✅ Usecases apply plans (handlers don't)
- ✅ Transactional outbox for events
- ✅ Change tracking for optimized updates

### Code Quality (20%)
- ✅ Idiomatic Go code
- ✅ Proper error handling
- ✅ Clear naming and structure
- ✅ Minimal public APIs
- ✅ No over-engineering
- ✅ Code comments where needed

### Testing (15%)
- ✅ E2E tests cover main flows
- ✅ Tests verify business rules
- ✅ Tests check side effects (outbox events)
- ✅ Proper test setup/teardown
- ✅ Clear test names and assertions

---

## Submission Guidelines

1. **Repository Structure**
   - Follow the exact structure outlined above
   - Include `docker-compose.yml` for Spanner emulator
   - Include clear README with setup instructions

2. **README Must Include**
   - How to start Spanner emulator
   - How to run migrations
   - How to run tests
   - How to start the gRPC server
   - Design decisions and trade-offs

3. **Running Instructions**
   ```bash
   # Start Spanner emulator
   docker-compose up -d

   # Run migrations
   make migrate

   # Run tests
   make test

   # Start server
   make run
   ```

4. **What NOT to Implement**
   - Authentication/authorization
   - Background outbox processor (just store events)
   - Actual Pub/Sub publishing
   - Metrics/monitoring beyond basic logging
   - API gateway or REST endpoints

---

## Hints & Tips

1. **Start with Domain**
   - Define aggregates, value objects, and domain services first
   - Write domain tests to validate business logic
   - Only then move to infrastructure

2. **Use the Pattern**
   - Every write operation: Load → Call domain → Build plan → Apply
   - Don't deviate from this pattern

3. **Money Handling**
   ```go
   // Store as numerator/denominator
   price := big.NewRat(1999, 100) // $19.99

   // Calculate discount
   discount := new(big.Rat).Mul(price, big.NewRat(20, 100)) // 20% off
   finalPrice := new(big.Rat).Sub(price, discount)
   ```

4. **Change Tracking**
   ```go
   type ChangeTracker struct {
       dirtyFields map[string]bool
   }

   func (ct *ChangeTracker) MarkDirty(field string) {
       ct.dirtyFields[field] = true
   }

   func (ct *ChangeTracker) Dirty(field string) bool {
       return ct.dirtyFields[field]
   }
   ```

5. **Testing with Emulator**
   ```yaml
   # docker-compose.yml
   version: '3.8'
   services:
     spanner-emulator:
       image: gcr.io/cloud-spanner-emulator/emulator
       ports:
         - "9010:9010"
         - "9020:9020"
   ```

---

## Questions?

If you have questions about requirements or architecture:
1. Check the tech-docs repository (especially service-architecture/ and ddd/)
2. Refer to the Golden Mutation Pattern documentation
3. Ask for clarification - assumptions should be documented in README

---

## Good Luck!

This test task reflects real production patterns we use daily. Completing it demonstrates:
- Understanding of DDD and Clean Architecture
- Ability to work with distributed systems patterns
- Proficiency with Go and cloud-native technologies
- Attention to detail and code quality

We're looking for engineers who can write maintainable, well-architected code that follows established patterns. Quality over speed!