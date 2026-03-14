package domain

import (
	"testing"
	"time"
)

func TestModelCreation(t *testing.T) {
	now := time.Now()

	user := User{
		ID:        1,
		FirstName: "Test",
	}

	duty := Duty{
		ID:       1,
		UserID:   user.ID,
		DutyDate: now,
		User:     &user,
	}

	if duty.User.FirstName != "Test" {
		t.Errorf("Expected user FirstName to be 'Test', got '%s'", duty.User.FirstName)
	}
}
