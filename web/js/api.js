// This module will encapsulate all communication with the backend API.

function isCalendarDebugEnabled() {
    if (typeof window === 'undefined') {
        return false;
    }
    const params = new URLSearchParams(window.location.search);
    if (params.get('debugCalendar') === '1') {
        return true;
    }
    try {
        return window.localStorage?.getItem('debugCalendar') === '1';
    } catch {
        return false;
    }
}

function calendarDebug(...args) {
    if (isCalendarDebugEnabled()) {
        console.log('[CalendarDebug]', ...args);
    }
}

/**
 * Gets authentication headers with Telegram Web App initData if available.
 * @returns {object} Headers object with optional Authorization header.
 */
function getAuthHeaders() {
    const headers = {
        'Content-Type': 'application/json',
    };

    // Add Telegram Web App authentication if available
    if (window.Telegram?.WebApp?.initData) {
        headers['Authorization'] = 'tma ' + window.Telegram.WebApp.initData;
    }

    return headers;
}

/**
 * A helper function for making POST requests.
 * @param {string} url - The URL to send the request to.
 * @param {object} data - The data to send in the request body.
 * @returns {Promise<any>} The response JSON data.
 */
async function postData(url = '', data = {}) {
    const response = await fetch(url, {
        method: 'POST',
        headers: getAuthHeaders(),
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
        calendarDebug('Request schedule', { year, month });
        const response = await fetch(`/api/v1/schedule/${year}/${month}`, {
            headers: getAuthHeaders()
        });
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        const payload = await response.json();
        const duties = Array.isArray(payload?.duties) ? payload.duties : [];
        calendarDebug('Schedule response', {
            year,
            month,
            dutiesCount: duties.length,
            firstDates: duties.slice(0, 5).map(d => d?.date),
            lastDates: duties.slice(-5).map(d => d?.date),
        });
        return payload;
    } catch (error) {
        calendarDebug('Schedule request failed', { year, month, error: String(error) });
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
        const payload = await response.json();
        const prognosis = Array.isArray(payload?.prognosis) ? payload.prognosis : [];
        calendarDebug('Prognosis response', { year, month, prognosisCount: prognosis.length });
        return payload;
    } catch (error) {
        calendarDebug('Prognosis request failed', { year, month, error: String(error) });
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
        const response = await fetch('/api/v1/users', {
            headers: getAuthHeaders()
        });
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
 * Fetches active chores (assigned but not yet completed).
 * @returns {Promise<any>} Active chores payload.
 */
export async function getActiveChores() {
    try {
        const response = await fetch('/api/v1/chores/active', {
            headers: getAuthHeaders()
        });
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        return await response.json();
    } catch (error) {
        console.error("Failed to fetch active chores:", error);
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
