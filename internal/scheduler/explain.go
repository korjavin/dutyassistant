package scheduler

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"

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

	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Последнее назначение: @%s (%s)\n", lastDuty.User.FirstName, lastDuty.DutyDate.Format("2006-01-02 15:00")))

	// Try to reconstruct the context of the decision for that day.
	date := lastDuty.DutyDate

	// Re-fetch all available users to determine who was considered
	allUsers, err := s.store.ListActiveUsers(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get active users: %w", err)
	}

	offDutyUsers, err := s.store.GetOffDutyUsers(ctx, date)
	if err != nil {
		return "", fmt.Errorf("failed to get off duty users: %w", err)
	}
	offDutyMap := make(map[int64]bool)
	for _, u := range offDutyUsers {
		offDutyMap[u.ID] = true
	}

	// Calculate considered candidates and exclusions
	var candidates []string
	var exclusions []string

	// Calculate 14 day window for round-robin
	start := date.AddDate(0, 0, -14)
	duties, err := s.store.GetCompletedDutiesInRange(ctx, start, date)
	if err != nil {
		return "", fmt.Errorf("failed to get completed duties: %w", err)
	}

	dutyCounts := make(map[int64]int)
	for _, duty := range duties {
		if duty.AssignmentType != store.AssignmentTypeAdmin {
			dutyCounts[duty.UserID]++
		}
	}

	// Handle max queues for voluntary/admin assignments where decrements occurred
	maxQueueCount := 0
	if lastDuty.AssignmentType == store.AssignmentTypeVoluntary || lastDuty.AssignmentType == store.AssignmentTypeAdmin {
		for _, u := range allUsers {
			if offDutyMap[u.ID] {
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
			if !offDutyMap[u.ID] {
				if count := dutyCounts[u.ID]; count < minCount {
					minCount = count
				}
			}
		}
	}

	for _, user := range allUsers {
		candidates = append(candidates, fmt.Sprintf("@%s", user.FirstName))

		offDuty := offDutyMap[user.ID]

		if offDuty {
			exclusions = append(exclusions, fmt.Sprintf("@%s — отсутствует по расписанию", user.FirstName))
		} else if lastDuty.AssignmentType == store.AssignmentTypeRoundRobin {
			if count := dutyCounts[user.ID]; count > minCount {
				exclusions = append(exclusions, fmt.Sprintf("@%s — %d дежурств за последние 14 дней (минимум %d)", user.FirstName, count, minCount))
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
			}
		}
	}

	sort.Strings(candidates)
	buf.WriteString(fmt.Sprintf("Кандидаты: %s\n", strings.Join(candidates, ", ")))

	if len(exclusions) > 0 {
		buf.WriteString("Исключены:\n")
		for _, exc := range exclusions {
			buf.WriteString(fmt.Sprintf("%s\n", exc))
		}
	}

	// Remaining candidates
	var remainingCandidates []string
	for _, user := range allUsers {
		if offDutyMap[user.ID] {
			continue
		}
		if lastDuty.AssignmentType == store.AssignmentTypeRoundRobin {
			if dutyCounts[user.ID] == minCount {
				remainingCandidates = append(remainingCandidates, fmt.Sprintf("@%s", user.FirstName))
			}
		} else if lastDuty.AssignmentType == store.AssignmentTypeVoluntary || lastDuty.AssignmentType == store.AssignmentTypeAdmin {
			queue := 0
			if lastDuty.AssignmentType == store.AssignmentTypeVoluntary {
				queue = user.VolunteerQueueDays
			} else {
				queue = user.AdminQueueDays
			}

			if user.ID == lastDuty.UserID {
				queue++
			}

			if queue == maxQueueCount && queue > 0 {
				remainingCandidates = append(remainingCandidates, fmt.Sprintf("@%s", user.FirstName))
			}
		}
	}

	if len(remainingCandidates) > 0 {
		sort.Strings(remainingCandidates)
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

	return buf.String(), nil
}
