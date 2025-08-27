package order

import (
	"context"
	"database/sql"
)

// PGStore persists orders to PostgreSQL.
type PGStore struct {
	db *sql.DB
}

// NewPGStore returns a Postgres-backed store. The caller must ensure the
// provided database has an orders table:
// CREATE TABLE IF NOT EXISTS orders (id TEXT PRIMARY KEY, item TEXT, quantity INT);
func NewPGStore(db *sql.DB) *PGStore {
	return &PGStore{db: db}
}

// Create inserts the given order into Postgres.
func (s *PGStore) Create(ctx context.Context, o Order) error {
	_, err := s.db.ExecContext(ctx, "INSERT INTO orders (id, item, quantity) VALUES ($1,$2,$3)", o.ID, o.Item, o.Quantity)
	return err
}

// List retrieves all orders from Postgres.
func (s *PGStore) List(ctx context.Context) ([]Order, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, item, quantity FROM orders")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var orders []Order
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.ID, &o.Item, &o.Quantity); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, rows.Err()
}
