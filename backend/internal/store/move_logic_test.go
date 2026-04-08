package store

import (
	"database/sql"
	"errors"
	"testing"
)

func strPtr(v string) *string {
	return &v
}

func TestNormalizedOptionalID(t *testing.T) {
	if got := normalizedOptionalID(nil); got != "" {
		t.Fatalf("normalizedOptionalID(nil) = %q, want empty", got)
	}

	raw := "  sprint-1  "
	if got := normalizedOptionalID(&raw); got != "sprint-1" {
		t.Fatalf("normalizedOptionalID(trimmed) = %q, want %q", got, "sprint-1")
	}
}

func TestResolveNextSprintBinding(t *testing.T) {
	tests := []struct {
		name          string
		boardType     string
		targetStatus  string
		currentSprint string
		targetSprint  *string
		want          sql.NullString
		wantErr       error
	}{
		{
			name:          "to backlog clears sprint",
			boardType:     "scrum",
			targetStatus:  "backlog",
			currentSprint: "sprint-a",
			targetSprint:  strPtr("sprint-b"),
			want:          sql.NullString{},
		},
		{
			name:          "explicit sprint wins",
			boardType:     "scrum",
			targetStatus:  "todo",
			currentSprint: "sprint-a",
			targetSprint:  strPtr("sprint-b"),
			want:          sql.NullString{String: "sprint-b", Valid: true},
		},
		{
			name:          "keep current sprint",
			boardType:     "scrum",
			targetStatus:  "done",
			currentSprint: "sprint-a",
			targetSprint:  nil,
			want:          sql.NullString{String: "sprint-a", Valid: true},
		},
		{
			name:          "scrum requires sprint outside backlog",
			boardType:     "scrum",
			targetStatus:  "todo",
			currentSprint: "",
			targetSprint:  nil,
			want:          sql.NullString{},
			wantErr:       ErrSprintRequiredForTask,
		},
		{
			name:          "kanban can stay without sprint",
			boardType:     "kanban",
			targetStatus:  "todo",
			currentSprint: "",
			targetSprint:  nil,
			want:          sql.NullString{},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := resolveNextSprintBinding(tt.boardType, tt.targetStatus, tt.currentSprint, tt.targetSprint)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("resolveNextSprintBinding() error = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("resolveNextSprintBinding() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestValidateRemoveTaskFromSprint(t *testing.T) {
	tests := []struct {
		name         string
		boardType    string
		sprintStatus string
		wantErr      error
	}{
		{
			name:         "scrum planned allowed",
			boardType:    "scrum",
			sprintStatus: "planned",
		},
		{
			name:         "scrum active allowed",
			boardType:    "scrum",
			sprintStatus: "active",
		},
		{
			name:         "scrum closed allowed",
			boardType:    "scrum",
			sprintStatus: "closed",
		},
		{
			name:         "kanban not allowed",
			boardType:    "kanban",
			sprintStatus: "closed",
			wantErr:      ErrSprintBoardNotScrum,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateRemoveTaskFromSprint(tt.boardType, tt.sprintStatus)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("validateRemoveTaskFromSprint() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
