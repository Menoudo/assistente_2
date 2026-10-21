package markdown_test

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/priahin-i/assistente_2/internal/domain"
	mdstore "github.com/priahin-i/assistente_2/internal/store/markdown"
)

func TestRenderAndParseWaitingBody(t *testing.T) {
	waiting := domain.Waiting{
		ID:             "sample",
		Status:         domain.StatusActive,
		Responsible:    "dev",
		ExpectedResult: "Sample result",
		NextCheck:      time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC),
		Context:        domain.ContextWork,
		Notes:          "Some notes",
		CheckHistory: []domain.CheckRecord{
			{
				Date:    time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC),
				Action:  "created",
				Message: "Created item.",
			},
		},
		CreatedAt: time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC),
	}

	root := t.TempDir()
	store := mdstore.NewStore(root)
	if err := store.UpsertPerson(t.Context(), domain.Person{
		ID: "dev", Name: "Dev", Relation: domain.RelationDeveloper,
		DefaultCheckCadence: domain.CadenceWeekly, Active: true,
	}, false); err != nil {
		t.Fatalf("upsert person: %v", err)
	}
	if err := store.CreateWaiting(t.Context(), waiting); err != nil {
		t.Fatalf("create waiting: %v", err)
	}

	raw, err := store.ReadWaitingMarkdown(t.Context(), "sample")
	if err != nil {
		t.Fatalf("read markdown: %v", err)
	}
	got, err := store.GetWaiting(t.Context(), waiting.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Notes != waiting.Notes || len(got.CheckHistory) != 1 || got.CheckHistory[0].Action != waiting.CheckHistory[0].Action || got.CheckHistory[0].Message != waiting.CheckHistory[0].Message || !reflect.DeepEqual(got.Tags, waiting.Tags) {
		t.Fatalf("round trip mismatch: %#v", got)
	}
	for _, section := range []string{"## Notes", "## Check history", "expected_result", "next_check"} {
		if !strings.Contains(raw, section) {
			t.Fatalf("markdown missing %q:\n%s", section, raw)
		}
	}
}
