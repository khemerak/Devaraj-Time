.PHONY: up migrate seed test build
up:
	docker compose up --build -d
migrate:
	docker compose run --rm migrate
seed:
	docker compose run --rm seed
test:
	cd backend && go test ./...
	cd frontend && npm test
build:
	cd backend && go build ./cmd/api
	cd frontend && npm run build
