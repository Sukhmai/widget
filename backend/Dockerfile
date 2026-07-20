FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/worker ./cmd/worker


FROM alpine:3.21 AS base
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app


FROM base AS api
COPY --from=builder /out/api .
EXPOSE 8080
CMD ["./api"]


FROM base AS worker
COPY --from=builder /out/worker .
CMD ["./worker"]
