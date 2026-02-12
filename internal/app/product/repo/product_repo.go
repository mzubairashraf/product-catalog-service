package repo

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/spanner"
	"google.golang.org/grpc/codes"

	"product-catalog-service/internal/app/product/contracts"
	"product-catalog-service/internal/app/product/domain"
	"product-catalog-service/internal/models/m_product"
)

// ProductRepo is the Spanner implementation of ProductRepository
type ProductRepo struct {
	client *spanner.Client
	model  *m_product.Model
}

// NewProductRepo creates a new product repository
func NewProductRepo(client *spanner.Client) *ProductRepo {
	return &ProductRepo{
		client: client,
		model:  m_product.NewModel(),
	}
}

// FindByID retrieves a product by ID for write operations
func (r *ProductRepo) FindByID(ctx context.Context, productID string) (*domain.Product, error) {
	row, err := r.client.Single().ReadRow(
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

	return r.dataToDomain(data)
}

// Exists checks if a product exists
func (r *ProductRepo) Exists(ctx context.Context, productID string) (bool, error) {
	row, err := r.client.Single().ReadRow(
		ctx,
		m_product.TableName,
		spanner.Key{productID},
		[]string{m_product.ProductID},
	)
	if err != nil {
		if spanner.ErrCode(err) == codes.NotFound {
			return false, nil
		}
		return false, fmt.Errorf("failed to check product existence: %w", err)
	}

	var id string
	if err := row.Column(0, &id); err != nil {
		return false, fmt.Errorf("failed to read product id: %w", err)
	}

	return true, nil
}

// InsertMut returns a mutation for inserting a new product
func (r *ProductRepo) InsertMut(product *domain.Product) *spanner.Mutation {
	data := r.domainToData(product)
	return r.model.InsertMut(data)
}

// UpdateMut returns a mutation for updating an existing product
func (r *ProductRepo) UpdateMut(product *domain.Product) *spanner.Mutation {
	if !product.Changes().HasChanges() {
		return nil
	}

	updates := make(map[string]interface{})

	if product.Changes().Dirty(domain.FieldName) {
		updates[m_product.Name] = product.Name()
	}
	if product.Changes().Dirty(domain.FieldDescription) {
		updates[m_product.Description] = product.Description()
	}
	if product.Changes().Dirty(domain.FieldCategory) {
		updates[m_product.Category] = product.Category()
	}
	if product.Changes().Dirty(domain.FieldBasePrice) {
		updates[m_product.BasePriceNumerator] = product.BasePrice().Numerator()
		updates[m_product.BasePriceDenominator] = product.BasePrice().Denominator()
	}
	if product.Changes().Dirty(domain.FieldDiscount) {
		if discount := product.Discount(); discount != nil {
			updates[m_product.DiscountPercent] = int64(discount.Percentage())
			updates[m_product.DiscountStartDate] = discount.StartDate()
			updates[m_product.DiscountEndDate] = discount.EndDate()
		} else {
			updates[m_product.DiscountPercent] = spanner.NullInt64{}
			updates[m_product.DiscountStartDate] = spanner.NullTime{}
			updates[m_product.DiscountEndDate] = spanner.NullTime{}
		}
	}
	if product.Changes().Dirty(domain.FieldStatus) {
		updates[m_product.Status] = string(product.Status())
		if product.Status() == domain.ProductStatusArchived && product.ArchivedAt() != nil {
			updates[m_product.ArchivedAt] = *product.ArchivedAt()
		}
	}

	updates[m_product.UpdatedAt] = product.UpdatedAt()

	if len(updates) == 0 {
		return nil
	}

	return r.model.UpdateMut(product.ID(), updates)
}

// domainToData converts a domain product to database model
func (r *ProductRepo) domainToData(product *domain.Product) *m_product.Data {
	data := &m_product.Data{
		ProductID:            product.ID(),
		Name:                 product.Name(),
		Description:          product.Description(),
		Category:             product.Category(),
		BasePriceNumerator:   product.BasePrice().Numerator(),
		BasePriceDenominator: product.BasePrice().Denominator(),
		Status:               string(product.Status()),
		CreatedAt:            product.CreatedAt(),
		UpdatedAt:            product.UpdatedAt(),
	}

	if discount := product.Discount(); discount != nil {
		data.DiscountPercent = spanner.NullInt64{Int64: int64(discount.Percentage()), Valid: true}
		data.DiscountStartDate = spanner.NullTime{Time: discount.StartDate(), Valid: true}
		data.DiscountEndDate = spanner.NullTime{Time: discount.EndDate(), Valid: true}
	}

	if archivedAt := product.ArchivedAt(); archivedAt != nil {
		data.ArchivedAt = spanner.NullTime{Time: *archivedAt, Valid: true}
	}

	return data
}

// dataToDomain converts database model to domain product
func (r *ProductRepo) dataToDomain(data *m_product.Data) (*domain.Product, error) {
	basePrice, err := domain.NewMoney(data.BasePriceNumerator, data.BasePriceDenominator)
	if err != nil {
		return nil, fmt.Errorf("invalid base price: %w", err)
	}

	var discount *domain.Discount
	if data.DiscountPercent.Valid {
		discount, err = domain.NewDiscount(
			int(data.DiscountPercent.Int64),
			data.DiscountStartDate.Time,
			data.DiscountEndDate.Time,
		)
		if err != nil {
			return nil, fmt.Errorf("invalid discount: %w", err)
		}
	}

	var archivedAt *time.Time
	if data.ArchivedAt.Valid {
		archivedAt = &data.ArchivedAt.Time
	}

	product := domain.LoadProduct(
		data.ProductID,
		data.Name,
		data.Description,
		data.Category,
		basePrice,
		discount,
		domain.ProductStatus(data.Status),
		data.CreatedAt,
		data.UpdatedAt,
		archivedAt,
	)

	return product, nil
}

var _ contracts.ProductRepository = (*ProductRepo)(nil)
