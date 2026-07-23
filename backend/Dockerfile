FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/worker ./cmd/worker
RUN CGO_ENABLED=0 GOOS=linux GOBIN=/out go install github.com/pressly/goose/v3/cmd/goose@v3.27.3


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


FROM base AS migrate
COPY --from=builder /out/goose .
COPY sql/migrations ./sql/migrations
# DATABASE_URL comes from the container env (not an arg) so overriding the
# command with just a subcommand (e.g. `run migrate status`) still works.
ENTRYPOINT ["sh", "-c", "exec ./goose -dir sql/migrations postgres \"$DATABASE_URL\" \"$@\"", "--"]
CMD ["up"]
