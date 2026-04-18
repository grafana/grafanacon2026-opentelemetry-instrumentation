.PHONY: test test-backend test-frontend load spell links format format-check

test: test-backend test-frontend

test-backend:
	docker compose -f tests/docker-compose.test.yaml -p tapas-test up -d --wait db
	cd tests/backend && TEST_DB_URL=postgres://postgres:postgres@localhost:5433/tapas?sslmode=disable go test ./...
	docker compose -f tests/docker-compose.test.yaml -p tapas-test down -v

test-frontend:
	cd tests/frontend && npm test

load:
	docker compose run --rm load-test

spell:
	npx cspell "**/*.md" "**/*.go" "**/*.js"

links:
	lychee .

format:
	npx prettier --write "**/*.md" "**/*.js"
	gofmt -w backend/

format-check:
	npx prettier --check "**/*.md" "**/*.js"
	@test -z "$$(gofmt -l backend/)" || (echo "Go files need formatting:"; gofmt -l backend/; exit 1)
