package markdown

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/adrg/frontmatter"
	"github.com/priahin-i/assistente_2/internal/domain"
)

type Store struct {
	root string
	fs   *os.Root
	mu   sync.Mutex
}

func NewStore(root string) *Store {
	return &Store{root: root}
}

func (s *Store) waitingDir() string {
	return "waiting"
}

func (s *Store) doneWaitingDir() string {
	return filepath.Join(s.waitingDir(), "done")
}

func (s *Store) activeWaitingPath(id string) string {
	return filepath.Join(s.waitingDir(), id+".md")
}

func (s *Store) doneWaitingPath(id string) string {
	return filepath.Join(s.doneWaitingDir(), id+".md")
}

func (s *Store) waitingPathFor(waiting domain.Waiting) string {
	if waiting.Status == domain.StatusDone {
		return s.doneWaitingPath(waiting.ID)
	}
	return s.activeWaitingPath(waiting.ID)
}

func (s *Store) locateWaitingPath(id string) (string, error) {
	if err := domain.ValidateID(id); err != nil {
		return "", err
	}
	activePath := s.activeWaitingPath(id)
	if _, err := s.fs.Stat(activePath); err == nil {
		return activePath, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}

	donePath := s.doneWaitingPath(id)
	if _, err := s.fs.Stat(donePath); err == nil {
		return donePath, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}

	return "", domain.ErrNotFound
}

func (s *Store) peopleDir() string {
	return "people"
}

func (s *Store) personPath(id string) string {
	return filepath.Join(s.peopleDir(), id+".md")
}

func (s *Store) ensureDirs() error {
	if err := s.fs.MkdirAll(s.waitingDir(), 0o755); err != nil {
		return err
	}
	if err := s.fs.MkdirAll(s.doneWaitingDir(), 0o755); err != nil {
		return err
	}
	return s.fs.MkdirAll(s.peopleDir(), 0o755)
}

func (s *Store) CreateWaiting(ctx context.Context, waiting domain.Waiting) error {
	unlock, err := s.lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()

	if err := domain.ValidateWaiting(waiting); err != nil {
		return err
	}
	if err := s.ensureDirs(); err != nil {
		return err
	}
	if _, err := s.readPerson(waiting.Responsible); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.ErrResponsibleAbsent
		}
		return err
	}
	if _, err := s.locateWaitingPath(waiting.ID); err == nil {
		return domain.ErrAlreadyExists
	} else if !errors.Is(err, domain.ErrNotFound) {
		return err
	}

	return s.saveWaiting(waiting)
}

func (s *Store) GetWaiting(ctx context.Context, id string) (domain.Waiting, error) {
	unlock, err := s.lock(ctx)
	if err != nil {
		return domain.Waiting{}, err
	}
	defer unlock()
	return s.readWaiting(id)
}

func (s *Store) UpdateWaiting(ctx context.Context, id string, patch domain.WaitingPatch) (domain.Waiting, error) {
	unlock, err := s.lock(ctx)
	if err != nil {
		return domain.Waiting{}, err
	}
	defer unlock()
	waiting, err := s.readWaiting(id)
	if err != nil {
		return domain.Waiting{}, err
	}
	patch.Apply(&waiting)
	waiting.UpdatedAt = time.Now().UTC()
	if err := domain.ValidateWaiting(waiting); err != nil {
		return domain.Waiting{}, err
	}
	if _, err := s.readPerson(waiting.Responsible); err != nil {
		return domain.Waiting{}, fmt.Errorf("responsible %q: %w", waiting.Responsible, err)
	}
	if err := s.saveWaiting(waiting); err != nil {
		return domain.Waiting{}, err
	}
	return waiting, nil
}

func (s *Store) ListWaiting(ctx context.Context, filter domain.WaitingFilter) ([]domain.Waiting, error) {
	unlock, err := s.lock(ctx)
	if err != nil {
		return nil, err
	}
	defer unlock()

	var result []domain.Waiting
	for _, dir := range s.listWaitingDirs(filter) {
		entries, err := s.readDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			id := strings.TrimSuffix(entry.Name(), ".md")
			waiting, err := s.readWaiting(id)
			if err != nil {
				return nil, err
			}
			if !matchesWaitingFilter(waiting, filter) {
				continue
			}
			result = append(result, waiting)
		}
	}
	return result, nil
}

func (s *Store) listWaitingDirs(filter domain.WaitingFilter) []string {
	if filter.Status != nil {
		switch *filter.Status {
		case domain.StatusDone:
			return []string{s.doneWaitingDir()}
		case domain.StatusActive, domain.StatusCancelled:
			return []string{s.waitingDir()}
		}
	}

	dirs := []string{s.waitingDir()}
	if filter.IncludeDone {
		dirs = append(dirs, s.doneWaitingDir())
	}
	return dirs
}

func (s *Store) RecordCheck(
	ctx context.Context,
	id string,
	record domain.CheckRecord,
	nextCheck time.Time,
	blocker *string,
	promisedDeadline *time.Time,
) error {
	unlock, err := s.lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()

	waiting, err := s.readWaiting(id)
	if err != nil {
		return err
	}
	waiting.CheckHistory = append(waiting.CheckHistory, record)
	waiting.NextCheck = nextCheck.UTC()
	if blocker != nil {
		waiting.Blocker = *blocker
	}
	if promisedDeadline != nil {
		waiting.PromisedDeadline = promisedDeadline
	}
	waiting.UpdatedAt = time.Now().UTC()
	return s.saveWaiting(waiting)
}

func (s *Store) SetWaitingStatus(ctx context.Context, id string, status domain.Status, record *domain.CheckRecord) error {
	unlock, err := s.lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()

	waiting, err := s.readWaiting(id)
	if err != nil {
		return err
	}
	waiting.Status = status
	if record != nil {
		waiting.CheckHistory = append(waiting.CheckHistory, *record)
	}
	waiting.UpdatedAt = time.Now().UTC()
	return s.saveWaiting(waiting)
}

func (s *Store) ReadWaitingMarkdown(ctx context.Context, id string) (string, error) {
	unlock, err := s.lock(ctx)
	if err != nil {
		return "", err
	}
	defer unlock()

	path, err := s.locateWaitingPath(id)
	if err != nil {
		return "", err
	}
	data, err := s.fs.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *Store) RenderDueReviewMarkdown(ctx context.Context, asOf time.Time) (string, error) {
	items, err := s.ListWaiting(ctx, domain.WaitingFilter{
		Status:        ptrStatus(domain.StatusActive),
		DueOnOrBefore: &asOf,
	})
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	builder.WriteString("# Waiting review due\n\n")
	builder.WriteString("As of: ")
	builder.WriteString(formatDate(asOf))
	builder.WriteString("\n\n")
	if len(items) == 0 {
		builder.WriteString("No due waiting items.\n")
		return builder.String(), nil
	}

	for _, waiting := range items {
		days := domain.DaysOverdue(waiting.NextCheck, asOf)
		builder.WriteString("## ")
		builder.WriteString(waiting.ID)
		builder.WriteString("\n\n")
		builder.WriteString("- responsible: ")
		builder.WriteString(waiting.Responsible)
		builder.WriteString("\n- expected_result: ")
		builder.WriteString(waiting.ExpectedResult)
		builder.WriteString("\n- next_check: ")
		builder.WriteString(formatDate(waiting.NextCheck))
		builder.WriteString("\n- days_overdue: ")
		builder.WriteString(itoa(days))
		builder.WriteString("\n")
		if waiting.Blocker != "" {
			builder.WriteString("- blocker: ")
			builder.WriteString(waiting.Blocker)
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
	}
	return builder.String(), nil
}

func (s *Store) readWaiting(id string) (result domain.Waiting, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("waiting %q: %w", id, err)
		}
	}()
	path, err := s.locateWaitingPath(id)
	if err != nil {
		return domain.Waiting{}, err
	}

	data, err := s.fs.ReadFile(path)
	if err != nil {
		return domain.Waiting{}, err
	}

	var meta waitingFrontmatter
	rest, err := frontmatter.Parse(bytes.NewReader(data), &meta)
	if err != nil {
		return domain.Waiting{}, err
	}
	if err := validateWaitingFrontmatter(meta, id); err != nil {
		return domain.Waiting{}, err
	}

	body, err := parseWaitingBody(string(rest))
	if err != nil {
		return domain.Waiting{}, err
	}

	nextCheck, err := parseDate(meta.NextCheck)
	if err != nil {
		return domain.Waiting{}, err
	}
	resultDeadline, err := optionalDate(meta.ResultDeadline)
	if err != nil {
		return domain.Waiting{}, err
	}
	promisedDeadline, err := optionalDate(meta.PromisedDeadline)
	if err != nil {
		return domain.Waiting{}, err
	}
	createdAt, err := parseDate(meta.CreatedAt)
	if err != nil {
		return domain.Waiting{}, err
	}
	updatedAt, err := parseDate(meta.UpdatedAt)
	if err != nil {
		return domain.Waiting{}, err
	}

	return domain.Waiting{
		ID:               meta.ID,
		Status:           domain.Status(meta.Status),
		Responsible:      meta.Responsible,
		ExpectedResult:   meta.ExpectedResult,
		ResultDeadline:   resultDeadline,
		PromisedDeadline: promisedDeadline,
		NextCheck:        nextCheck,
		Context:          domain.Context(meta.Context),
		Tags:             meta.Tags,
		Blocker:          meta.Blocker,
		Notes:            body.Notes,
		CheckHistory:     body.History,
		CreatedAt:        createdAt,
		UpdatedAt:        updatedAt,
	}, nil
}

func (s *Store) saveWaiting(waiting domain.Waiting) error {
	if err := domain.ValidateWaiting(waiting); err != nil {
		return err
	}
	currentPath, err := s.locateWaitingPath(waiting.ID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return err
	}

	targetPath := s.waitingPathFor(waiting)
	data, err := renderWaitingDocument(waiting)
	if err != nil {
		return err
	}
	if err := s.atomicWrite(targetPath, data); err != nil {
		return err
	}

	if err == nil && currentPath != "" && currentPath != targetPath {
		if removeErr := s.fs.Remove(currentPath); removeErr != nil && !os.IsNotExist(removeErr) {
			return removeErr
		}
	}
	return nil
}

func matchesWaitingFilter(waiting domain.Waiting, filter domain.WaitingFilter) bool {
	if filter.Status != nil && waiting.Status != *filter.Status {
		return false
	}
	if !filter.IncludeDone && waiting.Status == domain.StatusDone && filter.Status == nil {
		return false
	}
	if !filter.IncludeCanceled && waiting.Status == domain.StatusCancelled && filter.Status == nil {
		return false
	}
	if filter.Responsible != "" && waiting.Responsible != filter.Responsible {
		return false
	}
	if filter.Context != nil && waiting.Context != *filter.Context {
		return false
	}
	if filter.DueOnOrBefore != nil {
		dueDate := domain.CalendarDate(*filter.DueOnOrBefore)
		nextCheck := domain.CalendarDate(waiting.NextCheck)
		if nextCheck.After(dueDate) {
			return false
		}
	}
	return true
}

func ptrStatus(status domain.Status) *domain.Status {
	return &status
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	digits := []byte{}
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	if negative {
		return "-" + string(digits)
	}
	return string(digits)
}
