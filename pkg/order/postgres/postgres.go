package postgres

import (
	"context"
	"database/sql"

	"orderflow/pkg/order"
)

// Repository persists orders in PostgreSQL.
type Repository struct {
	db *sql.DB
}

// New creates a PostgreSQL repository.
func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Create inserts a new order.
func (r *Repository) Create(ctx context.Context, o order.Order) error {
	_, err := r.db.ExecContext(ctx, "INSERT INTO orders (id,item,quantity) VALUES ($1,$2,$3)", o.ID, o.Item, o.Quantity)
	return err
}

// Get retrieves an order by ID.
func (r *Repository) Get(ctx context.Context, id string) (order.Order, error) {
	var o order.Order
	err := r.db.QueryRowContext(ctx, "SELECT id,item,quantity FROM orders WHERE id=$1", id).Scan(&o.ID, &o.Item, &o.Quantity)
	if err == sql.ErrNoRows {
		return order.Order{}, order.ErrNotFound
	}
	return o, err
}

// List fetches all orders.
func (r *Repository) List(ctx context.Context) ([]order.Order, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id,item,quantity FROM orders")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var orders []order.Order
	for rows.Next() {
		var o order.Order
		if err := rows.Scan(&o.ID, &o.Item, &o.Quantity); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, rows.Err()
}

// Update updates an existing order.
func (r *Repository) Update(ctx context.Context, o order.Order) error {
	res, err := r.db.ExecContext(ctx, "UPDATE orders SET item=$2, quantity=$3 WHERE id=$1", o.ID, o.Item, o.Quantity)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return order.ErrNotFound
	}
	return nil
}

// Delete removes an order by ID.
func (r *Repository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM orders WHERE id=$1", id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return order.ErrNotFound
	}
	return nil
}
