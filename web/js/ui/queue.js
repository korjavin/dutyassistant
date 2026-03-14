/**
 * Displays the queue summary for all users with pending queues.
 * @param {Array} users - Array of user objects with queue information
 */
export function displayQueueSummary(users) {
    const queueList = document.getElementById('queue-list');
    if (!queueList) return;

    if (!users || users.length === 0) {
        queueList.innerHTML = '<p class="text-gray-500">No users found.</p>';
        return;
    }

    // Filter active users with queues (API returns PascalCase)
    const usersWithQueues = users.filter(u =>
        u.IsActive && // Only show active users
        ((u.VolunteerQueueDays && u.VolunteerQueueDays > 0) ||
         (u.AdminQueueDays && u.AdminQueueDays > 0))
    );

    if (usersWithQueues.length === 0) {
        queueList.innerHTML = '<p class="text-gray-500">No pending queues.</p>';
        return;
    }

    // Build queue list HTML
    const queueHTML = usersWithQueues.map(user => {
        const parts = [];
        if (user.VolunteerQueueDays > 0) {
            parts.push(`<span class="text-green-600 font-semibold">V:${user.VolunteerQueueDays}</span>`);
        }
        if (user.AdminQueueDays > 0) {
            parts.push(`<span class="text-blue-600 font-semibold">A:${user.AdminQueueDays}</span>`);
        }
        return `<div class="mb-1">👤 <strong>${user.FirstName}</strong>: ${parts.join(', ')}</div>`;
    }).join('');

    queueList.innerHTML = queueHTML;
}
