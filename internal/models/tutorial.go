package models

// TutorialStep is one step of a feature walkthrough. N is 1-based and matches
// the "ขั้นที่ n / N" rail rendered on screen.
type TutorialStep struct {
	N        int    `json:"n"`
	TitleTH  string `json:"title_th"`  // ชื่อขั้น เช่น "ตั้งเงื่อนไขให้ตัดงบ"
	ActionTH string `json:"action_th"` // สิ่งที่ต้องกด เช่น "เลือก Cost per result แล้วใส่ 400 บาท"
	UITarget string `json:"ui_target"` // ชื่อเมนู/ปุ่มที่ต้องไฮไลต์ — ต้องอยู่ใน UIVocab
	ValueTH  string `json:"value_th,omitempty"`
}

// TutorialFeature is one row of the tutorial catalog = one clip.
// UIVocab is the ONLY vocabulary allowed to appear on screen inside a UI mock;
// the render gate rejects any label the model invents outside this list.
type TutorialFeature struct {
	ID            string         `json:"id"`
	FeatureKey    string         `json:"feature_key"`
	DisplayNameTH string         `json:"display_name_th"`
	Surface       string         `json:"surface"`
	MenuPath      []string       `json:"menu_path"`
	UIVocab       []string       `json:"ui_vocab"`
	Steps         []TutorialStep `json:"steps"`
	TrapTH        string         `json:"trap_th"`
	PainPoint     string         `json:"pain_point"`
	WhyMattersTH  string         `json:"why_matters_th"`
	NeedsVerify   bool           `json:"needs_verify"`
	Weight        int            `json:"weight"`
	Enabled       bool           `json:"enabled"`
}
