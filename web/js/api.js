// This module will encapsulate all communication with the backend API.

/**
 * A helper function for making POST requests.
 * @param {string} url - The URL to send the request to.
 * @param {object} data - The data to send in the request body.
 * @returns {Promise<any>} The response JSON data.
 */
async function postData(url = '', data = {}) {
    const response = await fetch(url, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify(data),
    });

    if (!response.ok) {
        const errorText = await response.text();
        throw new Error(`HTTP error! status: ${response.status}, body: ${errorText}`);
    }
    return response.json();
}

/**
 * Fetches the schedule for a given month.
 * @param {number} year - The year.
 * @param {number} month - The month.
 * @returns {Promise<any>} The schedule data.
 */
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

/**
 * Fetches the round-robin prognosis for a given month.
 * @param {number} year - The year.
 * @param {number} month - The month.
 * @returns {Promise<any>} The prognosis data.
 */
export async function getPrognosis(year, month) {
    try {
        const response = await fetch(`/api/v1/prognosis/${year}/${month}`);
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        return await response.json();
    } catch (error) {
        console.error("Failed to fetch prognosis:", error);
        return null;
    }
}

/**
 * Fetches all users.
 * @returns {Promise<any>} A list of users.
 */
export async function getUsers() {
    try {
        const response = await fetch('/api/v1/users');
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        return await response.json();
    } catch (error) {
        console.error("Failed to fetch users:", error);
        return null;
    }
}

/**
 * Allows the current user to volunteer for a specific duty.
 * @param {number} dutyId - The ID of the duty.
 * @returns {Promise<any>} The result of the operation.
 */
export async function volunteerForDuty(dutyId) {
    return postData(`/api/v1/duties/${dutyId}/volunteer`);
}

/**
 * Allows the current user to withdraw from a specific duty.
 * @param {number} dutyId - The ID of the duty.
 * @returns {Promise<any>} The result of the operation.
 */
export async function withdrawFromDuty(dutyId) {
    return postData(`/api/v1/duties/${dutyId}/withdraw`);
}

/**
 * Allows an admin to assign a user to a duty.
 * @param {number} dutyId - The ID of the duty.
 * @param {number} userId - The ID of the user to assign.
 * @returns {Promise<any>} The result of the operation.
 */
export async function assignDuty(dutyId, userId) {
    return postData(`/api/v1/duties/${dutyId}/assign`, { user_id: userId });
}