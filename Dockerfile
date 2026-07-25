FROM golang:1.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/orders-service ./cmd/orders-service

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=build /out/orders-service /usr/local/bin/orders-service
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/orders-service"]
