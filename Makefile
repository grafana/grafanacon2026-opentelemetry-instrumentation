.PHONY: test test-backend test-frontend load spell links format format-check

test: test-backend test-frontend

test-backend:
	docker compose -f tests/docker-compose.test.yml -p tapas-test up -d --wait db
	cd tests/backend && TEST_DB_URL=postgres://postgres:postgres@localhost:5433/tapas?sslmode=disable go test ./...
	docker compose -f tests/docker-compose.test.yml -p tapas-test down -v

test-frontend:
	cd tests/frontend && npm test

load:
	@if which k6 > /dev/null 2>&1; then \
	  k6 run load-test.js; \
	else \
	  docker run --rm -i --network host -v "$(PWD):/scripts" grafana/k6 run /scripts/load-test.js; \
	fi

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
