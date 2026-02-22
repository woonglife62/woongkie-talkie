package mongodb

import "errors"

// Sentinel errors for domain-specific error handling.
var (
	ErrForbidden          = errors.New("forbidden")
	ErrEditWindowExpired  = errors.New("edit window expired")
	ErrMessageDeleted     = errors.New("message deleted")
	ErrNotFound           = errors.New("not found")
	ErrDuplicateRoomName  = errors.New("duplicate room name")
)
