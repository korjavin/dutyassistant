package fsm

const (
	StateInit State = "INIT"

	// Chore Flow States
	StateChoreDesc     State = "CHORE_DESC"
	StateChoreDuration State = "CHORE_DURATION"
	StateChoreConfirm  State = "CHORE_CONFIRM"

	// Daily Ratings Flow States
	StateRatingPrompt  State = "RATING_PROMPT"
	StateRatingConfirm State = "RATING_CONFIRM"
)

const (
	EventStartChore    Event = "START_CHORE"
	EventInputDesc     Event = "INPUT_DESC"
	EventInputDuration Event = "INPUT_DURATION"
	EventConfirmChore  Event = "CONFIRM_CHORE"
	EventCancel        Event = "CANCEL"

	EventStartRating   Event = "START_RATING"
	EventInputScore    Event = "INPUT_SCORE"
	EventConfirmRating Event = "CONFIRM_RATING"
)

// InitializeChoreFlow sets up the state transitions for the chore creation flow.
func InitializeChoreFlow() *FSM {
	machine := NewFSM(StateInit)

	machine.AddTransition(StateInit, EventStartChore, StateChoreDesc)
	machine.AddTransition(StateChoreDesc, EventInputDesc, StateChoreDuration)
	machine.AddTransition(StateChoreDuration, EventInputDuration, StateChoreConfirm)
	machine.AddTransition(StateChoreConfirm, EventConfirmChore, StateInit)

	// Global cancel
	machine.AddTransition(StateChoreDesc, EventCancel, StateInit)
	machine.AddTransition(StateChoreDuration, EventCancel, StateInit)
	machine.AddTransition(StateChoreConfirm, EventCancel, StateInit)

	return machine
}

// InitializeDailyRatingsFlow sets up the state transitions for the daily ratings flow.
func InitializeDailyRatingsFlow() *FSM {
	machine := NewFSM(StateInit)

	machine.AddTransition(StateInit, EventStartRating, StateRatingPrompt)
	machine.AddTransition(StateRatingPrompt, EventInputScore, StateRatingConfirm)
	machine.AddTransition(StateRatingConfirm, EventConfirmRating, StateInit)

	// Global cancel
	machine.AddTransition(StateRatingPrompt, EventCancel, StateInit)
	machine.AddTransition(StateRatingConfirm, EventCancel, StateInit)

	return machine
}
