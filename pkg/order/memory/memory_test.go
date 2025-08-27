package memory

import (
	"context"
	"testing"

	"orderflow/pkg/order"
)

func TestRepository(t *testing.T) {
	ctx := context.Background()
	repo := New()
	o := order.Order{ID: "1", Item: "Widget", Quantity: 2}
	if err := repo.Create(ctx, o); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := repo.Get(ctx, "1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Item != "Widget" {
		t.Fatalf("expected Widget, got %s", got.Item)
	}
	o.Item = "Gadget"
	if err := repo.Update(ctx, o); err != nil {
		t.Fatalf("update: %v", err)
	}
	list, err := repo.List(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v len=%d", err, len(list))
	}
	if err := repo.Delete(ctx, "1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.Get(ctx, "1"); err == nil {
		t.Fatal("expected error after delete")
	}
}
