FROM golang:1.19 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY apis.go dbutil.go apis_utils.go ./
RUN CGO_ENABLED=1 GOOS=linux go build -o apis apis.go apis_utils.go dbutil.go
RUN ls -l /app && echo "Listing complete" || echo "Error listing /app"
#RUN go build -o apis apis.go dbutil.go

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y libsqlite3-0
WORKDIR /app
COPY --from=builder /app/apis .

CMD ["./apis"]
