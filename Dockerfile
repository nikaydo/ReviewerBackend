FROM golang:1.24.4   AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server ./cmd

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/server .
COPY .env .
COPY web ./web
COPY db ./db

EXPOSE 8080

CMD ["./server"]
