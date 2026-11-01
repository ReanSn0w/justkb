package ids

import "github.com/google/uuid"

// New returns an application-generated identifier suitable for SQLite TEXT keys.
func New() string {
	return uuid.NewString()
}
