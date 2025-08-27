package order

import "sync"

// Order represents a customer order in the system.
type Order struct {
	ID       string `json:"id"`
	Item     string `json:"item"`
	Quantity int    `json:"quantity"`
}

// Store provides safe concurrent access to orders.
type Store struct {
	mu     sync.Mutex
	orders map[string]Order
}

// NewStore returns an initialized Store.
func NewStore() *Store {
	return &Store{orders: make(map[string]Order)}
}

// Create adds a new order to the store.
func (s *Store) Create(o Order) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.orders[o.ID] = o
}

// List returns all stored orders.
func (s *Store) List() []Order {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Order, 0, len(s.orders))
	for _, o := range s.orders {
		out = append(out, o)
	}
	return out
}
