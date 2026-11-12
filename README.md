# product-catalog-service

[![CI](https://github.com/mzubairashraf/product-catalog-service/actions/workflows/ci.yml/badge.svg)](https://github.com/mzubairashraf/product-catalog-service/actions/workflows/ci.yml)

A gRPC service in Go that manages a product catalogue on Google Cloud Spanner: products, their lifecycle, and time-bound percentage discounts. It is a compact worked example of a domain model kept free of infrastructure, CQRS, and the transactional outbox pattern, with every state change and its event committed in one Spanner transaction.

About 2,900 lines of Go plus 1,500 lines of tests. The end-to-end tests run against the Spanner emulator in CI.

## Quick start

Requires Go 1.26 or newer and Docker.

```bash
git clone https://github.com/mzubairashraf/product-catalog-service && cd product-catalog-service
make test     # unit and transport tests, no Docker needed
make e2e      # starts the Spanner emulator and runs the end-to-end tests
make run      # creates the database on the emulator and serves gRPC on 127.0.0.1:50051
```

With the server running, call it with [grpcurl](https://github.com/fullstorydev/grpcurl) (server reflection is enabled, so no `.proto` is needed):

```bash
grpcurl -plaintext -d '{"name":"Mechanical Keyboard","category":"Electronics","price_numerator":1999,"price_denominator":100}' \
  127.0.0.1:50051 product.v1.ProductService/CreateProduct
```

```json
{ "productId": "d2be17e6-9005-4740-9554-6cf01f73f590" }
```

After `ActivateProduct` and a 15% `ApplyDiscount`, `GetProduct` returns the price as an exact fraction:

```json
"basePrice":      { "numerator": "1999",  "denominator": "100",  "displayValue": 19.99 },
"effectivePrice": { "numerator": "33983", "denominator": "2000", "displayValue": 16.9915 }
```

## API

Defined in [proto/product/v1/product_service.proto](proto/product/v1/product_service.proto).

| RPC | Kind | Behaviour |
| --- | --- | --- |
| `CreateProduct` | command | Creates a product in the `inactive` state. |
| `UpdateProduct` | command | Changes name, description and category. |
| `ActivateProduct` / `DeactivateProduct` | command | Moves between `inactive` and `active`. |
| `ApplyDiscount` | command | Adds a percentage discount (1 to 100) for a period that includes now. Requires an active product with no active discount. |
| `RemoveDiscount` | command | Removes the current discount. |
| `GetProduct` | query | Returns the product with its effective price at the time of the call. |
| `ListProducts` | query | Filter by category and status; `limit` (default 50, maximum 200) and `offset`; returns `has_more`. |

Errors use standard gRPC codes, chosen so a client knows what to do without parsing the message:

| Code | When |
| --- | --- |
| `InvalidArgument` | The request is malformed or breaks a domain rule on its own (empty name, discount of 150%). |
| `FailedPrecondition` | The request is valid but the product's current state forbids it (activating an active product, a second discount). Do not retry unchanged. |
| `NotFound` | No product with that ID. |
| `Internal` | Anything else. The detail is logged on the server and not returned to the caller. |

## Design

```
transport/grpcapi   protobuf <-> use cases, error -> status code
app/product/usecases, queries   one package per command or query
app/product/domain  Product aggregate, Money, Discount, events (standard library only)
app/product/repo    Spanner repository, read model, outbox
```

**The domain has no infrastructure imports.** `Product` is an aggregate with unexported fields; state changes only through methods (`Activate`, `ApplyDiscount`, ...) that enforce the rules and record a domain event. It can be tested without a database, and is.

**Repositories return mutations; they do not write.** A use case loads the aggregate, calls one domain method, then collects the aggregate's mutation and one outbox mutation per event into a single plan, which is applied in one Spanner transaction. Either the state change and its event are both stored, or neither is. That is the transactional outbox: a relay can later publish `outbox_events` rows to a broker with at-least-once delivery, without a distributed transaction.

**Writes and reads are separate paths (CQRS).** Commands go through the aggregate. Queries read rows straight into DTOs through a read model and never construct an aggregate.

**Money is a rational number.** Prices are stored as numerator and denominator and computed with `math/big.Rat`, on both the write path and the read path. 19.99 less 15% is exactly 33983/2000; no float is involved until the optional `display_value`.

**Change tracking.** The aggregate records which fields changed, and the repository updates only those columns.

**Discount periods are half-open,** `[start, end)`, so two back-to-back discounts never both apply at the boundary instant.

## Testing

| Layer | What it covers | Needs |
| --- | --- | --- |
| Domain | Aggregate rules, money arithmetic, discount periods, change tracking | nothing |
| Use cases | One table over all six commands: each success commits exactly one plan holding the aggregate mutation and its event; each failure commits nothing | in-memory fakes |
| Transport | Real gRPC client and server over an in-memory connection: every status code in the table above, opaque `Internal` errors, panic recovery, paging defaults | in-memory fakes |
| End to end | The public API against Spanner with the real schema from `migrations/`: full lifecycle, exact prices, list filters and paging, and the contents of `outbox_events` | Spanner emulator |

The end-to-end tests use the same wiring (`internal/bootstrap`) as the server binary, and are skipped when `SPANNER_EMULATOR_HOST` is not set.

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `SPANNER_DATABASE` | required | `projects/P/instances/I/databases/D` |
| `SPANNER_EMULATOR_HOST` | unset | Set to `localhost:9010` to use the emulator; unset to use Cloud Spanner with application default credentials. |
| `GRPC_ADDR` | `127.0.0.1:50051` | Listen address. |

`go run ./cmd/migrate` creates the database from `migrations/`; on the emulator it also creates the instance.

## Not included

- The outbox relay. Events are written transactionally but nothing publishes them yet.
- Authentication and TLS on the gRPC listener; in a deployment these would come from a mesh or gateway, or from interceptors.
- An `ArchiveProduct` RPC. The aggregate supports archiving and the rule is tested, but it is not exposed.
- Scheduling a discount for the future: a discount must be active when it is applied.
- Offset paging is simple but reads every skipped row; a large catalogue would use a keyset cursor.

## Licence

MIT. See [LICENSE](LICENSE).
