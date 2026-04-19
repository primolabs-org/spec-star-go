FROM golang:1.26.2-bookworm AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/bootstrap ./cmd/http-lambda

FROM public.ecr.aws/lambda/provided:al2023

COPY --from=build /out/bootstrap ${LAMBDA_RUNTIME_DIR}/bootstrap

CMD ["function.handler"]
