package progress

import (
	"sync"
	"time"
)

type StepStatus string

const (
	StatusPending   StepStatus = "pending"
	StatusRunning   StepStatus = "running"
	StatusCompleted StepStatus = "completed"
	StatusFailed    StepStatus = "failed"
)

type Step struct {
	Name           string     `json:"name"`
	Status         StepStatus `json:"status"`
	ElapsedSeconds float64    `json:"elapsed_seconds"`
	Error          string     `json:"error,omitempty"`
	startedAt      time.Time
}

type ProductionStatus struct {
	Active      bool   `json:"active"`
	CurrentClip int    `json:"current_clip"`
	TotalClips  int    `json:"total_clips"`
	ClipTitle   string `json:"clip_title"`
	Steps       []Step `json:"steps"`
}

var stepNames = []string{"question", "script", "image_prompts", "voice", "images", "assembly", "complete"}

type Tracker struct {
	mu     sync.RWMutex
	status ProductionStatus
}

func NewTracker() *Tracker {
	return &Tracker{}
}

func (t *Tracker) StartProduction(totalClips int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status = ProductionStatus{
		Active:      true,
		CurrentClip: 0,
		TotalClips:  totalClips,
	}
}

func (t *Tracker) StartClip(clipNum int, title string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status.CurrentClip = clipNum
	t.status.ClipTitle = title
	t.status.Steps = make([]Step, len(stepNames))
	for i, name := range stepNames {
		t.status.Steps[i] = Step{Name: name, Status: StatusPending}
	}
}

func (t *Tracker) StartStep(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for i := range t.status.Steps {
		if t.status.Steps[i].Name == name {
			t.status.Steps[i].Status = StatusRunning
			t.status.Steps[i].startedAt = time.Now()
			break
		}
	}
}

func (t *Tracker) CompleteStep(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for i := range t.status.Steps {
		if t.status.Steps[i].Name == name {
			t.status.Steps[i].Status = StatusCompleted
			if !t.status.Steps[i].startedAt.IsZero() {
				t.status.Steps[i].ElapsedSeconds = time.Since(t.status.Steps[i].startedAt).Seconds()
			}
			break
		}
	}
}

func (t *Tracker) FailStep(name string, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for i := range t.status.Steps {
		if t.status.Steps[i].Name == name {
			t.status.Steps[i].Status = StatusFailed
			t.status.Steps[i].Error = err.Error()
			if !t.status.Steps[i].startedAt.IsZero() {
				t.status.Steps[i].ElapsedSeconds = time.Since(t.status.Steps[i].startedAt).Seconds()
			}
			break
		}
	}
}

func (t *Tracker) FinishProduction() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status.Active = false
}

func (t *Tracker) GetStatus() ProductionStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()
	cp := t.status
	cp.Steps = make([]Step, len(t.status.Steps))
	copy(cp.Steps, t.status.Steps)
	for i := range cp.Steps {
		if cp.Steps[i].Status == StatusRunning && !t.status.Steps[i].startedAt.IsZero() {
			cp.Steps[i].ElapsedSeconds = time.Since(t.status.Steps[i].startedAt).Seconds()
		}
	}
	return cp
}
