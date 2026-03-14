package domain

import (
	"context"
	"time"

	"github.com/korjavin/dutyassistant/internal/store"
)

// StoreAdapter adapts store.Store to domain.Repository
type StoreAdapter struct {
	store store.Store
}

func NewStoreAdapter(s store.Store) *StoreAdapter {
	return &StoreAdapter{store: s}
}

// Map from store models to domain models here if needed.

func (a *StoreAdapter) GetUserByTelegramID(ctx context.Context, id int64) (*User, error) {
	u, err := a.store.GetUserByTelegramID(ctx, id)
	if err != nil || u == nil {
		return nil, err
	}
	return (*User)(u), nil
}

func (a *StoreAdapter) GetUserByName(ctx context.Context, name string) (*User, error) {
	u, err := a.store.GetUserByName(ctx, name)
	if err != nil || u == nil {
		return nil, err
	}
	return (*User)(u), nil
}

func (a *StoreAdapter) ListActiveUsers(ctx context.Context) ([]*User, error) {
	users, err := a.store.ListActiveUsers(ctx)
	if err != nil {
		return nil, err
	}
	res := make([]*User, len(users))
	for i, u := range users {
		res[i] = (*User)(u)
	}
	return res, nil
}

func (a *StoreAdapter) ListAllUsers(ctx context.Context) ([]*User, error) {
	users, err := a.store.ListAllUsers(ctx)
	if err != nil {
		return nil, err
	}
	res := make([]*User, len(users))
	for i, u := range users {
		res[i] = (*User)(u)
	}
	return res, nil
}

func (a *StoreAdapter) CreateUser(ctx context.Context, user *User) error {
	return a.store.CreateUser(ctx, (*store.User)(user))
}

func (a *StoreAdapter) UpdateUser(ctx context.Context, user *User) error {
	return a.store.UpdateUser(ctx, (*store.User)(user))
}

func (a *StoreAdapter) GetUserStats(ctx context.Context, userID int64) (*UserStats, error) {
	stats, err := a.store.GetUserStats(ctx, userID)
	if err != nil || stats == nil {
		return nil, err
	}
	return (*UserStats)(stats), nil
}

func (a *StoreAdapter) CreateDuty(ctx context.Context, duty *Duty) error {
	d := &store.Duty{
		ID:             duty.ID,
		UserID:         duty.UserID,
		DutyDate:       duty.DutyDate,
		AssignmentType: store.AssignmentType(duty.AssignmentType),
		CreatedAt:      duty.CreatedAt,
		CompletedAt:    duty.CompletedAt,
		User:           (*store.User)(duty.User),
	}
	return a.store.CreateDuty(ctx, d)
}

func (a *StoreAdapter) GetDutyByDate(ctx context.Context, date time.Time) (*Duty, error) {
	d, err := a.store.GetDutyByDate(ctx, date)
	if err != nil || d == nil {
		return nil, err
	}
	return &Duty{
		ID:             d.ID,
		UserID:         d.UserID,
		DutyDate:       d.DutyDate,
		AssignmentType: AssignmentType(d.AssignmentType),
		CreatedAt:      d.CreatedAt,
		CompletedAt:    d.CompletedAt,
		User:           (*User)(d.User),
	}, nil
}

func (a *StoreAdapter) UpdateDuty(ctx context.Context, duty *Duty) error {
	d := &store.Duty{
		ID:             duty.ID,
		UserID:         duty.UserID,
		DutyDate:       duty.DutyDate,
		AssignmentType: store.AssignmentType(duty.AssignmentType),
		CreatedAt:      duty.CreatedAt,
		CompletedAt:    duty.CompletedAt,
		User:           (*store.User)(duty.User),
	}
	return a.store.UpdateDuty(ctx, d)
}

func (a *StoreAdapter) DeleteDuty(ctx context.Context, date time.Time) error {
	return a.store.DeleteDuty(ctx, date)
}

func (a *StoreAdapter) GetDutiesByMonth(ctx context.Context, year int, month time.Month) ([]*Duty, error) {
	duties, err := a.store.GetDutiesByMonth(ctx, year, month)
	if err != nil {
		return nil, err
	}
	res := make([]*Duty, len(duties))
	for i, d := range duties {
		res[i] = &Duty{
			ID:             d.ID,
			UserID:         d.UserID,
			DutyDate:       d.DutyDate,
			AssignmentType: AssignmentType(d.AssignmentType),
			CreatedAt:      d.CreatedAt,
			CompletedAt:    d.CompletedAt,
			User:           (*User)(d.User),
		}
	}
	return res, nil
}

func (a *StoreAdapter) CompleteDuty(ctx context.Context, date time.Time) error {
	return a.store.CompleteDuty(ctx, date)
}

func (a *StoreAdapter) GetTodaysDuty(ctx context.Context) (*Duty, error) {
	d, err := a.store.GetTodaysDuty(ctx)
	if err != nil || d == nil {
		return nil, err
	}
	return &Duty{
		ID:             d.ID,
		UserID:         d.UserID,
		DutyDate:       d.DutyDate,
		AssignmentType: AssignmentType(d.AssignmentType),
		CreatedAt:      d.CreatedAt,
		CompletedAt:    d.CompletedAt,
		User:           (*User)(d.User),
	}, nil
}

func (a *StoreAdapter) GetCompletedDutiesInRange(ctx context.Context, start, end time.Time) ([]*Duty, error) {
	duties, err := a.store.GetCompletedDutiesInRange(ctx, start, end)
	if err != nil {
		return nil, err
	}
	res := make([]*Duty, len(duties))
	for i, d := range duties {
		res[i] = &Duty{
			ID:             d.ID,
			UserID:         d.UserID,
			DutyDate:       d.DutyDate,
			AssignmentType: AssignmentType(d.AssignmentType),
			CreatedAt:      d.CreatedAt,
			CompletedAt:    d.CompletedAt,
			User:           (*User)(d.User),
		}
	}
	return res, nil
}

func (a *StoreAdapter) GetLastDuty(ctx context.Context) (*Duty, error) {
	d, err := a.store.GetLastDuty(ctx)
	if err != nil || d == nil {
		return nil, err
	}
	return &Duty{
		ID:             d.ID,
		UserID:         d.UserID,
		DutyDate:       d.DutyDate,
		AssignmentType: AssignmentType(d.AssignmentType),
		CreatedAt:      d.CreatedAt,
		CompletedAt:    d.CompletedAt,
		User:           (*User)(d.User),
	}, nil
}

func mapStoreChoreToDomain(c *store.Chore) *Chore {
	if c == nil {
		return nil
	}
	return &Chore{
		ID:          c.ID,
		UserID:      c.UserID,
		Description: c.Description,
		AssignedAt:  c.AssignedAt,
		DeadlineAt:  c.DeadlineAt,
		CompletedAt: c.CompletedAt,
		CancelledAt: c.CancelledAt,
		ReminderID:  c.ReminderID,
		User:        (*User)(c.User),
	}
}

func mapDomainChoreToStore(c *Chore) *store.Chore {
	if c == nil {
		return nil
	}
	return &store.Chore{
		ID:          c.ID,
		UserID:      c.UserID,
		Description: c.Description,
		AssignedAt:  c.AssignedAt,
		DeadlineAt:  c.DeadlineAt,
		CompletedAt: c.CompletedAt,
		CancelledAt: c.CancelledAt,
		ReminderID:  c.ReminderID,
		User:        (*store.User)(c.User),
	}
}

func (a *StoreAdapter) CreateChore(ctx context.Context, chore *Chore) error {
	return a.store.CreateChore(ctx, mapDomainChoreToStore(chore))
}

func (a *StoreAdapter) GetChoreByReminderID(ctx context.Context, reminderID string) (*Chore, error) {
	c, err := a.store.GetChoreByReminderID(ctx, reminderID)
	if err != nil || c == nil {
		return nil, err
	}
	return mapStoreChoreToDomain(c), nil
}

func (a *StoreAdapter) GetActiveChores(ctx context.Context) ([]*Chore, error) {
	chores, err := a.store.GetActiveChores(ctx)
	if err != nil {
		return nil, err
	}
	res := make([]*Chore, len(chores))
	for i, c := range chores {
		res[i] = mapStoreChoreToDomain(c)
	}
	return res, nil
}

func (a *StoreAdapter) GetActiveChoresByUserID(ctx context.Context, userID int64) ([]*Chore, error) {
	chores, err := a.store.GetActiveChoresByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	res := make([]*Chore, len(chores))
	for i, c := range chores {
		res[i] = mapStoreChoreToDomain(c)
	}
	return res, nil
}

func (a *StoreAdapter) GetOverdueChores(ctx context.Context) ([]*Chore, error) {
	chores, err := a.store.GetOverdueChores(ctx)
	if err != nil {
		return nil, err
	}
	res := make([]*Chore, len(chores))
	for i, c := range chores {
		res[i] = mapStoreChoreToDomain(c)
	}
	return res, nil
}

func (a *StoreAdapter) CompleteChoreByReminderID(ctx context.Context, reminderID string) error {
	return a.store.CompleteChoreByReminderID(ctx, reminderID)
}

func (a *StoreAdapter) CancelChore(ctx context.Context, id int64) (*Chore, error) {
	c, err := a.store.CancelChore(ctx, id)
	if err != nil || c == nil {
		return nil, err
	}
	return mapStoreChoreToDomain(c), nil
}

func (a *StoreAdapter) ListActiveChores(ctx context.Context) ([]*Chore, error) {
	chores, err := a.store.ListActiveChores(ctx)
	if err != nil {
		return nil, err
	}
	res := make([]*Chore, len(chores))
	for i, c := range chores {
		res[i] = mapStoreChoreToDomain(c)
	}
	return res, nil
}

func (a *StoreAdapter) GetTopOverdueChores(ctx context.Context, limit int) ([]*ChoreStat, error) {
	stats, err := a.store.GetTopOverdueChores(ctx, limit)
	if err != nil {
		return nil, err
	}
	res := make([]*ChoreStat, len(stats))
	for i, s := range stats {
		res[i] = (*ChoreStat)(s)
	}
	return res, nil
}

func (a *StoreAdapter) GetTopCompletedChoresUsers(ctx context.Context, limit int) ([]*UserChoreStat, error) {
	stats, err := a.store.GetTopCompletedChoresUsers(ctx, limit)
	if err != nil {
		return nil, err
	}
	res := make([]*UserChoreStat, len(stats))
	for i, s := range stats {
		res[i] = (*UserChoreStat)(s)
	}
	return res, nil
}

func (a *StoreAdapter) GetUserWeeklyStats(ctx context.Context, since time.Time) ([]*UserWeeklyStats, error) {
	stats, err := a.store.GetUserWeeklyStats(ctx, since)
	if err != nil {
		return nil, err
	}
	res := make([]*UserWeeklyStats, len(stats))
	for i, s := range stats {
		res[i] = (*UserWeeklyStats)(s)
	}
	return res, nil
}

func (a *StoreAdapter) GetLastChoreDigestDate(ctx context.Context) (string, error) {
	return a.store.GetLastChoreDigestDate(ctx)
}

func (a *StoreAdapter) SetLastChoreDigestDate(ctx context.Context, date string) error {
	return a.store.SetLastChoreDigestDate(ctx, date)
}

func (a *StoreAdapter) SaveDailyParticipantRatings(ctx context.Context, date time.Time, ratings []*ParticipantDailyRating) error {
	r := make([]*store.ParticipantDailyRating, len(ratings))
	for i, rat := range ratings {
		r[i] = (*store.ParticipantDailyRating)(rat)
	}
	return a.store.SaveDailyParticipantRatings(ctx, date, r)
}

func (a *StoreAdapter) GetParticipantsForRating(ctx context.Context) ([]*User, error) {
	users, err := a.store.GetParticipantsForRating(ctx)
	if err != nil {
		return nil, err
	}
	res := make([]*User, len(users))
	for i, u := range users {
		res[i] = (*User)(u)
	}
	return res, nil
}

func (a *StoreAdapter) GetCurrentMonthParticipantRatings(ctx context.Context, now time.Time) ([]*ParticipantDailyRating, error) {
	ratings, err := a.store.GetCurrentMonthParticipantRatings(ctx, now)
	if err != nil {
		return nil, err
	}
	res := make([]*ParticipantDailyRating, len(ratings))
	for i, r := range ratings {
		res[i] = (*ParticipantDailyRating)(r)
	}
	return res, nil
}

func (a *StoreAdapter) GetMonthlyParticipantTotals(ctx context.Context, year int, month time.Month) ([]*ParticipantMonthlyTotal, error) {
	totals, err := a.store.GetMonthlyParticipantTotals(ctx, year, month)
	if err != nil {
		return nil, err
	}
	res := make([]*ParticipantMonthlyTotal, len(totals))
	for i, t := range totals {
		res[i] = (*ParticipantMonthlyTotal)(t)
	}
	return res, nil
}

func (a *StoreAdapter) AddToVolunteerQueue(ctx context.Context, userID int64, days int) error {
	return a.store.AddToVolunteerQueue(ctx, userID, days)
}

func (a *StoreAdapter) AddToAdminQueue(ctx context.Context, userID int64, days int) error {
	return a.store.AddToAdminQueue(ctx, userID, days)
}

func (a *StoreAdapter) DecrementVolunteerQueue(ctx context.Context, userID int64) error {
	return a.store.DecrementVolunteerQueue(ctx, userID)
}

func (a *StoreAdapter) DecrementAdminQueue(ctx context.Context, userID int64) error {
	return a.store.DecrementAdminQueue(ctx, userID)
}

func (a *StoreAdapter) ReduceAdminQueue(ctx context.Context, userID int64, days int) error {
	return a.store.ReduceAdminQueue(ctx, userID, days)
}

func (a *StoreAdapter) GetUsersWithVolunteerQueue(ctx context.Context) ([]*User, error) {
	users, err := a.store.GetUsersWithVolunteerQueue(ctx)
	if err != nil {
		return nil, err
	}
	res := make([]*User, len(users))
	for i, u := range users {
		res[i] = (*User)(u)
	}
	return res, nil
}

func (a *StoreAdapter) GetUsersWithAdminQueue(ctx context.Context) ([]*User, error) {
	users, err := a.store.GetUsersWithAdminQueue(ctx)
	if err != nil {
		return nil, err
	}
	res := make([]*User, len(users))
	for i, u := range users {
		res[i] = (*User)(u)
	}
	return res, nil
}

func (a *StoreAdapter) SetOffDuty(ctx context.Context, userID int64, start, end time.Time) error {
	return a.store.SetOffDuty(ctx, userID, start, end)
}

func (a *StoreAdapter) ClearOffDuty(ctx context.Context, userID int64) error {
	return a.store.ClearOffDuty(ctx, userID)
}

func (a *StoreAdapter) IsUserOffDuty(ctx context.Context, userID int64, date time.Time) (bool, error) {
	return a.store.IsUserOffDuty(ctx, userID, date)
}

func (a *StoreAdapter) GetOffDutyUsers(ctx context.Context, date time.Time) ([]*User, error) {
	users, err := a.store.GetOffDutyUsers(ctx, date)
	if err != nil {
		return nil, err
	}
	res := make([]*User, len(users))
	for i, u := range users {
		res[i] = (*User)(u)
	}
	return res, nil
}

func (a *StoreAdapter) SetVacationMode(ctx context.Context, enabled bool) error {
	return a.store.SetVacationMode(ctx, enabled)
}

func (a *StoreAdapter) IsVacationMode(ctx context.Context) (bool, error) {
	return a.store.IsVacationMode(ctx)
}

func (a *StoreAdapter) CreateRecurringChore(ctx context.Context, chore *RecurringChore) error {
	return a.store.CreateRecurringChore(ctx, (*store.RecurringChore)(chore))
}

func (a *StoreAdapter) GetRecurringChore(ctx context.Context, id int64) (*RecurringChore, error) {
	c, err := a.store.GetRecurringChore(ctx, id)
	if err != nil || c == nil {
		return nil, err
	}
	return (*RecurringChore)(c), nil
}

func (a *StoreAdapter) GetActiveRecurringChores(ctx context.Context) ([]*RecurringChore, error) {
	chores, err := a.store.GetActiveRecurringChores(ctx)
	if err != nil {
		return nil, err
	}
	res := make([]*RecurringChore, len(chores))
	for i, c := range chores {
		res[i] = (*RecurringChore)(c)
	}
	return res, nil
}

func (a *StoreAdapter) GetDueRecurringChores(ctx context.Context, before time.Time) ([]*RecurringChore, error) {
	chores, err := a.store.GetDueRecurringChores(ctx, before)
	if err != nil {
		return nil, err
	}
	res := make([]*RecurringChore, len(chores))
	for i, c := range chores {
		res[i] = (*RecurringChore)(c)
	}
	return res, nil
}

func (a *StoreAdapter) UpdateRecurringChoreNextRun(ctx context.Context, id int64, nextRun time.Time) error {
	return a.store.UpdateRecurringChoreNextRun(ctx, id, nextRun)
}

func (a *StoreAdapter) UpdateRecurringChoreDescription(ctx context.Context, id int64, description string) error {
	return a.store.UpdateRecurringChoreDescription(ctx, id, description)
}

func (a *StoreAdapter) CancelRecurringChore(ctx context.Context, id int64) error {
	return a.store.CancelRecurringChore(ctx, id)
}
