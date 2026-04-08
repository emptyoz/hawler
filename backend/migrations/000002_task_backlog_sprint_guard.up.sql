-- Backfill inconsistent rows: backlog tasks must not be assigned to sprint.
UPDATE tasks AS t
SET sprint_id = NULL
FROM board_columns AS bc
WHERE t.column_id = bc.id
  AND bc.kind = 'backlog'
  AND t.sprint_id IS NOT NULL;

CREATE OR REPLACE FUNCTION enforce_task_backlog_sprint_invariants()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  col_kind TEXT;
BEGIN
  IF NEW.column_id IS NULL THEN
    RETURN NEW;
  END IF;

  SELECT kind
    INTO col_kind
  FROM board_columns
  WHERE id = NEW.column_id;

  IF col_kind = 'backlog' AND NEW.sprint_id IS NOT NULL THEN
    RAISE EXCEPTION 'backlog task cannot have sprint_id';
  END IF;

  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_tasks_backlog_sprint_invariants ON tasks;
CREATE TRIGGER trg_tasks_backlog_sprint_invariants
BEFORE INSERT OR UPDATE OF column_id, sprint_id
ON tasks
FOR EACH ROW
EXECUTE FUNCTION enforce_task_backlog_sprint_invariants();

CREATE OR REPLACE FUNCTION enforce_column_kind_backlog_invariants()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF NEW.kind = 'backlog' AND OLD.kind IS DISTINCT FROM NEW.kind THEN
    IF EXISTS (
      SELECT 1
      FROM tasks
      WHERE column_id = NEW.id
        AND sprint_id IS NOT NULL
    ) THEN
      RAISE EXCEPTION 'cannot set column kind to backlog while sprint tasks exist';
    END IF;
  END IF;

  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_columns_backlog_kind_invariants ON board_columns;
CREATE TRIGGER trg_columns_backlog_kind_invariants
BEFORE UPDATE OF kind
ON board_columns
FOR EACH ROW
EXECUTE FUNCTION enforce_column_kind_backlog_invariants();
