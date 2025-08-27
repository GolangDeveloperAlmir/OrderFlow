package order

import "testing"

func TestStore(t *testing.T) {
	s := NewStore()
	s.Create(Order{ID: "1", Item: "Widget", Quantity: 2})

	orders := s.List()
	if len(orders) != 1 {
		t.Fatalf("expected 1 order, got %d", len(orders))
	}
	if orders[0].Item != "Widget" {
		t.Fatalf("unexpected item: %s", orders[0].Item)
	}
}
