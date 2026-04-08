# Hawler Backend (Go + Postgres)

Backend для студенческого task tracker (Jira/Trello-подобный).

## Что уже есть

- JWT auth:
  - `POST /api/v1/auth/register`
  - `POST /api/v1/auth/login`
  - `GET /api/v1/auth/me`
- Workspace роли: `owner`, `mentor`, `student`
- Управление участниками workspace:
  - `GET /api/v1/workspaces/{workspaceID}/members`
  - `POST /api/v1/workspaces/{workspaceID}/members`
- Бизнес-сущности:
  - `workspaces`
  - `projects`
  - `boards` (`kanban | scrum`)
  - `board_columns` (кастомные колонки на доске)
  - `sprints`
  - `tasks` (create/list/update/move/delete)
- Миграции в папке `backend/migrations` через `golang-migrate`.
- Для Docker Compose миграции накатываются отдельным сервисом `migrate` перед backend.

## Роли и доступ

- `owner`: полный доступ + управление участниками workspace
- `mentor`: создание `projects/boards/sprints`
- `student`: чтение + создание задач
- В workspace всегда должен оставаться хотя бы один `owner`
- `sprint` можно создавать только для `board.type = scrum`
- При создании доски автоматически создаются дефолтные колонки:
  - `kanban`: `todo`, `in_progress`, `done`
  - `scrum`: `backlog`, `todo`, `in_progress`, `done`
- Для задач основной маршрут перемещения/фильтрации теперь через `column_id` (`status` поддерживается как алиас по `kind`)

## Запуск

1. Подними Postgres и создай базу `hawler`.
2. Настрой переменные окружения из `.env.example`.
3. Запусти backend:

```bash
cd backend
go run ./cmd
```

## Миграции (golang-migrate)

- SQL-миграции лежат в `backend/migrations`.
- Формат файлов:
  - `NNNNNN_name.up.sql`
  - `NNNNNN_name.down.sql`
- В Docker Compose используется официальный образ `migrate/migrate`, который выполняет `up` до старта backend.

### Запуск через Docker Compose (backend + frontend + db)

```bash
cp .env.example .env
docker compose up --build
```

Сервисы после запуска:
- frontend: `http://localhost:5173`
- backend API: `http://localhost:8080`
- migrate: one-shot сервис для наката миграций перед backend

## Обязательные env

- `HTTP_ADDR` (по умолчанию `:8080`)
- `DATABASE_URL`
- `JWT_SECRET`
- `JWT_TTL` (например `72h`)

## Быстрый сценарий

1. Зарегистрируй пользователя:

```http
POST /api/v1/auth/register
Content-Type: application/json

{
  "name": "Vadim",
  "email": "vadim@example.com",
  "password": "strongpass123"
}
```

2. Возьми `token` из ответа и передавай:

```http
Authorization: Bearer <token>
```

3. Создай workspace (создатель автоматически `owner`):

```http
POST /api/v1/workspaces
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "Группа ИВТ-21"
}
```

4. Добавь участника в workspace:

```http
POST /api/v1/workspaces/{workspaceID}/members
Authorization: Bearer <owner-token>
Content-Type: application/json

{
  "email": "student@example.com",
  "role": "student"
}
```

5. Посмотри колонки доски:

```http
GET /api/v1/boards/{boardID}/columns
Authorization: Bearer <token>
```

6. Добавь новую колонку:

```http
POST /api/v1/boards/{boardID}/columns
Authorization: Bearer <owner-or-mentor-token>
Content-Type: application/json

{
  "name": "Review",
  "kind": "review",
  "position": 2
}
```

7. Измени колонку (название / kind / позицию):

```http
PATCH /api/v1/boards/{boardID}/columns/{columnID}
Authorization: Bearer <owner-or-mentor-token>
Content-Type: application/json

{
  "name": "Code Review",
  "kind": "review",
  "position": 2
}
```

8. Удали колонку:

```http
DELETE /api/v1/boards/{boardID}/columns/{columnID}?target_column_id=<another-column-id>
Authorization: Bearer <owner-or-mentor-token>
```

`target_column_id` обязателен, если в удаляемой колонке есть задачи.

9. Обнови задачу:

```http
PATCH /api/v1/tasks/{taskID}
Authorization: Bearer <token>
Content-Type: application/json

{
  "title": "Поднять API v2",
  "due_date": "2026-03-20",
  "sprint_id": "",
  "column_id": "<column-id>"
}
```

10. Перемести задачу по доске:

```http
PATCH /api/v1/tasks/{taskID}/move
Authorization: Bearer <token>
Content-Type: application/json

{
  "column_id": "<target-column-id>",
  "position": 0,
  "sprint_id": "<optional-sprint-id>"
}
```

Для Scrum:
- при переносе в `backlog` поле `sprint_id` очищается автоматически;
- при переносе из `backlog` в рабочую колонку нужно передать `sprint_id`, иначе API вернет ошибку;
- новая задача без `sprint_id` создается в `backlog` без привязки к спринту.

11. Удали задачу:

```http
DELETE /api/v1/tasks/{taskID}
Authorization: Bearer <token>
```

12. Запусти спринт:

```http
POST /api/v1/sprints/{sprintID}/start
Authorization: Bearer <owner-or-mentor-token>
```

13. Закрой спринт:

```http
POST /api/v1/sprints/{sprintID}/close
Authorization: Bearer <owner-or-mentor-token>
```

При закрытии спринта все незавершенные задачи автоматически переносятся в `backlog` и получают `sprint_id = null`.

14. Добавь задачу в спринт (backlog grooming):

```http
POST /api/v1/sprints/{sprintID}/tasks/{taskID}
Authorization: Bearer <owner-or-mentor-token>
```

Если задача в `backlog`, она автоматически переедет в `todo` (если колонка `todo` существует).

15. Убери задачу из спринта обратно в backlog:

```http
DELETE /api/v1/sprints/{sprintID}/tasks/{taskID}
Authorization: Bearer <owner-or-mentor-token>
```

16. Получи отчет по спринту:

```http
GET /api/v1/sprints/{sprintID}/report
Authorization: Bearer <token>
```

Ответ включает: `total_tasks`, `completed_tasks`, `remaining_tasks`, `velocity_tasks`, `completion_rate`, разбивку по колонкам.
