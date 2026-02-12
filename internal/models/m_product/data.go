package m_product

import (
	"time"

	"cloud.google.com/go/spanner"
)

// Data represents a product row in the database
// This is the persistence model, separate from the domain model
type Data struct {
	ProductID            string
	Name                 string
	Description          string
	Category             string
	BasePriceNumerator   int64
	BasePriceDenominator int64
	DiscountPercent      spanner.NullInt64
	DiscountStartDate    spanner.NullTime
	DiscountEndDate      spanner.NullTime
	Status               string
	CreatedAt            time.Time
	UpdatedAt            time.Time
	ArchivedAt           spanner.NullTime
}

// Model provides type-safe operations for the products table
type Model struct {
	tableName string
}

// NewModel creates a new products table model
func NewModel() *Model {
	return &Model{
		tableName: TableName,
	}
}

// InsertMut creates an insert mutation for a product
func (m *Model) InsertMut(data *Data) *spanner.Mutation {
	return spanner.Insert(
		m.tableName,
		AllColumns(),
		[]interface{}{
			data.ProductID,
			data.Name,
			data.Description,
			data.Category,
			data.BasePriceNumerator,
			data.BasePriceDenominator,
			data.DiscountPercent,
			data.DiscountStartDate,
			data.DiscountEndDate,
			data.Status,
			data.CreatedAt,
			data.UpdatedAt,
			data.ArchivedAt,
		},
	)
}

// UpdateMut creates an update mutation for specific fields
func (m *Model) UpdateMut(productID string, updates map[string]interface{}) *spanner.Mutation {
	columns := []string{ProductID}
	values := []interface{}{productID}

	for col, val := range updates {
		columns = append(columns, col)
		values = append(values, val)
	}

	return spanner.Update(m.tableName, columns, values)
}

// DeleteMut creates a delete mutation (not used for soft delete)
func (m *Model) DeleteMut(productID string) *spanner.Mutation {
	return spanner.Delete(m.tableName, spanner.Key{productID})
}

// TableName returns the table name
func (m *Model) TableName() string {
	return m.tableName
}
