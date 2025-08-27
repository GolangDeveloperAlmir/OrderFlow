package order

import (
	"context"
	"errors"
)

// Order represents a customer purchase order.
type Order struct {
	ID       string `json:"id"`
	Item     string `json:"item"`
	Quantity int    `json:"quantity"`
}

// Repository defines behavior for persisting orders.
type Repository interface {
	Create(ctx context.Context, o Order) error
	Get(ctx context.Context, id string) (Order, error)
	List(ctx context.Context) ([]Order, error)
	Update(ctx context.Context, o Order) error
	Delete(ctx context.Context, id string) error
}

// ErrNotFound indicates the requested order does not exist.
var ErrNotFound = errors.New("order not found")
