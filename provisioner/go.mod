module provisioner

go 1.19

require (
	github.com/google/uuid v1.6.0
	github.com/lib/pq v1.10.9
)

require orion/common v0.0.0-00010101000000-000000000000 // indirect
replace orion/common => ../common
