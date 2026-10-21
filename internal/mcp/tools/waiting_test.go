package tools

import (
	mdstore "github.com/priahin-i/assistente_2/internal/store/markdown"
	"testing"
	"time"
)

func TestReviewDueUsesLocalDateAtMidnight(t *testing.T) {
	location := time.FixedZone("UTC+3", 3*3600)
	tools := NewWaitingTools(mdstore.NewStore(t.TempDir()), location)
	tools.now = func() time.Time { return time.Date(2026, 10, 21, 21, 30, 0, 0, time.UTC) }
	_, result, err := tools.ReviewDue(t.Context(), nil, WaitingReviewDueInput{})
	if err != nil {
		t.Fatal(err)
	}
	if result.AsOf != "2026-10-22" {
		t.Fatal(result.AsOf)
	}
	_, explicit, err := tools.ReviewDue(t.Context(), nil, WaitingReviewDueInput{AsOf: "2026-10-21"})
	if err != nil || explicit.AsOf != "2026-10-21" {
		t.Fatalf("explicit date: %+v %v", explicit, err)
	}
}
