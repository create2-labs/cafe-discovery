package handler

import (
	"cafe-discovery/internal/domain"

	"github.com/google/uuid"
)

type handlerPlanUserRepo struct {
	user *domain.User
}

func (r *handlerPlanUserRepo) Create(*domain.User) error { return nil }
func (r *handlerPlanUserRepo) FindByID(string) (*domain.User, error) {
	return r.user, nil
}
func (r *handlerPlanUserRepo) FindByEmail(string) (*domain.User, error) { return nil, nil }
func (r *handlerPlanUserRepo) ExistsByEmail(string) (bool, error)       { return false, nil }

type handlerPlanPlanRepo struct {
	plan *domain.Plan
}

func (r *handlerPlanPlanRepo) Create(*domain.Plan) error { return nil }
func (r *handlerPlanPlanRepo) FindByID(uuid.UUID) (*domain.Plan, error) {
	return r.plan, nil
}
func (r *handlerPlanPlanRepo) FindByType(domain.PlanType) (*domain.Plan, error) { return nil, nil }
func (r *handlerPlanPlanRepo) FindAll() ([]*domain.Plan, error)                 { return nil, nil }
func (r *handlerPlanPlanRepo) FindActive() ([]*domain.Plan, error)              { return nil, nil }
