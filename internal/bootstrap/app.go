// Package bootstrap wires the concrete Spanner implementations into the use
// cases. It is the only place that knows every layer, and it is shared by
// the server binary and the end-to-end tests so both run the same graph.
package bootstrap

import (
	"cloud.google.com/go/spanner"

	"github.com/mzubairashraf/product-catalog-service/internal/app/product/queries/get_product"
	"github.com/mzubairashraf/product-catalog-service/internal/app/product/queries/list_products"
	"github.com/mzubairashraf/product-catalog-service/internal/app/product/repo"
	"github.com/mzubairashraf/product-catalog-service/internal/app/product/usecases/activate_product"
	"github.com/mzubairashraf/product-catalog-service/internal/app/product/usecases/apply_discount"
	"github.com/mzubairashraf/product-catalog-service/internal/app/product/usecases/create_product"
	"github.com/mzubairashraf/product-catalog-service/internal/app/product/usecases/deactivate_product"
	"github.com/mzubairashraf/product-catalog-service/internal/app/product/usecases/remove_discount"
	"github.com/mzubairashraf/product-catalog-service/internal/app/product/usecases/update_product"
	"github.com/mzubairashraf/product-catalog-service/internal/pkg/clock"
	"github.com/mzubairashraf/product-catalog-service/internal/pkg/committer"
	"github.com/mzubairashraf/product-catalog-service/internal/transport/grpcapi"
)

// NewApplication builds the use cases on top of a Spanner client.
func NewApplication(client *spanner.Client, clk clock.Clock) grpcapi.Application {
	products := repo.NewProductRepo(client)
	outbox := repo.NewOutboxRepo()
	commit := committer.NewSpannerCommitter(client)
	readModel := repo.NewProductReadModel(client, clk)

	return grpcapi.Application{
		CreateProduct:     create_product.NewInteractor(products, outbox, commit, clk),
		UpdateProduct:     update_product.NewInteractor(products, outbox, commit, clk),
		ActivateProduct:   activate_product.NewInteractor(products, outbox, commit, clk),
		DeactivateProduct: deactivate_product.NewInteractor(products, outbox, commit, clk),
		ApplyDiscount:     apply_discount.NewInteractor(products, outbox, commit, clk),
		RemoveDiscount:    remove_discount.NewInteractor(products, outbox, commit, clk),
		GetProduct:        get_product.NewQuery(readModel),
		ListProducts:      list_products.NewQuery(readModel),
	}
}
