FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/hawler ./cmd

FROM alpine:3.20

RUN adduser -D -g '' appuser

WORKDIR /app
COPY --from=builder /out/hawler /app/hawler

EXPOSE 8080

USER appuser

CMD ["/app/hawler"]
