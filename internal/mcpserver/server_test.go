package mcpserver

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/priahin-i/assistente_2/internal/domain"
	mdstore "github.com/priahin-i/assistente_2/internal/store/markdown"
)

func client(t *testing.T) *mcp.ClientSession {
	t.Helper()
	server := NewServer(mdstore.NewStore(t.TempDir()), time.UTC)
	a, b := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(t.Context(), a, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { serverSession.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "regression-test", Version: "1"}, nil)
	session, err := client.Connect(t.Context(), b, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}

func invoke(t *testing.T, client *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	result, err := client.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func decoded(t *testing.T, result *mcp.CallToolResult) map[string]any {
	t.Helper()
	if result.IsError {
		t.Fatalf("tool error: %+v", result.Content)
	}
	raw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func createPerson(t *testing.T, c *mcp.ClientSession) {
	t.Helper()
	decoded(t, invoke(t, c, "person_upsert", map[string]any{"id": "tester", "name": "Tester", "relation": "colleague", "default_check_cadence": "weekly"}))
}

func createArgs(id string) map[string]any {
	return map[string]any{"id": id, "responsible": "tester", "expected_result": "Result", "next_check": "2026-10-20", "context": "work"}
}

func TestMCPRegressionLifecycle(t *testing.T) {
	c := client(t)
	createPerson(t, c)
	markdown := "Intro\n\n## Details\nImportant detail\n\n## Check history\n### 2026-10-19 — ordinary text"
	args := createArgs("sample")
	args["notes"] = markdown
	args["result_deadline"] = "2026-10-30"
	args["promised_deadline"] = "2026-10-29"
	decoded(t, invoke(t, c, "waiting_create", args))
	got := decoded(t, invoke(t, c, "waiting_get", map[string]any{"id": "sample"}))["waiting"].(map[string]any)
	if got["notes"] != markdown {
		t.Fatalf("notes lost: %#v", got)
	}
	decoded(t, invoke(t, c, "waiting_record_check", map[string]any{"id": "sample", "action": "checked", "message": markdown, "next_check": "2026-10-20"}))
	due := decoded(t, invoke(t, c, "waiting_review_due", map[string]any{"as_of": "2026-10-21"}))
	if got := due["items"].([]any)[0].(map[string]any)["days_overdue"]; got != float64(1) {
		t.Fatalf("days overdue=%v", got)
	}
	patched := decoded(t, invoke(t, c, "waiting_update", map[string]any{"id": "sample", "notes": "", "clear_result_deadline": true, "clear_promised_deadline": true}))["waiting"].(map[string]any)
	saved := decoded(t, invoke(t, c, "waiting_get", map[string]any{"id": "sample"}))["waiting"].(map[string]any)
	if saved["notes"] != nil || saved["result_deadline"] != nil || saved["promised_deadline"] != nil {
		t.Fatalf("fields not cleared: %#v", saved)
	}
	if patched["updated_at"] != saved["updated_at"] {
		t.Fatal("returned stale updated_at")
	}
	history := saved["check_history"].([]any)
	if len(history) != 2 || history[1].(map[string]any)["message"] != markdown {
		t.Fatal("history lost")
	}
	for _, uri := range []string{"waiting://sample", "people://tester", "waiting://due/today"} {
		result, err := c.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: uri})
		if err != nil || len(result.Contents) == 0 {
			t.Fatalf("resource %s: %v", uri, err)
		}
	}
}

func TestMCPInvalidInputCannotPoisonList(t *testing.T) {
	c := client(t)
	createPerson(t, c)
	for _, args := range []map[string]any{
		createArgs("../escape"),
		func() map[string]any { a := createArgs("empty"); a["expected_result"] = ""; return a }(),
		func() map[string]any { a := createArgs("blank"); a["expected_result"] = "  "; return a }(),
	} {
		if result := invoke(t, c, "waiting_create", args); !result.IsError {
			t.Fatalf("accepted invalid input: %#v", args)
		}
	}
	if got := decoded(t, invoke(t, c, "waiting_list", map[string]any{}))["count"]; got != float64(0) {
		t.Fatal(got)
	}
	decoded(t, invoke(t, c, "waiting_create", createArgs("valid")))
	for _, args := range []map[string]any{
		{"id": "valid", "expected_result": ""},
		{"id": "valid", "next_check": ""},
		{"id": "valid", "result_deadline": "2026-10-30", "clear_result_deadline": true},
		{"id": "valid", "blocker": "x", "clear_blocker": true},
	} {
		if result := invoke(t, c, "waiting_update", args); !result.IsError {
			t.Fatalf("accepted invalid patch: %#v", args)
		}
	}
	if got := decoded(t, invoke(t, c, "waiting_list", map[string]any{}))["count"]; got != float64(1) {
		t.Fatal(got)
	}
}

func TestMCPConcurrentUpdateAndCheckKeepHistory(t *testing.T) {
	c := client(t)
	createPerson(t, c)
	decoded(t, invoke(t, c, "waiting_create", createArgs("sample")))
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			invoke(t, c, "waiting_update", map[string]any{"id": "sample", "notes": "updated"})
		}()
		go func() {
			defer wg.Done()
			invoke(t, c, "waiting_record_check", map[string]any{"id": "sample", "action": "checked", "next_check": "2026-10-20"})
		}()
	}
	wg.Wait()
	got := decoded(t, invoke(t, c, "waiting_get", map[string]any{"id": "sample"}))["waiting"].(map[string]any)
	if len(got["check_history"].([]any)) != 11 {
		t.Fatal("history lost")
	}
}

func TestResourceUsesConfiguredCalendarDay(t *testing.T) {
	// Use the farthest-east IANA zone so this test also covers the UTC-date seam
	// for part of every UTC day; a deterministic clock test covers the rest.
	zone := time.FixedZone("UTC+14", 14*3600)
	s := mdstore.NewStore(t.TempDir())
	today := domain.CalendarDate(time.Now().In(zone))
	if err := s.UpsertPerson(t.Context(), domain.Person{ID: "tester", Name: "Tester", Relation: domain.RelationColleague, DefaultCheckCadence: domain.CadenceWeekly}, false); err != nil {
		t.Fatal(err)
	}
	w := domain.Waiting{ID: "today", Responsible: "tester", ExpectedResult: "Result", Context: domain.ContextWork, Status: domain.StatusActive, NextCheck: today, CreatedAt: today, UpdatedAt: today}
	if err := s.CreateWaiting(t.Context(), w); err != nil {
		t.Fatal(err)
	}
	server := NewServer(s, zone)
	a, b := mcp.NewInMemoryTransports()
	ss, err := server.Connect(t.Context(), a, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	c, err := mcp.NewClient(&mcp.Implementation{Name: "timezone-test", Version: "1"}, nil).Connect(t.Context(), b, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	resource, err := c.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: "waiting://due/today"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resource.Contents[0].Text, "As of: "+today.Format("2006-01-02")) {
		t.Fatal(resource.Contents[0].Text)
	}
	tool := decoded(t, invoke(t, c, "waiting_review_due", map[string]any{}))
	if tool["as_of"] != today.Format("2006-01-02") {
		t.Fatal(tool)
	}
}
