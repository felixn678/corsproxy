// Package session persists started proxy servers so they can be resumed
// later, mimicking how Claude Code saves sessions. Only the configuration
// needed to start again is stored — no runtime state, no request logs.
package session

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Session is one saved server configuration.
type Session struct {
	ID         string    `json:"id"`             // uuid, stable across restarts
	Name       string    `json:"name,omitempty"` // optional user-given label
	Target     string    `json:"target"`         // backend URL as a string
	Port       int       `json:"port"`           // proxy listen port
	Origin     string    `json:"origin"`         // allowed origin setting
	LastUsedAt time.Time `json:"lastUsedAt"`
}

// ErrNotFound is returned by Find when no session matches the given key.
var ErrNotFound = errors.New("session not found")

// AmbiguousError is returned by Find when a name matches several sessions.
// It carries the matches so the caller can list them for the user.
type AmbiguousError struct {
	Key     string
	Matches []Session
}

func (e *AmbiguousError) Error() string {
	return fmt.Sprintf("name %q matches %d saved servers", e.Key, len(e.Matches))
}

// Upsert adds incoming to sessions, deduplicating by (Target, Port) so that
// restarting the same proxy updates the existing entry instead of growing the
// store. On a match the existing ID is kept — previously printed resume hints
// must stay valid — and the name is only overwritten when incoming.Name is
// non-empty, so restarting without --name never erases a chosen name.
// Returns the updated slice and the session exactly as stored.
func Upsert(sessions []Session, incoming Session) ([]Session, Session) {
	for i, s := range sessions {
		if s.Target == incoming.Target && s.Port == incoming.Port {
			s.LastUsedAt = incoming.LastUsedAt
			s.Origin = incoming.Origin
			if incoming.Name != "" {
				s.Name = incoming.Name
			}
			sessions[i] = s
			return sessions, s
		}
	}
	return append(sessions, incoming), incoming
}

// Find resolves key against saved sessions: an exact ID match wins, then a
// case-insensitive name match. A name shared by several sessions returns
// *AmbiguousError instead of silently picking one.
func Find(sessions []Session, key string) (Session, error) {
	for _, s := range sessions {
		if s.ID == key {
			return s, nil
		}
	}

	var matches []Session
	for _, s := range sessions {
		if s.Name != "" && strings.EqualFold(s.Name, key) {
			matches = append(matches, s)
		}
	}
	switch len(matches) {
	case 0:
		return Session{}, ErrNotFound
	case 1:
		return matches[0], nil
	default:
		return Session{}, &AmbiguousError{Key: key, Matches: matches}
	}
}
