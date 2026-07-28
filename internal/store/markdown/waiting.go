package markdown

import (
	"bytes"
	"context"
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
	mu   sync.Mutex
}

func NewStore(root string) *Store {
	return &Store{root: root}
}

func (s *Store) waitingDir() string {
	return filepath.Join(s.root, "waiting")
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
	activePath := s.activeWaitingPath(id)
	if _, err := os.Stat(activePath); err == nil {
		return activePath, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}

	donePath := s.doneWaitingPath(id)
	if _, err := os.Stat(donePath); err == nil {
		return donePath, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}

	return "", domain.ErrNotFound
}

func (s *Store) waitingExists(id string) bool {
	_, err := s.locateWaitingPath(id)
	return err == nil
}

func (s *Store) peopleDir() string {
	return filepath.Join(s.root, "people")
}

func (s *Store) personPath(id string) string {
	return filepath.Join(s.peopleDir(), id+".md")
}

func (s *Store) ensureDirs() error {
	if err := os.MkdirAll(s.waitingDir(), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(s.doneWaitingDir(), 0o755); err != nil {
		return err
	}
	return os.MkdirAll(s.peopleDir(), 0o755)
}

func (s *Store) CreateWaiting(ctx context.Context, waiting domain.Waiting) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureDirs(); err != nil {
		return err
	}
	if _, err := os.Stat(s.personPath(waiting.Responsible)); err != nil {
		if os.IsNotExist(err) {
			return domain.ErrResponsibleAbsent
		}
		return err
	}
	if s.waitingExists(waiting.ID) {
		return domain.ErrAlreadyExists
	}

	return s.saveWaiting(waiting)
}

func (s *Store) GetWaiting(ctx context.Context, id string) (domain.Waiting, error) {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readWaiting(id)
}

func (s *Store) UpdateWaiting(ctx context.Context, waiting domain.Waiting) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := os.Stat(s.personPath(waiting.Responsible)); err != nil {
		if os.IsNotExist(err) {
			return domain.ErrResponsibleAbsent
		}
		return err
	}
	if _, err := s.locateWaitingPath(waiting.ID); err != nil {
		if err == domain.ErrNotFound {
			return domain.ErrNotFound
		}
		return err
	}
	waiting.UpdatedAt = time.Now().UTC()
	return s.saveWaiting(waiting)
}

func (s *Store) ListWaiting(ctx context.Context, filter domain.WaitingFilter) ([]domain.Waiting, error) {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()

	var result []domain.Waiting
	for _, dir := range s.listWaitingDirs(filter) {
		entries, err := os.ReadDir(dir)
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
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()

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
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()

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
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()

	path, err := s.locateWaitingPath(id)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
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
		days := daysOverdue(waiting.NextCheck, asOf)
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

func (s *Store) readWaiting(id string) (domain.Waiting, error) {
	path, err := s.locateWaitingPath(id)
	if err != nil {
		return domain.Waiting{}, err
	}

	data, err := os.ReadFile(path)
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
	currentPath, err := s.locateWaitingPath(waiting.ID)
	if err != nil && err != domain.ErrNotFound {
		return err
	}

	targetPath := s.waitingPathFor(waiting)
	data, err := renderWaitingDocument(waiting)
	if err != nil {
		return err
	}
	if err := atomicWrite(targetPath, data); err != nil {
		return err
	}

	if err == nil && currentPath != "" && currentPath != targetPath {
		if removeErr := os.Remove(currentPath); removeErr != nil && !os.IsNotExist(removeErr) {
			return removeErr
		}
	}
	return nil
}

func matchesWaitingFilter(waiting domain.Waiting, filter domain.WaitingFilter) bool {
	if filter.Status != nil && waiting.Status != *filter.Status {
		return false
	}
	if !filter.IncludeDone && waiting.Status == domain.StatusDone {
		return false
	}
	if !filter.IncludeCanceled && waiting.Status == domain.StatusCancelled {
		return false
	}
	if filter.Responsible != "" && waiting.Responsible != filter.Responsible {
		return false
	}
	if filter.Context != nil && waiting.Context != *filter.Context {
		return false
	}
	if filter.DueOnOrBefore != nil {
		dueDate := dateOnly(filter.DueOnOrBefore.UTC())
		nextCheck := dateOnly(waiting.NextCheck.UTC())
		if nextCheck.After(dueDate) {
			return false
		}
	}
	return true
}

func daysOverdue(nextCheck, asOf time.Time) int {
	next := dateOnly(nextCheck.UTC())
	current := dateOnly(asOf.UTC())
	if !current.After(next) {
		return 0
	}
	return int(current.Sub(next).Hours()/24) + 1
}

func dateOnly(value time.Time) time.Time {
	year, month, day := value.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
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

func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		os.Remove(tempPath)
		return err
	}
	if err := temp.Close(); err != nil {
		os.Remove(tempPath)
		return err
	}
	return os.Rename(tempPath, path)
}
