# Build stage
FROM golang:1.25 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY messages-service ./messages-service

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/bin/messages-service ./messages-service/cmd

# Runtime stage
FROM gcr.io/distroless/base-debian12

WORKDIR /app/messages-service

COPY --from=builder /app/bin/messages-service ./messages-service
COPY messages-service/configs ./configs

EXPOSE 8080

ENTRYPOINT ["/app/messages-service/messages-service"]
