package models

import "encoding/json"

type AgentConfig struct {
	ID             string          `json:"id"`
	AgentName      string          `json:"agent_name"`
	SystemPrompt   string          `json:"system_prompt"`
	PromptTemplate string          `json:"prompt_template"`
	Model          string          `json:"model"`
	Temperature    float64         `json:"temperature"`
	Enabled        bool            `json:"enabled"`
	Skills         string          `json:"skills"`
	Insights       string          `json:"insights"`
	Config         json.RawMessage `json:"config"`
}

func (c *AgentConfig) BuildSystemPrompt() string {
	prompt := c.SystemPrompt
	if c.Skills != "" {
		prompt += "\n\n## Skills & Guidelines\n" + c.Skills
	}
	if c.Insights != "" {
		prompt += "\n\n## Performance Insights\n" + c.Insights
	}
	return prompt
}
