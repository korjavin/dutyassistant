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

	minCount, maxQueueCount := s.getAssignmentThresholds(lastDuty, allUsers, dutyCounts, offDutyStatus)
	candidates, exclusions, remainingCandidates := s.categorizeUsers(lastDuty, allUsers, dutyCounts, offDutyStatus, minCount, maxQueueCount)

	return buildExplanationMessage(lastDuty, date, candidates, exclusions, remainingCandidates), nil
}

// buildExplanationMessage constructs the final explanation string for the last assignment.
func buildExplanationMessage(lastDuty *store.Duty, date time.Time, candidates, exclusions, remainingCandidates []string) string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Последнее назначение: @%s (%s)\n", lastDuty.User.FirstName, date.Format("2006-01-02 15:00")))
	buf.WriteString(fmt.Sprintf("Кандидаты: %s\n", strings.Join(candidates, ", ")))

	if len(exclusions) > 0 {
		buf.WriteString("Исключены:\n")
		for _, exc := range exclusions {
			buf.WriteString(fmt.Sprintf("%s\n", exc))
		}
	}

	if len(remainingCandidates) > 0 {
		buf.WriteString(fmt.Sprintf("Оставшиеся кандидаты: %s\n", strings.Join(remainingCandidates, ", ")))
	}

	var finalReason string
	switch lastDuty.AssignmentType {
	case store.AssignmentTypeVoluntary:
		finalReason = "доброволец с наибольшим количеством дней (tie-break случайный при равенстве)."
	case store.AssignmentTypeAdmin:
		finalReason = "назначен администратором с наибольшим количеством дней в очереди (tie-break случайный при равенстве)."
	case store.AssignmentTypeRoundRobin:
		finalReason = "имел наименьшее число дежурств за 14 дней (tie-break случайный при равенстве)."
	}

	buf.WriteString(fmt.Sprintf("Итог: назначен @%s, так как %s", lastDuty.User.FirstName, finalReason))

	return buf.String()
}

// getDutyCounts returns duty counts for each user in the 14 days preceding the given date.
func (s *Scheduler) getDutyCounts(ctx context.Context, date time.Time) (map[int64]int, error) {
	start := date.AddDate(0, 0, -14)
	duties, err := s.store.GetCompletedDutiesInRange(ctx, start, date)
	if err != nil {
		return nil, err
	}

	dutyCounts := make(map[int64]int)
	for _, duty := range duties {
		if duty.AssignmentType != store.AssignmentTypeAdmin {
			dutyCounts[duty.UserID]++
		}
	}
	return dutyCounts, nil
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

// getAssignmentThresholds calculates minCount for round-robin and maxQueueCount for voluntary/admin assignments.
func (s *Scheduler) getAssignmentThresholds(lastDuty *store.Duty, allUsers []*store.User, dutyCounts map[int64]int, offDutyStatus map[int64]bool) (int, int) {
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

	minCount := int(^uint(0) >> 1)
	if lastDuty.AssignmentType == store.AssignmentTypeRoundRobin {
		for _, u := range allUsers {
			if !offDutyStatus[u.ID] {
				if count := dutyCounts[u.ID]; count < minCount {
					minCount = count
				}
			}
		}
	}
	return minCount, maxQueueCount
}

// categorizeUsers builds candidates, exclusions, and remainingCandidates lists.
func (s *Scheduler) categorizeUsers(
	lastDuty *store.Duty,
	allUsers []*store.User,
	dutyCounts map[int64]int,
	offDutyStatus map[int64]bool,
	minCount, maxQueueCount int,
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
			count := dutyCounts[user.ID]
			if count > minCount {
				exclusions = append(exclusions, fmt.Sprintf("@%s — %d дежурств за последние 14 дней (минимум %d)", user.FirstName, count, minCount))
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
