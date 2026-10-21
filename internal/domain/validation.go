package domain

import (
	"fmt"
	"strings"
	"time"
	"unicode"
)

// ValidateID accepts one filename component, never a path.
func ValidateID(id string) error {
	if id == "" || id != strings.TrimSpace(id) || id == "." || id == ".." || strings.ContainsAny(id, "/\\") || strings.ContainsFunc(id, unicode.IsControl) {
		return fmt.Errorf("%w: invalid id %q", ErrInvalidInput, id)
	}
	return nil
}

func ValidateWaiting(w Waiting) error {
	if err := ValidateID(w.ID); err != nil {
		return err
	}
	if err := ValidateID(w.Responsible); err != nil {
		return err
	}
	if strings.TrimSpace(w.ExpectedResult) == "" {
		return fmt.Errorf("%w: expected_result is required", ErrInvalidInput)
	}
	if w.Status != StatusActive && w.Status != StatusDone && w.Status != StatusCancelled {
		return fmt.Errorf("%w: invalid status", ErrInvalidInput)
	}
	if w.Context != ContextWork && w.Context != ContextPersonal {
		return fmt.Errorf("%w: invalid context", ErrInvalidInput)
	}
	if w.NextCheck.IsZero() || w.CreatedAt.IsZero() || w.UpdatedAt.IsZero() {
		return fmt.Errorf("%w: next_check, created_at and updated_at are required", ErrInvalidInput)
	}
	for _, d := range []*time.Time{w.ResultDeadline, w.PromisedDeadline} {
		if d != nil && d.IsZero() {
			return fmt.Errorf("%w: empty deadline", ErrInvalidInput)
		}
	}
	for _, record := range w.CheckHistory {
		if err := ValidateCheck(record); err != nil {
			return err
		}
	}
	return nil
}

func ValidateCheck(record CheckRecord) error {
	if record.Date.IsZero() || strings.TrimSpace(record.Action) == "" || strings.ContainsAny(record.Action, "\r\n") {
		return fmt.Errorf("%w: check date and single-line action are required", ErrInvalidInput)
	}
	return nil
}

func ValidatePerson(p Person) error {
	if err := ValidateID(p.ID); err != nil {
		return err
	}
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidInput)
	}
	switch p.Relation {
	case RelationDeveloper, RelationCustomer, RelationAssistant, RelationColleague, RelationPersonal:
	default:
		return fmt.Errorf("%w: invalid relation", ErrInvalidInput)
	}
	switch p.DefaultCheckCadence {
	case CadenceDaily, Cadence3Days, CadenceWeekly, CadenceMonthly:
	default:
		return fmt.Errorf("%w: invalid default_check_cadence", ErrInvalidInput)
	}
	return nil
}

// CalendarDate keeps the local calendar day, represented at midnight UTC.
// A date is a calendar value, not an instant to convert between zones.
func CalendarDate(value time.Time) time.Time {
	y, m, d := value.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func DaysOverdue(nextCheck, asOf time.Time) int {
	days := int(CalendarDate(asOf).Sub(CalendarDate(nextCheck)).Hours() / 24)
	if days < 0 {
		return 0
	}
	return days
}
