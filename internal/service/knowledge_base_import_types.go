package service

import (
	"errors"

	"github.com/fisk086/aiops/internal/kbimport"
	"github.com/fisk086/aiops/internal/model"
)

// ImportURLFailure describes a single URL that could not be imported.
type ImportURLFailure struct {
	URL     string `json:"url"`
	Message string `json:"message"`
}

// ImportURLsResult is the outcome of a batch URL import.
type ImportURLsResult struct {
	Imported []model.KBDocument `json:"imported"`
	Failed   []ImportURLFailure `json:"failed"`
}

func importErrorMessage(err error) string {
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
	case errors.Is(err, ErrDocumentExists):
		return "同名文档已存在，请先删除旧文档或重命名后再导入"
	default:
		return err.Error()
	}
}
