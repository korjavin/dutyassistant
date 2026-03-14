package fsm

import "context"

type State string

type Event string

type Transition struct {
	From  State
	Event Event
	To    State
}

type Action func(ctx context.Context, data interface{}) error

type FSM struct {
	currentState State
	transitions  map[State]map[Event]State
	actions      map[State]Action
}

func NewFSM(initial State) *FSM {
	return &FSM{
		currentState: initial,
		transitions:  make(map[State]map[Event]State),
		actions:      make(map[State]Action),
	}
}

func (f *FSM) AddTransition(from State, event Event, to State) {
	if _, ok := f.transitions[from]; !ok {
		f.transitions[from] = make(map[Event]State)
	}
	f.transitions[from][event] = to
}

func (f *FSM) AddAction(state State, action Action) {
	f.actions[state] = action
}

func (f *FSM) ProcessEvent(ctx context.Context, event Event, data interface{}) error {
	if nextState, ok := f.transitions[f.currentState][event]; ok {
		f.currentState = nextState
		if action, ok := f.actions[nextState]; ok {
			return action(ctx, data)
		}
	}
	return nil
}

func (f *FSM) CurrentState() State {
	return f.currentState
}
