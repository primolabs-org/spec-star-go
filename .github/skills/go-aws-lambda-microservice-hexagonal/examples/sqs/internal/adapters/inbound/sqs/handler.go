package sqs

import (
    "context"
    "encoding/json"

    "github.com/aws/aws-lambda-go/events"

    "example/internal/application/createorder"
)

type Handler struct {
    createOrder createorder.Handler
}

type createOrderMessage struct {
    OrderID    string `json:"order_id"`
    CustomerID string `json:"customer_id"`
}

func NewHandler(createOrder createorder.Handler) Handler {
    return Handler{createOrder: createOrder}
}

func (h Handler) Handle(ctx context.Context, event events.SQSEvent) (events.SQSEventResponse, error) {
    failures := make([]events.SQSBatchItemFailure, 0)

    for _, record := range event.Records {
        var msg createOrderMessage
        if err := json.Unmarshal([]byte(record.Body), &msg); err != nil {
            failures = append(failures, events.SQSBatchItemFailure{ItemIdentifier: record.MessageId})
            continue
        }

        _, err := h.createOrder.Handle(ctx, createorder.Command{
            OrderID:    msg.OrderID,
            CustomerID: msg.CustomerID,
        })
        if err != nil {
            failures = append(failures, events.SQSBatchItemFailure{ItemIdentifier: record.MessageId})
        }
    }

    return events.SQSEventResponse{BatchItemFailures: failures}, nil
}
