FROM golang:1.19 AS builder
WORKDIR /app

COPY . .
RUN ls -l /app && echo "Listing complete" || echo "Error listing /app"

WORKDIR /app/apis
RUN go mod tidy
RUN go build -o apis .

FROM debian:bookworm-slim
WORKDIR /app
RUN apt-get update && apt-get install -y postgresql-client && rm -rf /var/lib/apt/lists/*
COPY --from=builder /app/apis .

CMD ["./apis"]
