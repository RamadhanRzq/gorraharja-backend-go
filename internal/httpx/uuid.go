package httpx

import "github.com/google/uuid"

// uuidValue is an alias keeping the public helpers free of a uuid import in callers.
type uuidValue = uuid.UUID

func parseUUID(raw string) (uuid.UUID, error) {
	return uuid.Parse(raw)
}
