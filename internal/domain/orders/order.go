package orders

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

const StatusCreated = "created"

type Order struct {
	ID          string    `json:"id"`
	CustomerID  string    `json:"customer_id"`
	AmountCents int       `json:"amount_cents"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

func New(customerID string, amountCents int, now time.Time) (Order, error) {
	customerID = strings.TrimSpace(customerID)
	if customerID == "" {
		return Order{}, ErrInvalidCustomerID
	}
	if amountCents <= 0 {
		return Order{}, ErrInvalidAmount
	}
	return Order{
		ID:          uuid.NewString(),
		CustomerID:  customerID,
		AmountCents: amountCents,
		Status:      StatusCreated,
		CreatedAt:   now.UTC(),
	}, nil
}
