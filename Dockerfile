FROM golang:1.26.2 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /mesh-node ./cmd/mesh-node

FROM alpine:latest

RUN apk add --no-cache ca-certificates

COPY --from=builder /mesh-node /usr/local/bin/mesh-node
RUN chmod +x /usr/local/bin/mesh-node

RUN mkdir -p /root/mesh-network && chmod 777 /root/mesh-network

EXPOSE 6040/udp

ENTRYPOINT ["mesh-node"]
CMD ["--interfaces", "eth0"]