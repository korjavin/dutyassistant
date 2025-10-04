// This module will encapsulate all communication with the backend API.

// Example function to fetch the schedule for a given month.
export async function getSchedule(year, month) {
    try {
        const response = await fetch(`/api/v1/schedule/${year}/${month}`);
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        return await response.json();
    } catch (error) {
        console.error("Failed to fetch schedule:", error);
        return null;
    }
}

// Add other API functions here (e.g., volunteer, assignDuty, etc.)