// Package pagination provides cursor-based pagination utilities for the AEGIS platform.
//
// Cursor-based pagination is used instead of offset-based pagination for:
//   - Consistent results under concurrent writes
//   - Efficient database queries (no COUNT(*), uses indexed seeks)
//   - Stable pagination for real-time data
//
// Cursors are opaque base64-encoded strings containing the sort key(s) of the last
// returned item. Clients pass the cursor to get the next page.
package pagination

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

const (
	// DefaultLimit is the default number of items per page.
	DefaultLimit = 50
	// MaxLimit is the maximum number of items per page.
	MaxLimit = 100
)

// CursorPage represents pagination parameters from a client request.
type CursorPage struct {
	// Cursor is the opaque pagination cursor from the previous response.
	// Empty string means start from the beginning.
	Cursor string
	// Limit is the maximum number of items to return.
	Limit int
}

// NormalizeLimit ensures the limit is within valid bounds.
func (p *CursorPage) NormalizeLimit() {
	if p.Limit <= 0 {
		p.Limit = DefaultLimit
	}
	if p.Limit > MaxLimit {
		p.Limit = MaxLimit
	}
}

// PageResult holds a page of results with pagination metadata.
type PageResult[T any] struct {
	// Items is the list of items on this page.
	Items []T `json:"items"`
	// NextCursor is the cursor for the next page. Empty if no more pages.
	NextCursor string `json:"next_cursor,omitempty"`
	// HasMore indicates whether more pages are available.
	HasMore bool `json:"has_more"`
	// TotalCount is the optional total count (only populated if explicitly requested).
	TotalCount *int64 `json:"total_count,omitempty"`
}

// NewPageResult creates a PageResult from a slice of items.
// If len(items) > limit, it means there are more pages; the extra item is trimmed.
func NewPageResult[T any](items []T, limit int, cursorFn func(T) string) PageResult[T] {
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}

	var nextCursor string
	if hasMore && len(items) > 0 {
		nextCursor = cursorFn(items[len(items)-1])
	}

	return PageResult[T]{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}
}

// CursorData holds the decoded cursor fields for database queries.
type CursorData struct {
	// ID is the primary sort key (UUID as string).
	ID string `json:"id"`
	// CreatedAt is the secondary sort key (ISO 8601 timestamp string).
	CreatedAt string `json:"created_at,omitempty"`
	// SortValue is an optional additional sort value.
	SortValue string `json:"sort_value,omitempty"`
}

// EncodeCursor creates an opaque cursor string from cursor data.
func EncodeCursor(data CursorData) string {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	return base64.URLEncoding.EncodeToString(jsonBytes)
}

// DecodeCursor parses an opaque cursor string into cursor data.
func DecodeCursor(cursor string) (*CursorData, error) {
	if cursor == "" {
		return nil, nil
	}

	jsonBytes, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor encoding: %w", err)
	}

	var data CursorData
	if err := json.Unmarshal(jsonBytes, &data); err != nil {
		return nil, fmt.Errorf("invalid cursor format: %w", err)
	}

	if data.ID == "" {
		return nil, fmt.Errorf("cursor missing required 'id' field")
	}

	return &data, nil
}
