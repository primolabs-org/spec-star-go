package main

import (
    "github.com/aws/aws-lambda-go/lambda"

    "example/internal/adapters/inbound/httpapi"
)

var handler httpapi.Handler

func init() {
    // Build app dependencies here and assign the inbound adapter handler.
    // Example:
    // app := bootstrap.MustBuild(realClient, table)
    // handler = httpapi.NewHandler(app.CreateOrder)
}

func main() {
    lambda.Start(handler.Handle)
}
