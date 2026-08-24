package applyinator

import (
	"os"
	"testing"
	"time"
)

func TestMarshalParseRoundtrip(t *testing.T) {
	owner := newInterlockOwner(time.Now())
	parsed, ok := parseInterlockOwner(owner.marshal())
	if !ok {
		t.Fatal("parseInterlockOwner returned ok=false for a freshly marshalled owner")
	}
	if parsed.PID != owner.PID {
		t.Errorf("PID mismatch: got %d, want %d", parsed.PID, owner.PID)
	}
	if parsed.BootID != owner.BootID {
		t.Errorf("BootID mismatch: got %q, want %q", parsed.BootID, owner.BootID)
	}
}

func TestIsAliveSelf(t *testing.T) {
	owner := newInterlockOwner(time.Now())
	if !owner.isAlive() {
		t.Error("current process should report as alive")
	}
}

func TestIsAliveDeadPID(t *testing.T) {
	// PID 99999999 is extremely unlikely to exist
	owner := interlockOwner{PID: 99999999, BootID: currentBootID()}
	if owner.isAlive() {
		t.Error("non-existent PID should not be reported as alive")
	}
}

func TestLegacyEmptyFileParsed(t *testing.T) {
	// Old install.sh wrote a bare `touch` — empty file. Must return ok=false.
	_, ok := parseInterlockOwner([]byte{})
	if ok {
		t.Error("empty file (legacy touch) should return ok=false")
	}
}

func TestStaleInterlockRemovedOnApply(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/applyinator-active"

	// Write a stale interlock owned by a dead PID
	stale := interlockOwner{PID: 99999999, BootID: currentBootID(), Written: time.Now()}
	if err := os.WriteFile(path, stale.marshal(), 0600); err != nil {
		t.Fatal(err)
	}

	a := NewApplyinator(t.TempDir(), false, "", dir, nil)
	// Apply with a no-op plan — we only care about interlock cleanup
	_, err := a.Apply(nil, ApplyInput{CalculatedPlan: CalculatedPlan{}})
	// err is expected (nil image util), but the interlock file must be gone
	_ = err
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Error("stale interlock file was not removed by Apply()")
	}
}
