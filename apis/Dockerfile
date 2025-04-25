FROM golang:1.19 AS builder
WORKDIR /app

COPY . .
RUN ls -l /app && echo "Listing complete" || echo "Error listing /app"

RUN go mod tidy
RUN go build -o apis ./apis

FROM debian:bookworm-slim
WORKDIR /app
COPY --from=builder /app/apis .

CMD ["./apis"]
