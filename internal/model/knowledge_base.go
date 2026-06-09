package model

import "time"

// Document status values.
const (
	KBDocStatusPending  = "pending"  // row created, not yet sent to OpenViking
	KBDocStatusIndexing = "indexing" // upload accepted, async indexing in progress
	KBDocStatusIndexed  = "indexed"  // searchable
	KBDocStatusFailed   = "failed"   // indexing failed (see Error)
)

// Knowledge base visibility.
const (
	KBVisibilityPrivate = "private" // only the owner can see/manage
	KBVisibilityPublic  = "public"  // all logged-in users can view + search; owner manages
)

// KnowledgeBase is a user-owned collection of documents backed by OpenViking.
// The actual files and vectors live in OpenViking under VikingPath; this row
// only holds metadata. Isolation is by OwnerID.
type KnowledgeBase struct {
	ID          int64     `json:"id"`
	OwnerID     int64     `json:"owner_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Visibility  string    `json:"visibility"`
	VikingPath  string    `json:"viking_path"`
	DocCount    int64     `json:"doc_count"`
	IsOwner     bool      `json:"is_owner"`   // computed per request; true if the caller owns this KB
	CanManage   bool      `json:"can_manage"` // computed; true if caller is owner OR admin
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// KBDocument is a single uploaded file within a knowledge base.
type KBDocument struct {
	ID          int64     `json:"id"`
	KBID        int64     `json:"kb_id"`
	OwnerID     int64     `json:"owner_id"`
	Filename    string    `json:"filename"`
	StoragePath string    `json:"-"` // local on-disk path; not exposed to clients
	VikingURI   string    `json:"viking_uri"`
	Size        int64     `json:"size"`
	Status      string    `json:"status"`
	Error       string    `json:"error,omitempty"`
	TaskID      string    `json:"task_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
