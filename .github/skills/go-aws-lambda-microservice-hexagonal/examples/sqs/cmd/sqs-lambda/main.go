package main

import (
    "github.com/aws/aws-lambda-go/lambda"

    sqsinbound "example/internal/adapters/inbound/sqs"
)

var handler sqsinbound.Handler

func init() {
    // Build app dependencies here and assign the inbound adapter handler.
    // Example:
    // app := bootstrap.MustBuild(realClient, table)
    // handler = sqsinbound.NewHandler(app.CreateOrder)
}

func main() {
    lambda.Start(handler.Handle)
}
