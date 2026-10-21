package markdown_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/priahin-i/assistente_2/internal/domain"
	mdstore "github.com/priahin-i/assistente_2/internal/store/markdown"
)

func seed(t *testing.T, root string) (*mdstore.Store, domain.Waiting) {
	t.Helper()
	s := mdstore.NewStore(root)
	if err := s.UpsertPerson(t.Context(), domain.Person{ID: "tester", Name: "Tester", Relation: domain.RelationColleague, DefaultCheckCadence: domain.CadenceWeekly, Active: true}, false); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	w := domain.Waiting{ID: "sample", Responsible: "tester", ExpectedResult: "Result", Status: domain.StatusActive, Context: domain.ContextWork, NextCheck: now, CreatedAt: now, UpdatedAt: now}
	return s, w
}

func TestArbitraryMarkdownRoundTrip(t *testing.T) {
	for _, text := range []string{
		"Intro\n\n## Details\n\nImportant detail\n# Inner title",
		"```md\n## Notes\n## Check history\n### 2026-10-20 — fake\n```",
		"## Notes\n\n## Check history\n### 2026-10-20 — fake\n<!-- waiting-mcp:check -->\n<!-- waiting-mcp:/notes -->\n<!-- waiting-mcp:/message -->",
		"\\literal\n\\<!-- waiting-mcp:/message -->\n\\\\two slashes\n",
		"\n\n  keep leading and trailing whitespace  \n\n",
		"",
	} {
		t.Run(fmt.Sprintf("case-%d", len(text)), func(t *testing.T) {
			s, w := seed(t, t.TempDir())
			w.Notes = text
			w.CheckHistory = []domain.CheckRecord{{Date: w.NextCheck, Action: "checked", Message: text}}
			if err := s.CreateWaiting(t.Context(), w); err != nil {
				t.Fatal(err)
			}
			got, err := s.GetWaiting(t.Context(), w.ID)
			if err != nil {
				t.Fatal(err)
			}
			if got.Notes != text || got.CheckHistory[0].Message != text {
				t.Fatalf("round trip mismatch: %#v", got)
			}
			if _, err := s.UpdateWaiting(t.Context(), w.ID, domain.WaitingPatch{}); err != nil {
				t.Fatal(err)
			}
			got, err = s.GetWaiting(t.Context(), w.ID)
			if err != nil {
				t.Fatal(err)
			}
			if got.Notes != text || got.CheckHistory[0].Message != text {
				t.Fatal("content lost on subsequent save")
			}
			person, err := s.GetPerson(t.Context(), "tester")
			if err != nil {
				t.Fatal(err)
			}
			person.Notes = text
			if err := s.UpsertPerson(t.Context(), person, true); err != nil {
				t.Fatal(err)
			}
			person, err = s.GetPerson(t.Context(), "tester")
			if err != nil || person.Notes != text {
				t.Fatalf("person round trip: %q %v", person.Notes, err)
			}
		})
	}
}

func TestLegacyMarkdownPreservesHeadings(t *testing.T) {
	root := t.TempDir()
	s, w := seed(t, root)
	if err := s.CreateWaiting(t.Context(), w); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "waiting", "sample.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	front, _, _ := strings.Cut(string(data), "<!-- waiting-mcp:body-v2 -->")
	notes := "Intro\n\n## Details\n\n# Inner title\n\n```md\n## Check history\n```"
	message := "Message\n\n## Detail\n\n### Ordinary heading\n\n~~~md\n### 2026-10-19 — fake\n~~~"
	legacy := "# Legacy\n\n## Notes\n\n" + notes + "\n\n## Check history\n\n### 2026-10-20 — checked\n\n" + message + "\n"
	if err := os.WriteFile(path, []byte(front+legacy), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetWaiting(t.Context(), w.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Notes != notes || len(got.CheckHistory) != 1 || got.CheckHistory[0].Message != message {
		t.Fatalf("legacy content lost: %#v", got)
	}
}

func TestIDsCannotEscapeDataDirectory(t *testing.T) {
	s, w := seed(t, t.TempDir())
	for _, id := range []string{"", "..", "../escaped", "../../escaped", "/absolute", "a/b", `a\b`, " bad", "bad\x00"} {
		t.Run(fmt.Sprintf("id-%q", id), func(t *testing.T) {
			bad := w
			bad.ID = id
			if err := s.CreateWaiting(t.Context(), bad); !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("create: %v", err)
			}
			if _, err := s.GetWaiting(t.Context(), id); !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("get: %v", err)
			}
			if _, err := s.ReadWaitingMarkdown(t.Context(), id); !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("resource: %v", err)
			}
			if _, err := s.GetPerson(t.Context(), id); !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("person get: %v", err)
			}
			if _, err := s.ReadPersonMarkdown(t.Context(), id); !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("person resource: %v", err)
			}
			if _, err := s.UpdateWaiting(t.Context(), id, domain.WaitingPatch{}); !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("update: %v", err)
			}
			if err := s.SetWaitingStatus(t.Context(), id, domain.StatusDone, nil); !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("status: %v", err)
			}
			bad = w
			bad.Responsible = id
			if err := s.CreateWaiting(t.Context(), bad); !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("responsible: %v", err)
			}
		})
	}
}

func TestSymlinksCannotEscapeDataDirectory(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	s, w := seed(t, root)
	external := filepath.Join(outside, "secret.md")
	if err := os.WriteFile(external, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"waiting", "people"} {
		link := filepath.Join(root, dir, "escape.md")
		if err := os.Symlink(external, link); err != nil {
			t.Skip(err)
		}
	}
	if _, err := s.ReadWaitingMarkdown(t.Context(), "escape"); err == nil {
		t.Fatal("read outside root")
	}
	if _, err := s.ReadPersonMarkdown(t.Context(), "escape"); err == nil {
		t.Fatal("read person outside root")
	}
	w.ID = "escape"
	if err := s.CreateWaiting(t.Context(), w); err == nil {
		t.Fatal("accepted external symlink")
	}
	data, _ := os.ReadFile(external)
	if string(data) != "secret" {
		t.Fatal("external file modified")
	}
	// Also reject a directory symlink while creating files.
	if err := os.MkdirAll(filepath.Join(outside, "people"), 0700); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(t.TempDir(), "new-data")
	if err := os.MkdirAll(other, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(other, "waiting")); err != nil {
		t.Fatal(err)
	}
	if err := mdstore.NewStore(other).UpsertPerson(t.Context(), domain.Person{ID: "a", Name: "A", Relation: domain.RelationColleague, DefaultCheckCadence: domain.CadenceWeekly}, false); err == nil {
		t.Fatal("accepted external directory symlink")
	}
}

func TestInvalidWritesLeaveReadableStore(t *testing.T) {
	root := t.TempDir()
	s, w := seed(t, root)
	for _, mutate := range []func(*domain.Waiting){
		func(w *domain.Waiting) { w.ExpectedResult = " " },
		func(w *domain.Waiting) { w.Status = "unknown" },
		func(w *domain.Waiting) { w.Context = "unknown" },
		func(w *domain.Waiting) { w.NextCheck = time.Time{} },
		func(w *domain.Waiting) { w.CheckHistory = []domain.CheckRecord{{Date: w.CreatedAt, Action: ""}} },
	} {
		bad := w
		mutate(&bad)
		if err := s.CreateWaiting(t.Context(), bad); !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("accepted invalid write: %v", err)
		}
	}
	if err := s.CreateWaiting(t.Context(), w); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "waiting", "sample.md")
	before, _ := os.ReadFile(path)
	empty := ""
	if _, err := s.UpdateWaiting(t.Context(), w.ID, domain.WaitingPatch{ExpectedResult: &empty}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("invalid update: %v", err)
	}
	after, _ := os.ReadFile(path)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("invalid update changed file")
	}
	if _, err := s.ListWaiting(t.Context(), domain.WaitingFilter{}); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentPatchesKeepHistoryAndCompletion(t *testing.T) {
	root := t.TempDir()
	s, w := seed(t, root)
	if err := s.CreateWaiting(t.Context(), w); err != nil {
		t.Fatal(err)
	}
	const count = 20
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			other := mdstore.NewStore(root)
			text := fmt.Sprintf("note %d", i)
			if _, err := other.UpdateWaiting(t.Context(), w.ID, domain.WaitingPatch{Notes: &text}); err != nil {
				t.Error(err)
			}
		}(i)
		go func() {
			defer wg.Done()
			other := mdstore.NewStore(root)
			if err := other.RecordCheck(t.Context(), w.ID, domain.CheckRecord{Date: w.CreatedAt, Action: "checked"}, w.NextCheck, nil, nil); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := s.SetWaitingStatus(t.Context(), w.ID, domain.StatusDone, nil); err != nil {
			t.Error(err)
		}
	}()
	wg.Wait()
	got, err := s.GetWaiting(t.Context(), w.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.CheckHistory) != count || got.Status != domain.StatusDone {
		t.Fatalf("lost mutation: history=%d status=%s", len(got.CheckHistory), got.Status)
	}
	if _, err := os.Stat(filepath.Join(root, "waiting", "sample.md")); !os.IsNotExist(err) {
		t.Fatalf("resurrected active file: %v", err)
	}
}

func TestCrossProcessChecksKeepAllRecords(t *testing.T) {
	if root := os.Getenv("WAITING_TEST_CHILD_ROOT"); root != "" {
		s := mdstore.NewStore(root)
		date := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
		for i := 0; i < 10; i++ {
			if err := s.RecordCheck(context.Background(), "sample", domain.CheckRecord{Date: date, Action: "checked"}, date, nil, nil); err != nil {
				t.Fatal(err)
			}
		}
		return
	}
	root := t.TempDir()
	s, w := seed(t, root)
	if err := s.CreateWaiting(t.Context(), w); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := exec.Command(os.Args[0], "-test.run=^TestCrossProcessChecksKeepAllRecords$")
			cmd.Env = append(os.Environ(), "WAITING_TEST_CHILD_ROOT="+root)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Errorf("child: %v %s", err, out)
			}
		}()
	}
	wg.Wait()
	got, err := s.GetWaiting(t.Context(), w.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.CheckHistory) != 30 {
		t.Fatalf("history=%d want 30", len(got.CheckHistory))
	}
}

func TestPatchCanClearOptionalFields(t *testing.T) {
	s, w := seed(t, t.TempDir())
	w.Notes = "Notes"
	w.Blocker = "Blocker"
	w.ResultDeadline = &w.CreatedAt
	w.PromisedDeadline = &w.CreatedAt
	if err := s.CreateWaiting(t.Context(), w); err != nil {
		t.Fatal(err)
	}
	empty := ""
	tags := []string{}
	got, err := s.UpdateWaiting(t.Context(), w.ID, domain.WaitingPatch{Notes: &empty, Blocker: &empty, Tags: &tags, ClearResultDeadline: true, ClearPromisedDeadline: true})
	if err != nil {
		t.Fatal(err)
	}
	if got.Notes != "" || got.Blocker != "" || got.ResultDeadline != nil || got.PromisedDeadline != nil {
		t.Fatalf("not cleared: %#v", got)
	}
	saved, err := s.GetWaiting(t.Context(), w.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !saved.UpdatedAt.Equal(got.UpdatedAt) {
		t.Fatalf("response timestamp differs from disk: %v != %v", got.UpdatedAt, saved.UpdatedAt)
	}
}
