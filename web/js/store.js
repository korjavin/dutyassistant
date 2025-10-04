// This module will manage the client-side state of the application.

// A simple object to hold the state.
const state = {
    currentUser: null,
    schedule: {}, // Caches schedule data, e.g., { '2024-10': [...] }
    users: [],
    currentYear: new Date().getFullYear(),
    currentMonth: new Date().getMonth() + 1,
};

// Functions to update and access the state will go here.
export function getState() {
    return state;
}

export function setSchedule(year, month, data) {
    state.schedule[`${year}-${month}`] = data;
}

// Add other state management functions as needed.