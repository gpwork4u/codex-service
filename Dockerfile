FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o codex-service .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/codex-service /usr/local/bin/codex-service
EXPOSE 8787
ENTRYPOINT ["codex-service"]
