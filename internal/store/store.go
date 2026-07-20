package store

import (
	"context"
	"time"

	"github.com/priahin-i/assistente_2/internal/domain"
)

type Repository interface {
	CreateWaiting(ctx context.Context, waiting domain.Waiting) error
	GetWaiting(ctx context.Context, id string) (domain.Waiting, error)
	UpdateWaiting(ctx context.Context, waiting domain.Waiting) error
	ListWaiting(ctx context.Context, filter domain.WaitingFilter) ([]domain.Waiting, error)
	RecordCheck(ctx context.Context, id string, record domain.CheckRecord, nextCheck time.Time, blocker *string, promisedDeadline *time.Time) error
	SetWaitingStatus(ctx context.Context, id string, status domain.Status, record *domain.CheckRecord) error

	UpsertPerson(ctx context.Context, person domain.Person) error
	GetPerson(ctx context.Context, id string) (domain.Person, error)
	ListPeople(ctx context.Context) ([]domain.Person, error)

	ReadWaitingMarkdown(ctx context.Context, id string) (string, error)
	ReadPersonMarkdown(ctx context.Context, id string) (string, error)
	RenderDueReviewMarkdown(ctx context.Context, asOf time.Time) (string, error)
}
