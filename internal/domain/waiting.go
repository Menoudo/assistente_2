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
