package order

import (
    "errors"
    "strings"
)

var ErrInvalidCustomerID = errors.New("invalid customer id")

type ID string

type Order struct {
    id         ID
    customerID string
}

func New(id ID, customerID string) (Order, error) {
    if strings.TrimSpace(customerID) == "" {
        return Order{}, ErrInvalidCustomerID
    }

    return Order{
        id:         id,
        customerID: customerID,
    }, nil
}

func (o Order) ID() ID {
    return o.id
}

func (o Order) CustomerID() string {
    return o.customerID
}
