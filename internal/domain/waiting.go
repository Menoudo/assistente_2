package domain

import "time"

type Status string

const (
	StatusActive    Status = "active"
	StatusDone      Status = "done"
	StatusCancelled Status = "cancelled"
)

type Context string

const (
	ContextWork     Context = "work"
	ContextPersonal Context = "personal"
)

type Waiting struct {
	ID               string
	Status           Status
	Responsible      string
	ExpectedResult   string
	ResultDeadline   *time.Time
	PromisedDeadline *time.Time
	NextCheck        time.Time
	Context          Context
	Tags             []string
	Blocker          string
	Notes            string
	CheckHistory     []CheckRecord
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type CheckRecord struct {
	Date    time.Time
	Action  string
	Message string
}

type WaitingFilter struct {
	Status          *Status
	Responsible     string
	Context         *Context
	DueOnOrBefore   *time.Time
	IncludeDone     bool
	IncludeCanceled bool
}

type DueWaitingItem struct {
	Waiting     Waiting
	DaysOverdue int
}

// WaitingPatch uses pointers to distinguish omitted fields from empty values.
// Deadline clearing is explicit to keep the MCP schema simple for clients.
type WaitingPatch struct {
	Responsible           *string
	ExpectedResult        *string
	NextCheck             *time.Time
	Context               *Context
	Notes                 *string
	ResultDeadline        *time.Time
	PromisedDeadline      *time.Time
	ClearResultDeadline   bool
	ClearPromisedDeadline bool
	Tags                  *[]string
	Blocker               *string
}

func (p WaitingPatch) Apply(w *Waiting) {
	if p.Responsible != nil {
		w.Responsible = *p.Responsible
	}
	if p.ExpectedResult != nil {
		w.ExpectedResult = *p.ExpectedResult
	}
	if p.NextCheck != nil {
		w.NextCheck = *p.NextCheck
	}
	if p.Context != nil {
		w.Context = *p.Context
	}
	if p.Notes != nil {
		w.Notes = *p.Notes
	}
	if p.ClearResultDeadline {
		w.ResultDeadline = nil
	} else if p.ResultDeadline != nil {
		w.ResultDeadline = p.ResultDeadline
	}
	if p.ClearPromisedDeadline {
		w.PromisedDeadline = nil
	} else if p.PromisedDeadline != nil {
		w.PromisedDeadline = p.PromisedDeadline
	}
	if p.Tags != nil {
		w.Tags = *p.Tags
	}
	if p.Blocker != nil {
		w.Blocker = *p.Blocker
	}
}
