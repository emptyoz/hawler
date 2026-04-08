# Hawler

Трекер задач для студенческих команд (в стиле Jira/Trello): Go backend + Vue frontend + Postgres.

## Запуск всего проекта одной командой

```bash
cp .env.example .env
docker compose up --build
```

Или через Makefile:

```bash
make up
```

Сервисы:
- frontend: `http://localhost:5173`
- backend API: `http://localhost:8080`
- Postgres: `localhost:5432`
- migrate: one-shot контейнер для наката `backend/migrations` перед стартом backend

Остановить:

```bash
docker compose down
```

Или:

```bash
make down
```
# hawler
