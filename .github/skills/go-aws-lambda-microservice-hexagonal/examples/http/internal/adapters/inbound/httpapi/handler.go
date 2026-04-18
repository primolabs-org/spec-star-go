package httpapi

import (
    "context"
    "encoding/json"
    "net/http"

    "github.com/aws/aws-lambda-go/events"

    "example/internal/application/createorder"
)

type Handler struct {
    createOrder createorder.Handler
}

type createOrderRequest struct {
    OrderID    string `json:"order_id"`
    CustomerID string `json:"customer_id"`
}

func NewHandler(createOrder createorder.Handler) Handler {
    return Handler{createOrder: createOrder}
}

func (h Handler) Handle(ctx context.Context, event events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
    var req createOrderRequest
    if err := json.Unmarshal([]byte(event.Body), &req); err != nil {
        return events.APIGatewayV2HTTPResponse{StatusCode: http.StatusBadRequest}, nil
    }

    result, err := h.createOrder.Handle(ctx, createorder.Command{
        OrderID:    req.OrderID,
        CustomerID: req.CustomerID,
    })
    if err != nil {
        return events.APIGatewayV2HTTPResponse{StatusCode: http.StatusUnprocessableEntity}, nil
    }

    body, _ := json.Marshal(map[string]string{"order_id": result.OrderID})

    return events.APIGatewayV2HTTPResponse{
        StatusCode: http.StatusCreated,
        Body:       string(body),
        Headers: map[string]string{
            "content-type": "application/json",
        },
    }, nil
}
