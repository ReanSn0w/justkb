package mcpserver

import (
	"testing"
	"time"
)

func TestMovementPeriod(t *testing.T) {
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	from, to, err := movementPeriod("", "", now)
	if err != nil || !to.Equal(now) || !from.Equal(now.Add(-24*time.Hour)) {
		t.Fatalf("from=%s to=%s err=%v", from, to, err)
	}
	explicitFrom := "2026-10-30T09:00:00Z"
	explicitTo := "2026-11-01T09:00:00Z"
	from, to, err = movementPeriod(explicitFrom, explicitTo, now)
	if err != nil || from.Format(time.RFC3339) != explicitFrom || to.Format(time.RFC3339) != explicitTo {
		t.Fatalf("explicit from=%s to=%s err=%v", from, to, err)
	}
	if _, _, err := movementPeriod(explicitTo, explicitFrom, now); err == nil {
		t.Fatal("reverse period accepted")
	}
}
