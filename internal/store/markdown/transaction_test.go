package markdown

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestTransactionLockRespectsCancellation(t *testing.T) {
	root := t.TempDir()
	first := NewStore(root)
	unlock, err := first.lock(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Millisecond)
	defer cancel()
	second := NewStore(root)
	if release, err := second.lock(ctx); !errors.Is(err, context.DeadlineExceeded) {
		if release != nil {
			release()
		}
		t.Fatalf("lock error=%v", err)
	}
}
