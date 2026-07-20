package markdown

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/adrg/frontmatter"
	"github.com/priahin-i/assistente_2/internal/domain"
)

func (s *Store) UpsertPerson(ctx context.Context, person domain.Person) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureDirs(); err != nil {
		return err
	}
	if _, err := os.Stat(s.personPath(person.ID)); err == nil {
		person.UpdatedAt = time.Now().UTC()
		return s.writePerson(person)
	} else if !os.IsNotExist(err) {
		return err
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
