package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/fisk086/aiops/internal/model"
	"github.com/jackc/pgx/v5"
)

func (s *PostgresStorage) CreateModelConfig(ctx context.Context, cfg *model.ModelConfig) (*model.ModelConfig, error) {
	var id int64
	var createdAt, updatedAt time.Time
	err := s.pool.QueryRow(ctx,
		`INSERT INTO model_configs (name, provider, model_name, base_url, api_key, is_active, purpose)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, created_at, updated_at`,
		cfg.Name, cfg.Provider, cfg.ModelName, cfg.BaseURL, cfg.APIKey, cfg.IsActive, cfg.Purpose,
	).Scan(&id, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	cfg.ID = id
	cfg.CreatedAt = createdAt
	cfg.UpdatedAt = updatedAt
	return cfg, nil
}

func scanModelConfig(scanner interface {
	Scan(dest ...any) error
}) (*model.ModelConfig, error) {
	var c model.ModelConfig
	err := scanner.Scan(&c.ID, &c.Name, &c.Provider, &c.ModelName, &c.BaseURL, &c.APIKey, &c.IsActive, &c.Purpose, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &c, nil
}

func (s *PostgresStorage) GetModelConfig(ctx context.Context, id int64) (*model.ModelConfig, error) {
	return scanModelConfig(s.pool.QueryRow(ctx,
		`SELECT id, name, provider, model_name, base_url, api_key, is_active, purpose, created_at, updated_at
		 FROM model_configs WHERE id = $1`, id))
}

func (s *PostgresStorage) ListModelConfigs(ctx context.Context) ([]*model.ModelConfig, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, provider, model_name, base_url, api_key, is_active, purpose, created_at, updated_at
		 FROM model_configs ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.ModelConfig
	for rows.Next() {
		c, err := scanModelConfig(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

func (s *PostgresStorage) ListModelConfigsByPurpose(ctx context.Context, purpose string) ([]*model.ModelConfig, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, provider, model_name, base_url, api_key, is_active, purpose, created_at, updated_at
		 FROM model_configs WHERE purpose = $1 ORDER BY id`, purpose)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.ModelConfig
	for rows.Next() {
		c, err := scanModelConfig(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

func (s *PostgresStorage) UpdateModelConfig(ctx context.Context, id int64, cfg *model.ModelConfig) (*model.ModelConfig, error) {
	var now time.Time
	var err error
	if cfg.APIKey != "" {
		err = s.pool.QueryRow(ctx,
			`UPDATE model_configs SET name = $1, provider = $2, model_name = $3, base_url = $4, api_key = $5,
			 is_active = $6, purpose = $7, updated_at = NOW()
			 WHERE id = $8
			 RETURNING updated_at`,
			cfg.Name, cfg.Provider, cfg.ModelName, cfg.BaseURL, cfg.APIKey, cfg.IsActive, cfg.Purpose, id,
		).Scan(&now)
	} else {
		err = s.pool.QueryRow(ctx,
			`UPDATE model_configs SET name = $1, provider = $2, model_name = $3, base_url = $4,
			 is_active = $5, purpose = $6, updated_at = NOW()
			 WHERE id = $7
			 RETURNING updated_at`,
			cfg.Name, cfg.Provider, cfg.ModelName, cfg.BaseURL, cfg.IsActive, cfg.Purpose, id,
		).Scan(&now)
	}
	if err != nil {
		return nil, err
	}
	cfg.ID = id
	cfg.UpdatedAt = now
	return cfg, nil
}

func (s *PostgresStorage) DeleteModelConfig(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM model_configs WHERE id = $1`, id)
	return err
}
