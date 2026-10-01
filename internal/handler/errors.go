package handler

import (
	"strings"

	"github.com/google/uuid"

	"github.com/RamadhanRzq/gorraharja-backend-go/internal/apperr"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/model"
)

// parseStrictUUID parses a UUID string for query parameters.
func parseStrictUUID(raw, key string) (uuid.UUID, error) {
	id, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil {
		return uuid.UUID{}, apperr.BadRequest("query parameter '" + key + "' must be a valid UUID")
	}
	return id, nil
}

// parseFacilityStatus normalises a facility status query value.
func parseFacilityStatus(raw string) (model.FacilityStatus, error) {
	s := model.FacilityStatus(strings.ToUpper(strings.TrimSpace(raw)))
	if !s.Valid() {
		return "", apperr.BadRequest("query parameter 'status' must be one of ACTIVE, MAINTENANCE, CLOSED")
	}
	return s, nil
}

// parseUserStatus normalises a user status query value.
func parseUserStatus(raw string) (model.UserStatus, error) {
	s := model.UserStatus(strings.ToUpper(strings.TrimSpace(raw)))
	if s != model.UserStatusActive && s != model.UserStatusSuspended {
		return "", apperr.BadRequest("query parameter 'status' must be one of ACTIVE, SUSPENDED")
	}
	return s, nil
}

// parseRegistrationStatus normalises an event registration status query value.
func parseRegistrationStatus(raw string) (model.RegistrationStatus, error) {
	s := model.RegistrationStatus(strings.ToUpper(strings.TrimSpace(raw)))
	if s != model.RegistrationRegistered && s != model.RegistrationCancelled {
		return "", apperr.BadRequest("query parameter 'status' must be one of REGISTERED, CANCELLED")
	}
	return s, nil
}

func errMissingFacilityID() error {
	return apperr.BadRequest("query parameter 'facility_id' is required")
}

func errInvalidFacilityStatus() error {
	return apperr.BadRequest("query parameter 'status' must be one of ACTIVE, MAINTENANCE, CLOSED")
}

func errInvalidBookingStatus() error {
	return apperr.BadRequest("query parameter 'status' must be a valid booking status")
}

func errInvalidEventStatus() error {
	return apperr.BadRequest("query parameter 'status' must be a valid event status")
}
func errMissingSportID() error {
	return apperr.BadRequest("field 'sport_id' is required")
}

func errMissingPricingFields() error {
	return apperr.BadRequest("fields 'day_type', 'start_time', 'end_time' and 'price_per_hour' are required")
}

func errMissingClosureDates() error {
	return apperr.BadRequest("fields 'start_date' and 'end_date' are required")
}

func errInvalidPaymentStatus() error {
	return apperr.BadRequest("query parameter 'status' must be a valid payment status")
}

func errInvalidRole() error {
	return apperr.BadRequest("query parameter 'role' must be one of CUSTOMER, STAFF, ADMIN")
}
