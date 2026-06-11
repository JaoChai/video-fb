package progress

import (
	"sync"
	"testing"
)

// The production gate: only one render may run at a time. StartProduction is the
// single acquire point; a second acquire must fail until FinishProduction releases.
func TestStartProductionGate(t *testing.T) {
	tr := NewTracker()

	if !tr.StartProduction(1) {
		t.Fatal("first StartProduction should acquire the gate")
	}
	if tr.StartProduction(1) {
		t.Fatal("second StartProduction must be refused while one is active")
	}

	tr.FinishProduction()
	if tr.GetStatus().Active {
		t.Fatal("FinishProduction should clear Active")
	}
	if !tr.StartProduction(1) {
		t.Fatal("StartProduction should acquire again after FinishProduction")
	}
}

// SetTotalClips updates the count mid-run without releasing the gate.
func TestSetTotalClipsKeepsGate(t *testing.T) {
	tr := NewTracker()
	tr.StartProduction(1)
	tr.SetTotalClips(8)

	s := tr.GetStatus()
	if !s.Active {
		t.Fatal("SetTotalClips must not release the gate")
	}
	if s.TotalClips != 8 {
		t.Fatalf("TotalClips = %d, want 8", s.TotalClips)
	}
	if tr.StartProduction(1) {
		t.Fatal("gate still held after SetTotalClips, second acquire must fail")
	}
}

// Under concurrent callers, exactly one acquires the gate. Run with -race.
func TestStartProductionConcurrent(t *testing.T) {
	tr := NewTracker()
	const n = 50
	var wg sync.WaitGroup
	var won int32
	var mu sync.Mutex

	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			if tr.StartProduction(1) {
				mu.Lock()
				won++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if won != 1 {
		t.Fatalf("exactly one goroutine should acquire the gate, got %d", won)
	}
}
