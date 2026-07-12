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

// TopicCategory — หมวดหัวข้อ v2 (map pain คนวงใน)
type TopicCategory struct {
	ID               string `json:"id"`
	CategoryName     string `json:"category_name"`
	DisplayName      string `json:"display_name"`
	AngleInstruction string `json:"angle_instruction"`
	Enabled          bool   `json:"enabled"`
	Weight           int    `json:"weight"`
}

// TitleArchetype — รูปหัวข้อ/hook
type TitleArchetype struct {
	ID            string `json:"id"`
	ArchetypeName string `json:"archetype_name"`
	DisplayName   string `json:"display_name"`
	Instruction   string `json:"instruction"`
	Example       string `json:"example"`
	Enabled       bool   `json:"enabled"`
	Weight        int    `json:"weight"`
}
