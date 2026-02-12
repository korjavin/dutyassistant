package handlers

import (
	"context"
	"fmt"
	"html"
	"log"
	"math/rand"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/store"
)

// HandleChore handles the /chore command for admins. Format: /chore [description]
// It assigns a random active user to the described chore.
func (h *Handlers) HandleChore(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	// 1. Admin check
	isAdmin, err := h.checkAdmin(m.From.ID)
	if err != nil || !isAdmin {
		return tgbotapi.NewMessage(m.Chat.ID, adminOnlyMessage), nil
	}

	args := strings.TrimSpace(m.CommandArguments())

	// 2. Argument validation
	if args == "" {
		msg := tgbotapi.NewMessage(m.Chat.ID, "To assign a chore, please provide a description.\n\nExample: <code>/chore Clean the coffee machine</code>")
		msg.ParseMode = tgbotapi.ModeHTML
		return msg, nil
	}

	description := args

	// Escape description to prevent HTML injection
	description = html.EscapeString(description)

	// 3. Get candidates
	users, err := h.Store.ListActiveUsers(context.Background())
	if err != nil {
		return tgbotapi.NewMessage(m.Chat.ID, "Failed to retrieve user list."), nil
	}
	if len(users) == 0 {
		return tgbotapi.NewMessage(m.Chat.ID, "No active users found to assign the chore to."), nil
	}

	// Filter candidates (exclude those on off-duty)
	var candidates []*store.User
	now := time.Now()
	// Using noon to check off-duty status as it's a daily check usually
	checkDate := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, now.Location())

	for _, u := range users {
		// Check if user is off-duty
		isOff, err := h.Store.IsUserOffDuty(context.Background(), u.ID, checkDate)
		if err != nil {
			log.Printf("Error checking off-duty status for user %d: %v", u.ID, err)
			// Assume not off-duty on error to be safe or skip? Skipping is safer for the user.
			continue
		}
		if !isOff {
			candidates = append(candidates, u)
		}
	}

	if len(candidates) == 0 {
		return tgbotapi.NewMessage(m.Chat.ID, "All active users are currently off-duty."), nil
	}

	// 4. Weighted random assignment
	// Weight = 1.0 + (AdminQueueDays * 0.02)
	// Additional +2% chance for every pending admin assigned day

	type weightedUser struct {
		user   *store.User
		weight float64
	}

	var weightedCandidates []weightedUser
	var totalWeight float64

	for _, u := range candidates {
		// Base weight
		weight := 1.0
		// Add 2% for each AdminQueueDay
		if u.AdminQueueDays > 0 {
			weight += float64(u.AdminQueueDays) * 0.02
		}

		weightedCandidates = append(weightedCandidates, weightedUser{user: u, weight: weight})
		totalWeight += weight
	}

	// Select user
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	target := r.Float64() * totalWeight

	var selectedUser *store.User
	currentWeight := 0.0
	for _, wu := range weightedCandidates {
		currentWeight += wu.weight
		if target <= currentWeight {
			selectedUser = wu.user
			break
		}
	}
	// Fallback (should not happen mathematically if totalWeight > 0)
	if selectedUser == nil && len(candidates) > 0 {
		// Just pick randomly
		selectedUser = candidates[r.Intn(len(candidates))]
	}

	if selectedUser == nil {
		return tgbotapi.NewMessage(m.Chat.ID, "Failed to select a user."), nil
	}

	// 5. Notifications

	// Message to the admin/caller
	escapedName := html.EscapeString(selectedUser.FirstName)
	responseMsg := tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("✅ Assigned chore to <b>%s</b>.", escapedName))
	responseMsg.ParseMode = tgbotapi.ModeHTML

	// Construct group announcement
	groupText := fmt.Sprintf("🎲 <b>Random Chore Assignment</b>\n\n🎯 <b>%s</b> has been assigned a chore:\n\n<i>%s</i>", escapedName, description)

	// Handle group announcement
	if h.GroupID != 0 {
		if m.Chat.ID != h.GroupID {
			// Triggered from DM, need to announce in group
			if h.Bot != nil {
				groupMsg := tgbotapi.NewMessage(h.GroupID, groupText)
				groupMsg.ParseMode = tgbotapi.ModeHTML
				if _, err := h.Bot.Send(groupMsg); err != nil {
					log.Printf("Failed to send chore announcement to group %d: %v", h.GroupID, err)
					responseMsg.Text += "\n\n⚠️ Failed to announce in group."
				} else {
					responseMsg.Text += "\n\n📢 Announced in group."
				}
			} else {
				responseMsg.Text += "\n\n⚠️ Bot API not available for group announcement."
			}
		} else {
			// Triggered in group, just return the announcement as response
			responseMsg.Text = groupText
		}
	} else {
		// No group configured
		responseMsg.Text = fmt.Sprintf("✅ Assigned chore to <b>%s</b>:\n%s", escapedName, description)
	}

	return responseMsg, nil
}
