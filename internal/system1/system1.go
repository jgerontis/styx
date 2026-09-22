// Package system1 defines the "System 1" gate: a fast, narrow typed-decision
// engine (e.g. a Laya-style non-autoregressive classifier) consulted before
// falling back to the slower, generative LLM ("System 2"). It answers a fixed
// set of typed questions about a piece of state and returns a calibrated
// confidence, but never generates text, tool calls, or arguments itself.
package system1

import "context"

// Primitive is the type of typed decision a Question asks.
type Primitive string

const (
	// Choice picks the best-fitting label from Criteria (map[string]string).
	Choice Primitive = "choice"
	// Score rates state against an ordinal rubric (Criteria is []string).
	Score Primitive = "score"
	// Noul returns a calibrated probability that a binary condition holds.
	Noul Primitive = "noul"
)

// Question is a single typed decision to answer against some state.
type Question struct {
	Name         string      `json:"name"`
	Primitive    Primitive   `json:"type"`
	Instructions string      `json:"instructions"`
	Criteria     interface{} `json:"criteria,omitempty"`
}

// Answer is a System 1 model's response to one Question. Only the field
// matching the Question's Primitive is populated.
type Answer struct {
	Choice     string  `json:"choice,omitempty"`
	Score      float64 `json:"score,omitempty"`
	Noul       float64 `json:"noul,omitempty"`
	Confidence float64 `json:"confidence"`
}

// HighConfidence reports whether the answer clears threshold.
func (a Answer) HighConfidence(threshold float64) bool {
	return a.Confidence >= threshold
}

// Gate answers typed Questions about state. Implementations must be safe to
// call frequently, must not block on user input, and must return quickly —
// callers depend on Gate being cheap enough to run on every tool call or
// phase transition without disrupting the interactive loop.
type Gate interface {
	Answer(ctx context.Context, q Question, state map[string]any) (Answer, error)
}

// NoopGate always returns a zero-confidence Answer, forcing callers to fall
// back to System 2 (or existing static rules). It is the default Gate so
// Styx runs fully without a System 1 model installed or reachable.
type NoopGate struct{}

// Answer implements Gate by declining every question.
func (NoopGate) Answer(context.Context, Question, map[string]any) (Answer, error) {
	return Answer{}, nil
}
