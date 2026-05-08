package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lazerdude-labs/bandolier/api/internal/store"
)

type Outcome string

const (
	OutcomeSuccess Outcome = "success"
	OutcomeFailure Outcome = "failure"
)

type Entry struct {
	ActorID int64
	Action  string
	Target  string
	Outcome Outcome
	Details map[string]any
}

func Write(ctx context.Context, s *store.Store, e Entry) (int64, error) {
	if e.Action == "" {
		return 0, fmt.Errorf("audit: action required")
	}
	switch e.Outcome {
	case OutcomeSuccess, OutcomeFailure, OutcomeStarted, OutcomeSucceeded, OutcomeFailed:
	default:
		return 0, fmt.Errorf("audit: invalid outcome %q", e.Outcome)
	}
	var details *string
	if len(e.Details) > 0 {
		b, err := json.Marshal(e.Details)
		if err != nil {
			return 0, fmt.Errorf("audit: marshal details: %w", err)
		}
		t := string(b)
		details = &t
	}
	return s.InsertAuditEntry(ctx, store.AuditEntry{
		ActorID: e.ActorID,
		Action:  e.Action,
		Target:  e.Target,
		Outcome: string(e.Outcome),
		Details: details,
	})
}
