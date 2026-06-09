package model

import "time"

type ModelConfig struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Provider  string    `json:"provider"`
	ModelName string    `json:"model"`
	BaseURL   string    `json:"base_url"`
	APIKey    string    `json:"api_key,omitempty"`
	IsActive  bool      `json:"is_active"`
	Purpose   string    `json:"purpose"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
