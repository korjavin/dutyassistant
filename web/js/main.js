import { initializeCalendar } from './ui/calendar.js';
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
}

// This is where the application will be initialized.
document.addEventListener('DOMContentLoaded', initializeApp);