package repo

import "cloud.google.com/go/spanner"

// NewProductRepository is an alias for NewProductRepo for backward compatibility
func NewProductRepository(client *spanner.Client) *ProductRepo {
	return NewProductRepo(client)
}

// NewOutboxRepository is an alias for NewOutboxRepo for backward compatibility
func NewOutboxRepository() *OutboxRepo {
	return NewOutboxRepo()
}
