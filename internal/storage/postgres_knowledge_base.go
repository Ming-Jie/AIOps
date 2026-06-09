package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/fisk086/aiops/internal/model"
	"github.com/jackc/pgx/v5"
)

// KnowledgeBaseStore is a focused store for the knowledge-base feature, satisfied
// only by *PostgresStorage. It is intentionally NOT part of the big Storage
// interface so the in-memory fallback need not implement it; the feature is
// gated on a real Postgres store (see cmd/server/main.go).
//
// Access model: a user can READ a KB if they own it, it is public, or they are
// an admin. A user can MANAGE (edit/delete/upload) a KB if they own it or are an
// admin. Document access is gated by the KB's access (verified in the service),
// so document queries here are KB-scoped, not owner-scoped.
type KnowledgeBaseStore interface {
	CreateKB(ctx context.Context, kb *model.KnowledgeBase) (*model.KnowledgeBase, error)
	// ListKBsVisibleToUser returns own + public KBs (or all, if isAdmin).
	ListKBsVisibleToUser(ctx context.Context, userID int64, isAdmin bool) ([]model.KnowledgeBase, error)
	// GetKBForManage returns a KB if the user owns it or is an admin (write ops).
	GetKBForManage(ctx context.Context, id, userID int64, isAdmin bool) (*model.KnowledgeBase, error)
	// GetKBForRead returns a KB if the user owns it, it is public, or isAdmin.
	GetKBForRead(ctx context.Context, id, userID int64, isAdmin bool) (*model.KnowledgeBase, error)
	UpdateKBVikingPath(ctx context.Context, id int64, vikingPath string) error
	// UpdateKB updates name/description/visibility; owner or admin.
	UpdateKB(ctx context.Context, id, userID int64, isAdmin bool, name, description, visibility string) error
	DeleteKB(ctx context.Context, id, userID int64, isAdmin bool) error

	CreateDocument(ctx context.Context, doc *model.KBDocument) (*model.KBDocument, error)
	// ListDocumentsByKB lists by KB id only; the caller must verify read access first.
	ListDocumentsByKB(ctx context.Context, kbID int64) ([]model.KBDocument, error)
	GetDocument(ctx context.Context, id int64) (*model.KBDocument, error)
	// GetDocumentByName returns the document with the given filename in a KB, or
	// (nil, nil) if none exists. Used to reject duplicate uploads.
	GetDocumentByName(ctx context.Context, kbID int64, filename string) (*model.KBDocument, error)
	UpdateDocumentStatus(ctx context.Context, id int64, status, vikingURI, taskID, errMsg string) error
	DeleteDocument(ctx context.Context, id int64) error
}

var _ KnowledgeBaseStore = (*PostgresStorage)(nil)

func (s *PostgresStorage) CreateKB(ctx context.Context, kb *model.KnowledgeBase) (*model.KnowledgeBase, error) {
	err := s.pool.QueryRow(ctx,
		`INSERT INTO knowledge_bases (owner_id, name, description, visibility, viking_path)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, created_at, updated_at`,
		kb.OwnerID, kb.Name, kb.Description, kb.Visibility, kb.VikingPath).
		Scan(&kb.ID, &kb.CreatedAt, &kb.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return kb, nil
}

// kbSelectCols selects KB columns plus computed is_owner / can_manage flags.
// $1 = userID, $2 = isAdmin.
const kbSelectCols = `kb.id, kb.owner_id, kb.name, kb.description, kb.visibility, kb.viking_path,
	COALESCE(d.cnt, 0) AS doc_count,
	(kb.owner_id = $1) AS is_owner,
	(kb.owner_id = $1 OR $2) AS can_manage,
	kb.created_at, kb.updated_at`

const kbFromJoin = `FROM knowledge_bases kb
	LEFT JOIN (SELECT kb_id, COUNT(*) AS cnt FROM kb_documents GROUP BY kb_id) d ON d.kb_id = kb.id`

func scanKB(row pgx.Row) (*model.KnowledgeBase, error) {
	var kb model.KnowledgeBase
	err := row.Scan(&kb.ID, &kb.OwnerID, &kb.Name, &kb.Description, &kb.Visibility, &kb.VikingPath,
		&kb.DocCount, &kb.IsOwner, &kb.CanManage, &kb.CreatedAt, &kb.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &kb, nil
}

func (s *PostgresStorage) ListKBsVisibleToUser(ctx context.Context, userID int64, isAdmin bool) ([]model.KnowledgeBase, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+kbSelectCols+` `+kbFromJoin+`
		 WHERE $2 OR kb.owner_id = $1 OR kb.visibility = 'public'
		 ORDER BY kb.id DESC`, userID, isAdmin)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var kbs []model.KnowledgeBase
	for rows.Next() {
		kb, err := scanKB(rows)
		if err != nil {
			return nil, err
		}
		kbs = append(kbs, *kb)
	}
	return kbs, nil
}

func (s *PostgresStorage) GetKBForManage(ctx context.Context, id, userID int64, isAdmin bool) (*model.KnowledgeBase, error) {
	return s.getKB(ctx, id, userID, isAdmin, "(kb.owner_id = $1 OR $2)")
}

func (s *PostgresStorage) GetKBForRead(ctx context.Context, id, userID int64, isAdmin bool) (*model.KnowledgeBase, error) {
	return s.getKB(ctx, id, userID, isAdmin, "($2 OR kb.owner_id = $1 OR kb.visibility = 'public')")
}

func (s *PostgresStorage) getKB(ctx context.Context, id, userID int64, isAdmin bool, cond string) (*model.KnowledgeBase, error) {
	kb, err := scanKB(s.pool.QueryRow(ctx,
		`SELECT `+kbSelectCols+` `+kbFromJoin+`
		 WHERE kb.id = $3 AND `+cond, userID, isAdmin, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("knowledge base not found")
		}
		return nil, err
	}
	return kb, nil
}

func (s *PostgresStorage) UpdateKBVikingPath(ctx context.Context, id int64, vikingPath string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE knowledge_bases SET viking_path = $1, updated_at = NOW() WHERE id = $2`, vikingPath, id)
	return err
}

func (s *PostgresStorage) UpdateKB(ctx context.Context, id, userID int64, isAdmin bool, name, description, visibility string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE knowledge_bases SET name = $1, description = $2, visibility = $3, updated_at = NOW()
		 WHERE id = $4 AND (owner_id = $5 OR $6)`,
		name, description, visibility, id, userID, isAdmin)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("knowledge base not found")
	}
	return nil
}

func (s *PostgresStorage) DeleteKB(ctx context.Context, id, userID int64, isAdmin bool) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM knowledge_bases WHERE id = $1 AND (owner_id = $2 OR $3)`, id, userID, isAdmin)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("knowledge base not found")
	}
	return nil
}

func (s *PostgresStorage) CreateDocument(ctx context.Context, doc *model.KBDocument) (*model.KBDocument, error) {
	err := s.pool.QueryRow(ctx,
		`INSERT INTO kb_documents (kb_id, owner_id, filename, storage_path, viking_uri, size, status, task_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, created_at, updated_at`,
		doc.KBID, doc.OwnerID, doc.Filename, doc.StoragePath, doc.VikingURI, doc.Size, doc.Status, doc.TaskID).
		Scan(&doc.ID, &doc.CreatedAt, &doc.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return doc, nil
}

const docSelectCols = `id, kb_id, owner_id, filename, storage_path, viking_uri, size, status, error, task_id, created_at, updated_at`

func scanDoc(row pgx.Row) (*model.KBDocument, error) {
	var d model.KBDocument
	err := row.Scan(&d.ID, &d.KBID, &d.OwnerID, &d.Filename, &d.StoragePath, &d.VikingURI,
		&d.Size, &d.Status, &d.Error, &d.TaskID, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (s *PostgresStorage) ListDocumentsByKB(ctx context.Context, kbID int64) ([]model.KBDocument, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+docSelectCols+` FROM kb_documents WHERE kb_id = $1 ORDER BY id DESC`, kbID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []model.KBDocument
	for rows.Next() {
		d, err := scanDoc(rows)
		if err != nil {
			return nil, err
		}
		docs = append(docs, *d)
	}
	return docs, nil
}

func (s *PostgresStorage) GetDocument(ctx context.Context, id int64) (*model.KBDocument, error) {
	d, err := scanDoc(s.pool.QueryRow(ctx, `SELECT `+docSelectCols+` FROM kb_documents WHERE id = $1`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("document not found")
		}
		return nil, err
	}
	return d, nil
}

func (s *PostgresStorage) GetDocumentByName(ctx context.Context, kbID int64, filename string) (*model.KBDocument, error) {
	d, err := scanDoc(s.pool.QueryRow(ctx,
		`SELECT `+docSelectCols+` FROM kb_documents WHERE kb_id = $1 AND filename = $2 LIMIT 1`, kbID, filename))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return d, nil
}

// UpdateDocumentStatus updates the indexing status. viking_uri/task_id are only
// overwritten when the passed value is non-empty (so a later status flip keeps them).
func (s *PostgresStorage) UpdateDocumentStatus(ctx context.Context, id int64, status, vikingURI, taskID, errMsg string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE kb_documents
		 SET status = $1,
		     viking_uri = CASE WHEN $2 <> '' THEN $2 ELSE viking_uri END,
		     task_id = CASE WHEN $3 <> '' THEN $3 ELSE task_id END,
		     error = $4,
		     updated_at = NOW()
		 WHERE id = $5`,
		status, vikingURI, taskID, errMsg, id)
	return err
}

func (s *PostgresStorage) DeleteDocument(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM kb_documents WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("document not found")
	}
	return nil
}
