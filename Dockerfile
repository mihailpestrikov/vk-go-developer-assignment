FROM golang:1.24-alpine

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

COPY config/config.yaml /app/config/

RUN go build -o subscription-service ./cmd/subscription-service

EXPOSE 50051

CMD ["./subscription-service", "--config", "/app/config/config.yaml"]