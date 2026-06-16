package storage

import (
	"context"
	"fmt"
	"strings"

	"github.com/fisk086/aiops/internal/model"
)

// sanitizeAuditText ensures PostgreSQL UTF8 TEXT columns accept the value (Go strings may hold invalid UTF-8).
func sanitizeAuditText(s string) string {
	return strings.ToValidUTF8(s, "\uFFFD")
}

func (s *PostgresStorage) buildAuditLogWhere(filter *AuditLogFilter) (string, []any) {
	if filter == nil {
		return "1=1", nil
	}
	var clauses []string
	var args []any
	n := 0
	if filter.UserID != "" {
		n++
		clauses = append(clauses, fmt.Sprintf("user_id = $%d", n))
		args = append(args, filter.UserID)
	}
	if filter.AgentID != 0 {
		n++
		clauses = append(clauses, fmt.Sprintf("agent_id = $%d", n))
		args = append(args, filter.AgentID)
	}
	if filter.SessionID != "" {
		n++
		clauses = append(clauses, fmt.Sprintf("session_id = $%d", n))
		args = append(args, filter.SessionID)
	}
	if filter.ToolName != "" {
		n++
		clauses = append(clauses, fmt.Sprintf("tool_name = $%d", n))
		args = append(args, filter.ToolName)
	}
	if filter.RiskLevel != "" {
		n++
		clauses = append(clauses, fmt.Sprintf("risk_level = $%d", n))
		args = append(args, filter.RiskLevel)
	}
	if filter.Status != "" {
		n++
		clauses = append(clauses, fmt.Sprintf("status = $%d", n))
		args = append(args, filter.Status)
	}
	if len(clauses) == 0 {
		return "1=1", nil
	}
	return strings.Join(clauses, " AND "), args
}

func (s *PostgresStorage) CreateAuditLog(log *model.AuditLog) (*model.AuditLog, error) {
	var id int64
	err := s.pool.QueryRow(context.Background(),
		`INSERT INTO audit_logs (user_id, agent_id, session_id, tool_name, action, risk_level, input, output, error, status, duration_ms, ip_address)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		 RETURNING id`,
		sanitizeAuditText(log.UserID), log.AgentID, sanitizeAuditText(log.SessionID),
		sanitizeAuditText(log.ToolName), sanitizeAuditText(log.Action), sanitizeAuditText(log.RiskLevel),
		sanitizeAuditText(log.Input), sanitizeAuditText(log.Output), sanitizeAuditText(log.Error),
		sanitizeAuditText(log.Status), log.DurationMs, sanitizeAuditText(log.IPAddress),
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	log.ID = id
	return log, nil
}

func (s *PostgresStorage) ListAuditLogs(filter *AuditLogFilter) ([]*model.AuditLog, int64, error) {
	where, args := s.buildAuditLogWhere(filter)

	var total int64
	err := s.pool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM audit_logs WHERE "+where, args...,
	).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	page := 1
	pageSize := 50
	if filter != nil {
		if filter.Page > 0 {
			page = filter.Page
		}
		if filter.PageSize > 0 {
			pageSize = filter.PageSize
		}
	}
	offset := (page - 1) * pageSize

	n := len(args)
	query := fmt.Sprintf(`SELECT id, user_id, agent_id, session_id, tool_name, action, risk_level, input, output, error, status, duration_ms, ip_address, created_at 
		FROM audit_logs WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, where, n+1, n+2)
	qargs := append(args, pageSize, offset)
	rows, err := s.pool.Query(context.Background(), query, qargs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []*model.AuditLog
	for rows.Next() {
		var log model.AuditLog
		err := rows.Scan(
			&log.ID, &log.UserID, &log.AgentID, &log.SessionID, &log.ToolName, &log.Action, &log.RiskLevel,
			&log.Input, &log.Output, &log.Error, &log.Status, &log.DurationMs, &log.IPAddress, &log.CreatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		logs = append(logs, &log)
	}
	return logs, total, nil
}

func (s *PostgresStorage) GetAuditLog(id int64) (*model.AuditLog, error) {
	var log model.AuditLog
	err := s.pool.QueryRow(context.Background(),
		`SELECT id, user_id, agent_id, session_id, tool_name, action, risk_level, input, output, error, status, duration_ms, ip_address, created_at 
		 FROM audit_logs WHERE id = $1`,
		id,
	).Scan(
		&log.ID, &log.UserID, &log.AgentID, &log.SessionID, &log.ToolName, &log.Action, &log.RiskLevel,
		&log.Input, &log.Output, &log.Error, &log.Status, &log.DurationMs, &log.IPAddress, &log.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &log, nil
}

func (s *PostgresStorage) CountAuditLogs(filter *AuditLogFilter) (int64, error) {
	where, args := s.buildAuditLogWhere(filter)

	var count int64
	err := s.pool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM audit_logs WHERE "+where, args...,
	).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (s *PostgresStorage) DeleteAuditLogs(filter *AuditLogFilter) (int64, error) {
	where, args := s.buildAuditLogWhere(filter)

	result, err := s.pool.Exec(context.Background(),
		"DELETE FROM audit_logs WHERE "+where, args...,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}
