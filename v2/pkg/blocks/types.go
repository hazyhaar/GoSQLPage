// Package blocks provides the core block types for GoSQLPage v2.1.
// Blocks are the fundamental unit of content, inspired by SiYuan/Notion.
package blocks

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// Block represents a content block in the system.
type Block struct {
	ID          string     `json:"id"`
	ParentID    *string    `json:"parent_id,omitempty"`
	Type        string     `json:"type"`
	Content     string     `json:"content"`
	ContentHTML string     `json:"content_html,omitempty"`
	Position    string     `json:"position"`
	Hash        string     `json:"hash"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CreatedBy   string     `json:"created_by"`
	Published   bool       `json:"published"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`

	// Session-specific fields (not persisted to content.db)
	Dirty  bool   `json:"-"`
	Source string `json:"-"` // "new" or "copy"
}

// Ref represents a relation between two blocks.
type Ref struct {
	FromID    string    `json:"from_id"`
	ToID      string    `json:"to_id"`
	Type      string    `json:"type"`
	Anchor    *string   `json:"anchor,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	CreatedBy string    `json:"created_by"`

	// Session-specific
	Dirty bool `json:"-"`
}

// Attr represents a custom attribute on a block.
type Attr struct {
	BlockID string `json:"block_id"`
	Name    string `json:"name"`
	Value   string `json:"value"`

	// Session-specific
	Dirty bool `json:"-"`
}

// BlockType represents a block type definition from schema.db.
type BlockType struct {
	Name            string   `json:"name"`
	Label           string   `json:"label"`
	Icon            string   `json:"icon,omitempty"`
	Schema          string   `json:"schema,omitempty"` // JSON schema for validation
	RenderTemplate  string   `json:"render_template,omitempty"`
	AllowedChildren []string `json:"allowed_children,omitempty"`
	AllowedParents  []string `json:"allowed_parents,omitempty"`
	Category        string   `json:"category"`
	Version         int      `json:"version"`
}

// RelationType represents a relation type definition.
type RelationType struct {
	Name      string   `json:"name"`
	Label     string   `json:"label"`
	Inverse   string   `json:"inverse,omitempty"`
	FromTypes []string `json:"from_types,omitempty"`
	ToTypes   []string `json:"to_types,omitempty"`
	Symmetric bool     `json:"symmetric"`
}

// Category constants for block types.
const (
	CategoryContent    = "content"
	CategoryDiscussion = "discussion"
	CategoryKnowledge  = "knowledge"
	CategoryTask       = "task"
	CategoryBot        = "bot"
	CategorySystem     = "system"
)

// Common block types.
const (
	TypeDocument   = "document"
	TypeHeading    = "heading"
	TypeParagraph  = "paragraph"
	TypeList       = "list"
	TypeListItem   = "list_item"
	TypeCode       = "code"
	TypeTable      = "table"
	TypeQuote      = "quote"
	TypeEmbed      = "embed"
	TypeQuestion   = "question"
	TypeAnswer     = "answer"
	TypeClaim      = "claim"
	TypeTask       = "task"
	TypeBotRequest = "bot_request"
	TypeBotResponse = "bot_response"
)

// Common relation types.
const (
	RelParentOf    = "parent_of"
	RelChildOf     = "child_of"
	RelReferences  = "references"
	RelCites       = "cites"
	RelRefutes     = "refutes"
	RelExtends     = "extends"
	RelDepends     = "depends"
	RelSupersedes  = "supersedes"
	RelAnswers     = "answers"
	RelBlocks      = "blocks"
	RelRelatedTo   = "related_to"
)

// ComputeHash calculates the SHA256 hash of the block content.
func (b *Block) ComputeHash() string {
	h := sha256.New()
	h.Write([]byte(b.Content))
	return hex.EncodeToString(h.Sum(nil))
}

// UpdateHash updates the block's hash based on current content.
func (b *Block) UpdateHash() {
	b.Hash = b.ComputeHash()
}

// IsRoot returns true if this block has no parent.
func (b *Block) IsRoot() bool {
	return b.ParentID == nil
}

// BlockWithRefs represents a block with its relations.
type BlockWithRefs struct {
	Block     *Block  `json:"block"`
	Refs      []*Ref  `json:"refs,omitempty"`      // outgoing refs
	Backlinks []*Ref  `json:"backlinks,omitempty"` // incoming refs
	Attrs     []*Attr `json:"attrs,omitempty"`
}

// BlockTree represents a block with its children.
type BlockTree struct {
	Block    *Block       `json:"block"`
	Children []*BlockTree `json:"children,omitempty"`
	Attrs    []*Attr      `json:"attrs,omitempty"`
}

// Change represents a modification in a session.
type Change struct {
	ID        int64     `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Operation string    `json:"operation"` // insert, update, delete, link, unlink
	BlockID   string    `json:"block_id"`
	Field     *string   `json:"field,omitempty"` // null for full block ops
	Before    string    `json:"before,omitempty"`
	After     string    `json:"after,omitempty"`
	Merged    bool      `json:"merged"`
}

// Operation constants for changes.
const (
	OpInsert = "insert"
	OpUpdate = "update"
	OpDelete = "delete"
	OpLink   = "link"
	OpUnlink = "unlink"
)

// StructuralDep tracks dependencies for conflict detection.
type StructuralDep struct {
	BlockID        string            `json:"block_id"`
	DependsOn      []string          `json:"depends_on"`
	SnapshotHashes map[string]string `json:"snapshot_hashes"`
}
