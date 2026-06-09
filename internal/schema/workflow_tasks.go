package schema

type WorkflowTaskFieldPublic struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Default     any      `json:"default,omitempty"`
	Description string   `json:"description,omitempty"`
	Options     []string `json:"options,omitempty"`
}

type WorkflowTaskPublic struct {
	Type         string                             `json:"type"`
	Name         string                             `json:"name"`
	Description  string                             `json:"description"`
	Icon         string                             `json:"icon"`
	Color        string                             `json:"color"`
	Category     string                             `json:"category"`
	ConfigSchema map[string]WorkflowTaskFieldPublic `json:"config_schema"`
}
