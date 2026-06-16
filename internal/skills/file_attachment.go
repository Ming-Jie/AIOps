package skills

import (
	"context"
	"log"
)

type FileAttachment struct {
	Filename string `json:"filename"`
	MimeType string `json:"mime_type,omitempty"`
	Size     int64  `json:"size,omitempty"`
	Inline   string `json:"inline,omitempty"`
	URL      string `json:"url,omitempty"`
}

type attachKey struct{}

func WithAttachmentCollector(ctx context.Context) context.Context {
	return context.WithValue(ctx, attachKey{}, &[]*FileAttachment{})
}

func AddAttachment(ctx context.Context, att *FileAttachment) {
	if p, ok := ctx.Value(attachKey{}).(*[]*FileAttachment); ok {
		*p = append(*p, att)
	} else {
		log.Printf("[attach] WARN: context has no attachment collector for file %s", att.Filename)
	}
}

func GetAttachments(ctx context.Context) []*FileAttachment {
	if p, ok := ctx.Value(attachKey{}).(*[]*FileAttachment); ok {
		return *p
	}
	return nil
}

func ConsumeAttachments(ctx context.Context) []*FileAttachment {
	if p, ok := ctx.Value(attachKey{}).(*[]*FileAttachment); ok {
		out := *p
		*p = nil
		return out
	}
	return nil
}
