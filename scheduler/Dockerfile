FROM golang:latest AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -v -o scheduler

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/scheduler .
CMD ["./scheduler"]