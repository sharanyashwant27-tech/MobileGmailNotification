.PHONY: run test tidy docker-build docker-up docker-down docker-logs migrate

run:
	go run ./cmd/server

test:
	go test ./...

tidy:
	go mod tidy

docker-build:
	docker compose build --no-cache api

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f api

# Fresh DB only: migrations run via docker-entrypoint-initdb.d.
# Existing volumes need manual apply of new SQL under migrations/.
migrate:
	@echo "For a fresh volume: docker compose down -v && docker compose up -d"
	@echo "For existing DB, apply new files, e.g.:"
	@echo "  Get-Content migrations/002_qr_login.sql | docker exec -i backend-postgres-1 psql -U gmail_user -d gmail_notifications"
