package markdown

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/adrg/frontmatter"
	"github.com/priahin-i/assistente_2/internal/domain"
)

func normalizePersonName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func validatePersonID(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("%w: id is required", domain.ErrInvalidInput)
	}
	if strings.Contains(id, "/") || strings.Contains(id, "\\") || strings.Contains(id, "..") {
		return fmt.Errorf("%w: invalid person id %q", domain.ErrInvalidInput, id)
	}
	return nil
}

func (s *Store) findPersonIDByName(excludeID, name string) (string, error) {
	normalized := normalizePersonName(name)
	if normalized == "" {
		return "", fmt.Errorf("%w: name is required", domain.ErrInvalidInput)
	}

	entries, err := os.ReadDir(s.peopleDir())
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".md")
		if id == excludeID {
			continue
		}
		person, err := s.readPerson(id)
		if err != nil {
			return "", err
		}
		if normalizePersonName(person.Name) == normalized {
			return id, nil
		}
	}
	return "", nil
}

func (s *Store) UpsertPerson(ctx context.Context, person domain.Person, force bool) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := validatePersonID(person.ID); err != nil {
		return err
	}
	if err := s.ensureDirs(); err != nil {
		return err
	}

	exists := false
	if _, err := os.Stat(s.personPath(person.ID)); err == nil {
		exists = true
	} else if !os.IsNotExist(err) {
		return err
	}

	if exists && !force {
		return domain.ErrOverwriteForbidden
	}

	if duplicateID, err := s.findPersonIDByName(person.ID, person.Name); err != nil {
		return err
	} else if duplicateID != "" {
		return fmt.Errorf("%w: already used by person %q", domain.ErrNameNotUnique, duplicateID)
	}

	if exists {
		existing, err := s.readPerson(person.ID)
		if err != nil {
			return err
		}
		person.CreatedAt = existing.CreatedAt
		person.UpdatedAt = time.Now().UTC()
		return s.writePerson(person)
	}
	if person.CreatedAt.IsZero() {
		person.CreatedAt = time.Now().UTC()
	}
	if person.UpdatedAt.IsZero() {
		person.UpdatedAt = person.CreatedAt
	}
	return s.writePerson(person)
}

func (s *Store) GetPerson(ctx context.Context, id string) (domain.Person, error) {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readPerson(id)
}

func (s *Store) ListPeople(ctx context.Context) ([]domain.Person, error) {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.peopleDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var result []domain.Person
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".md")
		person, err := s.readPerson(id)
		if err != nil {
			return nil, err
		}
		result = append(result, person)
	}
	return result, nil
}

func (s *Store) ReadPersonMarkdown(ctx context.Context, id string) (string, error) {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.personPath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return "", domain.ErrNotFound
		}
		return "", err
	}
	return string(data), nil
}

func (s *Store) readPerson(id string) (domain.Person, error) {
	data, err := os.ReadFile(s.personPath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return domain.Person{}, domain.ErrNotFound
		}
		return domain.Person{}, err
	}

	var meta personFrontmatter
	rest, err := frontmatter.Parse(strings.NewReader(string(data)), &meta)
	if err != nil {
		return domain.Person{}, err
	}
	if err := validatePersonFrontmatter(meta, id); err != nil {
		return domain.Person{}, err
	}

	body, err := parsePersonBody(string(rest))
	if err != nil {
		return domain.Person{}, err
	}

	createdAt, err := parseDate(meta.CreatedAt)
	if err != nil {
		return domain.Person{}, err
	}
	updatedAt, err := parseDate(meta.UpdatedAt)
	if err != nil {
		return domain.Person{}, err
	}

	return domain.Person{
		ID:                  meta.ID,
		Name:                meta.Name,
		Relation:            domain.Relation(meta.Relation),
		DefaultCheckCadence: domain.CheckCadence(meta.DefaultCheckCadence),
		Contact:             meta.Contact,
		Active:              meta.Active,
		Notes:               body.Notes,
		CreatedAt:           createdAt,
		UpdatedAt:           updatedAt,
	}, nil
}

func (s *Store) writePerson(person domain.Person) error {
	data, err := renderPersonDocument(person)
	if err != nil {
		return err
	}
	return atomicWrite(s.personPath(person.ID), data)
}
