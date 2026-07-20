package markdown

import (
	"fmt"
	"strings"
	"time"

	"github.com/priahin-i/assistente_2/internal/domain"
)

const (
	kindWaiting = "waiting"
	kindPerson  = "person"
)

type waitingFrontmatter struct {
	Kind             string   `yaml:"kind"`
	ID               string   `yaml:"id"`
	Status           string   `yaml:"status"`
	Responsible      string   `yaml:"responsible"`
	ExpectedResult   string   `yaml:"expected_result"`
	ResultDeadline   string   `yaml:"result_deadline,omitempty"`
	PromisedDeadline string   `yaml:"promised_deadline,omitempty"`
	NextCheck        string   `yaml:"next_check"`
	Context          string   `yaml:"context"`
	Tags             []string `yaml:"tags,omitempty"`
	Blocker          string   `yaml:"blocker,omitempty"`
	CreatedAt        string   `yaml:"created_at"`
	UpdatedAt        string   `yaml:"updated_at"`
}

type personFrontmatter struct {
	Kind                string `yaml:"kind"`
	ID                  string `yaml:"id"`
	Name                string `yaml:"name"`
	Relation            string `yaml:"relation"`
	DefaultCheckCadence string `yaml:"default_check_cadence"`
	Contact             string `yaml:"contact,omitempty"`
	Active              bool   `yaml:"active"`
	CreatedAt           string `yaml:"created_at"`
	UpdatedAt           string `yaml:"updated_at"`
}

type waitingBody struct {
	Title   string
	Notes   string
	History []domain.CheckRecord
}

type personBody struct {
	Title string
	Notes string
}

func validateWaitingFrontmatter(meta waitingFrontmatter, fileID string) error {
	if meta.Kind != kindWaiting {
		return fmt.Errorf("%w: kind must be %q", domain.ErrInvalidInput, kindWaiting)
	}
	if strings.TrimSpace(meta.ID) == "" {
		return fmt.Errorf("%w: id is required", domain.ErrInvalidInput)
	}
	if meta.ID != fileID {
		return fmt.Errorf("%w: id %q does not match file name %q", domain.ErrInvalidInput, meta.ID, fileID)
	}
	if !isWaitingStatus(meta.Status) {
		return fmt.Errorf("%w: invalid status %q", domain.ErrInvalidInput, meta.Status)
	}
	if strings.TrimSpace(meta.Responsible) == "" {
		return fmt.Errorf("%w: responsible is required", domain.ErrInvalidInput)
	}
	if strings.TrimSpace(meta.ExpectedResult) == "" {
		return fmt.Errorf("%w: expected_result is required", domain.ErrInvalidInput)
	}
	if strings.TrimSpace(meta.NextCheck) == "" {
		return fmt.Errorf("%w: next_check is required", domain.ErrInvalidInput)
	}
	if !isContext(meta.Context) {
		return fmt.Errorf("%w: invalid context %q", domain.ErrInvalidInput, meta.Context)
	}
	if strings.TrimSpace(meta.CreatedAt) == "" || strings.TrimSpace(meta.UpdatedAt) == "" {
		return fmt.Errorf("%w: created_at and updated_at are required", domain.ErrInvalidInput)
	}
	return nil
}

func validatePersonFrontmatter(meta personFrontmatter, fileID string) error {
	if meta.Kind != kindPerson {
		return fmt.Errorf("%w: kind must be %q", domain.ErrInvalidInput, kindPerson)
	}
	if strings.TrimSpace(meta.ID) == "" {
		return fmt.Errorf("%w: id is required", domain.ErrInvalidInput)
	}
	if meta.ID != fileID {
		return fmt.Errorf("%w: id %q does not match file name %q", domain.ErrInvalidInput, meta.ID, fileID)
	}
	if strings.TrimSpace(meta.Name) == "" {
		return fmt.Errorf("%w: name is required", domain.ErrInvalidInput)
	}
	if !isRelation(meta.Relation) {
		return fmt.Errorf("%w: invalid relation %q", domain.ErrInvalidInput, meta.Relation)
	}
	if !isCadence(meta.DefaultCheckCadence) {
		return fmt.Errorf("%w: invalid default_check_cadence %q", domain.ErrInvalidInput, meta.DefaultCheckCadence)
	}
	if strings.TrimSpace(meta.CreatedAt) == "" || strings.TrimSpace(meta.UpdatedAt) == "" {
		return fmt.Errorf("%w: created_at and updated_at are required", domain.ErrInvalidInput)
	}
	return nil
}

func isWaitingStatus(value string) bool {
	switch domain.Status(value) {
	case domain.StatusActive, domain.StatusDone, domain.StatusCancelled:
		return true
	default:
		return false
	}
}

func isContext(value string) bool {
	switch domain.Context(value) {
	case domain.ContextWork, domain.ContextPersonal:
		return true
	default:
		return false
	}
}

func isRelation(value string) bool {
	switch domain.Relation(value) {
	case domain.RelationDeveloper, domain.RelationCustomer, domain.RelationAssistant, domain.RelationColleague, domain.RelationPersonal:
		return true
	default:
		return false
	}
}

func isCadence(value string) bool {
	switch domain.CheckCadence(value) {
	case domain.CadenceDaily, domain.Cadence3Days, domain.CadenceWeekly, domain.CadenceMonthly:
		return true
	default:
		return false
	}
}

func parseDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("%w: empty date", domain.ErrInvalidInput)
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.UTC(), nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: invalid date %q", domain.ErrInvalidInput, value)
	}
	return parsed.UTC(), nil
}

func formatDate(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format("2006-01-02")
}

func formatTimestamp(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func optionalDate(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := parseDate(value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}
