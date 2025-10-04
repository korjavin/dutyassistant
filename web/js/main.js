// Main entry point for the frontend application.
console.log("Roster Bot frontend script loaded.");

// This is where the application will be initialized.
// For example, fetching data and rendering the calendar.
document.addEventListener('DOMContentLoaded', () => {
  console.log("DOM fully loaded and parsed.");
  // Initialize the Telegram Web App SDK
  if (window.Telegram && window.Telegram.WebApp) {
    window.Telegram.WebApp.ready();
    console.log("Telegram Web App SDK is ready.");
  }
});