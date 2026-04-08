package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"hawler/backend/internal/store"
)

var (
	errForbidden      = errors.New("forbidden")
	errTaskIDRequired = errors.New("taskID is required")
)

type Server struct {
	store *store.Store
	auth  *AuthManager
}

func NewRouter(st *store.Store, jwtSecret string, jwtTTL time.Duration) http.Handler {
	s := &Server{
		store: st,
		auth:  NewAuthManager(jwtSecret, jwtTTL),
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(devCORS)

	r.Get("/health", s.health)

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/register", s.register)
		r.Post("/auth/login", s.login)

		r.Group(func(r chi.Router) {
			r.Use(s.requireAuth)

			r.Get("/auth/me", s.me)

			r.Get("/workspaces", s.listWorkspaces)
			r.Post("/workspaces", s.createWorkspace)
			r.Get("/workspaces/{workspaceID}/members", s.listWorkspaceMembers)
			r.Post("/workspaces/{workspaceID}/members", s.upsertWorkspaceMember)

			r.Get("/projects", s.listProjects)
			r.Post("/projects", s.createProject)

			r.Get("/boards", s.listBoards)
			r.Post("/boards", s.createBoard)
			r.Get("/boards/{boardID}/columns", s.listBoardColumns)
			r.Post("/boards/{boardID}/columns", s.createBoardColumn)
			r.Patch("/boards/{boardID}/columns/{columnID}", s.patchBoardColumn)
			r.Delete("/boards/{boardID}/columns/{columnID}", s.deleteBoardColumn)

			r.Get("/sprints", s.listSprints)
			r.Post("/sprints", s.createSprint)
			r.Get("/sprints/{sprintID}/report", s.sprintReport)
			r.Post("/sprints/{sprintID}/start", s.startSprint)
			r.Post("/sprints/{sprintID}/close", s.closeSprint)
			r.Post("/sprints/{sprintID}/tasks/{taskID}", s.addTaskToSprint)
			r.Delete("/sprints/{sprintID}/tasks/{taskID}", s.removeTaskFromSprint)

			r.Get("/tasks", s.listTasks)
			r.Post("/tasks", s.createTask)
			r.Patch("/tasks/{taskID}", s.patchTask)
			r.Patch("/tasks/{taskID}/move", s.moveTask)
			r.Delete("/tasks/{taskID}", s.deleteTask)
		})
	})

	return r
}

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		if authHeader == "" {
			writeError(w, http.StatusUnauthorized, "missing Authorization header")
			return
		}
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
			writeError(w, http.StatusUnauthorized, "invalid Authorization header")
			return
		}

		claims, err := s.auth.ParseToken(parts[1])
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		ctx := withCurrentUser(r.Context(), CurrentUser{ID: claims.UserID, Email: claims.Email})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	Token string     `json:"token"`
	User  store.User `json:"user"`
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if !isValidEmail(req.Email) {
		writeError(w, http.StatusBadRequest, "valid email is required")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	passwordHash, err := HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	user, err := s.store.CreateUser(r.Context(), req.Name, req.Email, passwordHash)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "user with this email already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	token, err := s.auth.GenerateToken(user.ID, user.Email)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, authResponse{Token: token, User: user})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if !isValidEmail(req.Email) || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	userAuth, err := s.store.GetUserAuthByEmail(r.Context(), req.Email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := VerifyPassword(userAuth.PasswordHash, req.Password); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, err := s.auth.GenerateToken(userAuth.ID, userAuth.Email)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, authResponse{Token: token, User: userAuth.User})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	entity, err := s.store.GetUserByID(r.Context(), user.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusUnauthorized, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, entity)
}

type createWorkspaceRequest struct {
	Name string `json:"name"`
}

func (s *Server) createWorkspace(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := currentUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req createWorkspaceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	workspace, err := s.store.CreateWorkspace(r.Context(), req.Name, currentUser.ID)
	if err != nil {
		if isFKError(err) {
			writeError(w, http.StatusBadRequest, "creator user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, workspace)
}

func (s *Server) listWorkspaces(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := currentUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	items, err := s.store.ListWorkspaces(r.Context(), currentUser.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) listWorkspaceMembers(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := currentUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	workspaceID := chi.URLParam(r, "workspaceID")
	if strings.TrimSpace(workspaceID) == "" {
		writeError(w, http.StatusBadRequest, "workspaceID is required")
		return
	}

	if err := s.requireWorkspaceRole(r.Context(), workspaceID, currentUser.ID); err != nil {
		s.writeAccessError(w, err)
		return
	}

	items, err := s.store.ListWorkspaceMembers(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

type upsertWorkspaceMemberRequest struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
}

func (s *Server) upsertWorkspaceMember(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := currentUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	workspaceID := chi.URLParam(r, "workspaceID")
	if strings.TrimSpace(workspaceID) == "" {
		writeError(w, http.StatusBadRequest, "workspaceID is required")
		return
	}

	if err := s.requireWorkspaceRole(r.Context(), workspaceID, currentUser.ID, store.RoleOwner); err != nil {
		s.writeAccessError(w, err)
		return
	}

	var req upsertWorkspaceMemberRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	req.Role = strings.ToLower(strings.TrimSpace(req.Role))
	if !isValidWorkspaceRole(req.Role) {
		writeError(w, http.StatusBadRequest, "role must be one of: owner, mentor, student")
		return
	}

	targetUserID := strings.TrimSpace(req.UserID)
	var targetUser store.User
	if targetUserID == "" {
		email := strings.ToLower(strings.TrimSpace(req.Email))
		if !isValidEmail(email) {
			writeError(w, http.StatusBadRequest, "provide user_id or valid email")
			return
		}
		userByEmail, err := s.store.GetUserByEmail(r.Context(), email)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, "user not found")
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		targetUser = userByEmail
		targetUserID = userByEmail.ID
	} else {
		userByID, err := s.store.GetUserByID(r.Context(), targetUserID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, "user not found")
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		targetUser = userByID
	}

	prevRole, err := s.store.UserRoleInWorkspace(r.Context(), workspaceID, targetUserID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if prevRole == store.RoleOwner && req.Role != store.RoleOwner {
		owners, err := s.store.CountWorkspaceOwners(r.Context(), workspaceID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if owners <= 1 {
			writeError(w, http.StatusBadRequest, "workspace must have at least one owner")
			return
		}
	}

	member, err := s.store.UpsertWorkspaceMember(r.Context(), workspaceID, targetUserID, req.Role)
	if err != nil {
		if isFKError(err) {
			writeError(w, http.StatusBadRequest, "workspace or user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	member.Name = targetUser.Name
	member.Email = targetUser.Email
	writeJSON(w, http.StatusOK, member)
}

type createProjectRequest struct {
	WorkspaceID string `json:"workspace_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (s *Server) createProject(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := currentUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req createProjectRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.WorkspaceID) == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	if err := s.requireWorkspaceRole(r.Context(), req.WorkspaceID, currentUser.ID, store.RoleOwner, store.RoleMentor); err != nil {
		s.writeAccessError(w, err)
		return
	}

	project, err := s.store.CreateProject(r.Context(), req.WorkspaceID, req.Name, req.Description)
	if err != nil {
		if isFKError(err) {
			writeError(w, http.StatusBadRequest, "workspace not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, project)
}

func (s *Server) listProjects(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := currentUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	workspaceID := strings.TrimSpace(r.URL.Query().Get("workspace_id"))
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	if err := s.requireWorkspaceRole(r.Context(), workspaceID, currentUser.ID); err != nil {
		s.writeAccessError(w, err)
		return
	}

	items, err := s.store.ListProjects(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

type createBoardRequest struct {
	ProjectID string `json:"project_id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
}

func (s *Server) createBoard(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := currentUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req createBoardRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.ProjectID) == "" {
		writeError(w, http.StatusBadRequest, "project_id is required")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	workspaceID, err := s.store.WorkspaceIDByProjectID(r.Context(), req.ProjectID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := s.requireWorkspaceRole(r.Context(), workspaceID, currentUser.ID, store.RoleOwner, store.RoleMentor); err != nil {
		s.writeAccessError(w, err)
		return
	}

	boardType := strings.ToLower(strings.TrimSpace(req.Type))
	if boardType == "" {
		boardType = "kanban"
	}
	if boardType != "kanban" && boardType != "scrum" {
		writeError(w, http.StatusBadRequest, "type must be one of: kanban, scrum")
		return
	}

	board, err := s.store.CreateBoard(r.Context(), req.ProjectID, req.Name, boardType)
	if err != nil {
		if isFKError(err) {
			writeError(w, http.StatusBadRequest, "project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, board)
}

func (s *Server) listBoards(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := currentUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	projectID := strings.TrimSpace(r.URL.Query().Get("project_id"))
	if projectID == "" {
		writeError(w, http.StatusBadRequest, "project_id is required")
		return
	}

	workspaceID, err := s.store.WorkspaceIDByProjectID(r.Context(), projectID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := s.requireWorkspaceRole(r.Context(), workspaceID, currentUser.ID); err != nil {
		s.writeAccessError(w, err)
		return
	}

	items, err := s.store.ListBoards(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) listBoardColumns(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := currentUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	boardID := strings.TrimSpace(chi.URLParam(r, "boardID"))
	if boardID == "" {
		writeError(w, http.StatusBadRequest, "boardID is required")
		return
	}

	workspaceID, err := s.store.WorkspaceIDByBoardID(r.Context(), boardID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "board not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.requireWorkspaceRole(r.Context(), workspaceID, currentUser.ID); err != nil {
		s.writeAccessError(w, err)
		return
	}

	items, err := s.store.ListBoardColumns(r.Context(), boardID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

type createBoardColumnRequest struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Position int    `json:"position"`
}

func (s *Server) createBoardColumn(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := currentUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	boardID := strings.TrimSpace(chi.URLParam(r, "boardID"))
	if boardID == "" {
		writeError(w, http.StatusBadRequest, "boardID is required")
		return
	}

	workspaceID, err := s.store.WorkspaceIDByBoardID(r.Context(), boardID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "board not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.requireWorkspaceRole(r.Context(), workspaceID, currentUser.ID, store.RoleOwner, store.RoleMentor); err != nil {
		s.writeAccessError(w, err)
		return
	}

	var req createBoardColumnRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Kind = normalizeColumnKind(req.Kind)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Kind == "" {
		writeError(w, http.StatusBadRequest, "kind is required")
		return
	}

	column, err := s.store.CreateBoardColumn(r.Context(), boardID, req.Name, req.Kind, req.Position)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "column with this kind already exists")
			return
		}
		if isFKError(err) {
			writeError(w, http.StatusBadRequest, "board not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, column)
}

type patchBoardColumnRequest struct {
	Name     *string `json:"name"`
	Kind     *string `json:"kind"`
	Position *int    `json:"position"`
}

func (s *Server) patchBoardColumn(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := currentUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	boardID := strings.TrimSpace(chi.URLParam(r, "boardID"))
	columnID := strings.TrimSpace(chi.URLParam(r, "columnID"))
	if boardID == "" || columnID == "" {
		writeError(w, http.StatusBadRequest, "boardID and columnID are required")
		return
	}

	workspaceID, err := s.store.WorkspaceIDByBoardID(r.Context(), boardID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "board not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.requireWorkspaceRole(r.Context(), workspaceID, currentUser.ID, store.RoleOwner, store.RoleMentor); err != nil {
		s.writeAccessError(w, err)
		return
	}

	if ok, err := s.store.BoardColumnBelongsToBoard(r.Context(), columnID, boardID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	} else if !ok {
		writeError(w, http.StatusNotFound, "column not found")
		return
	}

	var req patchBoardColumnRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	input := store.UpdateBoardColumnInput{}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			writeError(w, http.StatusBadRequest, "name cannot be empty")
			return
		}
		input.Name = &name
	}
	if req.Kind != nil {
		kind := normalizeColumnKind(*req.Kind)
		if kind == "" {
			writeError(w, http.StatusBadRequest, "kind cannot be empty")
			return
		}
		input.Kind = &kind
	}
	if req.Position != nil {
		input.Position = req.Position
	}

	column, err := s.store.UpdateBoardColumn(r.Context(), boardID, columnID, input)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "column not found")
			return
		}
		if errors.Is(err, store.ErrCannotModifyBacklog) {
			writeError(w, http.StatusBadRequest, "backlog column cannot be modified")
			return
		}
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "column with this kind already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, column)
}

func (s *Server) deleteBoardColumn(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := currentUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	boardID := strings.TrimSpace(chi.URLParam(r, "boardID"))
	columnID := strings.TrimSpace(chi.URLParam(r, "columnID"))
	if boardID == "" || columnID == "" {
		writeError(w, http.StatusBadRequest, "boardID and columnID are required")
		return
	}

	workspaceID, err := s.store.WorkspaceIDByBoardID(r.Context(), boardID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "board not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.requireWorkspaceRole(r.Context(), workspaceID, currentUser.ID, store.RoleOwner, store.RoleMentor); err != nil {
		s.writeAccessError(w, err)
		return
	}

	targetColumnIDRaw := strings.TrimSpace(r.URL.Query().Get("target_column_id"))
	var targetColumnID *string
	if targetColumnIDRaw != "" {
		targetColumnID = &targetColumnIDRaw
	}

	if err := s.store.DeleteBoardColumn(r.Context(), boardID, columnID, targetColumnID); err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			writeError(w, http.StatusNotFound, "column not found")
		case errors.Is(err, store.ErrCannotDeleteBacklog):
			writeError(w, http.StatusBadRequest, "backlog column cannot be deleted")
		case errors.Is(err, store.ErrCannotDeleteLastColumn):
			writeError(w, http.StatusBadRequest, "cannot delete last column")
		case errors.Is(err, store.ErrColumnHasTasks):
			writeError(w, http.StatusBadRequest, "column has tasks; provide target_column_id")
		case errors.Is(err, store.ErrInvalidColumnTarget):
			writeError(w, http.StatusBadRequest, "invalid target_column_id")
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type createSprintRequest struct {
	BoardID  string `json:"board_id"`
	Name     string `json:"name"`
	Goal     string `json:"goal"`
	StartsAt string `json:"starts_at"`
	EndsAt   string `json:"ends_at"`
	Status   string `json:"status"`
}

func (s *Server) createSprint(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := currentUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req createSprintRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.BoardID) == "" {
		writeError(w, http.StatusBadRequest, "board_id is required")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	workspaceID, boardType, err := s.store.WorkspaceAndTypeByBoardID(r.Context(), req.BoardID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "board not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := s.requireWorkspaceRole(r.Context(), workspaceID, currentUser.ID, store.RoleOwner, store.RoleMentor); err != nil {
		s.writeAccessError(w, err)
		return
	}
	if boardType != "scrum" {
		writeError(w, http.StatusBadRequest, "sprints are allowed only for scrum boards")
		return
	}

	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status == "" {
		status = "planned"
	}
	if status != "planned" && status != "active" && status != "closed" {
		writeError(w, http.StatusBadRequest, "status must be one of: planned, active, closed")
		return
	}

	startsAt, err := parseOptionalDate(req.StartsAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, "starts_at must be YYYY-MM-DD")
		return
	}

	endsAt, err := parseOptionalDate(req.EndsAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, "ends_at must be YYYY-MM-DD")
		return
	}

	sprint, err := s.store.CreateSprint(r.Context(), req.BoardID, req.Name, req.Goal, startsAt, endsAt, status)
	if err != nil {
		if isFKError(err) {
			writeError(w, http.StatusBadRequest, "board not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, sprint)
}

func (s *Server) listSprints(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := currentUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	boardID := strings.TrimSpace(r.URL.Query().Get("board_id"))
	if boardID == "" {
		writeError(w, http.StatusBadRequest, "board_id is required")
		return
	}
	status := strings.TrimSpace(r.URL.Query().Get("status"))

	workspaceID, err := s.store.WorkspaceIDByBoardID(r.Context(), boardID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "board not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := s.requireWorkspaceRole(r.Context(), workspaceID, currentUser.ID); err != nil {
		s.writeAccessError(w, err)
		return
	}

	items, err := s.store.ListSprints(r.Context(), boardID, status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) sprintReport(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := currentUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	sprintID := strings.TrimSpace(chi.URLParam(r, "sprintID"))
	if sprintID == "" {
		writeError(w, http.StatusBadRequest, "sprintID is required")
		return
	}

	sp, err := s.store.GetSprintByID(r.Context(), sprintID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "sprint not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	workspaceID, err := s.store.WorkspaceIDByBoardID(r.Context(), sp.BoardID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.requireWorkspaceRole(r.Context(), workspaceID, currentUser.ID); err != nil {
		s.writeAccessError(w, err)
		return
	}

	report, err := s.store.GetSprintReport(r.Context(), sprintID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "sprint not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, report)
}

func (s *Server) startSprint(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := currentUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	sprintID := strings.TrimSpace(chi.URLParam(r, "sprintID"))
	if sprintID == "" {
		writeError(w, http.StatusBadRequest, "sprintID is required")
		return
	}

	sprint, err := s.store.GetSprintByID(r.Context(), sprintID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "sprint not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	workspaceID, err := s.store.WorkspaceIDByBoardID(r.Context(), sprint.BoardID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.requireWorkspaceRole(r.Context(), workspaceID, currentUser.ID, store.RoleOwner, store.RoleMentor); err != nil {
		s.writeAccessError(w, err)
		return
	}

	updated, err := s.store.StartSprint(r.Context(), sprintID)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			writeError(w, http.StatusNotFound, "sprint not found")
		case errors.Is(err, store.ErrSprintBoardNotScrum):
			writeError(w, http.StatusBadRequest, "only scrum sprints can be started")
		case errors.Is(err, store.ErrSprintNotPlanned):
			writeError(w, http.StatusBadRequest, "only planned sprint can be started")
		case errors.Is(err, store.ErrActiveSprintExists):
			writeError(w, http.StatusConflict, "active sprint already exists on board")
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) closeSprint(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := currentUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	sprintID := strings.TrimSpace(chi.URLParam(r, "sprintID"))
	if sprintID == "" {
		writeError(w, http.StatusBadRequest, "sprintID is required")
		return
	}

	sprint, err := s.store.GetSprintByID(r.Context(), sprintID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "sprint not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	workspaceID, err := s.store.WorkspaceIDByBoardID(r.Context(), sprint.BoardID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.requireWorkspaceRole(r.Context(), workspaceID, currentUser.ID, store.RoleOwner, store.RoleMentor); err != nil {
		s.writeAccessError(w, err)
		return
	}

	result, err := s.store.CloseSprint(r.Context(), sprintID)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			writeError(w, http.StatusNotFound, "sprint not found")
		case errors.Is(err, store.ErrSprintBoardNotScrum):
			writeError(w, http.StatusBadRequest, "only scrum sprints can be closed")
		case errors.Is(err, store.ErrSprintNotActive):
			writeError(w, http.StatusBadRequest, "only active sprint can be closed")
		case errors.Is(err, store.ErrBacklogColumnNotFound):
			writeError(w, http.StatusBadRequest, "backlog column not found")
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *Server) addTaskToSprint(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := currentUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	sprintID := strings.TrimSpace(chi.URLParam(r, "sprintID"))
	taskID := strings.TrimSpace(chi.URLParam(r, "taskID"))
	if sprintID == "" || taskID == "" {
		writeError(w, http.StatusBadRequest, "sprintID and taskID are required")
		return
	}

	sprint, err := s.store.GetSprintByID(r.Context(), sprintID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "sprint not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	workspaceID, err := s.store.WorkspaceIDByBoardID(r.Context(), sprint.BoardID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.requireWorkspaceRole(r.Context(), workspaceID, currentUser.ID, store.RoleOwner, store.RoleMentor); err != nil {
		s.writeAccessError(w, err)
		return
	}

	task, err := s.store.AddTaskToSprint(r.Context(), sprintID, taskID)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			writeError(w, http.StatusNotFound, "sprint or task not found")
		case errors.Is(err, store.ErrSprintBoardNotScrum):
			writeError(w, http.StatusBadRequest, "only scrum sprints can be modified")
		case errors.Is(err, store.ErrSprintClosed):
			writeError(w, http.StatusBadRequest, "closed sprint cannot be modified")
		case errors.Is(err, store.ErrTaskBoardMismatch):
			writeError(w, http.StatusBadRequest, "task does not belong to sprint board")
		case errors.Is(err, store.ErrTaskAlreadyInSprint):
			writeError(w, http.StatusConflict, "task is already in this sprint")
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, task)
}

func (s *Server) removeTaskFromSprint(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := currentUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	sprintID := strings.TrimSpace(chi.URLParam(r, "sprintID"))
	taskID := strings.TrimSpace(chi.URLParam(r, "taskID"))
	if sprintID == "" || taskID == "" {
		writeError(w, http.StatusBadRequest, "sprintID and taskID are required")
		return
	}

	sprint, err := s.store.GetSprintByID(r.Context(), sprintID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "sprint not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	workspaceID, err := s.store.WorkspaceIDByBoardID(r.Context(), sprint.BoardID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.requireWorkspaceRole(r.Context(), workspaceID, currentUser.ID, store.RoleOwner, store.RoleMentor); err != nil {
		s.writeAccessError(w, err)
		return
	}

	task, err := s.store.RemoveTaskFromSprint(r.Context(), sprintID, taskID)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			writeError(w, http.StatusNotFound, "sprint or task not found")
		case errors.Is(err, store.ErrSprintBoardNotScrum):
			writeError(w, http.StatusBadRequest, "only scrum sprints can be modified")
		case errors.Is(err, store.ErrBacklogColumnNotFound):
			writeError(w, http.StatusBadRequest, "backlog column not found")
		case errors.Is(err, store.ErrTaskBoardMismatch):
			writeError(w, http.StatusBadRequest, "task does not belong to sprint board")
		case errors.Is(err, store.ErrTaskNotInSprint):
			writeError(w, http.StatusBadRequest, "task is not in sprint")
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, task)
}

type createTaskRequest struct {
	BoardID     string `json:"board_id"`
	ColumnID    string `json:"column_id"`
	SprintID    string `json:"sprint_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Assignee    string `json:"assignee"`
	DueDate     string `json:"due_date"`
	Position    int    `json:"position"`
}

func (s *Server) createTask(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := currentUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req createTaskRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.BoardID) == "" {
		writeError(w, http.StatusBadRequest, "board_id is required")
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}

	workspaceID, err := s.store.WorkspaceIDByBoardID(r.Context(), req.BoardID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "board not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.requireWorkspaceRole(r.Context(), workspaceID, currentUser.ID); err != nil {
		s.writeAccessError(w, err)
		return
	}

	column, err := s.resolveBoardColumnForTask(r.Context(), req.BoardID, req.ColumnID, req.Status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusBadRequest, "column not found for board")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var sprintID *string
	if rawSprintID := strings.TrimSpace(req.SprintID); rawSprintID != "" {
		belongs, err := s.store.SprintBelongsToBoard(r.Context(), rawSprintID, req.BoardID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !belongs {
			writeError(w, http.StatusBadRequest, "sprint does not belong to board")
			return
		}
		sprintID = &rawSprintID

		if column.Kind == "backlog" {
			if todoColumn, err := s.store.GetBoardColumnByKind(r.Context(), req.BoardID, "todo"); err == nil {
				column = todoColumn
			}
		}
	}

	dueDate, err := parseOptionalDate(req.DueDate)
	if err != nil {
		writeError(w, http.StatusBadRequest, "due_date must be YYYY-MM-DD")
		return
	}

	task, err := s.store.CreateTask(
		r.Context(),
		req.BoardID,
		column.ID,
		sprintID,
		req.Title,
		req.Description,
		column.Kind,
		req.Assignee,
		dueDate,
		req.Position,
	)
	if err != nil {
		if isFKError(err) {
			writeError(w, http.StatusBadRequest, "board or sprint not found")
			return
		}
		if errors.Is(err, store.ErrBacklogColumnNotFound) {
			writeError(w, http.StatusBadRequest, "backlog column not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, task)
}

type patchTaskRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	ColumnID    *string `json:"column_id"`
	Status      *string `json:"status"`
	Assignee    *string `json:"assignee"`
	DueDate     *string `json:"due_date"`
	SprintID    *string `json:"sprint_id"`
	Position    *int    `json:"position"`
}

func (s *Server) patchTask(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := currentUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	taskID := strings.TrimSpace(chi.URLParam(r, "taskID"))
	task, err := s.authorizedTask(r.Context(), taskID, currentUser.ID)
	if err != nil {
		s.writeTaskAccessError(w, err)
		return
	}

	var req patchTaskRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	updateInput := store.UpdateTaskInput{}
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			writeError(w, http.StatusBadRequest, "title cannot be empty")
			return
		}
		updateInput.Title = &title
	}
	if req.Description != nil {
		desc := strings.TrimSpace(*req.Description)
		updateInput.Description = &desc
	}
	if req.Assignee != nil {
		assignee := strings.TrimSpace(*req.Assignee)
		updateInput.Assignee = &assignee
	}
	if req.DueDate != nil {
		updateInput.DueDateSet = true
		if strings.TrimSpace(*req.DueDate) == "" {
			updateInput.DueDate = nil
		} else {
			parsed, err := parseOptionalDate(*req.DueDate)
			if err != nil {
				writeError(w, http.StatusBadRequest, "due_date must be YYYY-MM-DD or empty")
				return
			}
			updateInput.DueDate = parsed
		}
	}
	if req.SprintID != nil {
		updateInput.SprintIDSet = true
		rawSprintID := strings.TrimSpace(*req.SprintID)
		if rawSprintID == "" {
			updateInput.SprintID = nil
		} else {
			belongs, err := s.store.SprintBelongsToBoard(r.Context(), rawSprintID, task.BoardID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !belongs {
				writeError(w, http.StatusBadRequest, "sprint does not belong to board")
				return
			}
			updateInput.SprintID = &rawSprintID
		}
	}

	updatedTask, err := s.store.UpdateTaskFields(r.Context(), task.ID, updateInput)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "task not found")
			return
		}
		if isFKError(err) {
			writeError(w, http.StatusBadRequest, "invalid sprint_id")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	needsMove := req.Status != nil || req.Position != nil || req.ColumnID != nil
	if needsMove {
		targetColumnID := ""
		if updatedTask.ColumnID != nil {
			targetColumnID = *updatedTask.ColumnID
		}
		if targetColumnID == "" {
			writeError(w, http.StatusBadRequest, "task has no column")
			return
		}

		requestedColumnID := ""
		if req.ColumnID != nil {
			requestedColumnID = strings.TrimSpace(*req.ColumnID)
		}
		requestedStatus := ""
		if req.Status != nil {
			requestedStatus = strings.TrimSpace(*req.Status)
			if requestedStatus == "" {
				writeError(w, http.StatusBadRequest, "status cannot be empty")
				return
			}
		}

		if req.ColumnID != nil || req.Status != nil {
			column, err := s.resolveBoardColumnForTask(r.Context(), task.BoardID, requestedColumnID, requestedStatus)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					writeError(w, http.StatusBadRequest, "column not found for board")
					return
				}
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			targetColumnID = column.ID
		}

		targetPosition := updatedTask.Position
		if req.Position != nil {
			targetPosition = *req.Position
		} else if targetColumnID != *updatedTask.ColumnID {
			targetPosition = 1 << 30
		}

		updatedTask, err = s.store.MoveTask(r.Context(), task.ID, targetColumnID, targetPosition, updatedTask.SprintID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, "task not found")
				return
			}
			if errors.Is(err, store.ErrSprintRequiredForTask) {
				writeError(w, http.StatusBadRequest, "sprint is required when moving scrum task out of backlog")
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	writeJSON(w, http.StatusOK, updatedTask)
}

type moveTaskRequest struct {
	ColumnID string  `json:"column_id"`
	Status   string  `json:"status"`
	Position int     `json:"position"`
	SprintID *string `json:"sprint_id"`
}

func (s *Server) moveTask(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := currentUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	taskID := strings.TrimSpace(chi.URLParam(r, "taskID"))
	task, err := s.authorizedTask(r.Context(), taskID, currentUser.ID)
	if err != nil {
		s.writeTaskAccessError(w, err)
		return
	}

	var req moveTaskRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	targetColumnID := ""
	if task.ColumnID != nil {
		targetColumnID = *task.ColumnID
	}
	if targetColumnID == "" {
		writeError(w, http.StatusBadRequest, "task has no column")
		return
	}

	if strings.TrimSpace(req.ColumnID) != "" || strings.TrimSpace(req.Status) != "" {
		column, err := s.resolveBoardColumnForTask(r.Context(), task.BoardID, req.ColumnID, req.Status)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusBadRequest, "column not found for board")
				return
			}
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		targetColumnID = column.ID
	}

	var targetSprintID *string
	if req.SprintID != nil {
		rawSprintID := strings.TrimSpace(*req.SprintID)
		if rawSprintID == "" {
			targetSprintID = nil
		} else {
			belongs, err := s.store.SprintBelongsToBoard(r.Context(), rawSprintID, task.BoardID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !belongs {
				writeError(w, http.StatusBadRequest, "sprint does not belong to board")
				return
			}
			targetSprintID = &rawSprintID
		}
	}

	updated, err := s.store.MoveTask(r.Context(), taskID, targetColumnID, req.Position, targetSprintID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "task not found")
			return
		}
		if errors.Is(err, store.ErrSprintRequiredForTask) {
			writeError(w, http.StatusBadRequest, "sprint is required when moving scrum task out of backlog")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) deleteTask(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := currentUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	taskID := strings.TrimSpace(chi.URLParam(r, "taskID"))
	if _, err := s.authorizedTask(r.Context(), taskID, currentUser.ID); err != nil {
		s.writeTaskAccessError(w, err)
		return
	}

	if err := s.store.DeleteTask(r.Context(), taskID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "task not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listTasks(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := currentUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	boardID := strings.TrimSpace(r.URL.Query().Get("board_id"))
	if boardID == "" {
		writeError(w, http.StatusBadRequest, "board_id is required")
		return
	}
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	sprintID := strings.TrimSpace(r.URL.Query().Get("sprint_id"))
	columnID := strings.TrimSpace(r.URL.Query().Get("column_id"))

	workspaceID, err := s.store.WorkspaceIDByBoardID(r.Context(), boardID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "board not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.requireWorkspaceRole(r.Context(), workspaceID, currentUser.ID); err != nil {
		s.writeAccessError(w, err)
		return
	}

	if sprintID != "" {
		belongs, err := s.store.SprintBelongsToBoard(r.Context(), sprintID, boardID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !belongs {
			writeError(w, http.StatusBadRequest, "sprint does not belong to board")
			return
		}
	}
	if columnID != "" {
		belongs, err := s.store.BoardColumnBelongsToBoard(r.Context(), columnID, boardID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !belongs {
			writeError(w, http.StatusBadRequest, "column does not belong to board")
			return
		}
	}

	items, err := s.store.ListTasks(r.Context(), boardID, status, sprintID, columnID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func normalizeColumnKind(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	raw = strings.ReplaceAll(raw, "-", "_")
	raw = strings.ReplaceAll(raw, " ", "_")
	return raw
}

func (s *Server) resolveBoardColumnForTask(ctx context.Context, boardID, rawColumnID, rawStatus string) (store.BoardColumn, error) {
	columnID := strings.TrimSpace(rawColumnID)
	statusKind := normalizeColumnKind(rawStatus)

	if columnID != "" {
		column, err := s.store.GetBoardColumnByID(ctx, columnID)
		if err != nil {
			return store.BoardColumn{}, err
		}
		if column.BoardID != boardID {
			return store.BoardColumn{}, sql.ErrNoRows
		}
		if statusKind != "" && statusKind != column.Kind {
			return store.BoardColumn{}, errors.New("status does not match selected column")
		}
		return column, nil
	}

	if statusKind != "" {
		return s.store.GetBoardColumnByKind(ctx, boardID, statusKind)
	}

	return s.store.GetBoardFirstColumn(ctx, boardID)
}

func parseOptionalDate(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		return err
	}

	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("invalid JSON payload")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, code int, message string) {
	writeJSON(w, code, map[string]string{"error": message})
}

func isFKError(err error) bool {
	if errors.Is(err, sql.ErrNoRows) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "foreign key")
}

func isUniqueViolation(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate key") || strings.Contains(msg, "unique")
}

func isValidEmail(email string) bool {
	return strings.Count(email, "@") == 1 && !strings.HasPrefix(email, "@") && !strings.HasSuffix(email, "@")
}

func isValidWorkspaceRole(role string) bool {
	return role == store.RoleOwner || role == store.RoleMentor || role == store.RoleStudent
}

func hasAllowedRole(role string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, item := range allowed {
		if role == item {
			return true
		}
	}
	return false
}

func (s *Server) authorizedTask(ctx context.Context, taskID, userID string) (store.Task, error) {
	if taskID == "" {
		return store.Task{}, errTaskIDRequired
	}

	task, err := s.store.GetTaskByID(ctx, taskID)
	if err != nil {
		return store.Task{}, err
	}

	workspaceID, err := s.store.WorkspaceIDByBoardID(ctx, task.BoardID)
	if err != nil {
		return store.Task{}, err
	}
	if err := s.requireWorkspaceRole(ctx, workspaceID, userID); err != nil {
		return store.Task{}, err
	}

	return task, nil
}

func (s *Server) requireWorkspaceRole(ctx context.Context, workspaceID, userID string, allowedRoles ...string) error {
	role, err := s.store.UserRoleInWorkspace(ctx, workspaceID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errForbidden
		}
		return err
	}
	if !hasAllowedRole(role, allowedRoles) {
		return errForbidden
	}
	return nil
}

func (s *Server) writeAccessError(w http.ResponseWriter, err error) {
	if errors.Is(err, errForbidden) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	writeError(w, http.StatusInternalServerError, err.Error())
}

func (s *Server) writeTaskAccessError(w http.ResponseWriter, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	if errors.Is(err, errForbidden) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if errors.Is(err, errTaskIDRequired) {
		writeError(w, http.StatusBadRequest, "taskID is required")
		return
	}
	writeError(w, http.StatusInternalServerError, err.Error())
}

func devCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
