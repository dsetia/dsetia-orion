FROM golang:1.19 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY apis.go dbutil.go apis_utils.go ./
RUN CGO_ENABLED=1 GOOS=linux go build -o apis apis.go apis_utils.go dbutil.go

FROM debian:bookworm-slim
WORKDIR /app
COPY --from=builder /app/apis .

CMD ["./apis"]
