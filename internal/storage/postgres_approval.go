package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/fisk086/aiops/internal/model"
)

func (s *PostgresStorage) buildApprovalRequestWhere(filter *ApprovalRequestFilter) (string, []any) {
	if filter == nil {
		return "1=1", nil
	}
	var clauses []string
	var args []any
	n := 0
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
	if filter.Status != "" {
		n++
		clauses = append(clauses, fmt.Sprintf("status = $%d", n))
		args = append(args, filter.Status)
	}
	if filter.ExternalID != "" {
		n++
		clauses = append(clauses, fmt.Sprintf("external_id = $%d", n))
		args = append(args, filter.ExternalID)
	}
	if filter.UserID != "" {
		n++
		clauses = append(clauses, fmt.Sprintf("user_id = $%d", n))
		args = append(args, filter.UserID)
	}
	if len(clauses) == 0 {
		return "1=1", nil
	}
	return strings.Join(clauses, " AND "), args
}

func (s *PostgresStorage) CreateApprovalRequest(req *model.ApprovalRequest) (*model.ApprovalRequest, error) {
	createdAt := req.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	var id int64
	err := s.pool.QueryRow(context.Background(),
		`INSERT INTO approval_requests (agent_id, session_id, user_id, tool_name, risk_level, input, status, approver_id, comment, created_at, approval_type, external_id, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		 RETURNING id`,
		req.AgentID, req.SessionID, req.UserID, req.ToolName, req.RiskLevel, req.Input,
		req.Status, req.ApproverID, req.Comment, createdAt, req.ApprovalType, req.ExternalID, req.ExpiresAt,
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	req.ID = id
	req.CreatedAt = createdAt
	return req, nil
}

func (s *PostgresStorage) ListApprovalRequests(filter *ApprovalRequestFilter) ([]*model.ApprovalRequest, int64, error) {
	where, args := s.buildApprovalRequestWhere(filter)

	var total int64
	err := s.pool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM approval_requests WHERE "+where, args...,
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
	query := fmt.Sprintf(`SELECT id, agent_id, session_id, user_id, tool_name, risk_level, input, status, approver_id, comment, approved_at, created_at, approval_type, external_id, expires_at
		FROM approval_requests WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, where, n+1, n+2)
	qargs := append(args, pageSize, offset)
	rows, err := s.pool.Query(context.Background(), query, qargs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var requests []*model.ApprovalRequest
	for rows.Next() {
		var req model.ApprovalRequest
		var approvedAt *time.Time
		var expiresAt *time.Time
		var externalID sql.NullString
		err := rows.Scan(
			&req.ID, &req.AgentID, &req.SessionID, &req.UserID, &req.ToolName, &req.RiskLevel,
			&req.Input, &req.Status, &req.ApproverID, &req.Comment, &approvedAt, &req.CreatedAt,
			&req.ApprovalType, &externalID, &expiresAt,
		)
		if err != nil {
			return nil, 0, err
		}
		if externalID.Valid {
			req.ExternalID = externalID.String
		}
		req.ApprovedAt = approvedAt
		req.ExpiresAt = expiresAt
		requests = append(requests, &req)
	}
	return requests, total, nil
}

func (s *PostgresStorage) GetApprovalRequest(id int64) (*model.ApprovalRequest, error) {
	var req model.ApprovalRequest
	var approvedAt *time.Time
	var expiresAt *time.Time
	var externalID sql.NullString
	err := s.pool.QueryRow(context.Background(),
		`SELECT id, agent_id, session_id, user_id, tool_name, risk_level, input, status, approver_id, comment, approved_at, created_at, approval_type, external_id, expires_at
		 FROM approval_requests WHERE id = $1`,
		id,
	).Scan(
		&req.ID, &req.AgentID, &req.SessionID, &req.UserID, &req.ToolName, &req.RiskLevel,
		&req.Input, &req.Status, &req.ApproverID, &req.Comment, &approvedAt, &req.CreatedAt,
		&req.ApprovalType, &externalID, &expiresAt,
	)
	if err != nil {
		return nil, err
	}
	if externalID.Valid {
		req.ExternalID = externalID.String
	}
	req.ApprovedAt = approvedAt
	req.ExpiresAt = expiresAt
	return &req, nil
}

func (s *PostgresStorage) UpdateApprovalRequest(id int64, status, approverID, comment string) error {
	_, err := s.pool.Exec(context.Background(),
		`UPDATE approval_requests SET status = $1, approver_id = $2, comment = $3, approved_at = NOW() WHERE id = $4`,
		status, approverID, comment, id,
	)
	return err
}
