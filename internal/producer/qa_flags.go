package producer

import "os"

// QAFailClosedEnabled flips the visual-QA infrastructure policy from fail-open
// to fail-closed: when the QA gate is enabled but cannot actually inspect the
// clip (agent config fetch failed, or zero frames could be extracted), the clip
// routes to needs_review instead of publishing unseen. Off (default) keeps the
// historical fail-open behavior. Per-scene vision errors stay fail-open either
// way — only "QA saw nothing at all" is gated here.
func QAFailClosedEnabled() bool { return os.Getenv("QA_FAIL_CLOSED_ENABLED") == "true" }
