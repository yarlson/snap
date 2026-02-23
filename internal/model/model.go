package model

// Type represents the abstract model type passed to provider CLIs.
type Type string

const (
	Fast     Type = "fast"
	Thinking Type = "thinking"
)
