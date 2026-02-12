package update_product

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"product-catalog-service/internal/app/product/domain"
)

type MockProductRepo struct{ mock.Mock }

func (m *MockProductRepo) FindByID(ctx context.Context, id string) (*domain.Product, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Product), args.Error(1)
}
func (m *MockProductRepo) InsertMut(p *domain.Product) interface{} { return m.Called(p).Get(0) }
func (m *MockProductRepo) UpdateMut(p *domain.Product) interface{} { return m.Called(p).Get(0) }
func (m *MockProductRepo) Exists(ctx context.Context, id string) (bool, error) {
	args := m.Called(ctx, id)
	return args.Bool(0), args.Error(1)
}

type MockOutboxRepo struct{ mock.Mock }

func (m *MockOutboxRepo) InsertMut(event interface{}) interface{} { return m.Called(event).Get(0) }

type MockCommitter struct{ mock.Mock }

func (m *MockCommitter) Apply(ctx context.Context, muts []interface{}) error {
	return m.Called(ctx, muts).Error(0)
}

type MockClock struct{ mock.Mock }

func (m *MockClock) Now() time.Time { return m.Called().Get(0).(time.Time) }

func TestUpdateProduct_Success(t *testing.T) {
	now := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	basePrice, _ := domain.NewMoney(10000, 100)

	product := domain.LoadProduct("prod-123", "Old Name", "Old Desc", "Old Cat",
		basePrice, nil, domain.ProductStatusActive, now, now, nil)

	productRepo := new(MockProductRepo)
	outboxRepo := new(MockOutboxRepo)
	committer := new(MockCommitter)
	clock := new(MockClock)

	clock.On("Now").Return(now)
	productRepo.On("FindByID", mock.Anything, "prod-123").Return(product, nil)
	productRepo.On("UpdateMut", mock.AnythingOfType("*domain.Product")).Return(&struct{}{})
	outboxRepo.On("InsertMut", mock.Anything).Return(&struct{}{})
	committer.On("Apply", mock.Anything, mock.AnythingOfType("[]interface {}")).Return(nil)

	interactor := NewInteractor(productRepo, outboxRepo, committer, clock)

	err := interactor.Execute(context.Background(), Request{
		ProductID:   "prod-123",
		Name:        "New Name",
		Description: "New Desc",
		Category:    "New Cat",
	})

	require.NoError(t, err)
	assert.Equal(t, "New Name", product.Name())
}

func TestUpdateProduct_InvalidName(t *testing.T) {
	now := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	basePrice, _ := domain.NewMoney(10000, 100)

	product := domain.LoadProduct("prod-123", "Old Name", "Old Desc", "Old Cat",
		basePrice, nil, domain.ProductStatusActive, now, now, nil)

	productRepo := new(MockProductRepo)
	clock := new(MockClock)

	clock.On("Now").Return(now)
	productRepo.On("FindByID", mock.Anything, "prod-123").Return(product, nil)

	interactor := NewInteractor(productRepo, new(MockOutboxRepo), new(MockCommitter), clock)

	err := interactor.Execute(context.Background(), Request{
		ProductID:   "prod-123",
		Name:        "",
		Description: "Desc",
		Category:    "Cat",
	})

	assert.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrInvalidProductName)
}
