package main

import (
	"time"

	"github.com/google/uuid"
)

// systemClock is the production domain.Clock. It is the only place, alongside the composition root,
// where the wall clock is read; the domain never reads it directly.
type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// newMessageID produces a unique Message-ID local part for each outgoing message.
func newMessageID() string {
	return uuid.NewString() + "@pigeonpost"
}

// newOutboxID produces a unique identifier for a queued outbox item.
func newOutboxID() string {
	return uuid.NewString()
}

// newRuleID produces a unique identifier for a filter rule.
func newRuleID() string {
	return uuid.NewString()
}
