DROP TRIGGER IF EXISTS trg_columns_backlog_kind_invariants ON board_columns;
DROP FUNCTION IF EXISTS enforce_column_kind_backlog_invariants();

DROP TRIGGER IF EXISTS trg_tasks_backlog_sprint_invariants ON tasks;
DROP FUNCTION IF EXISTS enforce_task_backlog_sprint_invariants();
