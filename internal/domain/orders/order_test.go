package orders

import (
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	now := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)

	order, err := New("cust-123", 1999, now)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if order.ID == "" {
		t.Fatal("expected generated id")
	}
	if order.CustomerID != "cust-123" {
		t.Fatalf("CustomerID = %q", order.CustomerID)
	}
	if order.AmountCents != 1999 {
		t.Fatalf("AmountCents = %d", order.AmountCents)
	}
	if order.Status != StatusCreated {
		t.Fatalf("Status = %q", order.Status)
	}
	if !order.CreatedAt.Equal(now) {
		t.Fatalf("CreatedAt = %s", order.CreatedAt)
	}
}

func TestNewRejectsInvalidInput(t *testing.T) {
	now := time.Now()

	if _, err := New("  ", 100, now); err != ErrInvalidCustomerID {
		t.Fatalf("expected ErrInvalidCustomerID, got %v", err)
	}
	if _, err := New("cust-123", 0, now); err != ErrInvalidAmount {
		t.Fatalf("expected ErrInvalidAmount, got %v", err)
	}
}
