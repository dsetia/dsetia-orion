module apis

go 1.25.0

require github.com/hashicorp/go-version v1.7.0

require github.com/lib/pq v1.10.9

require (
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/google/uuid v1.6.0
	golang.org/x/crypto v0.50.0
	orion/common v0.0.0
)

replace orion/common => ../common
