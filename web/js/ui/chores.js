/**
 * Escapes user-provided content before inserting it into HTML.
 * @param {string} value
 * @returns {string}
 */
export function escapeHTML(value) {
    return String(value ?? '')
        .replaceAll('&', '&amp;')
        .replaceAll('<', '&lt;')
        .replaceAll('>', '&gt;')
        .replaceAll('"', '&quot;')
        .replaceAll("'", '&#39;');
}

/**
 * Formats RFC3339 timestamp for display in the user's locale.
 * @param {string} dateValue
 * @returns {string}
 */
export function formatDateTime(dateValue) {
    if (!dateValue) {
        return 'unknown';
    }

    const parsed = new Date(dateValue);
    if (Number.isNaN(parsed.getTime())) {
        return escapeHTML(dateValue);
    }

    return parsed.toLocaleString();
}

/**
 * Displays active (not completed) chores above the calendar.
 * @param {object|null} choresData - API payload from /api/v1/chores/active.
 */
export function displayPendingChores(choresData) {
    const choresList = document.getElementById('pending-chores-list');
    if (!choresList) return;

    const chores = Array.isArray(choresData?.chores) ? choresData.chores : [];
    if (chores.length === 0) {
        choresList.innerHTML = '<p class="text-gray-500">No active chores.</p>';
        return;
    }

    const choresHTML = chores.map(chore => {
        const description = escapeHTML(chore.description || 'No description');
        const assignee = escapeHTML(chore.user_name || 'Unknown');
        const createdAt = formatDateTime(chore.assigned_at);
        return `
            <div class="mb-2">
                <div><strong>${description}</strong></div>
                <div class="text-sm text-gray-600">Created: ${createdAt} | Assigned to: ${assignee}</div>
            </div>
        `;
    }).join('');

    choresList.innerHTML = choresHTML;
}
