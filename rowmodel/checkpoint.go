package rowmodel

import "maps"

// Checkpoint is a resource's immutable resumable cursor.
type Checkpoint interface {
	Resource() string
	Int(key string) int
	String(key string) string
	Set(key string, value any) Checkpoint
	Raw() map[string]any
}

// CheckpointData is the concrete checkpoint representation.
type CheckpointData struct {
	ResourceName string         `json:"resource"`
	Cursor       map[string]any `json:"cursor"`
}

var _ Checkpoint = (*CheckpointData)(nil)

// NewCheckpoint returns an empty checkpoint for resource.
func NewCheckpoint(resource string) *CheckpointData {
	return &CheckpointData{ResourceName: resource, Cursor: map[string]any{}}
}

// Resource returns the resource this checkpoint advances.
func (c *CheckpointData) Resource() string { return c.ResourceName }

// Int reads key as an integer, accepting common JSON numeric representations.
func (c *CheckpointData) Int(key string) int {
	switch n := c.Cursor[key].(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}

func (c *CheckpointData) String(key string) string {
	value, _ := c.Cursor[key].(string)
	return value
}

// Set returns an independent checkpoint with key updated.
func (c *CheckpointData) Set(key string, value any) Checkpoint {
	next := make(map[string]any, len(c.Cursor)+1)
	maps.Copy(next, c.Cursor)
	next[key] = value
	return &CheckpointData{ResourceName: c.ResourceName, Cursor: next}
}

// Raw returns the cursor payload used by persistence adapters.
func (c *CheckpointData) Raw() map[string]any { return c.Cursor }
