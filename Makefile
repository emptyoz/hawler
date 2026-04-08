.PHONY: help env up down restart build logs ps clean test-backend test-backend-cover

help:
	@echo "Команды:"
	@echo "  make up       - создать .env (если нет) и запустить все сервисы"
	@echo "  make down     - остановить все сервисы"
	@echo "  make restart  - перезапустить все сервисы"
	@echo "  make build    - собрать все образы"
	@echo "  make logs     - смотреть логи compose"
	@echo "  make ps       - показать статус сервисов compose"
	@echo "  make clean    - остановить сервисы и удалить volumes"
	@echo "  make test-backend       - запустить тесты backend"
	@echo "  make test-backend-cover - запустить тесты backend c отчетом покрытия"

env:
	@test -f .env || cp .env.example .env

run:
	make down && make up

up: env
	docker compose up --build -d

down:
	docker compose down

restart: down up

build: env
	docker compose build

logs:
	docker compose logs -f --tail=150

ps:
	docker compose ps

clean:
	docker compose down -v

test-backend:
	cd backend && go test ./...

test-backend-cover:
	cd backend && go test ./... -coverprofile=coverage.out && go tool cover -func=coverage.out
