package models

type ContentFormat struct {
	ID                  string `json:"id"`
	FormatName          string `json:"format_name"`
	DisplayName         string `json:"display_name"`
	QuestionInstruction string `json:"question_instruction"`
	ScriptInstruction   string `json:"script_instruction"`
	Enabled             bool   `json:"enabled"`
	Weight              int    `json:"weight"`
}

// FormatUsage pairs a format with how many clips used it recently.
type FormatUsage struct {
	Format    ContentFormat
	UsedCount int
}
