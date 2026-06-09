package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fisk086/aiops/internal/kbimport"
	"github.com/fisk086/aiops/internal/logger"
	"github.com/fisk086/aiops/internal/model"
	"github.com/fisk086/aiops/internal/openviking"
	"github.com/fisk086/aiops/internal/storage"
	"github.com/google/uuid"
)

// ErrDocumentExists is returned by AddDocument when a document with the same
// filename already exists in the knowledge base.
var ErrDocumentExists = errors.New("a document with this name already exists")

// KnowledgeBaseService wires the KB metadata store to the OpenViking backend.
// Document indexing runs asynchronously (the OpenViking "wait" call is slow);
// in-flight indexing is drained on shutdown via Wait().
type KnowledgeBaseService struct {
	store storage.KnowledgeBaseStore
	ov    *openviking.Client
	wg    sync.WaitGroup
}

func NewKnowledgeBaseService(store storage.KnowledgeBaseStore, ov *openviking.Client) *KnowledgeBaseService {
	return &KnowledgeBaseService{store: store, ov: ov}
}

// Wait blocks until all background indexing goroutines finish (call on shutdown).
func (s *KnowledgeBaseService) Wait() {
	s.wg.Wait()
}

// CreateKB persists the KB and provisions its OpenViking directory.
func (s *KnowledgeBaseService) CreateKB(ctx context.Context, ownerID int64, name, description, visibility string) (*model.KnowledgeBase, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if visibility != model.KBVisibilityPublic {
		visibility = model.KBVisibilityPrivate
	}
	kb, err := s.store.CreateKB(ctx, &model.KnowledgeBase{
		OwnerID:     ownerID,
		Name:        name,
		Description: strings.TrimSpace(description),
		Visibility:  visibility,
	})
	if err != nil {
		return nil, err
	}
	kb.IsOwner = true
	kb.CanManage = true

	path := openviking.KBPath(kb.ID)
	if err := s.ov.Mkdir(ctx, path); err != nil {
		logger.Warn("kb: ov mkdir failed", "kb_id", kb.ID, "path", path, "err", err)
	}
	if err := s.store.UpdateKBVikingPath(ctx, kb.ID, path); err != nil {
		logger.Warn("kb: persist viking_path failed", "kb_id", kb.ID, "err", err)
	}
	kb.VikingPath = path
	return kb, nil
}

func (s *KnowledgeBaseService) ListKBs(ctx context.Context, userID int64, isAdmin bool) ([]model.KnowledgeBase, error) {
	return s.store.ListKBsVisibleToUser(ctx, userID, isAdmin)
}

func (s *KnowledgeBaseService) GetKB(ctx context.Context, id, userID int64, isAdmin bool) (*model.KnowledgeBase, error) {
	return s.store.GetKBForRead(ctx, id, userID, isAdmin)
}

// UpdateKB updates name/description/visibility (owner or admin). Lets the owner
// or an admin switch a KB between public and private.
func (s *KnowledgeBaseService) UpdateKB(ctx context.Context, id, userID int64, isAdmin bool, name, description, visibility string) (*model.KnowledgeBase, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if visibility != model.KBVisibilityPublic {
		visibility = model.KBVisibilityPrivate
	}
	if err := s.store.UpdateKB(ctx, id, userID, isAdmin, name, strings.TrimSpace(description), visibility); err != nil {
		return nil, err
	}
	return s.store.GetKBForRead(ctx, id, userID, isAdmin)
}

// DeleteKB removes the KB (cascade deletes documents) and its OpenViking directory.
func (s *KnowledgeBaseService) DeleteKB(ctx context.Context, id, userID int64, isAdmin bool) error {
	kb, err := s.store.GetKBForManage(ctx, id, userID, isAdmin)
	if err != nil {
		return err
	}
	if err := s.store.DeleteKB(ctx, id, userID, isAdmin); err != nil {
		return err
	}
	if err := s.ov.Rm(ctx, openviking.KBPath(kb.ID)); err != nil {
		logger.Warn("kb: ov rm dir failed", "kb_id", kb.ID, "err", err)
	}
	return nil
}

func (s *KnowledgeBaseService) ListDocuments(ctx context.Context, kbID, userID int64, isAdmin bool) ([]model.KBDocument, error) {
	// Read access: owner, public KB, or admin.
	if _, err := s.store.GetKBForRead(ctx, kbID, userID, isAdmin); err != nil {
		return nil, err
	}
	return s.store.ListDocumentsByKB(ctx, kbID)
}

// AddDocument records a freshly-saved local file and kicks off async indexing.
// The file at storagePath must already be persisted on disk by the caller.
// Requires manage access (owner or admin) to the KB.
func (s *KnowledgeBaseService) AddDocument(ctx context.Context, kbID, userID int64, isAdmin bool, filename, storagePath string, size int64) (*model.KBDocument, error) {
	kb, err := s.store.GetKBForManage(ctx, kbID, userID, isAdmin)
	if err != nil {
		return nil, err
	}
	// Reject duplicate filenames within the same KB (avoids duplicate chunks
	// polluting retrieval; the user is told to delete the old one first).
	existing, err := s.store.GetDocumentByName(ctx, kbID, filename)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrDocumentExists
	}
	doc, err := s.store.CreateDocument(ctx, &model.KBDocument{
		KBID:        kbID,
		OwnerID:     kb.OwnerID, // documents belong to the KB owner, even when an admin uploads
		Filename:    filename,
		StoragePath: storagePath,
		Size:        size,
		Status:      model.KBDocStatusIndexing,
	})
	if err != nil {
		return nil, err
	}
	s.indexDocumentAsync(doc)
	return doc, nil
}

// ImportDocumentFromURL downloads a HTTPS resource and ingests it like a local upload.
func (s *KnowledgeBaseService) ImportDocumentFromURL(ctx context.Context, kbID, userID int64, isAdmin bool, rawURL, preferredName, uploadDir string) (*model.KBDocument, error) {
	fetched, err := kbimport.Fetch(ctx, rawURL, preferredName)
	if err != nil {
		return nil, err
	}

	uid := strconv.FormatInt(userID, 10)
	dir := filepath.Join(uploadDir, "kb", uid)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create upload dir: %w", err)
	}
	ext := strings.ToLower(filepath.Ext(fetched.Filename))
	storageName := fmt.Sprintf("%d_%s%s", time.Now().UnixNano(), uuid.New().String()[:8], ext)
	storagePath := filepath.Join(dir, storageName)

	if err := os.WriteFile(storagePath, fetched.Body, 0o644); err != nil {
		return nil, fmt.Errorf("save file: %w", err)
	}

	doc, err := s.AddDocument(ctx, kbID, userID, isAdmin, fetched.Filename, storagePath, int64(len(fetched.Body)))
	if err != nil {
		_ = os.Remove(storagePath)
		return nil, err
	}
	logger.Info("kb: document imported from url", "doc_id", doc.ID, "kb_id", kbID, "source", fetched.SourceURL)
	return doc, nil
}

const maxImportURLsPerRequest = 30

// ImportDocumentsFromURLs imports multiple HTTPS resources; partial success is allowed.
func (s *KnowledgeBaseService) ImportDocumentsFromURLs(ctx context.Context, kbID, userID int64, isAdmin bool, urls []string, uploadDir string) (*ImportURLsResult, error) {
	if _, err := s.store.GetKBForManage(ctx, kbID, userID, isAdmin); err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(urls))
	result := &ImportURLsResult{
		Imported: make([]model.KBDocument, 0),
		Failed:   make([]ImportURLFailure, 0),
	}
	for _, raw := range urls {
		u := strings.TrimSpace(raw)
		if u == "" {
			continue
		}
		key := strings.ToLower(u)
		if _, ok := seen[key]; ok {
			result.Failed = append(result.Failed, ImportURLFailure{
				URL:     u,
				Message: "duplicate URL in request",
			})
			continue
		}
		seen[key] = struct{}{}
		if len(result.Imported)+len(result.Failed) >= maxImportURLsPerRequest {
			result.Failed = append(result.Failed, ImportURLFailure{
				URL:     u,
				Message: fmt.Sprintf("exceeds max %d URLs per request", maxImportURLsPerRequest),
			})
			continue
		}
		doc, err := s.ImportDocumentFromURL(ctx, kbID, userID, isAdmin, u, "", uploadDir)
		if err != nil {
			result.Failed = append(result.Failed, ImportURLFailure{
				URL:     u,
				Message: importErrorMessage(err),
			})
			continue
		}
		result.Imported = append(result.Imported, *doc)
	}
	return result, nil
}

// indexDocumentAsync uploads the file to OpenViking and waits for indexing,
// then flips the document status. It uses a fresh context (never the request
// ctx, which is cancelled when the HTTP response returns).
func (s *KnowledgeBaseService) indexDocumentAsync(doc *model.KBDocument) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		logger.Info("kb: indexing started", "doc_id", doc.ID, "kb_id", doc.KBID, "filename", doc.Filename)
		tempID, err := s.ov.TempUpload(ctx, doc.StoragePath)
		if err != nil {
			logger.Error("kb: temp_upload failed", "doc_id", doc.ID, "err", err)
			_ = s.store.UpdateDocumentStatus(ctx, doc.ID, model.KBDocStatusFailed, "", "", err.Error())
			return
		}
		toURI := openviking.DocURI(doc.KBID, doc.ID, doc.Filename)
		res, err := s.ov.AddResource(ctx, tempID, toURI, "knowledge base upload", true)
		if err != nil {
			logger.Error("kb: add_resource failed", "doc_id", doc.ID, "err", err)
			_ = s.store.UpdateDocumentStatus(ctx, doc.ID, model.KBDocStatusFailed, "", "", err.Error())
			return
		}
		vikingURI := res.RootURI
		if vikingURI == "" {
			vikingURI = toURI
		}
		if err := s.store.UpdateDocumentStatus(ctx, doc.ID, model.KBDocStatusIndexed, vikingURI, res.TaskID, ""); err != nil {
			logger.Error("kb: update doc status failed", "doc_id", doc.ID, "err", err)
			return
		}
		logger.Info("kb: document indexed", "doc_id", doc.ID, "kb_id", doc.KBID, "uri", vikingURI)
	}()
}

// DeleteDocument removes the document record and its OpenViking resource.
// Requires manage access (owner or admin) to the KB.
func (s *KnowledgeBaseService) DeleteDocument(ctx context.Context, kbID, docID, userID int64, isAdmin bool) error {
	if _, err := s.store.GetKBForManage(ctx, kbID, userID, isAdmin); err != nil {
		return err
	}
	doc, err := s.store.GetDocument(ctx, docID)
	if err != nil {
		return err
	}
	if doc.KBID != kbID {
		return fmt.Errorf("document not found")
	}
	if err := s.store.DeleteDocument(ctx, docID); err != nil {
		return err
	}
	uri := doc.VikingURI
	if uri == "" {
		uri = openviking.DocURI(doc.KBID, doc.ID, doc.Filename)
	}
	if err := s.ov.Rm(ctx, uri); err != nil {
		logger.Warn("kb: ov rm document failed", "doc_id", docID, "uri", uri, "err", err)
	}
	return nil
}

// GetDocumentForPreview returns a document the caller may read. Requires read access to the KB.
func (s *KnowledgeBaseService) GetDocumentForPreview(ctx context.Context, kbID, docID, userID int64, isAdmin bool) (*model.KBDocument, error) {
	if _, err := s.store.GetKBForRead(ctx, kbID, userID, isAdmin); err != nil {
		return nil, err
	}
	doc, err := s.store.GetDocument(ctx, docID)
	if err != nil {
		return nil, err
	}
	if doc.KBID != kbID {
		return nil, fmt.Errorf("document not found")
	}
	return doc, nil
}

// Search runs a retrieval query scoped to the knowledge base.
func (s *KnowledgeBaseService) Search(ctx context.Context, kbID, userID int64, isAdmin bool, query string, topK int) ([]openviking.MatchedContext, error) {
	// Read access: owner, public KB, or admin.
	if _, err := s.store.GetKBForRead(ctx, kbID, userID, isAdmin); err != nil {
		return nil, err
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if topK <= 0 {
		topK = 10
	}
	return s.searchKBPaths(ctx, []int64{kbID}, query, topK)
}

// SearchForAgent retrieves across the agent's bound knowledge bases. It trusts
// the binding (no per-user access check) — the agent's creator chose these KBs,
// so any chat user of the agent gets their context. Results from all bound KBs
// are merged and the top-k by score returned.
func (s *KnowledgeBaseService) SearchForAgent(ctx context.Context, kbIDs []int64, query string, topK int) ([]openviking.MatchedContext, error) {
	query = strings.TrimSpace(query)
	if query == "" || len(kbIDs) == 0 {
		return nil, nil
	}
	if topK <= 0 {
		topK = 8
	}
	return s.searchKBPaths(ctx, kbIDs, query, topK)
}

// BuildRAGContext searches bound knowledge bases and formats hits for prompt injection.
// Returns empty string when disabled, no hits, or on recoverable errors.
func (s *KnowledgeBaseService) BuildRAGContext(ctx context.Context, agentID int64, kbIDs []int64, userText string, topK int) string {
	if s == nil || len(kbIDs) == 0 {
		return ""
	}
	query := strings.TrimSpace(userText)
	if query == "" {
		logger.Info("knowledge base retrieval skipped: empty query", "agent_id", agentID, "kb_ids", kbIDs)
		return ""
	}
	if topK <= 0 {
		topK = 5
	}
	logger.Info("knowledge base retrieval started", "agent_id", agentID, "kb_ids", kbIDs, "top_k", topK, "query_len", len(query))

	results, err := s.SearchForAgent(ctx, kbIDs, query, topK)
	if err != nil {
		logger.Warn("knowledge base search failed", "agent_id", agentID, "kb_ids", kbIDs, "err", err)
		return ""
	}
	if len(results) == 0 {
		logger.Info("knowledge base retrieval: no hits", "agent_id", agentID, "kb_ids", kbIDs)
		return ""
	}

	topScore := results[0].Score
	logger.Info("knowledge base retrieval completed",
		"agent_id", agentID,
		"kb_ids", kbIDs,
		"hit_count", len(results),
		"top_score", fmt.Sprintf("%.4f", topScore),
	)

	var b strings.Builder
	b.WriteString("The following lines are retrieved from knowledge base; use only if relevant.\n")
	for i, r := range results {
		if i > 0 {
			b.WriteString("\n---\n")
		}
		content := strings.TrimSpace(r.Content)
		if content == "" {
			content = strings.TrimSpace(r.Abstract)
		}
		if content == "" {
			content = strings.TrimSpace(r.Overview)
		}
		if content == "" {
			continue
		}
		if name := openviking.DisplayNameFromURI(r.URI); name != "" {
			fmt.Fprintf(&b, "[%s]\n", name)
		}
		if len(content) > openviking.MaxSnippetChars {
			content = content[:openviking.MaxSnippetChars] + "…"
		}
		b.WriteString(content)
	}
	return strings.TrimSpace(b.String())
}

// searchKBPaths runs vector Find (fallback SmartSearch), merges hits, hydrates raw
// content via content/read (abstract alone is often a summary without version facts).
func (s *KnowledgeBaseService) searchKBPaths(ctx context.Context, kbIDs []int64, query string, topK int) ([]openviking.MatchedContext, error) {
	fetchLimit := topK * openviking.SearchFetchMultiplier
	if fetchLimit < 10 {
		fetchLimit = 10
	}

	var all []openviking.MatchedContext
	for _, kbID := range kbIDs {
		target := openviking.KBPath(kbID)
		hits, err := s.ov.Find(ctx, query, target, fetchLimit, 0)
		if err != nil || len(hits) == 0 {
			if err != nil {
				logger.Warn("kb: find failed, falling back to smart search", "kb_id", kbID, "err", err)
			}
			hits, err = s.ov.SmartSearch(ctx, query, target, fetchLimit, 0)
			if err != nil {
				logger.Warn("kb: smart search failed", "kb_id", kbID, "err", err)
				continue
			}
		}
		all = append(all, hits...)
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].Score > all[j].Score })
	if len(all) > topK {
		all = all[:topK]
	}
	for i := range all {
		all[i].Content = openviking.ResolveHitText(ctx, s.ov, all[i])
	}
	return all, nil
}
