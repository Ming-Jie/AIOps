package controller

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/fisk086/aiops/internal/auth"
	"github.com/fisk086/aiops/internal/kbimport"
	"github.com/fisk086/aiops/internal/schema"
	"github.com/fisk086/aiops/internal/service"
	"github.com/fisk086/aiops/internal/storage"
	"github.com/google/uuid"
)

// KnowledgeBaseController exposes the knowledge-base management API. Every route
// is JWT-protected; rows are scoped to the authenticated user (owner_id).
type KnowledgeBaseController struct {
	kbService *service.KnowledgeBaseService
	userStore storage.UserStore
	jwtCfg    auth.JWTConfig
	uploadDir string
}

func NewKnowledgeBaseController(kbService *service.KnowledgeBaseService, userStore storage.UserStore, jwtCfg auth.JWTConfig, uploadDir string) *KnowledgeBaseController {
	return &KnowledgeBaseController{
		kbService: kbService,
		userStore: userStore,
		jwtCfg:    jwtCfg,
		uploadDir: uploadDir,
	}
}

func (ctrl *KnowledgeBaseController) RegisterRoutes(r *server.Hertz) {
	g := r.Group("/api/v1/knowledge-base")
	g.Use(auth.JWTMiddleware(ctrl.jwtCfg, ctrl.getUserForMiddleware))

	g.GET("", ctrl.ListKBs)
	g.POST("", ctrl.CreateKB)
	g.PUT("/:id", ctrl.UpdateKB)
	g.DELETE("/:id", ctrl.DeleteKB)
	g.GET("/:id/documents", ctrl.ListDocuments)
	g.POST("/:id/documents", ctrl.UploadDocument)
	g.POST("/:id/documents/import-url", ctrl.ImportDocumentFromURL)
	g.POST("/:id/documents/import-urls", ctrl.ImportDocumentsFromURLs)
	g.GET("/:id/documents/:doc_id/preview", ctrl.PreviewDocument)
	g.DELETE("/:id/documents/:doc_id", ctrl.DeleteDocument)
	g.POST("/:id/search", ctrl.Search)
}

func (ctrl *KnowledgeBaseController) getUserForMiddleware(userID int64) (*auth.User, error) {
	user, err := ctrl.userStore.GetUserByID(userID)
	if err != nil {
		return nil, err
	}
	return &auth.User{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
		Status:   string(user.Status),
		IsAdmin:  user.IsAdmin,
	}, nil
}

// allowed upload extensions (aligned with OpenViking supported resource types).
var kbAllowedExts = map[string]bool{
	".pdf": true, ".md": true, ".markdown": true, ".txt": true, ".text": true,
	".html": true, ".htm": true, ".docx": true, ".epub": true,
	".xlsx": true, ".xls": true, ".pptx": true, ".csv": true, ".json": true,
}

const kbMaxUploadBytes = 50 * 1024 * 1024 // 50MB

func (ctrl *KnowledgeBaseController) ListKBs(ctx context.Context, c *app.RequestContext) {
	user := auth.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, schema.ErrorResponse("unauthorized"))
		return
	}
	kbs, err := ctrl.kbService.ListKBs(ctx, user.ID, user.IsAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusOK, schema.SuccessResponse(kbs))
}

func (ctrl *KnowledgeBaseController) CreateKB(ctx context.Context, c *app.RequestContext) {
	user := auth.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, schema.ErrorResponse("unauthorized"))
		return
	}
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Visibility  string `json:"visibility"`
	}
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}
	kb, err := ctrl.kbService.CreateKB(ctx, user.ID, req.Name, req.Description, req.Visibility)
	if err != nil {
		c.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusOK, schema.SuccessResponse(kb))
}

func (ctrl *KnowledgeBaseController) UpdateKB(ctx context.Context, c *app.RequestContext) {
	user := auth.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, schema.ErrorResponse("unauthorized"))
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid id"))
		return
	}
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Visibility  string `json:"visibility"`
	}
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}
	kb, err := ctrl.kbService.UpdateKB(ctx, id, user.ID, user.IsAdmin, req.Name, req.Description, req.Visibility)
	if err != nil {
		c.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusOK, schema.SuccessResponse(kb))
}

func (ctrl *KnowledgeBaseController) DeleteKB(ctx context.Context, c *app.RequestContext) {
	user := auth.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, schema.ErrorResponse("unauthorized"))
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid id"))
		return
	}
	if err := ctrl.kbService.DeleteKB(ctx, id, user.ID, user.IsAdmin); err != nil {
		c.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusOK, schema.SuccessResponse(nil))
}

func (ctrl *KnowledgeBaseController) ListDocuments(ctx context.Context, c *app.RequestContext) {
	user := auth.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, schema.ErrorResponse("unauthorized"))
		return
	}
	kbID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid id"))
		return
	}
	docs, err := ctrl.kbService.ListDocuments(ctx, kbID, user.ID, user.IsAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusOK, schema.SuccessResponse(docs))
}

func (ctrl *KnowledgeBaseController) UploadDocument(ctx context.Context, c *app.RequestContext) {
	user := auth.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, schema.ErrorResponse("unauthorized"))
		return
	}
	kbID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid id"))
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, schema.ErrorResponse("no file provided"))
		return
	}
	if file.Size > kbMaxUploadBytes {
		c.JSON(http.StatusBadRequest, schema.ErrorResponse("file too large, max 50MB"))
		return
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !kbAllowedExts[ext] {
		c.JSON(http.StatusBadRequest, schema.ErrorResponse("unsupported file type"))
		return
	}

	uid := strconv.FormatInt(user.ID, 10)
	dir := filepath.Join(ctrl.uploadDir, "kb", uid)
	if err := os.MkdirAll(dir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, schema.ErrorResponse("failed to create upload directory"))
		return
	}
	// Collision-proof on-disk name; original name is kept in the DB filename column.
	storageName := fmt.Sprintf("%d_%s%s", time.Now().UnixNano(), uuid.New().String()[:8], ext)
	storagePath := filepath.Join(dir, storageName)

	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, schema.ErrorResponse("failed to open file"))
		return
	}
	defer src.Close()
	dst, err := os.Create(storagePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, schema.ErrorResponse("failed to create file"))
		return
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		c.JSON(http.StatusInternalServerError, schema.ErrorResponse("failed to save file"))
		return
	}

	doc, err := ctrl.kbService.AddDocument(ctx, kbID, user.ID, user.IsAdmin, file.Filename, storagePath, file.Size)
	if err != nil {
		// clean up the orphaned file on failure
		_ = os.Remove(storagePath)
		if errors.Is(err, service.ErrDocumentExists) {
			c.JSON(http.StatusConflict, schema.ErrorResponse("同名文档已存在，请先删除旧文档或重命名后再上传"))
			return
		}
		c.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusOK, schema.SuccessResponse(doc))
}

func mapKBImportError(err error) string {
	switch {
	case errors.Is(err, kbimport.ErrInvalidURL):
		return "请输入有效的 HTTPS 链接"
	case errors.Is(err, kbimport.ErrURLNotAllowed):
		return "该链接不允许导入（仅支持 HTTPS，且不能访问内网地址）"
	case errors.Is(err, kbimport.ErrFileTooLarge):
		return "文件过大，最大 50MB"
	case errors.Is(err, kbimport.ErrUnsupportedType):
		return "不支持的文件类型"
	case errors.Is(err, kbimport.ErrEmptyBody):
		return "链接返回内容为空"
	case errors.Is(err, service.ErrDocumentExists):
		return "同名文档已存在，请先删除旧文档或重命名后再导入"
	default:
		return err.Error()
	}
}

func (ctrl *KnowledgeBaseController) ImportDocumentFromURL(ctx context.Context, c *app.RequestContext) {
	user := auth.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, schema.ErrorResponse("unauthorized"))
		return
	}
	kbID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid id"))
		return
	}
	var req struct {
		URL      string `json:"url"`
		Filename string `json:"filename"`
	}
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}
	doc, err := ctrl.kbService.ImportDocumentFromURL(ctx, kbID, user.ID, user.IsAdmin, req.URL, req.Filename, ctrl.uploadDir)
	if err != nil {
		c.JSON(http.StatusBadRequest, schema.ErrorResponse(mapKBImportError(err)))
		return
	}
	c.JSON(http.StatusOK, schema.SuccessResponse(doc))
}

func (ctrl *KnowledgeBaseController) ImportDocumentsFromURLs(ctx context.Context, c *app.RequestContext) {
	user := auth.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, schema.ErrorResponse("unauthorized"))
		return
	}
	kbID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid id"))
		return
	}
	var req struct {
		URLs []string `json:"urls"`
	}
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}
	if len(req.URLs) == 0 {
		c.JSON(http.StatusBadRequest, schema.ErrorResponse("请至少提供一个链接"))
		return
	}
	result, err := ctrl.kbService.ImportDocumentsFromURLs(ctx, kbID, user.ID, user.IsAdmin, req.URLs, ctrl.uploadDir)
	if err != nil {
		c.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusOK, schema.SuccessResponse(result))
}

func (ctrl *KnowledgeBaseController) PreviewDocument(ctx context.Context, c *app.RequestContext) {
	user := auth.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, schema.ErrorResponse("unauthorized"))
		return
	}
	kbID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid id"))
		return
	}
	docID, err := strconv.ParseInt(c.Param("doc_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid doc_id"))
		return
	}
	doc, err := ctrl.kbService.GetDocumentForPreview(ctx, kbID, docID, user.ID, user.IsAdmin)
	if err != nil {
		c.JSON(http.StatusNotFound, schema.ErrorResponse("document not found"))
		return
	}
	if doc.StoragePath == "" {
		c.JSON(http.StatusNotFound, schema.ErrorResponse("preview not available"))
		return
	}
	cleanPath := filepath.Clean(doc.StoragePath)
	uploadRoot := filepath.Clean(ctrl.uploadDir)
	if !strings.HasPrefix(cleanPath, uploadRoot+string(os.PathSeparator)) && cleanPath != uploadRoot {
		c.JSON(http.StatusForbidden, schema.ErrorResponse("invalid storage path"))
		return
	}
	f, err := os.Open(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, schema.ErrorResponse("file not found on disk"))
			return
		}
		c.JSON(http.StatusInternalServerError, schema.ErrorResponse("failed to open file"))
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		c.JSON(http.StatusNotFound, schema.ErrorResponse("file not found"))
		return
	}
	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(doc.Filename)))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", mimeFormatInline(doc.Filename))
	c.SetStatusCode(http.StatusOK)
	_, _ = io.Copy(c.Response.BodyWriter(), f)
}

func mimeFormatInline(filename string) string {
	safe := strings.ReplaceAll(filename, `"`, `_`)
	return fmt.Sprintf(`inline; filename="%s"`, safe)
}

func (ctrl *KnowledgeBaseController) DeleteDocument(ctx context.Context, c *app.RequestContext) {
	user := auth.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, schema.ErrorResponse("unauthorized"))
		return
	}
	kbID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid id"))
		return
	}
	docID, err := strconv.ParseInt(c.Param("doc_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid doc_id"))
		return
	}
	if err := ctrl.kbService.DeleteDocument(ctx, kbID, docID, user.ID, user.IsAdmin); err != nil {
		c.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusOK, schema.SuccessResponse(nil))
}

func (ctrl *KnowledgeBaseController) Search(ctx context.Context, c *app.RequestContext) {
	user := auth.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, schema.ErrorResponse("unauthorized"))
		return
	}
	kbID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, schema.ErrorResponse("invalid id"))
		return
	}
	var req struct {
		Query string `json:"query"`
		TopK  int    `json:"top_k"`
	}
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(http.StatusBadRequest, schema.ErrorResponse(err.Error()))
		return
	}
	hits, err := ctrl.kbService.Search(ctx, kbID, user.ID, user.IsAdmin, req.Query, req.TopK)
	if err != nil {
		c.JSON(http.StatusInternalServerError, schema.ErrorResponse(err.Error()))
		return
	}
	c.JSON(http.StatusOK, schema.SuccessResponse(hits))
}
