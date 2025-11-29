# Build stage
FROM golang:1.25 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN cd llm-service && go mod download
RUN cd llm-service && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/bin/llm-service

# Runtime stage
FROM gcr.io/distroless/base-debian12

WORKDIR /app/llm-service

COPY --from=builder /app/bin/llm-service ./llm-service
COPY llm-service/systemprompt.txt ./systemprompt.txt
COPY llm-service/org.json ./org.json

EXPOSE 8080

ENTRYPOINT ["/app/llm-service/llm-service"]
