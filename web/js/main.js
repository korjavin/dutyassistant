import { initializeCalendar } from './ui/calendar.js';
import { displayQueueSummary } from './ui/queue.js';
import { displayPendingChores } from './ui/chores.js';
import { getUsers, getActiveChores } from './api.js';
import { setState } from './store.js';

// Main entry point for the frontend application.
console.log("Roster Bot frontend script loaded.");

function initializeApp() {
    console.log("DOM fully loaded and parsed.");

    // Initialize the Telegram Web App SDK
    if (window.Telegram && window.Telegram.WebApp) {
        window.Telegram.WebApp.ready();
        console.log("Telegram Web App SDK is ready.");

        // You can get user info from the SDK
        const user = window.Telegram.WebApp.initDataUnsafe?.user;
        if (user) {
            setState({ currentUser: user });
        }
    } else {
        console.warn("Telegram Web App SDK not found. Running in standalone mode.");
        // For local development, you can set a mock user
        setState({ currentUser: { id: 123, first_name: 'Dev', last_name: 'User', username: 'devuser' } });
    }

    // Initialize the calendar
    initializeCalendar();

    // Fetch and display queue and chores independently
    getUsers()
        .then(users => displayQueueSummary(users))
        .catch(error => console.error("Failed to fetch users:", error));

    getActiveChores()
        .then(chores => displayPendingChores(chores))
        .catch(error => console.error("Failed to fetch chores:", error));
}

// This is where the application will be initialized.
document.addEventListener('DOMContentLoaded', initializeApp);