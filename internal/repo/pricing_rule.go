package repo

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/RamadhanRzq/gorraharja-backend-go/internal/apperr"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/database"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/model"
)

const pricingRuleColumns = `id, facility_id, day_type, start_time, end_time, price_per_hour, is_active, created_at, updated_at`

// PricingRuleRepo persists per-facility price windows.
type PricingRuleRepo struct {
	db *database.DB
}

// NewPricingRuleRepo builds a pricing rule repository.
func NewPricingRuleRepo(db *database.DB) *PricingRuleRepo { return &PricingRuleRepo{db: db} }

func scanPricingRule(row pgx.Row) (*model.PricingRule, error) {
	var p model.PricingRule
	err := row.Scan(&p.ID, &p.FacilityID, &p.DayType, &p.StartTime, &p.EndTime,
		&p.PricePerHour, &p.IsActive, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("pricing rule not found")
		}
		return nil, err
	}
	return &p, nil
}

// ListByFacility returns the rules of a facility ordered for deterministic
// overlap checks.
func (r *PricingRuleRepo) ListByFacility(ctx context.Context, facilityID uuid.UUID, activeOnly bool) ([]model.PricingRule, error) {
	rows, err := r.db.Pool().Query(ctx, `
		SELECT `+pricingRuleColumns+`
		FROM pricing_rules
		WHERE facility_id = $1 AND ($2::boolean IS FALSE OR is_active)
		ORDER BY day_type, start_time`,
		facilityID, activeOnly)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []model.PricingRule
	for rows.Next() {
		var p model.PricingRule
		if err := rows.Scan(&p.ID, &p.FacilityID, &p.DayType, &p.StartTime, &p.EndTime,
			&p.PricePerHour, &p.IsActive, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		rules = append(rules, p)
	}
	return rules, rows.Err()
}

// FindByID returns a pricing rule by identifier.
func (r *PricingRuleRepo) FindByID(ctx context.Context, id uuid.UUID) (*model.PricingRule, error) {
	return scanPricingRule(r.db.Pool().QueryRow(ctx,
		`SELECT `+pricingRuleColumns+` FROM pricing_rules WHERE id = $1`, id))
}

// Create inserts a pricing rule.
func (r *PricingRuleRepo) Create(ctx context.Context, facilityID uuid.UUID, dayType model.DayType, start, end model.Clock, pricePerHour int64) (*model.PricingRule, error) {
	return scanPricingRule(r.db.Pool().QueryRow(ctx, `
		INSERT INTO pricing_rules (facility_id, day_type, start_time, end_time, price_per_hour)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING `+pricingRuleColumns,
		facilityID, dayType, start, end, pricePerHour))
}

// PricingRuleUpdate carries the mutable fields of a pricing rule.
type PricingRuleUpdate struct {
	DayType      *model.DayType
	StartTime    *model.Clock
	EndTime      *model.Clock
	PricePerHour *int64
	IsActive     *bool
}

// Update applies a partial update to a pricing rule.
func (r *PricingRuleRepo) Update(ctx context.Context, id uuid.UUID, in PricingRuleUpdate) (*model.PricingRule, error) {
	return scanPricingRule(r.db.Pool().QueryRow(ctx, `
		UPDATE pricing_rules SET
		    day_type       = COALESCE($2, day_type),
		    start_time     = COALESCE($3, start_time),
		    end_time       = COALESCE($4, end_time),
		    price_per_hour = COALESCE($5, price_per_hour),
		    is_active      = COALESCE($6, is_active)
		WHERE id = $1
		RETURNING `+pricingRuleColumns,
		id, in.DayType, in.StartTime, in.EndTime, in.PricePerHour, in.IsActive))
}

// Delete removes a pricing rule.
func (r *PricingRuleRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Pool().Exec(ctx, `DELETE FROM pricing_rules WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("pricing rule not found")
	}
	return nil
}
