package order

import "errors"

var (
	ErrNotFound        = errors.New("order: not found")
	ErrEmptyCart       = errors.New("order: must have at least one item")
	ErrInvalidQuantity = errors.New("order: item quantity must be greater than zero")
)
