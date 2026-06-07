package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/John-Santa/talos/platform/jira-evidence-loop/domain/evidence"
	"github.com/John-Santa/talos/platform/jira-evidence-loop/port"
)

// ---------------------------------------------------------------------------
// Typed errors (REQ-ERR)
// ---------------------------------------------------------------------------

// ErrDuplicateIssue is returned when the idempotency JQL pre-check finds more
// than one existing issue for the (change, phase, agent) tuple.
type ErrDuplicateIssue struct {
	Change string
	Phase  string
	Agent  string
	Count  int
}

func (e *ErrDuplicateIssue) Error() string {
	return fmt.Sprintf(
		"evidence loop: idempotency check found %d issues for (change=%s, phase=%s, agent=%s); expected at most 1",
		e.Count, e.Change, e.Phase, e.Agent,
	)
}

// ErrTransitionUnreachable is returned when no suitable transition exists for
// the desired status category (and the issue is not already in that category).
type ErrTransitionUnreachable struct {
	IssueKey       string
	TargetCategory string
}

func (e *ErrTransitionUnreachable) Error() string {
	return fmt.Sprintf(
		"evidence loop: no transition to category %q found for issue %s",
		e.TargetCategory, e.IssueKey,
	)
}

// ErrAttachmentFailed wraps the underlying error for a failed AddAttachment
// call (step 6, fail-loud per §7 and REQ-ATTACH).
type ErrAttachmentFailed struct {
	IssueKey string
	Cause    error
}

func (e *ErrAttachmentFailed) Error() string {
	return fmt.Sprintf("evidence loop: attachment upload failed for issue %s: %v", e.IssueKey, e.Cause)
}

func (e *ErrAttachmentFailed) Unwrap() error { return e.Cause }

// ErrRemoteLinkFailed wraps the underlying error for a failed CreateRemoteLink
// call (step 5, fail-loud per §7 and REQ-REMOTELINK).
type ErrRemoteLinkFailed struct {
	IssueKey string
	Cause    error
}

func (e *ErrRemoteLinkFailed) Error() string {
	return fmt.Sprintf("evidence loop: remote link creation failed for issue %s: %v", e.IssueKey, e.Cause)
}

func (e *ErrRemoteLinkFailed) Unwrap() error { return e.Cause }

// ---------------------------------------------------------------------------
// RunInput
// ---------------------------------------------------------------------------

// RunInput carries all caller-supplied values for a single EvidenceLoop.Run
// invocation. The service never reads environment variables directly.
type RunInput struct {
	// Identity / ownership
	Module string
	Agent  string
	Change string
	Phase  string

	// Issue content
	Summary     string
	Description evidence.ADFDocument

	// Comment for step 3
	Comment evidence.ADFDocument

	// Remote link for step 5
	PRURL string

	// Attachment for step 6
	Attachment evidence.Attachment

	// Worklog seconds for step 4 (falls back to Config.DefaultWorklogSeconds if 0)
	WorklogSeconds int

	// WorklogStarted is the ISO-8601 timestamp string for the worklog.
	// Defaults to the RFC3339 representation of now if empty — callers
	// should inject a deterministic value in tests.
	WorklogStarted string

	// OwnershipMap is the module→agent ownership table injected by the caller
	// (read from team-context/ownership.md at the composition root). Domain
	// receives it here; the domain never reads files.
	OwnershipMap map[string]string
}

// ---------------------------------------------------------------------------
// EvidenceLoop
// ---------------------------------------------------------------------------

// EvidenceLoop is the primary use case. It orchestrates the 7-step Jira
// evidence flow using the injected JiraClient port. It depends only on the
// port interface and Config — never on adapter/rest.
type EvidenceLoop struct {
	client port.JiraClient
	cfg    Config
}

// NewEvidenceLoop constructs an EvidenceLoop with the given client and config.
func NewEvidenceLoop(client port.JiraClient, cfg Config) *EvidenceLoop {
	return &EvidenceLoop{client: client, cfg: cfg}
}

// Run executes the 7-step evidence flow and returns the Jira issue key.
//
// Step 0  — Ownership guard (pure, no API call)
// Step 1  — Idempotent create (JQL Search → reuse or CreateIssue)
// Step 2  — Transition → indeterminate (In Progress)
// Step 3  — AddComment (ADF)
// Step 4  — AddWorklog
// Step 5  — CreateRemoteLink (fail-loud)
// Step 6  — AddAttachment (fail-loud)
// Step 7  — Transition → done
func (l *EvidenceLoop) Run(ctx context.Context, in RunInput) (string, error) {
	// -----------------------------------------------------------------------
	// Step 0: Ownership guard — before any API call (REQ-LABEL, Design §D0)
	// -----------------------------------------------------------------------
	if err := evidence.ValidateOwnership(in.Module, in.Agent, in.OwnershipMap); err != nil {
		return "", err
	}

	// -----------------------------------------------------------------------
	// Step 1: Idempotent create (REQ-IDEM)
	// -----------------------------------------------------------------------
	jql := fmt.Sprintf(
		`project = "%s" AND labels = "change:%s" AND labels = "phase:%s" AND labels = "agent:%s"`,
		l.cfg.ProjectKey, in.Change, in.Phase, in.Agent,
	)
	matches, err := l.client.Search(ctx, jql, 2)
	if err != nil {
		return "", fmt.Errorf("evidence loop step 1 (search): %w", err)
	}

	var issueKey string
	switch len(matches) {
	case 0:
		// No existing issue — create one.
		labels := evidence.LabelSet{
			{Key: "change", Value: in.Change},
			{Key: "phase", Value: in.Phase},
			{Key: "agent", Value: in.Agent},
			{Key: "module", Value: in.Module},
		}
		req := evidence.CreateIssueRequest{
			ProjectKey:    l.cfg.ProjectKey,
			ProjectID:     l.cfg.ProjectID,
			IssueTypeName: l.cfg.IssueTypeName,
			Summary:       in.Summary,
			Description:   in.Description,
			Labels:        labels,
		}
		created, err := l.client.CreateIssue(ctx, req)
		if err != nil {
			return "", fmt.Errorf("evidence loop step 1 (create): %w", err)
		}
		issueKey = created.Key

	case 1:
		// Exactly one match — reuse it.
		issueKey = matches[0].Key

	default:
		// More than one match — data integrity violation.
		return "", &ErrDuplicateIssue{
			Change: in.Change,
			Phase:  in.Phase,
			Agent:  in.Agent,
			Count:  len(matches),
		}
	}

	// -----------------------------------------------------------------------
	// Step 2: Transition → indeterminate (In Progress)
	// -----------------------------------------------------------------------
	if err := l.doTransitionToCategory(ctx, issueKey, StatusCategoryIndeterminate, 0); err != nil {
		return issueKey, fmt.Errorf("evidence loop step 2 (transition→indeterminate): %w", err)
	}

	// -----------------------------------------------------------------------
	// Step 3: AddComment
	// -----------------------------------------------------------------------
	if err := l.client.AddComment(ctx, issueKey, in.Comment); err != nil {
		return issueKey, fmt.Errorf("evidence loop step 3 (comment): %w", err)
	}

	// -----------------------------------------------------------------------
	// Step 4: AddWorklog
	// -----------------------------------------------------------------------
	seconds := in.WorklogSeconds
	if seconds <= 0 {
		seconds = l.cfg.DefaultWorklogSeconds
	}
	worklog := evidence.Worklog{
		TimeSpentSeconds: seconds,
		Comment:          in.Comment,
		Started:          in.WorklogStarted,
	}
	if err := l.client.AddWorklog(ctx, issueKey, worklog); err != nil {
		return issueKey, fmt.Errorf("evidence loop step 4 (worklog): %w", err)
	}

	// -----------------------------------------------------------------------
	// Step 5: CreateRemoteLink — FAIL LOUD (REQ-REMOTELINK)
	// -----------------------------------------------------------------------
	link := evidence.RemoteLink{
		PRURL:        in.PRURL,
		Relationship: l.cfg.RemoteLinkRelationship,
	}
	if err := l.client.CreateRemoteLink(ctx, issueKey, link); err != nil {
		return issueKey, &ErrRemoteLinkFailed{IssueKey: issueKey, Cause: err}
	}

	// -----------------------------------------------------------------------
	// Step 6: AddAttachment — FAIL LOUD (REQ-ATTACH, §7)
	// -----------------------------------------------------------------------
	if err := l.client.AddAttachment(ctx, issueKey, in.Attachment); err != nil {
		return issueKey, &ErrAttachmentFailed{IssueKey: issueKey, Cause: err}
	}

	// -----------------------------------------------------------------------
	// Step 7: Transition → done
	// -----------------------------------------------------------------------
	if err := l.doTransitionToCategory(ctx, issueKey, StatusCategoryDone, 0); err != nil {
		return issueKey, fmt.Errorf("evidence loop step 7 (transition→done): %w", err)
	}

	return issueKey, nil
}

// doTransitionToCategory resolves and applies a transition to the target
// status category using the category-primary / ordinal-secondary /
// name-tiebreak algorithm (Design §Transition resolution algorithm).
//
// If no transition to targetCat is available, it is treated as a no-op
// (the issue is assumed to already be in the target category — REQ-TRANS).
func (l *EvidenceLoop) doTransitionToCategory(ctx context.Context, issueKey, targetCat string, ordinal int) error {
	transitions, err := l.client.GetTransitions(ctx, issueKey)
	if err != nil {
		return fmt.Errorf("GetTransitions(%s): %w", issueKey, err)
	}

	// Filter by target category.
	var candidates []evidence.Transition
	for _, t := range transitions {
		if t.ToCategory == targetCat {
			candidates = append(candidates, t)
		}
	}

	switch len(candidates) {
	case 0:
		// No transition available for this category — treat as no-op (already
		// in target state or state machine configuration omits that transition).
		return nil

	case 1:
		return l.client.DoTransition(ctx, issueKey, candidates[0].ID)

	default:
		// Multiple candidates: use ordinal-secondary + name-tiebreak.
		orderedNames := l.cfg.OrderedStates[targetCat]
		if ordinal < len(orderedNames) {
			desired := strings.ToLower(orderedNames[ordinal])
			for _, c := range candidates {
				if strings.ToLower(c.ToName) == desired {
					return l.client.DoTransition(ctx, issueKey, c.ID)
				}
			}
		}
		// Fallback: ordinal position in candidates slice.
		if ordinal < len(candidates) {
			return l.client.DoTransition(ctx, issueKey, candidates[ordinal].ID)
		}
		return &ErrTransitionUnreachable{IssueKey: issueKey, TargetCategory: targetCat}
	}
}
