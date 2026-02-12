package repo

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/spanner"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"

	"product-catalog-service/internal/app/product/contracts"
	"product-catalog-service/internal/app/product/domain"
	"product-catalog-service/internal/models/m_product"
)

// ProductReadModel implements optimized read operations
// This bypasses the domain layer for query performance
type ProductReadModel struct {
	client *spanner.Client
}

// NewProductReadModel creates a new read model
func NewProductReadModel(client *spanner.Client) *ProductReadModel {
	return &ProductReadModel{
		client: client,
	}
}

// GetProduct retrieves a product with its effective price
func (rm *ProductReadModel) GetProduct(ctx context.Context, productID string) (*contracts.ProductDTO, error) {
	row, err := rm.client.Single().ReadRow(
		ctx,
		m_product.TableName,
		spanner.Key{productID},
		m_product.AllColumns(),
	)
	if err != nil {
		if spanner.ErrCode(err) == codes.NotFound {
			return nil, domain.ErrProductNotFound
		}
		return nil, fmt.Errorf("failed to read product: %w", err)
	}

	data := &m_product.Data{}
	if err := row.ToStruct(data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal product: %w", err)
	}

	return rm.dataToDTO(data, time.Now()), nil
}

// ListProducts retrieves products with pagination and filtering
func (rm *ProductReadModel) ListProducts(ctx context.Context, req contracts.ListProductsRequest) (*contracts.ProductList, error) {
	// Build query with filters
	query := `SELECT ` + buildColumnList() + ` FROM ` + m_product.TableName + ` WHERE 1=1`
	params := make(map[string]interface{})

	if req.Category != "" {
		query += ` AND category = @category`
		params["category"] = req.Category
	}

	if req.Status != "" {
		query += ` AND status = @status`
		params["status"] = req.Status
	}

	// Add ordering
	query += ` ORDER BY created_at DESC`

	// Add pagination
	if req.Limit > 0 {
		query += ` LIMIT @limit`
		params["limit"] = req.Limit
	}
	if req.Offset > 0 {
		query += ` OFFSET @offset`
		params["offset"] = req.Offset
	}

	stmt := spanner.Statement{
		SQL:    query,
		Params: params,
	}

	iter := rm.client.Single().Query(ctx, stmt)
	defer iter.Stop()

	var products []*contracts.ProductDTO
	now := time.Now()

	for {
		row, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to iterate products: %w", err)
		}

		data := &m_product.Data{}
		if err := row.ToStruct(data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal product: %w", err)
		}

		products = append(products, rm.dataToDTO(data, now))
	}

	// Check if there are more results
	hasMore := false
	if req.Limit > 0 && len(products) == req.Limit {
		hasMore = true
	}

	return &contracts.ProductList{
		Products:   products,
		TotalCount: len(products),
		HasMore:    hasMore,
	}, nil
}

// dataToDTO converts database model to DTO with effective price calculation
func (rm *ProductReadModel) dataToDTO(data *m_product.Data, now time.Time) *contracts.ProductDTO {
	dto := &contracts.ProductDTO{
		ProductID:   data.ProductID,
		Name:        data.Name,
		Description: data.Description,
		Category:    data.Category,
		BasePrice: contracts.MoneyDTO{
			Numerator:    data.BasePriceNumerator,
			Denominator:  data.BasePriceDenominator,
			Float64Value: float64(data.BasePriceNumerator) / float64(data.BasePriceDenominator),
		},
		Status:    data.Status,
		CreatedAt: data.CreatedAt,
		UpdatedAt: data.UpdatedAt,
	}

	// Add discount if present
	if data.DiscountPercent.Valid {
		isActive := !now.Before(data.DiscountStartDate.Time) && !now.After(data.DiscountEndDate.Time)
		dto.Discount = &contracts.DiscountDTO{
			Percentage: int(data.DiscountPercent.Int64),
			StartDate:  data.DiscountStartDate.Time,
			EndDate:    data.DiscountEndDate.Time,
			IsActive:   isActive,
		}

		// Calculate effective price
		if isActive {
			multiplier := float64(100-data.DiscountPercent.Int64) / 100.0
			effectiveValue := (float64(data.BasePriceNumerator) / float64(data.BasePriceDenominator)) * multiplier
			dto.EffectivePrice = contracts.MoneyDTO{
				Numerator:    int64(effectiveValue * float64(data.BasePriceDenominator)),
				Denominator:  data.BasePriceDenominator,
				Float64Value: effectiveValue,
			}
		} else {
			dto.EffectivePrice = dto.BasePrice
		}
	} else {
		dto.EffectivePrice = dto.BasePrice
	}

	return dto
}

func buildColumnList() string {
	return m_product.ProductID + ", " +
		m_product.Name + ", " +
		m_product.Description + ", " +
		m_product.Category + ", " +
		m_product.BasePriceNumerator + ", " +
		m_product.BasePriceDenominator + ", " +
		m_product.DiscountPercent + ", " +
		m_product.DiscountStartDate + ", " +
		m_product.DiscountEndDate + ", " +
		m_product.Status + ", " +
		m_product.CreatedAt + ", " +
		m_product.UpdatedAt + ", " +
		m_product.ArchivedAt
}

// Ensure ProductReadModel implements the interface
var _ contracts.ReadModel = (*ProductReadModel)(nil)
