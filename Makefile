.PHONY: test test-backend test-frontend

test: test-backend test-frontend

test-backend:
	docker compose -f tests/docker-compose.test.yml -p tapas-test up -d --wait db
	cd tests/backend && TEST_DB_URL=postgres://postgres:postgres@localhost:5433/tapas?sslmode=disable go test ./...
	docker compose -f tests/docker-compose.test.yml -p tapas-test down -v

test-frontend:
	cd tests/frontend && npm test
