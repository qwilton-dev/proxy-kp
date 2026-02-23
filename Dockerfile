FROM golang:1.25-alpine AS builder

WORKDIR /app
RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o proxy cmd/proxy/main.go

FROM alpine:latest

RUN apk --no-cache add ca-certificates wget

WORKDIR /app

COPY --from=builder /app/proxy .

COPY config.yaml.example /app/config.yaml.example

EXPOSE 8080 8443

ENTRYPOINT ["./proxy"]
CMD ["-config", "/app/config.yaml"]
