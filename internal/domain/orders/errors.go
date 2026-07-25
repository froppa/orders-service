package orders

import "errors"

var (
	ErrInvalidCustomerID = errors.New("customer_id must be set")
	ErrInvalidAmount     = errors.New("amount_cents must be greater than zero")
	ErrNotFound          = errors.New("order not found")
)
