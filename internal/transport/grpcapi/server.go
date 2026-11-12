// Package grpcapi is the gRPC transport. It translates protobuf requests
// into use-case calls and domain errors into gRPC status codes, and holds no
// business logic of its own.
package grpcapi

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mzubairashraf/product-catalog-service/internal/app/product/contracts"
	"github.com/mzubairashraf/product-catalog-service/internal/app/product/domain"
	"github.com/mzubairashraf/product-catalog-service/internal/app/product/queries/get_product"
	"github.com/mzubairashraf/product-catalog-service/internal/app/product/queries/list_products"
	"github.com/mzubairashraf/product-catalog-service/internal/app/product/usecases/activate_product"
	"github.com/mzubairashraf/product-catalog-service/internal/app/product/usecases/apply_discount"
	"github.com/mzubairashraf/product-catalog-service/internal/app/product/usecases/create_product"
	"github.com/mzubairashraf/product-catalog-service/internal/app/product/usecases/deactivate_product"
	"github.com/mzubairashraf/product-catalog-service/internal/app/product/usecases/remove_discount"
	"github.com/mzubairashraf/product-catalog-service/internal/app/product/usecases/update_product"
	productpb "github.com/mzubairashraf/product-catalog-service/proto/product/v1"
)

const (
	defaultPageSize = 50
	maxPageSize     = 200
)

// Application is the set of use cases the transport exposes. Each field is
// the narrow interface the handler needs, so tests can substitute any of them.
type Application struct {
	CreateProduct interface {
		Execute(context.Context, create_product.Request) (*create_product.Response, error)
	}
	UpdateProduct interface {
		Execute(context.Context, update_product.Request) error
	}
	ActivateProduct interface {
		Execute(context.Context, activate_product.Request) error
	}
	DeactivateProduct interface {
		Execute(context.Context, deactivate_product.Request) error
	}
	ApplyDiscount interface {
		Execute(context.Context, apply_discount.Request) error
	}
	RemoveDiscount interface {
		Execute(context.Context, remove_discount.Request) error
	}
	GetProduct interface {
		Execute(context.Context, get_product.Request) (*contracts.ProductDTO, error)
	}
	ListProducts interface {
		Execute(context.Context, list_products.Request) (*contracts.ProductList, error)
	}
}

// Server implements productpb.ProductServiceServer.
type Server struct {
	productpb.UnimplementedProductServiceServer
	app    Application
	logger *slog.Logger
}

// NewServer returns the gRPC service implementation.
func NewServer(app Application, logger *slog.Logger) *Server {
	return &Server{app: app, logger: orDiscard(logger)}
}

// ---- commands ----

func (s *Server) CreateProduct(ctx context.Context, req *productpb.CreateProductRequest) (*productpb.CreateProductReply, error) {
	price, err := domain.NewMoney(req.GetPriceNumerator(), req.GetPriceDenominator())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "price_numerator and price_denominator must both be positive")
	}
	resp, err := s.app.CreateProduct.Execute(ctx, create_product.Request{
		Name: req.GetName(), Description: req.GetDescription(), Category: req.GetCategory(), BasePrice: price,
	})
	if err != nil {
		return nil, s.toStatus(ctx, err)
	}
	return &productpb.CreateProductReply{ProductId: resp.ProductID}, nil
}

func (s *Server) UpdateProduct(ctx context.Context, req *productpb.UpdateProductRequest) (*productpb.UpdateProductReply, error) {
	if err := validateID(req.GetProductId()); err != nil {
		return nil, err
	}
	err := s.app.UpdateProduct.Execute(ctx, update_product.Request{
		ProductID: req.GetProductId(), Name: req.GetName(), Description: req.GetDescription(), Category: req.GetCategory(),
	})
	if err != nil {
		return nil, s.toStatus(ctx, err)
	}
	return &productpb.UpdateProductReply{}, nil
}

func (s *Server) ActivateProduct(ctx context.Context, req *productpb.ActivateProductRequest) (*productpb.ActivateProductReply, error) {
	if err := validateID(req.GetProductId()); err != nil {
		return nil, err
	}
	if err := s.app.ActivateProduct.Execute(ctx, activate_product.Request{ProductID: req.GetProductId()}); err != nil {
		return nil, s.toStatus(ctx, err)
	}
	return &productpb.ActivateProductReply{}, nil
}

func (s *Server) DeactivateProduct(ctx context.Context, req *productpb.DeactivateProductRequest) (*productpb.DeactivateProductReply, error) {
	if err := validateID(req.GetProductId()); err != nil {
		return nil, err
	}
	if err := s.app.DeactivateProduct.Execute(ctx, deactivate_product.Request{ProductID: req.GetProductId()}); err != nil {
		return nil, s.toStatus(ctx, err)
	}
	return &productpb.DeactivateProductReply{}, nil
}

func (s *Server) ApplyDiscount(ctx context.Context, req *productpb.ApplyDiscountRequest) (*productpb.ApplyDiscountReply, error) {
	if err := validateID(req.GetProductId()); err != nil {
		return nil, err
	}
	if req.GetStartDateUnix() <= 0 || req.GetEndDateUnix() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "start_date_unix and end_date_unix are required")
	}
	err := s.app.ApplyDiscount.Execute(ctx, apply_discount.Request{
		ProductID:       req.GetProductId(),
		DiscountPercent: int(req.GetDiscountPercent()),
		StartDate:       time.Unix(req.GetStartDateUnix(), 0).UTC(),
		EndDate:         time.Unix(req.GetEndDateUnix(), 0).UTC(),
	})
	if err != nil {
		return nil, s.toStatus(ctx, err)
	}
	return &productpb.ApplyDiscountReply{}, nil
}

func (s *Server) RemoveDiscount(ctx context.Context, req *productpb.RemoveDiscountRequest) (*productpb.RemoveDiscountReply, error) {
	if err := validateID(req.GetProductId()); err != nil {
		return nil, err
	}
	if err := s.app.RemoveDiscount.Execute(ctx, remove_discount.Request{ProductID: req.GetProductId()}); err != nil {
		return nil, s.toStatus(ctx, err)
	}
	return &productpb.RemoveDiscountReply{}, nil
}

// ---- queries ----

func (s *Server) GetProduct(ctx context.Context, req *productpb.GetProductRequest) (*productpb.GetProductReply, error) {
	if err := validateID(req.GetProductId()); err != nil {
		return nil, err
	}
	dto, err := s.app.GetProduct.Execute(ctx, get_product.Request{ProductID: req.GetProductId()})
	if err != nil {
		return nil, s.toStatus(ctx, err)
	}
	return &productpb.GetProductReply{Product: toProto(dto)}, nil
}

func (s *Server) ListProducts(ctx context.Context, req *productpb.ListProductsRequest) (*productpb.ListProductsReply, error) {
	limit := int(req.GetLimit())
	switch {
	case limit == 0:
		limit = defaultPageSize
	case limit < 0 || limit > maxPageSize:
		return nil, status.Errorf(codes.InvalidArgument, "limit must be between 1 and %d", maxPageSize)
	}
	if req.GetOffset() < 0 {
		return nil, status.Error(codes.InvalidArgument, "offset must not be negative")
	}
	switch domain.ProductStatus(req.GetStatus()) {
	case "", domain.ProductStatusActive, domain.ProductStatusInactive, domain.ProductStatusArchived:
	default:
		return nil, status.Error(codes.InvalidArgument, "status must be one of: active, inactive, archived")
	}

	list, err := s.app.ListProducts.Execute(ctx, list_products.Request{
		Category: req.GetCategory(), Status: req.GetStatus(), Limit: limit, Offset: int(req.GetOffset()),
	})
	if err != nil {
		return nil, s.toStatus(ctx, err)
	}
	reply := &productpb.ListProductsReply{HasMore: list.HasMore, Products: make([]*productpb.Product, 0, len(list.Products))}
	for _, dto := range list.Products {
		reply.Products = append(reply.Products, toProto(dto))
	}
	return reply, nil
}

// ---- mapping ----

func validateID(id string) error {
	if id == "" {
		return status.Error(codes.InvalidArgument, "product_id is required")
	}
	if _, err := uuid.Parse(id); err != nil {
		return status.Error(codes.InvalidArgument, "product_id must be a UUID")
	}
	return nil
}

// toStatus maps a use-case error to a gRPC status.
//
//   - a missing aggregate is NotFound;
//   - input the domain rejects is InvalidArgument;
//   - a valid request that the aggregate's current state forbids is
//     FailedPrecondition, which tells the client not to retry unchanged;
//   - anything else is Internal. Its detail is logged, not returned, so
//     storage errors never leak to callers.
func (s *Server) toStatus(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, domain.ErrProductNotFound):
		return status.Error(codes.NotFound, domain.ErrProductNotFound.Error())
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request cancelled")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "deadline exceeded")
	}
	for _, invalid := range []error{
		domain.ErrInvalidProductName, domain.ErrInvalidCategory, domain.ErrInvalidPrice,
		domain.ErrDiscountPercentInvalid, domain.ErrInvalidDiscountPeriod,
	} {
		if errors.Is(err, invalid) {
			return status.Error(codes.InvalidArgument, invalid.Error())
		}
	}
	for _, precondition := range []error{
		domain.ErrProductNotActive, domain.ErrProductAlreadyActive, domain.ErrProductAlreadyArchived,
		domain.ErrDiscountAlreadyActive, domain.ErrNoActiveDiscount,
	} {
		if errors.Is(err, precondition) {
			return status.Error(codes.FailedPrecondition, precondition.Error())
		}
	}
	s.logger.ErrorContext(ctx, "unhandled error", slog.String("error", err.Error()))
	return status.Error(codes.Internal, "internal error")
}

func toProto(dto *contracts.ProductDTO) *productpb.Product {
	p := &productpb.Product{
		ProductId:      dto.ProductID,
		Name:           dto.Name,
		Description:    dto.Description,
		Category:       dto.Category,
		BasePrice:      moneyToProto(dto.BasePrice),
		EffectivePrice: moneyToProto(dto.EffectivePrice),
		Status:         dto.Status,
		CreatedAtUnix:  dto.CreatedAt.Unix(),
		UpdatedAtUnix:  dto.UpdatedAt.Unix(),
	}
	if d := dto.Discount; d != nil {
		p.Discount = &productpb.Discount{
			Percentage:    int32(d.Percentage),
			StartDateUnix: d.StartDate.Unix(),
			EndDateUnix:   d.EndDate.Unix(),
			IsActive:      d.IsActive,
		}
	}
	return p
}

func moneyToProto(m contracts.MoneyDTO) *productpb.Money {
	return &productpb.Money{Numerator: m.Numerator, Denominator: m.Denominator, DisplayValue: m.Float64Value}
}
