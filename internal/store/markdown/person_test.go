package markdown_test

import (
	"context"
	"errors"
	"testing"

	"github.com/priahin-i/assistente_2/internal/domain"
	mdstore "github.com/priahin-i/assistente_2/internal/store/markdown"
)

func TestPersonOverwriteRequiresForce(t *testing.T) {
	store := mdstore.NewStore(t.TempDir())
	ctx := context.Background()

	person := domain.Person{
		ID:                  "alex",
		Name:                "Алекс",
		Relation:            domain.RelationAssistant,
		DefaultCheckCadence: domain.CadenceWeekly,
		Active:              true,
	}
	if err := store.UpsertPerson(ctx, person, false); err != nil {
		t.Fatalf("create person: %v", err)
	}

	person.Name = "Алексей"
	if err := store.UpsertPerson(ctx, person, false); !errors.Is(err, domain.ErrOverwriteForbidden) {
		t.Fatalf("expected ErrOverwriteForbidden, got %v", err)
	}

	if err := store.UpsertPerson(ctx, person, true); err != nil {
		t.Fatalf("force update: %v", err)
	}

	got, err := store.GetPerson(ctx, "alex")
	if err != nil {
		t.Fatalf("get person: %v", err)
	}
	if got.Name != "Алексей" {
		t.Fatalf("name = %q, want Алексей", got.Name)
	}
}

func TestPersonNameMustBeUnique(t *testing.T) {
	store := mdstore.NewStore(t.TempDir())
	ctx := context.Background()

	first := domain.Person{
		ID:                  "alex-lead",
		Name:                "Алекс",
		Relation:            domain.RelationAssistant,
		DefaultCheckCadence: domain.CadenceWeekly,
		Active:              true,
	}
	if err := store.UpsertPerson(ctx, first, false); err != nil {
		t.Fatalf("create first person: %v", err)
	}

	second := domain.Person{
		ID:                  "alex-dev",
		Name:                "алекс",
		Relation:            domain.RelationDeveloper,
		DefaultCheckCadence: domain.CadenceWeekly,
		Active:              true,
	}
	if err := store.UpsertPerson(ctx, second, false); !errors.Is(err, domain.ErrNameNotUnique) {
		t.Fatalf("expected ErrNameNotUnique, got %v", err)
	}
}

func TestPersonRenameChecksNameUniqueness(t *testing.T) {
	store := mdstore.NewStore(t.TempDir())
	ctx := context.Background()

	people := []domain.Person{
		{
			ID: "ivan-dev", Name: "Иван", Relation: domain.RelationDeveloper,
			DefaultCheckCadence: domain.CadenceWeekly, Active: true,
		},
		{
			ID: "petr-assistant", Name: "Пётр", Relation: domain.RelationAssistant,
			DefaultCheckCadence: domain.CadenceWeekly, Active: true,
		},
	}
	for _, person := range people {
		if err := store.UpsertPerson(ctx, person, false); err != nil {
			t.Fatalf("create %s: %v", person.ID, err)
		}
	}

	conflict := domain.Person{
		ID: "ivan-dev", Name: "Пётр", Relation: domain.RelationDeveloper,
		DefaultCheckCadence: domain.CadenceWeekly, Active: true,
	}
	if err := store.UpsertPerson(ctx, conflict, true); !errors.Is(err, domain.ErrNameNotUnique) {
		t.Fatalf("expected ErrNameNotUnique, got %v", err)
	}
}
