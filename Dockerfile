FROM golang:1.20-buster as builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build

FROM debian:buster-slim

WORKDIR /app

COPY --from=builder /app/esd /app

ENTRYPOINT ["/app/esd"]
