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
- Postgres: `localhost:55432`
- migrate: one-shot контейнер для наката `backend/migrations` перед стартом backend

Для подключения к базе с хоста используй `DATABASE_URL`, а внутри Docker Compose -
`DATABASE_URL_DOCKER`.

## Scrum flow

- Scrum-доска автоматически получает колонки `backlog`, `todo`, `in_progress`, `done`.
- Новые задачи без `sprint_id` попадают в `backlog` и остаются без привязки к спринту.
- Задачу из backlog можно добавить в спринт через
  `POST /api/v1/sprints/{sprintID}/tasks/{taskID}`.
- При удалении задачи из спринта она возвращается в backlog и получает `sprint_id = null`.
- При закрытии спринта незавершенные задачи автоматически возвращаются в backlog,
  а сам спринт переводится в `closed`.

Остановить:

```bash
docker compose down
```

Или:

```bash
make down
```
# hawler
