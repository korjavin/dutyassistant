package scheduler

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/korjavin/dutyassistant/internal/store"
)

// formatLoad renders a weighted duty load (scaled ×10) as a decimal day count,
// omitting the fractional part when the value is a whole number of days.
func formatLoad(load int) string {
	whole := load / 10
	frac := load % 10
	if frac == 0 {
		return fmt.Sprintf("%d", whole)
	}
	return fmt.Sprintf("%d.%d", whole, frac)
}

// ExplainLastAssignment implements the SchedulerInterface by explaining the last duty assignment.
func (s *Scheduler) ExplainLastAssignment(ctx context.Context) (string, error) {
	lastDuty, err := s.store.GetLastDuty(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get last duty: %w", err)
	}
	if lastDuty == nil || lastDuty.User == nil {
		return "Нет данных о последних назначениях.", nil
	}

	date := lastDuty.DutyDate
	allUsers, err := s.store.ListActiveUsers(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get active users: %w", err)
	}

	dutyCounts, err := s.getDutyCounts(ctx, date)
	if err != nil {
		return "", fmt.Errorf("failed to get duty counts: %w", err)
	}

	offDutyStatus, err := s.getOffDutyStatuses(ctx, allUsers, date)
	if err != nil {
		return "", fmt.Errorf("failed to check off duty status: %w", err)
	}

	minLoad, maxQueueCount := s.getAssignmentThresholds(lastDuty, allUsers, dutyCounts, offDutyStatus)
	candidates, exclusions, remainingCandidates := s.categorizeUsers(lastDuty, allUsers, dutyCounts, offDutyStatus, minLoad, maxQueueCount)

	return formatExplanation(lastDuty, candidates, exclusions, remainingCandidates), nil
}

// formatExplanation builds the explanation string from the given data.
func formatExplanation(lastDuty *store.Duty, candidates, exclusions, remainingCandidates []string) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Последнее назначение: @%s (%s)\n", lastDuty.User.FirstName, lastDuty.DutyDate.Format("2006-01-02 15:00"))
	fmt.Fprintf(&buf, "Кандидаты: %s\n", strings.Join(candidates, ", "))

	if len(exclusions) > 0 {
		buf.WriteString("Исключены:\n")
		for _, exc := range exclusions {
			fmt.Fprintf(&buf, "%s\n", exc)
		}
	}

	if len(remainingCandidates) > 0 {
		fmt.Fprintf(&buf, "Оставшиеся кандидаты: %s\n", strings.Join(remainingCandidates, ", "))
	}

	var finalReason string
	switch lastDuty.AssignmentType {
	case store.AssignmentTypeVoluntary:
		finalReason = "доброволец с наибольшим количеством дней (tie-break случайный при равенстве)."
	case store.AssignmentTypeAdmin:
		finalReason = "назначен администратором с наибольшим количеством дней в очереди (tie-break случайный при равенстве)."
	case store.AssignmentTypeRoundRobin:
		finalReason = "имел наименьшую нагрузку дежурств за год (добровольные дни × 1.2, админ-назначения не учитываются; tie-break случайный при равенстве)."
	}

	fmt.Fprintf(&buf, "Итог: назначен @%s, так как %s", lastDuty.User.FirstName, finalReason)

	return buf.String()
}

// getDutyCounts returns the weighted round-robin load per user over the last
// year preceding the given date. Voluntary days weigh 1.2× round-robin days
// (scaled ×10 for integer math) and admin-assigned days are excluded.
func (s *Scheduler) getDutyCounts(ctx context.Context, date time.Time) (map[int64]int, error) {
	start := date.AddDate(0, 0, -roundRobinLookbackDays)
	duties, err := s.store.GetCompletedDutiesInRange(ctx, start, date)
	if err != nil {
		return nil, err
	}

	dutyLoad := make(map[int64]int)
	for _, duty := range duties {
		dutyLoad[duty.UserID] += dutyLoadWeight(duty.AssignmentType)
	}
	return dutyLoad, nil
}

// getOffDutyStatuses returns a map of user IDs to their off-duty status on the given date.
func (s *Scheduler) getOffDutyStatuses(ctx context.Context, users []*store.User, date time.Time) (map[int64]bool, error) {
	offDutyUsers, err := s.store.GetOffDutyUsers(ctx, date)
	if err != nil {
		return nil, err
	}

	offDutyStatus := make(map[int64]bool, len(users))
	for _, u := range users {
		offDutyStatus[u.ID] = false
	}
	for _, u := range offDutyUsers {
		offDutyStatus[u.ID] = true
	}

	return offDutyStatus, nil
}

// getAssignmentThresholds calculates minLoad (scaled weighted duty load) for
// round-robin and maxQueueCount for voluntary/admin assignments.
func (s *Scheduler) getAssignmentThresholds(lastDuty *store.Duty, allUsers []*store.User, dutyLoad map[int64]int, offDutyStatus map[int64]bool) (int, int) {
	maxQueueCount := 0
	if lastDuty.AssignmentType == store.AssignmentTypeVoluntary || lastDuty.AssignmentType == store.AssignmentTypeAdmin {
		for _, u := range allUsers {
			if offDutyStatus[u.ID] {
				continue
			}

			queue := 0
			if lastDuty.AssignmentType == store.AssignmentTypeVoluntary {
				queue = u.VolunteerQueueDays
			} else {
				queue = u.AdminQueueDays
			}

			// Add 1 for the assigned user, since it was decremented
			if u.ID == lastDuty.UserID {
				queue++
			}

			if queue > maxQueueCount {
				maxQueueCount = queue
			}
		}
	}

	minLoad := int(^uint(0) >> 1)
	if lastDuty.AssignmentType == store.AssignmentTypeRoundRobin {
		for _, u := range allUsers {
			if !offDutyStatus[u.ID] {
				if load := dutyLoad[u.ID]; load < minLoad {
					minLoad = load
				}
			}
		}
	}
	return minLoad, maxQueueCount
}

// categorizeUsers builds candidates, exclusions, and remainingCandidates lists.
func (s *Scheduler) categorizeUsers(
	lastDuty *store.Duty,
	allUsers []*store.User,
	dutyLoad map[int64]int,
	offDutyStatus map[int64]bool,
	minLoad, maxQueueCount int,
) ([]string, []string, []string) {
	var candidates []string
	var exclusions []string
	var remainingCandidates []string

	for _, user := range allUsers {
		candidates = append(candidates, fmt.Sprintf("@%s", user.FirstName))

		offDuty := offDutyStatus[user.ID]

		if offDuty {
			exclusions = append(exclusions, fmt.Sprintf("@%s — отсутствует по расписанию", user.FirstName))
			continue
		}

		isRemaining := false
		if lastDuty.AssignmentType == store.AssignmentTypeRoundRobin {
			load := dutyLoad[user.ID]
			if load > minLoad {
				exclusions = append(exclusions, fmt.Sprintf("@%s — нагрузка %s за год (минимум %s)", user.FirstName, formatLoad(load), formatLoad(minLoad)))
			} else {
				isRemaining = true
			}
		} else if lastDuty.AssignmentType == store.AssignmentTypeVoluntary || lastDuty.AssignmentType == store.AssignmentTypeAdmin {
			queue := 0
			queueType := "admin"
			if lastDuty.AssignmentType == store.AssignmentTypeVoluntary {
				queue = user.VolunteerQueueDays
				queueType = "volunteer"
			} else {
				queue = user.AdminQueueDays
			}

			if user.ID == lastDuty.UserID {
				queue++
			}

			if queue < maxQueueCount {
				if queue == 0 {
					exclusions = append(exclusions, fmt.Sprintf("@%s — нет дней в очереди %s", user.FirstName, queueType))
				} else {
					exclusions = append(exclusions, fmt.Sprintf("@%s — %d дней в очереди %s (максимум %d)", user.FirstName, queue, queueType, maxQueueCount))
				}
			} else if queue == maxQueueCount && queue > 0 {
				isRemaining = true
			}
		}

		if isRemaining {
			remainingCandidates = append(remainingCandidates, fmt.Sprintf("@%s", user.FirstName))
		}
	}

	sort.Strings(candidates)
	sort.Strings(remainingCandidates)

	return candidates, exclusions, remainingCandidates
}
