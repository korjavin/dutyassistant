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

	// Calculate considered candidates and exclusions
	var candidates []string
	var exclusions []string

	// Calculate 14 day window for round-robin
	start := date.AddDate(0, 0, -14)
	duties, _ := s.store.GetCompletedDutiesInRange(ctx, start, date)
	dutyCounts := make(map[int64]int)
	for _, duty := range duties {
		if duty.AssignmentType != store.AssignmentTypeAdmin {
			dutyCounts[duty.UserID]++
		}
	}

    minCount := int(^uint(0) >> 1)
    if lastDuty.AssignmentType == store.AssignmentTypeRoundRobin {
        for _, u := range allUsers {
            if offDuty, _ := s.store.IsUserOffDuty(ctx, u.ID, date); !offDuty {
                if count := dutyCounts[u.ID]; count < minCount {
                    minCount = count
                }
            }
        }
    }

	for _, user := range allUsers {
		candidates = append(candidates, fmt.Sprintf("@%s", user.FirstName))
		offDuty, _ := s.store.IsUserOffDuty(ctx, user.ID, date)

		if offDuty {
			exclusions = append(exclusions, fmt.Sprintf("@%s — отсутствует по расписанию", user.FirstName))
		} else if lastDuty.AssignmentType == store.AssignmentTypeRoundRobin {
			if count := dutyCounts[user.ID]; count > minCount {
				exclusions = append(exclusions, fmt.Sprintf("@%s — %d дежурств за последние 14 дней (минимум %d)", user.FirstName, count, minCount))
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
        offDuty, _ := s.store.IsUserOffDuty(ctx, user.ID, date)
        if offDuty {
            continue
        }
        if lastDuty.AssignmentType == store.AssignmentTypeRoundRobin {
            if dutyCounts[user.ID] == minCount {
                remainingCandidates = append(remainingCandidates, fmt.Sprintf("@%s", user.FirstName))
            }
        } else if lastDuty.AssignmentType == store.AssignmentTypeVoluntary {
             if user.VolunteerQueueDays > 0 {
                 remainingCandidates = append(remainingCandidates, fmt.Sprintf("@%s", user.FirstName))
             }
        } else if lastDuty.AssignmentType == store.AssignmentTypeAdmin {
             if user.AdminQueueDays > 0 {
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
        finalReason = "доброволец с наивысшим приоритетом (tie-break случайный при равенстве)."
    case store.AssignmentTypeAdmin:
        finalReason = "назначен администратором (tie-break случайный при равенстве)."
    case store.AssignmentTypeRoundRobin:
        finalReason = "имел наименьшее число дежурств за 14 дней (tie-break случайный при равенстве)."
    }

	buf.WriteString(fmt.Sprintf("Итог: назначен @%s, так как %s", lastDuty.User.FirstName, finalReason))

	return buf.String(), nil
}
