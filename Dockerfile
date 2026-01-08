FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o austrian_invoice .

FROM alpine:latest
RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/austrian_invoice .
COPY --from=builder /app/static ./static

EXPOSE 8080

CMD ["./austrian_invoice"]
