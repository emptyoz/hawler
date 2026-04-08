CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    email TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_lower ON users ((lower(email)));

CREATE TABLE IF NOT EXISTS workspaces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS workspace_members (
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('owner', 'mentor', 'student')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, user_id)
);

CREATE TABLE IF NOT EXISTS projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS boards (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('kanban', 'scrum')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS board_columns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    board_id UUID NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    kind TEXT NOT NULL,
    position INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(board_id, kind)
);

CREATE TABLE IF NOT EXISTS sprints (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    board_id UUID NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    goal TEXT NOT NULL DEFAULT '',
    starts_at DATE,
    ends_at DATE,
    status TEXT NOT NULL DEFAULT 'planned' CHECK (status IN ('planned', 'active', 'closed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    board_id UUID NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    column_id UUID REFERENCES board_columns(id) ON DELETE SET NULL,
    sprint_id UUID REFERENCES sprints(id) ON DELETE SET NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'todo',
    assignee TEXT NOT NULL DEFAULT '',
    due_date DATE,
    position INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE tasks ADD COLUMN IF NOT EXISTS column_id UUID REFERENCES board_columns(id) ON DELETE SET NULL;
ALTER TABLE board_columns DROP CONSTRAINT IF EXISTS board_columns_board_id_position_key;

CREATE INDEX IF NOT EXISTS idx_workspace_members_user ON workspace_members(user_id);
CREATE INDEX IF NOT EXISTS idx_projects_workspace ON projects(workspace_id);
CREATE INDEX IF NOT EXISTS idx_boards_project ON boards(project_id);
CREATE INDEX IF NOT EXISTS idx_columns_board ON board_columns(board_id);
CREATE INDEX IF NOT EXISTS idx_columns_board_position ON board_columns(board_id, position);
CREATE INDEX IF NOT EXISTS idx_tasks_board ON tasks(board_id);
CREATE INDEX IF NOT EXISTS idx_tasks_column ON tasks(column_id);
CREATE INDEX IF NOT EXISTS idx_tasks_sprint ON tasks(sprint_id);

INSERT INTO board_columns(board_id, name, kind, position)
SELECT b.id, v.name, v.kind, v.position
FROM boards b
JOIN (
  VALUES
    ('To Do', 'todo', 0),
    ('In Progress', 'in_progress', 1),
    ('Done', 'done', 2)
) AS v(name, kind, position) ON TRUE
WHERE b.type = 'kanban'
  AND NOT EXISTS (SELECT 1 FROM board_columns bc WHERE bc.board_id = b.id);

INSERT INTO board_columns(board_id, name, kind, position)
SELECT b.id, v.name, v.kind, v.position
FROM boards b
JOIN (
  VALUES
    ('Backlog', 'backlog', 0),
    ('To Do', 'todo', 1),
    ('In Progress', 'in_progress', 2),
    ('Done', 'done', 3)
) AS v(name, kind, position) ON TRUE
WHERE b.type = 'scrum'
  AND NOT EXISTS (SELECT 1 FROM board_columns bc WHERE bc.board_id = b.id);

UPDATE tasks t
SET column_id = bc.id
FROM board_columns bc
WHERE t.column_id IS NULL
  AND bc.board_id = t.board_id
  AND bc.kind = t.status;

UPDATE tasks t
SET column_id = (
  SELECT bc.id
  FROM board_columns bc
  WHERE bc.board_id = t.board_id
  ORDER BY bc.position ASC, bc.created_at ASC
  LIMIT 1
)
WHERE t.column_id IS NULL;

UPDATE tasks t
SET status = bc.kind
FROM board_columns bc
WHERE t.column_id = bc.id
  AND t.status <> bc.kind;
