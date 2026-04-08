# Hawler Frontend (Vue 3 + Vite)

## Требования

- Node.js 20+
- npm 10+
- Запущенный backend на `http://localhost:8080`

## Настройка

1. Создай env-файл:

```bash
cd frontend
cp .env.example .env
```

2. Установи зависимости:

```bash
npm install
```

3. Запусти dev server:

```bash
npm run dev
```

## Быстрый запуск всего проекта в Docker

Из корня проекта:

```bash
cp .env.example .env
docker compose up --build
```

После старта:
- frontend: `http://localhost:5173`
- backend API: `http://localhost:8080`

## Переменные

- `VITE_API_BASE_URL` — адрес backend API (`http://localhost:8080` по умолчанию)

## Что уже работает

- JWT auth (`register/login/me`)
- Иерархия workspace → project → board
- Участники workspace: просмотр, добавление по email, смена роли (только `owner`)
- Kanban/Scrum board, колонки и задачи
- Управление колонками: создание/редактирование/удаление (с переносом задач при удалении)
- Перетаскивание задач между колонками и внутри колонки
- Встроенное редактирование задач (`title/description/assignee/due date/sprint`)
- Фильтры задач (поиск, `assignee`, `sprint/backlog`)
- Подтверждение удаления задач/колонок + toast-уведомления об операциях
- UI с учетом роли: `owner/mentor` элементы управления для `project/board/sprint`
- Sprint (создание/запуск/закрытие), отчет по sprint
- Интерфейс декомпозирован на компоненты: `ControlSidebar`, `BoardPanel`, `ToastStack`
