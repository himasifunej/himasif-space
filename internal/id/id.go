// Package id makes the stable identities of Space's objects.
//
// Every object gets a UUIDv7 when it is created and keeps it for good: it
// never derives from a title, a row number, a deadline or any other value
// that can change (PRD SYN-06). Version 7 sorts by creation time, which
// keeps database indexes compact.
package id

import "github.com/google/uuid"

// ID is the identity of an object.
type ID = uuid.UUID

// Nil is the absent identity.
var Nil = uuid.Nil

// New returns a new identity. It panics only when the system's random
// source fails, which leaves nothing safe to do.
func New() ID {
	return uuid.Must(uuid.NewV7())
}

// Parse reads an identity from its text form.
func Parse(s string) (ID, error) {
	return uuid.Parse(s)
}
