// Package memory implements an in-memory order repository.
package memory

import (
	"context"
	"sync"

	"orderflow/pkg/order"
)

// Repository provides an in-memory implementation of order.Repository.
type Repository struct {
	mu     sync.RWMutex
	orders map[string]order.Order
}

// New creates a new in-memory repository.
func New() *Repository {
	return &Repository{orders: make(map[string]order.Order)}
}

// Create stores the order.
func (r *Repository) Create(ctx context.Context, o order.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.orders[o.ID] = o
	return nil
}

// Get retrieves an order by ID.
func (r *Repository) Get(ctx context.Context, id string) (order.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	o, ok := r.orders[id]
	if !ok {
		return order.Order{}, order.ErrNotFound
	}
	return o, nil
}

// List returns all orders.
func (r *Repository) List(ctx context.Context) ([]order.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]order.Order, 0, len(r.orders))
	for _, o := range r.orders {
		out = append(out, o)
	}
	return out, nil
}

// Update replaces an existing order.
func (r *Repository) Update(ctx context.Context, o order.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.orders[o.ID]; !ok {
		return order.ErrNotFound
	}
	r.orders[o.ID] = o
	return nil
}

// Delete removes an order by ID.
func (r *Repository) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.orders[id]; !ok {
		return order.ErrNotFound
	}
	delete(r.orders, id)
	return nil
}
