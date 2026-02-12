package m_product

import (
	"time"

	"cloud.google.com/go/spanner"
)

// Field constants for product table
const (
	TableName = "products"

	// Shorter aliases for backward compatibility
	ProductID            = "product_id"
	Name                 = "name"
	Description          = "description"
	Category             = "category"
	BasePriceNumerator   = "base_price_numerator"
	BasePriceDenominator = "base_price_denominator"
	DiscountPercent      = "discount_percent"
	DiscountStartDate    = "discount_start_date"
	DiscountEndDate      = "discount_end_date"
	Status               = "status"
	CreatedAt            = "created_at"
	UpdatedAt            = "updated_at"
	ArchivedAt           = "archived_at"

	// Full field names
	FieldProductID       = "product_id"
	FieldName            = "name"
	FieldDescription     = "description"
	FieldCategory        = "category"
	FieldBasePriceNum    = "base_price_numerator"
	FieldBasePriceDenom  = "base_price_denominator"
	FieldDiscountPercent = "discount_percent"
	FieldDiscountStart   = "discount_start_date"
	FieldDiscountEnd     = "discount_end_date"
	FieldStatus          = "status"
	FieldCreatedAt       = "created_at"
	FieldUpdatedAt       = "updated_at"
	FieldArchivedAt      = "archived_at"
)

// AllFields returns all field names
func AllFields() []string {
	return []string{
		FieldProductID,
		FieldName,
		FieldDescription,
		FieldCategory,
		FieldBasePriceNum,
		FieldBasePriceDenom,
		FieldDiscountPercent,
		FieldDiscountStart,
		FieldDiscountEnd,
		FieldStatus,
		FieldCreatedAt,
		FieldUpdatedAt,
		FieldArchivedAt,
	}
}

// AllColumns is an alias for AllFields for backward compatibility
func AllColumns() []string {
	return AllFields()
}

// InsertMutation creates a Spanner mutation for inserting a product
func InsertMutation(values []interface{}) *spanner.Mutation {
	return spanner.Insert(TableName, AllFields(), values)
}

// UpdateMutation creates a Spanner mutation for updating a product
func UpdateMutation(productID string, columns []string, values []interface{}) *spanner.Mutation {
	// Prepend product_id to columns and values
	allColumns := append([]string{FieldProductID}, columns...)
	allValues := append([]interface{}{productID}, values...)
	return spanner.Update(TableName, allColumns, allValues)
}

// ToSpannerTime converts Go time to Spanner-compatible format
func ToSpannerTime(t time.Time) string {
	return t.Format(time.RFC3339Nano)
}
