package grpcapi_test

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/mzubairashraf/product-catalog-service/internal/app/product/contracts"
	"github.com/mzubairashraf/product-catalog-service/internal/app/product/domain"
	"github.com/mzubairashraf/product-catalog-service/internal/app/product/producttest"
	"github.com/mzubairashraf/product-catalog-service/internal/app/product/queries/get_product"
	"github.com/mzubairashraf/product-catalog-service/internal/app/product/queries/list_products"
	"github.com/mzubairashraf/product-catalog-service/internal/app/product/usecases/activate_product"
	"github.com/mzubairashraf/product-catalog-service/internal/app/product/usecases/apply_discount"
	"github.com/mzubairashraf/product-catalog-service/internal/app/product/usecases/create_product"
	"github.com/mzubairashraf/product-catalog-service/internal/app/product/usecases/deactivate_product"
	"github.com/mzubairashraf/product-catalog-service/internal/app/product/usecases/remove_discount"
	"github.com/mzubairashraf/product-catalog-service/internal/app/product/usecases/update_product"
	"github.com/mzubairashraf/product-catalog-service/internal/transport/grpcapi"
	productpb "github.com/mzubairashraf/product-catalog-service/proto/product/v1"
)

const (
	activeID     = "11111111-1111-4111-8111-111111111111"
	inactiveID   = "22222222-2222-4222-8222-222222222222"
	discountedID = "33333333-3333-4333-8333-333333333333"
	missingID    = "99999999-9999-4999-8999-999999999999"
)

// readModel is a stub contracts.ReadModel that records the last list request.
type readModel struct {
	product  *contracts.ProductDTO
	list     *contracts.ProductList
	lastList contracts.ListProductsRequest
	panics   bool
}

func (r *readModel) GetProduct(_ context.Context, id string) (*contracts.ProductDTO, error) {
	if r.panics {
		panic("boom")
	}
	if r.product == nil || r.product.ProductID != id {
		return nil, domain.ErrProductNotFound
	}
	return r.product, nil
}

func (r *readModel) ListProducts(_ context.Context, req contracts.ListProductsRequest) (*contracts.ProductList, error) {
	r.lastList = req
	return r.list, nil
}

type harness struct {
	client    productpb.ProductServiceClient
	repo      *producttest.Repo
	committer *producttest.Committer
	read      *readModel
}

// newHarness serves the real handlers and interactors over an in-memory
// gRPC connection, with fakes below the use cases.
func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{
		repo: producttest.NewRepo(
			producttest.Product(activeID, domain.ProductStatusActive, nil),
			producttest.Product(inactiveID, domain.ProductStatusInactive, nil),
			producttest.Product(discountedID, domain.ProductStatusActive, producttest.Discount(10)),
		),
		committer: &producttest.Committer{},
		read:      &readModel{list: &contracts.ProductList{}},
	}
	outbox, clk := &producttest.Outbox{}, producttest.Clock(producttest.Now)
	app := grpcapi.Application{
		CreateProduct:     create_product.NewInteractor(h.repo, outbox, h.committer, clk),
		UpdateProduct:     update_product.NewInteractor(h.repo, outbox, h.committer, clk),
		ActivateProduct:   activate_product.NewInteractor(h.repo, outbox, h.committer, clk),
		DeactivateProduct: deactivate_product.NewInteractor(h.repo, outbox, h.committer, clk),
		ApplyDiscount:     apply_discount.NewInteractor(h.repo, outbox, h.committer, clk),
		RemoveDiscount:    remove_discount.NewInteractor(h.repo, outbox, h.committer, clk),
		GetProduct:        get_product.NewQuery(h.read),
		ListProducts:      list_products.NewQuery(h.read),
	}

	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer(grpc.ChainUnaryInterceptor(grpcapi.Recovery(nil), grpcapi.Logging(nil)))
	productpb.RegisterProductServiceServer(server, grpcapi.NewServer(app, nil))
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	h.client = productpb.NewProductServiceClient(conn)
	return h
}

func TestStatusCodes(t *testing.T) {
	ctx := context.Background()
	start, end := producttest.Now.Add(-time.Hour).Unix(), producttest.Now.Add(time.Hour).Unix()
	create := func(name, category string, num, den int64) func(productpb.ProductServiceClient) error {
		return func(c productpb.ProductServiceClient) error {
			_, err := c.CreateProduct(ctx, &productpb.CreateProductRequest{Name: name, Category: category, PriceNumerator: num, PriceDenominator: den})
			return err
		}
	}
	discount := func(id string, percent int32, start, end int64) func(productpb.ProductServiceClient) error {
		return func(c productpb.ProductServiceClient) error {
			_, err := c.ApplyDiscount(ctx, &productpb.ApplyDiscountRequest{ProductId: id, DiscountPercent: percent, StartDateUnix: start, EndDateUnix: end})
			return err
		}
	}
	activate := func(id string) func(productpb.ProductServiceClient) error {
		return func(c productpb.ProductServiceClient) error {
			_, err := c.ActivateProduct(ctx, &productpb.ActivateProductRequest{ProductId: id})
			return err
		}
	}
	list := func(req *productpb.ListProductsRequest) func(productpb.ProductServiceClient) error {
		return func(c productpb.ProductServiceClient) error { _, err := c.ListProducts(ctx, req); return err }
	}

	tests := []struct {
		name        string
		call        func(productpb.ProductServiceClient) error
		want        codes.Code
		wantCommits int
	}{
		// Success.
		{"create", create("Widget", "Tools", 1999, 100), codes.OK, 1},
		{"activate", activate(inactiveID), codes.OK, 1},
		{"apply discount", discount(activeID, 20, start, end), codes.OK, 1},
		{"update", func(c productpb.ProductServiceClient) error {
			_, err := c.UpdateProduct(ctx, &productpb.UpdateProductRequest{ProductId: activeID, Name: "Gadget", Category: "Toys"})
			return err
		}, codes.OK, 1},
		{"deactivate", func(c productpb.ProductServiceClient) error {
			_, err := c.DeactivateProduct(ctx, &productpb.DeactivateProductRequest{ProductId: activeID})
			return err
		}, codes.OK, 1},
		{"remove discount", func(c productpb.ProductServiceClient) error {
			_, err := c.RemoveDiscount(ctx, &productpb.RemoveDiscountRequest{ProductId: discountedID})
			return err
		}, codes.OK, 1},

		// Rejected by the transport before any use case runs.
		{"create with zero price", create("Widget", "Tools", 0, 100), codes.InvalidArgument, 0},
		{"create with zero denominator", create("Widget", "Tools", 1999, 0), codes.InvalidArgument, 0},
		{"empty product id", activate(""), codes.InvalidArgument, 0},
		{"malformed product id", activate("not-a-uuid"), codes.InvalidArgument, 0},
		{"discount without dates", discount(activeID, 20, 0, 0), codes.InvalidArgument, 0},
		{"list with limit over the maximum", list(&productpb.ListProductsRequest{Limit: 201}), codes.InvalidArgument, 0},
		{"list with negative limit", list(&productpb.ListProductsRequest{Limit: -1}), codes.InvalidArgument, 0},
		{"list with negative offset", list(&productpb.ListProductsRequest{Offset: -1}), codes.InvalidArgument, 0},
		{"list with unknown status", list(&productpb.ListProductsRequest{Status: "deleted"}), codes.InvalidArgument, 0},

		// Rejected by the domain: bad input.
		{"create without a name", create("  ", "Tools", 1999, 100), codes.InvalidArgument, 0},
		{"create without a category", create("Widget", "", 1999, 100), codes.InvalidArgument, 0},
		{"discount of 150 percent", discount(activeID, 150, start, end), codes.InvalidArgument, 0},
		{"discount ending before it starts", discount(activeID, 20, end, start), codes.InvalidArgument, 0},

		// Rejected by the domain: valid input, wrong state.
		{"activate an active product", activate(activeID), codes.FailedPrecondition, 0},
		{"discount on an inactive product", discount(inactiveID, 20, start, end), codes.FailedPrecondition, 0},
		{"second discount", discount(discountedID, 20, start, end), codes.FailedPrecondition, 0},
		{"remove a discount that is not there", func(c productpb.ProductServiceClient) error {
			_, err := c.RemoveDiscount(ctx, &productpb.RemoveDiscountRequest{ProductId: activeID})
			return err
		}, codes.FailedPrecondition, 0},

		// Missing.
		{"activate a missing product", activate(missingID), codes.NotFound, 0},
		{"get a missing product", func(c productpb.ProductServiceClient) error {
			_, err := c.GetProduct(ctx, &productpb.GetProductRequest{ProductId: missingID})
			return err
		}, codes.NotFound, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			err := tt.call(h.client)
			if got := status.Code(err); got != tt.want {
				t.Fatalf("code = %s (%v), want %s", got, err, tt.want)
			}
			if got := len(h.committer.Applied); got != tt.wantCommits {
				t.Errorf("commits = %d, want %d", got, tt.wantCommits)
			}
		})
	}
}

func TestInternalErrorsDoNotLeakDetail(t *testing.T) {
	h := newHarness(t)
	h.committer.Err = errors.New("spanner: rpc error: table products, instance prod-eu-1")

	_, err := h.client.ActivateProduct(context.Background(), &productpb.ActivateProductRequest{ProductId: inactiveID})
	if status.Code(err) != codes.Internal || strings.Contains(err.Error(), "prod-eu-1") || status.Convert(err).Message() != "internal error" {
		t.Errorf("err = %v, want an opaque Internal status", err)
	}
}

func TestPanicBecomesInternalStatus(t *testing.T) {
	h := newHarness(t)
	h.read.panics = true

	_, err := h.client.GetProduct(context.Background(), &productpb.GetProductRequest{ProductId: activeID})
	if status.Code(err) != codes.Internal {
		t.Fatalf("code = %s, want Internal", status.Code(err))
	}
	// The server must still be serving.
	h.read.panics = false
	if _, err := h.client.GetProduct(context.Background(), &productpb.GetProductRequest{ProductId: activeID}); status.Code(err) != codes.NotFound {
		t.Errorf("after a panic, code = %s, want NotFound from a live server", status.Code(err))
	}
}

func TestGetProductMapping(t *testing.T) {
	h := newHarness(t)
	created, start, end := producttest.Now.Add(-48*time.Hour), producttest.Now.Add(-time.Hour), producttest.Now.Add(time.Hour)
	h.read.product = &contracts.ProductDTO{
		ProductID: activeID, Name: "Widget", Description: "A widget", Category: "Tools", Status: "active",
		BasePrice:      contracts.MoneyDTO{Numerator: 1999, Denominator: 100, Float64Value: 19.99},
		EffectivePrice: contracts.MoneyDTO{Numerator: 33983, Denominator: 2000, Float64Value: 16.9915},
		Discount:       &contracts.DiscountDTO{Percentage: 15, StartDate: start, EndDate: end, IsActive: true},
		CreatedAt:      created, UpdatedAt: producttest.Now,
	}

	reply, err := h.client.GetProduct(context.Background(), &productpb.GetProductRequest{ProductId: activeID})
	if err != nil {
		t.Fatal(err)
	}
	p := reply.GetProduct()
	if p.GetProductId() != activeID || p.GetName() != "Widget" || p.GetCategory() != "Tools" || p.GetStatus() != "active" ||
		p.GetBasePrice().GetNumerator() != 1999 || p.GetBasePrice().GetDenominator() != 100 ||
		p.GetEffectivePrice().GetNumerator() != 33983 || p.GetEffectivePrice().GetDenominator() != 2000 ||
		p.GetDiscount().GetPercentage() != 15 || !p.GetDiscount().GetIsActive() ||
		p.GetDiscount().GetStartDateUnix() != start.Unix() || p.GetDiscount().GetEndDateUnix() != end.Unix() ||
		p.GetCreatedAtUnix() != created.Unix() || p.GetUpdatedAtUnix() != producttest.Now.Unix() {
		t.Errorf("product = %v", p)
	}
}

func TestListProductsPaging(t *testing.T) {
	h := newHarness(t)
	h.read.list = &contracts.ProductList{HasMore: true, Products: []*contracts.ProductDTO{{ProductID: activeID}, {ProductID: inactiveID}}}

	reply, err := h.client.ListProducts(context.Background(), &productpb.ListProductsRequest{Category: "Tools", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	if len(reply.GetProducts()) != 2 || !reply.GetHasMore() {
		t.Errorf("reply = %v", reply)
	}
	if got := h.read.lastList; got.Limit != 50 || got.Offset != 0 || got.Category != "Tools" || got.Status != "active" {
		t.Errorf("an omitted limit must default to 50; read model got %+v", got)
	}

	if _, err := h.client.ListProducts(context.Background(), &productpb.ListProductsRequest{Limit: 200, Offset: 400}); err != nil {
		t.Fatal(err)
	}
	if got := h.read.lastList; got.Limit != 200 || got.Offset != 400 {
		t.Errorf("read model got %+v", got)
	}
}
