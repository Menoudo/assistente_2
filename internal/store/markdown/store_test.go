package markdown_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/priahin-i/assistente_2/internal/domain"
	mdstore "github.com/priahin-i/assistente_2/internal/store/markdown"
)

func TestWaitingLifecycle(t *testing.T) {
	root := t.TempDir()
	store := mdstore.NewStore(root)
	ctx := context.Background()

	person := domain.Person{
		ID:                  "dev-ivan",
		Name:                "Иван",
		Relation:            domain.RelationDeveloper,
		DefaultCheckCadence: domain.CadenceWeekly,
		Active:              true,
	}
	if err := store.UpsertPerson(ctx, person, false); err != nil {
		t.Fatalf("upsert person: %v", err)
	}

	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	nextCheck := time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)
	waiting := domain.Waiting{
		ID:             "feature-release",
		Status:         domain.StatusActive,
		Responsible:    "dev-ivan",
		ExpectedResult: "Готовый релиз функции",
		NextCheck:      nextCheck,
		Context:        domain.ContextWork,
		Notes:          "См. GitLab milestone",
		CheckHistory: []domain.CheckRecord{
			{Date: now, Action: "created", Message: "Поручено на созвоне."},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := store.CreateWaiting(ctx, waiting); err != nil {
		t.Fatalf("create waiting: %v", err)
	}

	got, err := store.GetWaiting(ctx, "feature-release")
	if err != nil {
		t.Fatalf("get waiting: %v", err)
	}
	if got.ExpectedResult != waiting.ExpectedResult {
		t.Fatalf("expected result mismatch: %q", got.ExpectedResult)
	}

	newNextCheck := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	blocker := "ждём ревью"
	if err := store.RecordCheck(ctx, "feature-release", domain.CheckRecord{
		Date:    now,
		Action:  "pinged",
		Message: "Спросил статус.",
	}, newNextCheck, &blocker, nil); err != nil {
		t.Fatalf("record check: %v", err)
	}

	got, err = store.GetWaiting(ctx, "feature-release")
	if err != nil {
		t.Fatalf("get waiting after check: %v", err)
	}
	if got.Blocker != blocker {
		t.Fatalf("blocker = %q, want %q", got.Blocker, blocker)
	}
	if !got.NextCheck.Equal(newNextCheck) {
		t.Fatalf("next_check = %v, want %v", got.NextCheck, newNextCheck)
	}
	if len(got.CheckHistory) != 2 {
		t.Fatalf("history len = %d, want 2", len(got.CheckHistory))
	}

	if err := store.SetWaitingStatus(ctx, "feature-release", domain.StatusDone, &domain.CheckRecord{
		Date: now, Action: "completed", Message: "Релиз выкатили.",
	}); err != nil {
		t.Fatalf("complete waiting: %v", err)
	}

	activePath := filepath.Join(root, "waiting", "feature-release.md")
	donePath := filepath.Join(root, "waiting", "done", "feature-release.md")
	if _, err := os.Stat(activePath); !os.IsNotExist(err) {
		t.Fatalf("active file should be removed, stat err=%v", err)
	}
	if _, err := os.Stat(donePath); err != nil {
		t.Fatalf("done file should exist: %v", err)
	}

	done, err := store.GetWaiting(ctx, "feature-release")
	if err != nil {
		t.Fatalf("get done waiting: %v", err)
	}
	if done.Status != domain.StatusDone {
		t.Fatalf("status = %q, want done", done.Status)
	}
}

func TestListWaitingDueFilter(t *testing.T) {
	root := t.TempDir()
	store := mdstore.NewStore(root)
	ctx := context.Background()

	if err := store.UpsertPerson(ctx, domain.Person{
		ID:                  "assistant",
		Name:                "Ассистент",
		Relation:            domain.RelationAssistant,
		DefaultCheckCadence: domain.CadenceWeekly,
		Active:              true,
	}, false); err != nil {
		t.Fatalf("upsert person: %v", err)
	}

	now := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	items := []domain.Waiting{
		{
			ID: "due-today", Status: domain.StatusActive, Responsible: "assistant",
			ExpectedResult: "A", NextCheck: now, Context: domain.ContextWork,
			CheckHistory: []domain.CheckRecord{{Date: now, Action: "created", Message: "x"}},
			CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: "due-later", Status: domain.StatusActive, Responsible: "assistant",
			ExpectedResult: "B", NextCheck: now.AddDate(0, 0, 7), Context: domain.ContextWork,
			CheckHistory: []domain.CheckRecord{{Date: now, Action: "created", Message: "x"}},
			CreatedAt: now, UpdatedAt: now,
		},
	}
	for _, item := range items {
		if err := store.CreateWaiting(ctx, item); err != nil {
			t.Fatalf("create %s: %v", item.ID, err)
		}
	}

	due, err := store.ListWaiting(ctx, domain.WaitingFilter{
		Status:        ptrStatus(domain.StatusActive),
		DueOnOrBefore: &now,
	})
	if err != nil {
		t.Fatalf("list due: %v", err)
	}
	if len(due) != 1 || due[0].ID != "due-today" {
		t.Fatalf("due items = %+v, want only due-today", due)
	}
}

func TestListWaitingIncludeDone(t *testing.T) {
	root := t.TempDir()
	store := mdstore.NewStore(root)
	ctx := context.Background()

	if err := store.UpsertPerson(ctx, domain.Person{
		ID: "dev", Name: "Dev", Relation: domain.RelationDeveloper,
		DefaultCheckCadence: domain.CadenceWeekly, Active: true,
	}, false); err != nil {
		t.Fatalf("upsert person: %v", err)
	}

	now := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	active := domain.Waiting{
		ID: "active-item", Status: domain.StatusActive, Responsible: "dev",
		ExpectedResult: "Active", NextCheck: now, Context: domain.ContextWork,
		CheckHistory: []domain.CheckRecord{{Date: now, Action: "created", Message: "x"}},
		CreatedAt: now, UpdatedAt: now,
	}
	if err := store.CreateWaiting(ctx, active); err != nil {
		t.Fatalf("create active: %v", err)
	}
	if err := store.SetWaitingStatus(ctx, "active-item", domain.StatusDone, &domain.CheckRecord{
		Date: now, Action: "completed", Message: "done",
	}); err != nil {
		t.Fatalf("complete: %v", err)
	}

	withoutDone, err := store.ListWaiting(ctx, domain.WaitingFilter{})
	if err != nil {
		t.Fatalf("list without done: %v", err)
	}
	if len(withoutDone) != 0 {
		t.Fatalf("without done = %+v, want empty", withoutDone)
	}

	withDone, err := store.ListWaiting(ctx, domain.WaitingFilter{IncludeDone: true})
	if err != nil {
		t.Fatalf("list with done: %v", err)
	}
	if len(withDone) != 1 || withDone[0].ID != "active-item" {
		t.Fatalf("with done = %+v, want active-item", withDone)
	}
}

func TestListWaitingStatusDone(t *testing.T) {
	root := t.TempDir()
	store := mdstore.NewStore(root)
	ctx := context.Background()

	if err := store.UpsertPerson(ctx, domain.Person{
		ID: "dev", Name: "Dev", Relation: domain.RelationDeveloper,
		DefaultCheckCadence: domain.CadenceWeekly, Active: true,
	}, false); err != nil {
		t.Fatalf("upsert person: %v", err)
	}

	now := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	item := domain.Waiting{
		ID: "archived-item", Status: domain.StatusActive, Responsible: "dev",
		ExpectedResult: "Done task", NextCheck: now, Context: domain.ContextWork,
		CheckHistory: []domain.CheckRecord{{Date: now, Action: "created", Message: "x"}},
		CreatedAt: now, UpdatedAt: now,
	}
	if err := store.CreateWaiting(ctx, item); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := store.SetWaitingStatus(ctx, "archived-item", domain.StatusDone, &domain.CheckRecord{
		Date: now, Action: "completed", Message: "done",
	}); err != nil {
		t.Fatalf("complete: %v", err)
	}

	doneOnly, err := store.ListWaiting(ctx, domain.WaitingFilter{
		Status: ptrStatus(domain.StatusDone),
	})
	if err != nil {
		t.Fatalf("list status done: %v", err)
	}
	if len(doneOnly) != 1 || doneOnly[0].ID != "archived-item" {
		t.Fatalf("status done = %+v, want archived-item", doneOnly)
	}
}

func TestParseExampleMarkdown(t *testing.T) {
	root := filepath.Join("..", "..", "..", "data")
	if _, err := os.Stat(root); err != nil {
		t.Skip("example data not present")
	}
	store := mdstore.NewStore(root)
	ctx := context.Background()

	waiting, err := store.GetWaiting(ctx, "report-draft-main-assistant")
	if err != nil {
		t.Fatalf("get example waiting: %v", err)
	}
	if waiting.Responsible != "main-assistant" {
		t.Fatalf("responsible = %q", waiting.Responsible)
	}
	if len(waiting.CheckHistory) == 0 {
		t.Fatal("expected history entries")
	}
}

func ptrStatus(status domain.Status) *domain.Status {
	return &status
}
