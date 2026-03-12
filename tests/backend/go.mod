module github.com/workshop/tapas-backend-tests

go 1.22

require (
	github.com/gorilla/mux v1.8.1
	github.com/lib/pq v1.11.2
	github.com/workshop/tapas-backend v0.0.0
)

replace github.com/workshop/tapas-backend => ../../backend
