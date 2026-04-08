package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	RoleOwner   = "owner"
	RoleMentor  = "mentor"
	RoleStudent = "student"
)

var (
	ErrColumnHasTasks         = errors.New("column has tasks")
	ErrCannotDeleteLastColumn = errors.New("cannot delete last column")
	ErrCannotDeleteBacklog    = errors.New("cannot delete backlog column")
	ErrCannotModifyBacklog    = errors.New("cannot modify backlog column")
	ErrInvalidColumnTarget    = errors.New("invalid column target")
	ErrSprintNotPlanned       = errors.New("sprint is not planned")
	ErrSprintNotActive        = errors.New("sprint is not active")
	ErrSprintClosed           = errors.New("sprint is closed")
	ErrSprintBoardNotScrum    = errors.New("sprint board is not scrum")
	ErrActiveSprintExists     = errors.New("active sprint already exists")
	ErrBacklogColumnNotFound  = errors.New("backlog column not found")
	ErrTaskBoardMismatch      = errors.New("task does not belong to sprint board")
	ErrTaskAlreadyInSprint    = errors.New("task is already in sprint")
	ErrTaskNotInSprint        = errors.New("task is not in sprint")
	ErrSprintRequiredForTask  = errors.New("sprint is required for non-backlog scrum task")
)

type Store struct {
	db *sql.DB
}

type User struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type UserAuth struct {
	User
	PasswordHash string
}

type Workspace struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Role      string    `json:"role,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type WorkspaceMember struct {
	WorkspaceID string    `json:"workspace_id"`
	UserID      string    `json:"user_id"`
	Name        string    `json:"name,omitempty"`
	Email       string    `json:"email,omitempty"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
}

type Project struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type Board struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
}

type Sprint struct {
	ID        string     `json:"id"`
	BoardID   string     `json:"board_id"`
	Name      string     `json:"name"`
	Goal      string     `json:"goal"`
	StartsAt  *time.Time `json:"starts_at,omitempty"`
	EndsAt    *time.Time `json:"ends_at,omitempty"`
	Status    string     `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
}

type BoardColumn struct {
	ID        string    `json:"id"`
	BoardID   string    `json:"board_id"`
	Name      string    `json:"name"`
	Kind      string    `json:"kind"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"created_at"`
}

type Task struct {
	ID          string     `json:"id"`
	BoardID     string     `json:"board_id"`
	ColumnID    *string    `json:"column_id,omitempty"`
	SprintID    *string    `json:"sprint_id,omitempty"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	Assignee    string     `json:"assignee"`
	DueDate     *time.Time `json:"due_date,omitempty"`
	Position    int        `json:"position"`
	CreatedAt   time.Time  `json:"created_at"`
}

type UpdateTaskInput struct {
	Title       *string
	Description *string
	Assignee    *string
	DueDateSet  bool
	DueDate     *time.Time
	SprintIDSet bool
	SprintID    *string
}

type UpdateBoardColumnInput struct {
	Name     *string
	Kind     *string
	Position *int
}

type SprintCloseResult struct {
	Sprint     Sprint `json:"sprint"`
	MovedTasks int    `json:"moved_tasks"`
}

type SprintReportColumn struct {
	ColumnID   string `json:"column_id"`
	ColumnName string `json:"column_name"`
	ColumnKind string `json:"column_kind"`
	Tasks      int    `json:"tasks"`
}

type SprintReport struct {
	Sprint         Sprint               `json:"sprint"`
	TotalTasks     int                  `json:"total_tasks"`
	CompletedTasks int                  `json:"completed_tasks"`
	RemainingTasks int                  `json:"remaining_tasks"`
	VelocityTasks  int                  `json:"velocity_tasks"`
	CompletionRate float64              `json:"completion_rate"`
	Columns        []SprintReportColumn `json:"columns"`
}

func normalizedOptionalID(raw *string) string {
	if raw == nil {
		return ""
	}
	return strings.TrimSpace(*raw)
}

func resolveNextSprintBinding(boardType, targetStatus, currentSprintID string, targetSprintID *string) (sql.NullString, error) {
	currentSprintID = strings.TrimSpace(currentSprintID)
	requestedSprintID := normalizedOptionalID(targetSprintID)

	switch {
	case targetStatus == "backlog":
		return sql.NullString{}, nil
	case requestedSprintID != "":
		return sql.NullString{String: requestedSprintID, Valid: true}, nil
	case currentSprintID != "":
		return sql.NullString{String: currentSprintID, Valid: true}, nil
	case boardType == "scrum":
		return sql.NullString{}, ErrSprintRequiredForTask
	default:
		return sql.NullString{}, nil
	}
}

func New(databaseURL string) (*Store, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("sql open: %w", err)
	}

	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("db ping: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) CreateUser(ctx context.Context, name, email, passwordHash string) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO users(name, email, password_hash)
		 VALUES ($1, $2, $3)
		 RETURNING id::text, name, email, created_at`,
		name, strings.ToLower(email), passwordHash,
	).Scan(&u.ID, &u.Name, &u.Email, &u.CreatedAt)
	if err != nil {
		return User{}, err
	}
	return u, nil
}

func (s *Store) GetUserByID(ctx context.Context, userID string) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx,
		`SELECT id::text, name, email, created_at FROM users WHERE id = $1`,
		userID,
	).Scan(&u.ID, &u.Name, &u.Email, &u.CreatedAt)
	if err != nil {
		return User{}, err
	}
	return u, nil
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx,
		`SELECT id::text, name, email, created_at FROM users WHERE lower(email) = lower($1)`,
		email,
	).Scan(&u.ID, &u.Name, &u.Email, &u.CreatedAt)
	if err != nil {
		return User{}, err
	}
	return u, nil
}

func (s *Store) GetUserAuthByEmail(ctx context.Context, email string) (UserAuth, error) {
	var u UserAuth
	err := s.db.QueryRowContext(ctx,
		`SELECT id::text, name, email, password_hash, created_at
		 FROM users
		 WHERE lower(email) = lower($1)`,
		email,
	).Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		return UserAuth{}, err
	}
	return u, nil
}

func (s *Store) CreateWorkspace(ctx context.Context, name, ownerUserID string) (Workspace, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Workspace{}, err
	}
	defer tx.Rollback()

	var w Workspace
	err = tx.QueryRowContext(ctx,
		`INSERT INTO workspaces(name)
		 VALUES ($1)
		 RETURNING id::text, name, created_at`,
		name,
	).Scan(&w.ID, &w.Name, &w.CreatedAt)
	if err != nil {
		return Workspace{}, err
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO workspace_members(workspace_id, user_id, role)
		 VALUES ($1, $2, 'owner')`,
		w.ID, ownerUserID,
	); err != nil {
		return Workspace{}, err
	}

	if err := tx.Commit(); err != nil {
		return Workspace{}, err
	}
	w.Role = RoleOwner
	return w, nil
}

func (s *Store) ListWorkspaces(ctx context.Context, userID string) ([]Workspace, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT w.id::text, w.name, wm.role, w.created_at
		 FROM workspaces w
		 JOIN workspace_members wm ON wm.workspace_id = w.id
		 WHERE wm.user_id = $1
		 ORDER BY w.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Workspace, 0)
	for rows.Next() {
		var w Workspace
		if err := rows.Scan(&w.ID, &w.Name, &w.Role, &w.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, w)
	}
	return items, rows.Err()
}

func (s *Store) UpsertWorkspaceMember(ctx context.Context, workspaceID, userID, role string) (WorkspaceMember, error) {
	var m WorkspaceMember
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO workspace_members(workspace_id, user_id, role)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (workspace_id, user_id)
		 DO UPDATE SET role = EXCLUDED.role
		 RETURNING workspace_id::text, user_id::text, role, created_at`,
		workspaceID, userID, role,
	).Scan(&m.WorkspaceID, &m.UserID, &m.Role, &m.CreatedAt)
	if err != nil {
		return WorkspaceMember{}, err
	}
	return m, nil
}

func (s *Store) ListWorkspaceMembers(ctx context.Context, workspaceID string) ([]WorkspaceMember, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT wm.workspace_id::text, wm.user_id::text, u.name, u.email, wm.role, wm.created_at
		 FROM workspace_members wm
		 JOIN users u ON u.id = wm.user_id
		 WHERE wm.workspace_id = $1
		 ORDER BY wm.created_at ASC`,
		workspaceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]WorkspaceMember, 0)
	for rows.Next() {
		var m WorkspaceMember
		if err := rows.Scan(&m.WorkspaceID, &m.UserID, &m.Name, &m.Email, &m.Role, &m.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, m)
	}
	return items, rows.Err()
}

func (s *Store) UserRoleInWorkspace(ctx context.Context, workspaceID, userID string) (string, error) {
	var role string
	err := s.db.QueryRowContext(ctx,
		`SELECT role FROM workspace_members WHERE workspace_id = $1 AND user_id = $2`,
		workspaceID, userID,
	).Scan(&role)
	if err != nil {
		return "", err
	}
	return role, nil
}

func (s *Store) CountWorkspaceOwners(ctx context.Context, workspaceID string) (int, error) {
	var cnt int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM workspace_members WHERE workspace_id = $1 AND role = 'owner'`,
		workspaceID,
	).Scan(&cnt)
	if err != nil {
		return 0, err
	}
	return cnt, nil
}

func (s *Store) WorkspaceIDByProjectID(ctx context.Context, projectID string) (string, error) {
	var workspaceID string
	err := s.db.QueryRowContext(ctx,
		`SELECT workspace_id::text FROM projects WHERE id = $1`,
		projectID,
	).Scan(&workspaceID)
	if err != nil {
		return "", err
	}
	return workspaceID, nil
}

func (s *Store) WorkspaceAndTypeByBoardID(ctx context.Context, boardID string) (string, string, error) {
	var workspaceID string
	var boardType string
	err := s.db.QueryRowContext(ctx,
		`SELECT p.workspace_id::text, b.type
		 FROM boards b
		 JOIN projects p ON p.id = b.project_id
		 WHERE b.id = $1`,
		boardID,
	).Scan(&workspaceID, &boardType)
	if err != nil {
		return "", "", err
	}
	return workspaceID, boardType, nil
}

func (s *Store) WorkspaceIDByBoardID(ctx context.Context, boardID string) (string, error) {
	var workspaceID string
	err := s.db.QueryRowContext(ctx,
		`SELECT p.workspace_id::text
		 FROM boards b
		 JOIN projects p ON p.id = b.project_id
		 WHERE b.id = $1`,
		boardID,
	).Scan(&workspaceID)
	if err != nil {
		return "", err
	}
	return workspaceID, nil
}

func (s *Store) SprintBelongsToBoard(ctx context.Context, sprintID, boardID string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM sprints WHERE id = $1 AND board_id = $2)`,
		sprintID, boardID,
	).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (s *Store) CreateProject(ctx context.Context, workspaceID, name, description string) (Project, error) {
	var p Project
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO projects(workspace_id, name, description)
		 VALUES ($1, $2, $3)
		 RETURNING id::text, workspace_id::text, name, description, created_at`,
		workspaceID, name, description,
	).Scan(&p.ID, &p.WorkspaceID, &p.Name, &p.Description, &p.CreatedAt)
	if err != nil {
		return Project{}, err
	}
	return p, nil
}

func (s *Store) ListProjects(ctx context.Context, workspaceID string) ([]Project, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id::text, workspace_id::text, name, description, created_at
		 FROM projects
		 WHERE workspace_id = $1
		 ORDER BY created_at DESC`,
		workspaceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Project, 0)
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.WorkspaceID, &p.Name, &p.Description, &p.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	return items, rows.Err()
}

func (s *Store) CreateBoard(ctx context.Context, projectID, name, boardType string) (Board, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Board{}, err
	}
	defer tx.Rollback()

	var b Board
	err = tx.QueryRowContext(ctx,
		`INSERT INTO boards(project_id, name, type)
		 VALUES ($1, $2, $3)
		 RETURNING id::text, project_id::text, name, type, created_at`,
		projectID, name, boardType,
	).Scan(&b.ID, &b.ProjectID, &b.Name, &b.Type, &b.CreatedAt)
	if err != nil {
		return Board{}, err
	}

	if err := s.insertDefaultColumnsTx(ctx, tx, b.ID, b.Type); err != nil {
		return Board{}, err
	}

	if err := tx.Commit(); err != nil {
		return Board{}, err
	}
	return b, nil
}

func (s *Store) ListBoards(ctx context.Context, projectID string) ([]Board, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id::text, project_id::text, name, type, created_at
		 FROM boards
		 WHERE project_id = $1
		 ORDER BY created_at DESC`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Board, 0)
	for rows.Next() {
		var b Board
		if err := rows.Scan(&b.ID, &b.ProjectID, &b.Name, &b.Type, &b.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, b)
	}
	return items, rows.Err()
}

func (s *Store) insertDefaultColumnsTx(ctx context.Context, tx *sql.Tx, boardID, boardType string) error {
	var defaults []struct {
		name string
		kind string
	}
	switch boardType {
	case "scrum":
		defaults = []struct {
			name string
			kind string
		}{
			{name: "Backlog", kind: "backlog"},
			{name: "To Do", kind: "todo"},
			{name: "In Progress", kind: "in_progress"},
			{name: "Done", kind: "done"},
		}
	default:
		defaults = []struct {
			name string
			kind string
		}{
			{name: "To Do", kind: "todo"},
			{name: "In Progress", kind: "in_progress"},
			{name: "Done", kind: "done"},
		}
	}

	for i, item := range defaults {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO board_columns(board_id, name, kind, position)
			 VALUES ($1, $2, $3, $4)`,
			boardID, item.name, item.kind, i,
		); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListBoardColumns(ctx context.Context, boardID string) ([]BoardColumn, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id::text, board_id::text, name, kind, position, created_at
		 FROM board_columns
		 WHERE board_id = $1
		 ORDER BY position ASC, created_at ASC`,
		boardID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]BoardColumn, 0)
	for rows.Next() {
		var c BoardColumn
		if err := rows.Scan(&c.ID, &c.BoardID, &c.Name, &c.Kind, &c.Position, &c.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

func (s *Store) GetBoardColumnByID(ctx context.Context, columnID string) (BoardColumn, error) {
	var c BoardColumn
	err := s.db.QueryRowContext(ctx,
		`SELECT id::text, board_id::text, name, kind, position, created_at
		 FROM board_columns
		 WHERE id = $1`,
		columnID,
	).Scan(&c.ID, &c.BoardID, &c.Name, &c.Kind, &c.Position, &c.CreatedAt)
	if err != nil {
		return BoardColumn{}, err
	}
	return c, nil
}

func (s *Store) GetBoardColumnByKind(ctx context.Context, boardID, kind string) (BoardColumn, error) {
	var c BoardColumn
	err := s.db.QueryRowContext(ctx,
		`SELECT id::text, board_id::text, name, kind, position, created_at
		 FROM board_columns
		 WHERE board_id = $1 AND kind = $2`,
		boardID, kind,
	).Scan(&c.ID, &c.BoardID, &c.Name, &c.Kind, &c.Position, &c.CreatedAt)
	if err != nil {
		return BoardColumn{}, err
	}
	return c, nil
}

func (s *Store) GetBoardFirstColumn(ctx context.Context, boardID string) (BoardColumn, error) {
	var c BoardColumn
	err := s.db.QueryRowContext(ctx,
		`SELECT id::text, board_id::text, name, kind, position, created_at
		 FROM board_columns
		 WHERE board_id = $1
		 ORDER BY position ASC, created_at ASC
		 LIMIT 1`,
		boardID,
	).Scan(&c.ID, &c.BoardID, &c.Name, &c.Kind, &c.Position, &c.CreatedAt)
	if err != nil {
		return BoardColumn{}, err
	}
	return c, nil
}

func (s *Store) BoardColumnBelongsToBoard(ctx context.Context, columnID, boardID string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM board_columns WHERE id = $1 AND board_id = $2)`,
		columnID, boardID,
	).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (s *Store) CreateBoardColumn(ctx context.Context, boardID, name, kind string, position int) (BoardColumn, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return BoardColumn{}, err
	}
	defer tx.Rollback()

	if position < 0 {
		position = 0
	}

	var total int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM board_columns WHERE board_id = $1`,
		boardID,
	).Scan(&total); err != nil {
		return BoardColumn{}, err
	}
	if position > total {
		position = total
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE board_columns
		 SET position = position + 1
		 WHERE board_id = $1 AND position >= $2`,
		boardID, position,
	); err != nil {
		return BoardColumn{}, err
	}

	var c BoardColumn
	err = tx.QueryRowContext(ctx,
		`INSERT INTO board_columns(board_id, name, kind, position)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id::text, board_id::text, name, kind, position, created_at`,
		boardID, name, kind, position,
	).Scan(&c.ID, &c.BoardID, &c.Name, &c.Kind, &c.Position, &c.CreatedAt)
	if err != nil {
		return BoardColumn{}, err
	}

	if err := tx.Commit(); err != nil {
		return BoardColumn{}, err
	}
	return c, nil
}

func (s *Store) UpdateBoardColumn(
	ctx context.Context,
	boardID, columnID string,
	input UpdateBoardColumnInput,
) (BoardColumn, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return BoardColumn{}, err
	}
	defer tx.Rollback()

	var current BoardColumn
	err = tx.QueryRowContext(ctx,
		`SELECT id::text, board_id::text, name, kind, position, created_at
		 FROM board_columns
		 WHERE id = $1 AND board_id = $2
		 FOR UPDATE`,
		columnID, boardID,
	).Scan(&current.ID, &current.BoardID, &current.Name, &current.Kind, &current.Position, &current.CreatedAt)
	if err != nil {
		return BoardColumn{}, err
	}
	if current.Kind == "backlog" && (input.Name != nil || input.Kind != nil || input.Position != nil) {
		return BoardColumn{}, ErrCannotModifyBacklog
	}

	targetName := current.Name
	if input.Name != nil {
		targetName = *input.Name
	}
	targetKind := current.Kind
	if input.Kind != nil {
		targetKind = *input.Kind
	}
	targetPosition := current.Position
	if input.Position != nil {
		targetPosition = *input.Position
		if targetPosition < 0 {
			targetPosition = 0
		}

		var total int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM board_columns WHERE board_id = $1`,
			boardID,
		).Scan(&total); err != nil {
			return BoardColumn{}, err
		}
		maxPos := total - 1
		if maxPos < 0 {
			maxPos = 0
		}
		if targetPosition > maxPos {
			targetPosition = maxPos
		}

		if targetPosition > current.Position {
			if _, err := tx.ExecContext(ctx,
				`UPDATE board_columns
				 SET position = position - 1
				 WHERE board_id = $1
				   AND position > $2
				   AND position <= $3
				   AND id <> $4`,
				boardID, current.Position, targetPosition, current.ID,
			); err != nil {
				return BoardColumn{}, err
			}
		} else if targetPosition < current.Position {
			if _, err := tx.ExecContext(ctx,
				`UPDATE board_columns
				 SET position = position + 1
				 WHERE board_id = $1
				   AND position >= $2
				   AND position < $3
				   AND id <> $4`,
				boardID, targetPosition, current.Position, current.ID,
			); err != nil {
				return BoardColumn{}, err
			}
		}
	}

	var updated BoardColumn
	err = tx.QueryRowContext(ctx,
		`UPDATE board_columns
		 SET name = $1, kind = $2, position = $3
		 WHERE id = $4
		 RETURNING id::text, board_id::text, name, kind, position, created_at`,
		targetName, targetKind, targetPosition, current.ID,
	).Scan(&updated.ID, &updated.BoardID, &updated.Name, &updated.Kind, &updated.Position, &updated.CreatedAt)
	if err != nil {
		return BoardColumn{}, err
	}

	if targetKind != current.Kind {
		if _, err := tx.ExecContext(ctx,
			`UPDATE tasks
			 SET status = $1
			 WHERE board_id = $2
			   AND column_id = $3`,
			targetKind, boardID, current.ID,
		); err != nil {
			return BoardColumn{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return BoardColumn{}, err
	}
	return updated, nil
}

func (s *Store) DeleteBoardColumn(ctx context.Context, boardID, columnID string, targetColumnID *string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var current BoardColumn
	err = tx.QueryRowContext(ctx,
		`SELECT id::text, board_id::text, name, kind, position, created_at
		 FROM board_columns
		 WHERE id = $1 AND board_id = $2
		 FOR UPDATE`,
		columnID, boardID,
	).Scan(&current.ID, &current.BoardID, &current.Name, &current.Kind, &current.Position, &current.CreatedAt)
	if err != nil {
		return err
	}
	if current.Kind == "backlog" {
		return ErrCannotDeleteBacklog
	}

	var totalColumns int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM board_columns WHERE board_id = $1`,
		boardID,
	).Scan(&totalColumns); err != nil {
		return err
	}
	if totalColumns <= 1 {
		return ErrCannotDeleteLastColumn
	}

	var tasksInColumn int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tasks WHERE board_id = $1 AND column_id = $2`,
		boardID, current.ID,
	).Scan(&tasksInColumn); err != nil {
		return err
	}

	if tasksInColumn > 0 {
		if targetColumnID == nil || strings.TrimSpace(*targetColumnID) == "" {
			return ErrColumnHasTasks
		}
		targetID := strings.TrimSpace(*targetColumnID)
		if targetID == current.ID {
			return ErrInvalidColumnTarget
		}

		var targetKind string
		if err := tx.QueryRowContext(ctx,
			`SELECT kind FROM board_columns WHERE id = $1 AND board_id = $2`,
			targetID, boardID,
		).Scan(&targetKind); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrInvalidColumnTarget
			}
			return err
		}

		var targetCount int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM tasks WHERE board_id = $1 AND column_id = $2`,
			boardID, targetID,
		).Scan(&targetCount); err != nil {
			return err
		}

		// Keep relative order of moved tasks and append after current target lane.
		if _, err := tx.ExecContext(ctx,
			`UPDATE tasks AS t
			 SET column_id = $1,
			     status = $2,
			     position = $3 + src.rn - 1
			 FROM (
			   SELECT id, row_number() OVER (ORDER BY position ASC, created_at ASC) AS rn
			   FROM tasks
			   WHERE board_id = $4 AND column_id = $5
			 ) AS src
			 WHERE t.id = src.id`,
			targetID, targetKind, targetCount, boardID, current.ID,
		); err != nil {
			return err
		}
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM board_columns WHERE id = $1`,
		current.ID,
	); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE board_columns
		 SET position = position - 1
		 WHERE board_id = $1
		   AND position > $2`,
		boardID, current.Position,
	); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (s *Store) CreateSprint(
	ctx context.Context,
	boardID, name, goal string,
	startsAt, endsAt *time.Time,
	status string,
) (Sprint, error) {
	var sp Sprint
	var rawStartsAt sql.NullTime
	var rawEndsAt sql.NullTime

	err := s.db.QueryRowContext(ctx,
		`INSERT INTO sprints(board_id, name, goal, starts_at, ends_at, status)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id::text, board_id::text, name, goal, starts_at, ends_at, status, created_at`,
		boardID, name, goal, startsAt, endsAt, status,
	).Scan(&sp.ID, &sp.BoardID, &sp.Name, &sp.Goal, &rawStartsAt, &rawEndsAt, &sp.Status, &sp.CreatedAt)
	if err != nil {
		return Sprint{}, err
	}

	if rawStartsAt.Valid {
		sp.StartsAt = &rawStartsAt.Time
	}
	if rawEndsAt.Valid {
		sp.EndsAt = &rawEndsAt.Time
	}
	return sp, nil
}

func (s *Store) ListSprints(ctx context.Context, boardID, status string) ([]Sprint, error) {
	parts := []string{`SELECT id::text, board_id::text, name, goal, starts_at, ends_at, status, created_at FROM sprints`}
	where := make([]string, 0, 2)
	args := make([]any, 0, 2)
	idx := 1

	if boardID != "" {
		where = append(where, fmt.Sprintf("board_id = $%d", idx))
		args = append(args, boardID)
		idx++
	}
	if status != "" {
		where = append(where, fmt.Sprintf("status = $%d", idx))
		args = append(args, status)
	}
	if len(where) > 0 {
		parts = append(parts, "WHERE "+strings.Join(where, " AND "))
	}
	parts = append(parts, "ORDER BY created_at DESC")

	rows, err := s.db.QueryContext(ctx, strings.Join(parts, " "), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Sprint, 0)
	for rows.Next() {
		var sp Sprint
		var rawStartsAt sql.NullTime
		var rawEndsAt sql.NullTime
		if err := rows.Scan(&sp.ID, &sp.BoardID, &sp.Name, &sp.Goal, &rawStartsAt, &rawEndsAt, &sp.Status, &sp.CreatedAt); err != nil {
			return nil, err
		}
		if rawStartsAt.Valid {
			sp.StartsAt = &rawStartsAt.Time
		}
		if rawEndsAt.Valid {
			sp.EndsAt = &rawEndsAt.Time
		}
		items = append(items, sp)
	}
	return items, rows.Err()
}

func (s *Store) GetSprintByID(ctx context.Context, sprintID string) (Sprint, error) {
	var sp Sprint
	var rawStartsAt sql.NullTime
	var rawEndsAt sql.NullTime
	err := s.db.QueryRowContext(ctx,
		`SELECT id::text, board_id::text, name, goal, starts_at, ends_at, status, created_at
		 FROM sprints
		 WHERE id = $1`,
		sprintID,
	).Scan(&sp.ID, &sp.BoardID, &sp.Name, &sp.Goal, &rawStartsAt, &rawEndsAt, &sp.Status, &sp.CreatedAt)
	if err != nil {
		return Sprint{}, err
	}
	if rawStartsAt.Valid {
		sp.StartsAt = &rawStartsAt.Time
	}
	if rawEndsAt.Valid {
		sp.EndsAt = &rawEndsAt.Time
	}
	return sp, nil
}

func (s *Store) StartSprint(ctx context.Context, sprintID string) (Sprint, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Sprint{}, err
	}
	defer tx.Rollback()

	var boardID string
	var boardType string
	var status string
	err = tx.QueryRowContext(ctx,
		`SELECT s.board_id::text, b.type, s.status
		 FROM sprints s
		 JOIN boards b ON b.id = s.board_id
		 WHERE s.id = $1
		 FOR UPDATE`,
		sprintID,
	).Scan(&boardID, &boardType, &status)
	if err != nil {
		return Sprint{}, err
	}

	if boardType != "scrum" {
		return Sprint{}, ErrSprintBoardNotScrum
	}
	if status != "planned" {
		return Sprint{}, ErrSprintNotPlanned
	}

	var activeExists bool
	if err := tx.QueryRowContext(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM sprints
			WHERE board_id = $1
			  AND status = 'active'
			  AND id <> $2
		)`,
		boardID, sprintID,
	).Scan(&activeExists); err != nil {
		return Sprint{}, err
	}
	if activeExists {
		return Sprint{}, ErrActiveSprintExists
	}

	var updated Sprint
	var rawStartsAt sql.NullTime
	var rawEndsAt sql.NullTime
	err = tx.QueryRowContext(ctx,
		`UPDATE sprints
		 SET status = 'active',
		     starts_at = COALESCE(starts_at, CURRENT_DATE)
		 WHERE id = $1
		 RETURNING id::text, board_id::text, name, goal, starts_at, ends_at, status, created_at`,
		sprintID,
	).Scan(&updated.ID, &updated.BoardID, &updated.Name, &updated.Goal, &rawStartsAt, &rawEndsAt, &updated.Status, &updated.CreatedAt)
	if err != nil {
		return Sprint{}, err
	}
	if rawStartsAt.Valid {
		updated.StartsAt = &rawStartsAt.Time
	}
	if rawEndsAt.Valid {
		updated.EndsAt = &rawEndsAt.Time
	}

	if err := tx.Commit(); err != nil {
		return Sprint{}, err
	}
	return updated, nil
}

func (s *Store) CloseSprint(ctx context.Context, sprintID string) (SprintCloseResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return SprintCloseResult{}, err
	}
	defer tx.Rollback()

	var boardID string
	var boardType string
	var sprintStatus string
	err = tx.QueryRowContext(ctx,
		`SELECT s.board_id::text, b.type, s.status
		 FROM sprints s
		 JOIN boards b ON b.id = s.board_id
		 WHERE s.id = $1
		 FOR UPDATE`,
		sprintID,
	).Scan(&boardID, &boardType, &sprintStatus)
	if err != nil {
		return SprintCloseResult{}, err
	}

	if boardType != "scrum" {
		return SprintCloseResult{}, ErrSprintBoardNotScrum
	}
	if sprintStatus != "active" {
		return SprintCloseResult{}, ErrSprintNotActive
	}

	var backlogColumnID string
	var backlogKind string
	if err := tx.QueryRowContext(ctx,
		`SELECT id::text, kind
		 FROM board_columns
		 WHERE board_id = $1 AND kind = 'backlog'`,
		boardID,
	).Scan(&backlogColumnID, &backlogKind); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SprintCloseResult{}, ErrBacklogColumnNotFound
		}
		return SprintCloseResult{}, err
	}

	var doneColumnID sql.NullString
	doneErr := tx.QueryRowContext(ctx,
		`SELECT id::text
		 FROM board_columns
		 WHERE board_id = $1 AND kind = 'done'`,
		boardID,
	).Scan(&doneColumnID)
	if doneErr != nil && !errors.Is(doneErr, sql.ErrNoRows) {
		return SprintCloseResult{}, doneErr
	}

	var backlogCount int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tasks WHERE board_id = $1 AND column_id = $2`,
		boardID, backlogColumnID,
	).Scan(&backlogCount); err != nil {
		return SprintCloseResult{}, err
	}

	var moveRes sql.Result
	if doneColumnID.Valid && doneColumnID.String != "" {
		moveRes, err = tx.ExecContext(ctx,
			`UPDATE tasks AS t
			 SET column_id = $1,
			     status = $2,
			     sprint_id = NULL,
			     position = $3 + src.rn - 1
			 FROM (
			   SELECT t2.id, row_number() OVER (ORDER BY COALESCE(bc.position, 9999), t2.position ASC, t2.created_at ASC) AS rn
			   FROM tasks t2
			   LEFT JOIN board_columns bc ON bc.id = t2.column_id
			   WHERE t2.board_id = $4
			     AND t2.sprint_id = $5
			     AND (t2.column_id IS DISTINCT FROM $6)
			 ) AS src
			 WHERE t.id = src.id`,
			backlogColumnID, backlogKind, backlogCount, boardID, sprintID, doneColumnID.String,
		)
	} else {
		moveRes, err = tx.ExecContext(ctx,
			`UPDATE tasks AS t
			 SET column_id = $1,
			     status = $2,
			     sprint_id = NULL,
			     position = $3 + src.rn - 1
			 FROM (
			   SELECT t2.id, row_number() OVER (ORDER BY COALESCE(bc.position, 9999), t2.position ASC, t2.created_at ASC) AS rn
			   FROM tasks t2
			   LEFT JOIN board_columns bc ON bc.id = t2.column_id
			   WHERE t2.board_id = $4
			     AND t2.sprint_id = $5
			 ) AS src
			 WHERE t.id = src.id`,
			backlogColumnID, backlogKind, backlogCount, boardID, sprintID,
		)
	}
	if err != nil {
		return SprintCloseResult{}, err
	}
	movedRows, err := moveRes.RowsAffected()
	if err != nil {
		return SprintCloseResult{}, err
	}

	if _, err := tx.ExecContext(ctx,
		`WITH ranked AS (
		   SELECT id, row_number() OVER (PARTITION BY column_id ORDER BY position ASC, created_at ASC) - 1 AS new_position
		   FROM tasks
		   WHERE board_id = $1
		 )
		 UPDATE tasks t
		 SET position = ranked.new_position
		 FROM ranked
		 WHERE t.id = ranked.id
		   AND t.position <> ranked.new_position`,
		boardID,
	); err != nil {
		return SprintCloseResult{}, err
	}

	var closed Sprint
	var rawStartsAt sql.NullTime
	var rawEndsAt sql.NullTime
	err = tx.QueryRowContext(ctx,
		`UPDATE sprints
		 SET status = 'closed',
		     ends_at = COALESCE(ends_at, CURRENT_DATE)
		 WHERE id = $1
		 RETURNING id::text, board_id::text, name, goal, starts_at, ends_at, status, created_at`,
		sprintID,
	).Scan(&closed.ID, &closed.BoardID, &closed.Name, &closed.Goal, &rawStartsAt, &rawEndsAt, &closed.Status, &closed.CreatedAt)
	if err != nil {
		return SprintCloseResult{}, err
	}
	if rawStartsAt.Valid {
		closed.StartsAt = &rawStartsAt.Time
	}
	if rawEndsAt.Valid {
		closed.EndsAt = &rawEndsAt.Time
	}

	if err := tx.Commit(); err != nil {
		return SprintCloseResult{}, err
	}

	return SprintCloseResult{
		Sprint:     closed,
		MovedTasks: int(movedRows),
	}, nil
}

func (s *Store) AddTaskToSprint(ctx context.Context, sprintID, taskID string) (Task, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Task{}, err
	}
	defer tx.Rollback()

	var boardID string
	var boardType string
	err = tx.QueryRowContext(ctx,
		`SELECT s.board_id::text, b.type
		 FROM sprints s
		 JOIN boards b ON b.id = s.board_id
		 WHERE s.id = $1
		 FOR UPDATE`,
		sprintID,
	).Scan(&boardID, &boardType)
	if err != nil {
		return Task{}, err
	}

	if boardType != "scrum" {
		return Task{}, ErrSprintBoardNotScrum
	}

	task, currentColumnKind, err := getTaskForUpdateTx(ctx, tx, taskID)
	if err != nil {
		return Task{}, err
	}
	if task.BoardID != boardID {
		return Task{}, ErrTaskBoardMismatch
	}
	if task.SprintID != nil && *task.SprintID == sprintID {
		return Task{}, ErrTaskAlreadyInSprint
	}

	targetColumnID := ""
	targetStatus := task.Status
	targetPosition := task.Position

	if currentColumnKind == "backlog" {
		var todoColumnID string
		var todoKind string
		moveFromBacklog := true
		err := tx.QueryRowContext(ctx,
			`SELECT id::text, kind
			 FROM board_columns
			 WHERE board_id = $1 AND kind = 'todo'`,
			boardID,
		).Scan(&todoColumnID, &todoKind)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				todoColumnID = *task.ColumnID
				todoKind = task.Status
				moveFromBacklog = false
			} else {
				return Task{}, err
			}
		}

		targetColumnID = todoColumnID
		targetStatus = todoKind

		if moveFromBacklog {
			if err := shiftAfterRemoveTaskPositionTx(ctx, tx, boardID, *task.ColumnID, task.Position, task.ID); err != nil {
				return Task{}, err
			}
			targetPosition, err = countTasksInColumnTx(ctx, tx, boardID, targetColumnID)
			if err != nil {
				return Task{}, err
			}
		} else {
			targetPosition = task.Position
		}
	} else {
		targetColumnID = *task.ColumnID
	}

	updated, err := updateTaskPlacementTx(ctx, tx, task.ID, targetColumnID, targetStatus, targetPosition, &sprintID)
	if err != nil {
		return Task{}, err
	}

	if err := tx.Commit(); err != nil {
		return Task{}, err
	}
	return updated, nil
}

func (s *Store) RemoveTaskFromSprint(ctx context.Context, sprintID, taskID string) (Task, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Task{}, err
	}
	defer tx.Rollback()

	var boardID string
	var boardType string
	var sprintStatus string
	err = tx.QueryRowContext(ctx,
		`SELECT s.board_id::text, b.type, s.status
		 FROM sprints s
		 JOIN boards b ON b.id = s.board_id
		 WHERE s.id = $1
		 FOR UPDATE`,
		sprintID,
	).Scan(&boardID, &boardType, &sprintStatus)
	if err != nil {
		return Task{}, err
	}

	if err := validateRemoveTaskFromSprint(boardType, sprintStatus); err != nil {
		return Task{}, err
	}

	var backlogColumnID string
	var backlogKind string
	err = tx.QueryRowContext(ctx,
		`SELECT id::text, kind
		 FROM board_columns
		 WHERE board_id = $1 AND kind = 'backlog'`,
		boardID,
	).Scan(&backlogColumnID, &backlogKind)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Task{}, ErrBacklogColumnNotFound
		}
		return Task{}, err
	}

	task, _, err := getTaskForUpdateTx(ctx, tx, taskID)
	if err != nil {
		return Task{}, err
	}
	if task.BoardID != boardID {
		return Task{}, ErrTaskBoardMismatch
	}
	if task.SprintID == nil || *task.SprintID != sprintID {
		return Task{}, ErrTaskNotInSprint
	}

	targetPosition, err := countTasksInColumnTx(ctx, tx, boardID, backlogColumnID)
	if err != nil {
		return Task{}, err
	}

	if task.ColumnID != nil && *task.ColumnID != backlogColumnID {
		if err := shiftAfterRemoveTaskPositionTx(ctx, tx, boardID, *task.ColumnID, task.Position, task.ID); err != nil {
			return Task{}, err
		}
	} else if task.ColumnID != nil && *task.ColumnID == backlogColumnID {
		targetPosition = task.Position
	}

	updated, err := updateTaskPlacementTx(ctx, tx, task.ID, backlogColumnID, backlogKind, targetPosition, nil)
	if err != nil {
		return Task{}, err
	}

	if err := tx.Commit(); err != nil {
		return Task{}, err
	}
	return updated, nil
}

func validateRemoveTaskFromSprint(boardType, sprintStatus string) error {
	if boardType != "scrum" {
		return ErrSprintBoardNotScrum
	}
	_ = sprintStatus
	// Removing to backlog is allowed for any sprint status, including closed.
	return nil
}

func (s *Store) GetSprintReport(ctx context.Context, sprintID string) (SprintReport, error) {
	sp, err := s.GetSprintByID(ctx, sprintID)
	if err != nil {
		return SprintReport{}, err
	}

	var totalTasks int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*)
		 FROM tasks t
		 JOIN board_columns bc ON bc.id = t.column_id
		 WHERE t.sprint_id = $1
		   AND bc.kind <> 'backlog'`,
		sprintID,
	).Scan(&totalTasks); err != nil {
		return SprintReport{}, err
	}

	var completedTasks int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*)
		 FROM tasks t
		 JOIN board_columns bc ON bc.id = t.column_id
		 WHERE t.sprint_id = $1
		   AND bc.kind = 'done'`,
		sprintID,
	).Scan(&completedTasks); err != nil {
		return SprintReport{}, err
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT bc.id::text, bc.name, bc.kind, COALESCE(cnt.tasks, 0)
		 FROM board_columns bc
		 LEFT JOIN (
		   SELECT t.column_id, COUNT(*) AS tasks
		   FROM tasks t
		   JOIN board_columns bc2 ON bc2.id = t.column_id
		   WHERE t.sprint_id = $1
		     AND bc2.kind <> 'backlog'
		   GROUP BY t.column_id
		 ) AS cnt ON cnt.column_id = bc.id
		 WHERE bc.board_id = $2
		 ORDER BY bc.position ASC, bc.created_at ASC`,
		sprintID, sp.BoardID,
	)
	if err != nil {
		return SprintReport{}, err
	}
	defer rows.Close()

	columns := make([]SprintReportColumn, 0)
	for rows.Next() {
		var c SprintReportColumn
		if err := rows.Scan(&c.ColumnID, &c.ColumnName, &c.ColumnKind, &c.Tasks); err != nil {
			return SprintReport{}, err
		}
		columns = append(columns, c)
	}
	if err := rows.Err(); err != nil {
		return SprintReport{}, err
	}

	remainingTasks := totalTasks - completedTasks
	if remainingTasks < 0 {
		remainingTasks = 0
	}

	completionRate := 0.0
	if totalTasks > 0 {
		completionRate = (float64(completedTasks) / float64(totalTasks)) * 100
	}

	return SprintReport{
		Sprint:         sp,
		TotalTasks:     totalTasks,
		CompletedTasks: completedTasks,
		RemainingTasks: remainingTasks,
		VelocityTasks:  completedTasks,
		CompletionRate: completionRate,
		Columns:        columns,
	}, nil
}

func getTaskForUpdateTx(ctx context.Context, tx *sql.Tx, taskID string) (Task, string, error) {
	var t Task
	var rawColumnID sql.NullString
	var rawSprintID sql.NullString
	var rawDueDate sql.NullTime
	var columnKind sql.NullString

	err := tx.QueryRowContext(ctx,
		`SELECT t.id::text, t.board_id::text, t.column_id::text, t.sprint_id::text, t.title, t.description, t.status, t.assignee, t.due_date, t.position, t.created_at, bc.kind
		 FROM tasks t
		 LEFT JOIN board_columns bc ON bc.id = t.column_id
		 WHERE t.id = $1
		 FOR UPDATE OF t`,
		taskID,
	).Scan(
		&t.ID,
		&t.BoardID,
		&rawColumnID,
		&rawSprintID,
		&t.Title,
		&t.Description,
		&t.Status,
		&t.Assignee,
		&rawDueDate,
		&t.Position,
		&t.CreatedAt,
		&columnKind,
	)
	if err != nil {
		return Task{}, "", err
	}

	if rawColumnID.Valid {
		t.ColumnID = &rawColumnID.String
	} else {
		return Task{}, "", fmt.Errorf("task has no column")
	}
	if rawSprintID.Valid {
		t.SprintID = &rawSprintID.String
	}
	if rawDueDate.Valid {
		t.DueDate = &rawDueDate.Time
	}

	return t, columnKind.String, nil
}

func countTasksInColumnTx(ctx context.Context, tx *sql.Tx, boardID, columnID string) (int, error) {
	var count int
	err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tasks WHERE board_id = $1 AND column_id = $2`,
		boardID, columnID,
	).Scan(&count)
	return count, err
}

func shiftAfterRemoveTaskPositionTx(ctx context.Context, tx *sql.Tx, boardID, columnID string, position int, excludeTaskID string) error {
	_, err := tx.ExecContext(ctx,
		`UPDATE tasks
		 SET position = position - 1
		 WHERE board_id = $1
		   AND column_id = $2
		   AND position > $3
		   AND id <> $4`,
		boardID, columnID, position, excludeTaskID,
	)
	return err
}

func updateTaskPlacementTx(
	ctx context.Context,
	tx *sql.Tx,
	taskID, columnID, status string,
	position int,
	sprintID *string,
) (Task, error) {
	var updated Task
	var rawColumnID sql.NullString
	var rawSprintID sql.NullString
	var rawDueDate sql.NullTime

	err := tx.QueryRowContext(ctx,
		`UPDATE tasks
		 SET column_id = $1,
		     status = $2,
		     position = $3,
		     sprint_id = $4
		 WHERE id = $5
		 RETURNING id::text, board_id::text, column_id::text, sprint_id::text, title, description, status, assignee, due_date, position, created_at`,
		columnID, status, position, sprintID, taskID,
	).Scan(
		&updated.ID,
		&updated.BoardID,
		&rawColumnID,
		&rawSprintID,
		&updated.Title,
		&updated.Description,
		&updated.Status,
		&updated.Assignee,
		&rawDueDate,
		&updated.Position,
		&updated.CreatedAt,
	)
	if err != nil {
		return Task{}, err
	}
	if rawColumnID.Valid {
		updated.ColumnID = &rawColumnID.String
	}
	if rawSprintID.Valid {
		updated.SprintID = &rawSprintID.String
	}
	if rawDueDate.Valid {
		updated.DueDate = &rawDueDate.Time
	}
	return updated, nil
}

func (s *Store) CreateTask(
	ctx context.Context,
	boardID string,
	columnID string,
	sprintID *string,
	title string,
	description string,
	status string,
	assignee string,
	dueDate *time.Time,
	position int,
) (Task, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Task{}, err
	}
	defer tx.Rollback()

	var boardType string
	if err := tx.QueryRowContext(ctx,
		`SELECT type FROM boards WHERE id = $1`,
		boardID,
	).Scan(&boardType); err != nil {
		return Task{}, err
	}

	var columnKind string
	if err := tx.QueryRowContext(ctx,
		`SELECT kind FROM board_columns WHERE id = $1 AND board_id = $2`,
		columnID, boardID,
	).Scan(&columnKind); err != nil {
		return Task{}, err
	}

	// Scrum: task without sprint always starts in backlog and has no sprint binding.
	if boardType == "scrum" {
		if sprintID == nil || strings.TrimSpace(*sprintID) == "" {
			var backlogColumnID string
			if err := tx.QueryRowContext(ctx,
				`SELECT id::text
				 FROM board_columns
				 WHERE board_id = $1 AND kind = 'backlog'`,
				boardID,
			).Scan(&backlogColumnID); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return Task{}, ErrBacklogColumnNotFound
				}
				return Task{}, err
			}
			columnID = backlogColumnID
			columnKind = "backlog"
			sprintID = nil
		} else if columnKind == "backlog" {
			// Backlog tasks are global and cannot be bound to any sprint.
			sprintID = nil
		}
	}
	status = columnKind

	if position < 0 {
		position = 0
	}

	var laneCount int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tasks WHERE board_id = $1 AND column_id = $2`,
		boardID, columnID,
	).Scan(&laneCount); err != nil {
		return Task{}, err
	}
	if position > laneCount {
		position = laneCount
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE tasks
		 SET position = position + 1
		 WHERE board_id = $1
		   AND column_id = $2
		   AND position >= $3`,
		boardID, columnID, position,
	); err != nil {
		return Task{}, err
	}

	var t Task
	var rawColumnID sql.NullString
	var rawSprintID sql.NullString
	var rawDueDate sql.NullTime

	err = tx.QueryRowContext(ctx,
		`INSERT INTO tasks(board_id, column_id, sprint_id, title, description, status, assignee, due_date, position)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id::text, board_id::text, column_id::text, sprint_id::text, title, description, status, assignee, due_date, position, created_at`,
		boardID, columnID, sprintID, title, description, status, assignee, dueDate, position,
	).Scan(&t.ID, &t.BoardID, &rawColumnID, &rawSprintID, &t.Title, &t.Description, &t.Status, &t.Assignee, &rawDueDate, &t.Position, &t.CreatedAt)
	if err != nil {
		return Task{}, err
	}

	if rawColumnID.Valid {
		t.ColumnID = &rawColumnID.String
	}
	if rawSprintID.Valid {
		t.SprintID = &rawSprintID.String
	}
	if rawDueDate.Valid {
		t.DueDate = &rawDueDate.Time
	}

	if err := tx.Commit(); err != nil {
		return Task{}, err
	}
	return t, nil
}

func (s *Store) GetTaskByID(ctx context.Context, taskID string) (Task, error) {
	var t Task
	var rawColumnID sql.NullString
	var rawSprintID sql.NullString
	var rawDueDate sql.NullTime

	err := s.db.QueryRowContext(ctx,
		`SELECT id::text, board_id::text, column_id::text, sprint_id::text, title, description, status, assignee, due_date, position, created_at
		 FROM tasks
		 WHERE id = $1`,
		taskID,
	).Scan(&t.ID, &t.BoardID, &rawColumnID, &rawSprintID, &t.Title, &t.Description, &t.Status, &t.Assignee, &rawDueDate, &t.Position, &t.CreatedAt)
	if err != nil {
		return Task{}, err
	}
	if rawColumnID.Valid {
		t.ColumnID = &rawColumnID.String
	}
	if rawSprintID.Valid {
		t.SprintID = &rawSprintID.String
	}
	if rawDueDate.Valid {
		t.DueDate = &rawDueDate.Time
	}
	return t, nil
}

func (s *Store) UpdateTaskFields(ctx context.Context, taskID string, input UpdateTaskInput) (Task, error) {
	sets := make([]string, 0, 5)
	args := make([]any, 0, 8)
	argPos := 1

	if input.Title != nil {
		sets = append(sets, fmt.Sprintf("title = $%d", argPos))
		args = append(args, *input.Title)
		argPos++
	}
	if input.Description != nil {
		sets = append(sets, fmt.Sprintf("description = $%d", argPos))
		args = append(args, *input.Description)
		argPos++
	}
	if input.Assignee != nil {
		sets = append(sets, fmt.Sprintf("assignee = $%d", argPos))
		args = append(args, *input.Assignee)
		argPos++
	}
	if input.DueDateSet {
		sets = append(sets, fmt.Sprintf("due_date = $%d", argPos))
		args = append(args, input.DueDate)
		argPos++
	}
	if input.SprintIDSet {
		sets = append(sets, fmt.Sprintf("sprint_id = $%d", argPos))
		args = append(args, input.SprintID)
		argPos++
	}

	if len(sets) == 0 {
		return s.GetTaskByID(ctx, taskID)
	}

	query := fmt.Sprintf(
		`UPDATE tasks
		 SET %s
		 WHERE id = $%d
		 RETURNING id::text, board_id::text, column_id::text, sprint_id::text, title, description, status, assignee, due_date, position, created_at`,
		strings.Join(sets, ", "),
		argPos,
	)
	args = append(args, taskID)

	var t Task
	var rawColumnID sql.NullString
	var rawSprintID sql.NullString
	var rawDueDate sql.NullTime
	err := s.db.QueryRowContext(ctx, query, args...).
		Scan(&t.ID, &t.BoardID, &rawColumnID, &rawSprintID, &t.Title, &t.Description, &t.Status, &t.Assignee, &rawDueDate, &t.Position, &t.CreatedAt)
	if err != nil {
		return Task{}, err
	}
	if rawColumnID.Valid {
		t.ColumnID = &rawColumnID.String
	}
	if rawSprintID.Valid {
		t.SprintID = &rawSprintID.String
	}
	if rawDueDate.Valid {
		t.DueDate = &rawDueDate.Time
	}
	return t, nil
}

func (s *Store) MoveTask(ctx context.Context, taskID, targetColumnID string, targetPosition int, targetSprintID *string) (Task, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Task{}, err
	}
	defer tx.Rollback()

	var current Task
	var rawColumnID sql.NullString
	var rawSprintID sql.NullString
	var rawDueDate sql.NullTime
	err = tx.QueryRowContext(ctx,
		`SELECT id::text, board_id::text, column_id::text, sprint_id::text, title, description, status, assignee, due_date, position, created_at
		 FROM tasks
		 WHERE id = $1
		 FOR UPDATE`,
		taskID,
	).Scan(
		&current.ID,
		&current.BoardID,
		&rawColumnID,
		&rawSprintID,
		&current.Title,
		&current.Description,
		&current.Status,
		&current.Assignee,
		&rawDueDate,
		&current.Position,
		&current.CreatedAt,
	)
	if err != nil {
		return Task{}, err
	}
	if rawColumnID.Valid {
		current.ColumnID = &rawColumnID.String
	}
	if rawSprintID.Valid {
		current.SprintID = &rawSprintID.String
	}
	if rawDueDate.Valid {
		current.DueDate = &rawDueDate.Time
	}

	currentColumnID := ""
	if current.ColumnID != nil {
		currentColumnID = *current.ColumnID
	}
	if currentColumnID == "" {
		return Task{}, fmt.Errorf("task has no column")
	}

	if targetColumnID == "" {
		targetColumnID = currentColumnID
	}

	if targetPosition < 0 {
		targetPosition = 0
	}

	var boardType string
	if err := tx.QueryRowContext(ctx,
		`SELECT type FROM boards WHERE id = $1`,
		current.BoardID,
	).Scan(&boardType); err != nil {
		return Task{}, err
	}

	var targetStatus string
	if err := tx.QueryRowContext(ctx,
		`SELECT kind FROM board_columns WHERE id = $1 AND board_id = $2`,
		targetColumnID, current.BoardID,
	).Scan(&targetStatus); err != nil {
		return Task{}, err
	}

	currentSprintID := normalizedOptionalID(current.SprintID)

	if targetColumnID == currentColumnID {
		var laneCount int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM tasks WHERE board_id = $1 AND column_id = $2`,
			current.BoardID, currentColumnID,
		).Scan(&laneCount); err != nil {
			return Task{}, err
		}

		maxPos := laneCount - 1
		if maxPos < 0 {
			maxPos = 0
		}
		if targetPosition > maxPos {
			targetPosition = maxPos
		}

		if targetPosition > current.Position {
			if _, err := tx.ExecContext(ctx,
				`UPDATE tasks
				 SET position = position - 1
				 WHERE board_id = $1
				   AND column_id = $2
				   AND position > $3
				   AND position <= $4
				   AND id <> $5`,
				current.BoardID, currentColumnID, current.Position, targetPosition, current.ID,
			); err != nil {
				return Task{}, err
			}
		} else if targetPosition < current.Position {
			if _, err := tx.ExecContext(ctx,
				`UPDATE tasks
				 SET position = position + 1
				 WHERE board_id = $1
				   AND column_id = $2
				   AND position >= $3
				   AND position < $4
				   AND id <> $5`,
				current.BoardID, currentColumnID, targetPosition, current.Position, current.ID,
			); err != nil {
				return Task{}, err
			}
		}
	} else {
		if _, err := tx.ExecContext(ctx,
			`UPDATE tasks
			 SET position = position - 1
			 WHERE board_id = $1
			   AND column_id = $2
			   AND position > $3`,
			current.BoardID, currentColumnID, current.Position,
		); err != nil {
			return Task{}, err
		}

		var targetLaneCount int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM tasks WHERE board_id = $1 AND column_id = $2`,
			current.BoardID, targetColumnID,
		).Scan(&targetLaneCount); err != nil {
			return Task{}, err
		}

		if targetPosition > targetLaneCount {
			targetPosition = targetLaneCount
		}

		if _, err := tx.ExecContext(ctx,
			`UPDATE tasks
			 SET position = position + 1
			 WHERE board_id = $1
			   AND column_id = $2
			   AND position >= $3`,
			current.BoardID, targetColumnID, targetPosition,
		); err != nil {
			return Task{}, err
		}
	}

	var updated Task
	var updatedColumnID sql.NullString
	var updatedSprintID sql.NullString
	var updatedDueDate sql.NullTime

	nextSprintID, err := resolveNextSprintBinding(boardType, targetStatus, currentSprintID, targetSprintID)
	if err != nil {
		return Task{}, err
	}

	err = tx.QueryRowContext(ctx,
		`UPDATE tasks
		 SET column_id = $1, status = $2, position = $3, sprint_id = $4
		 WHERE id = $5
		 RETURNING id::text, board_id::text, column_id::text, sprint_id::text, title, description, status, assignee, due_date, position, created_at`,
		targetColumnID, targetStatus, targetPosition, nextSprintID, current.ID,
	).Scan(
		&updated.ID,
		&updated.BoardID,
		&updatedColumnID,
		&updatedSprintID,
		&updated.Title,
		&updated.Description,
		&updated.Status,
		&updated.Assignee,
		&updatedDueDate,
		&updated.Position,
		&updated.CreatedAt,
	)
	if err != nil {
		return Task{}, err
	}
	if updatedColumnID.Valid {
		updated.ColumnID = &updatedColumnID.String
	}
	if updatedSprintID.Valid {
		updated.SprintID = &updatedSprintID.String
	}
	if updatedDueDate.Valid {
		updated.DueDate = &updatedDueDate.Time
	}

	if err := tx.Commit(); err != nil {
		return Task{}, err
	}
	return updated, nil
}

func (s *Store) DeleteTask(ctx context.Context, taskID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var boardID string
	var columnID sql.NullString
	var position int
	if err := tx.QueryRowContext(ctx,
		`SELECT board_id::text, column_id::text, position
		 FROM tasks
		 WHERE id = $1
		 FOR UPDATE`,
		taskID,
	).Scan(&boardID, &columnID, &position); err != nil {
		return err
	}
	if !columnID.Valid || columnID.String == "" {
		return fmt.Errorf("task has no column")
	}

	res, err := tx.ExecContext(ctx, `DELETE FROM tasks WHERE id = $1`, taskID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE tasks
		 SET position = position - 1
		 WHERE board_id = $1
		   AND column_id = $2
		   AND position > $3`,
		boardID, columnID.String, position,
	); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) ListTasks(ctx context.Context, boardID, status, sprintID, columnID string) ([]Task, error) {
	parts := []string{`SELECT t.id::text, t.board_id::text, t.column_id::text, t.sprint_id::text, t.title, t.description, t.status, t.assignee, t.due_date, t.position, t.created_at
		FROM tasks t
		LEFT JOIN board_columns bc ON bc.id = t.column_id`}
	where := make([]string, 0, 4)
	args := make([]any, 0, 4)
	idx := 1

	if boardID != "" {
		where = append(where, fmt.Sprintf("t.board_id = $%d", idx))
		args = append(args, boardID)
		idx++
	}
	if status != "" {
		where = append(where, fmt.Sprintf("t.status = $%d", idx))
		args = append(args, status)
		idx++
	}
	if sprintID != "" {
		where = append(where, fmt.Sprintf("t.sprint_id = $%d", idx))
		args = append(args, sprintID)
		idx++
	}
	if columnID != "" {
		where = append(where, fmt.Sprintf("t.column_id = $%d", idx))
		args = append(args, columnID)
	}
	if len(where) > 0 {
		parts = append(parts, "WHERE "+strings.Join(where, " AND "))
	}
	parts = append(parts, "ORDER BY bc.position ASC NULLS LAST, t.position ASC, t.created_at ASC")

	rows, err := s.db.QueryContext(ctx, strings.Join(parts, " "), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Task, 0)
	for rows.Next() {
		var t Task
		var rawColumnID sql.NullString
		var rawSprintID sql.NullString
		var rawDueDate sql.NullTime
		if err := rows.Scan(&t.ID, &t.BoardID, &rawColumnID, &rawSprintID, &t.Title, &t.Description, &t.Status, &t.Assignee, &rawDueDate, &t.Position, &t.CreatedAt); err != nil {
			return nil, err
		}
		if rawColumnID.Valid {
			t.ColumnID = &rawColumnID.String
		}
		if rawSprintID.Valid {
			t.SprintID = &rawSprintID.String
		}
		if rawDueDate.Valid {
			t.DueDate = &rawDueDate.Time
		}
		items = append(items, t)
	}
	return items, rows.Err()
}
