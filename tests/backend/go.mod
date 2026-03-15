module github.com/workshop/tapas-backend-tests

go 1.25.0

require (
	github.com/gorilla/mux v1.8.1
	github.com/lib/pq v1.12.3
	github.com/workshop/tapas-backend v0.0.0
)

require (
	github.com/XSAM/otelsql v0.41.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel v1.43.0 // indirect
	go.opentelemetry.io/otel/metric v1.43.0 // indirect
	go.opentelemetry.io/otel/trace v1.43.0 // indirect
)

replace github.com/workshop/tapas-backend => ../../backend
