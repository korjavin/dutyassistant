import { escapeHtml, pad2 } from './components.js';

function initials(name) {
    if (!name) return '??';
    const parts = String(name).trim().split(/\s+/);
    if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
    return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
}

/**
 * Renders the queue balance section for active users with pending queues.
 * @param {Array} users - Array of user objects (PascalCase fields from backend).
 */
export function displayQueueSummary(users) {
    const list = document.getElementById('queue-list');
    const counter = document.getElementById('queue-count');
    if (!list) return;

    const active = (Array.isArray(users) ? users : []).filter((u) =>
        u && u.IsActive && ((u.VolunteerQueueDays || 0) > 0 || (u.AdminQueueDays || 0) > 0)
    );

    active.sort((a, b) =>
        ((b.VolunteerQueueDays || 0) + (b.AdminQueueDays || 0)) -
        ((a.VolunteerQueueDays || 0) + (a.AdminQueueDays || 0))
    );

    if (counter) counter.textContent = `[${pad2(active.length)}]`;

    const queueTotal = active.reduce((s, u) => s + (u.VolunteerQueueDays || 0) + (u.AdminQueueDays || 0), 0);
    const statQueue = document.getElementById('stat-queue');
    if (statQueue) statQueue.textContent = pad2(queueTotal);

    if (active.length === 0) {
        list.innerHTML = `<div class="empty">// no pending queues</div>`;
        return;
    }

    const maxQ = active.reduce((m, u) => Math.max(m, (u.VolunteerQueueDays || 0) + (u.AdminQueueDays || 0)), 0);
    const cap = Math.max(maxQ, 5);

    list.innerHTML = `<div class="queue">${active.map((u) => {
        const v = u.VolunteerQueueDays || 0;
        const a = u.AdminQueueDays || 0;
        const padCount = Math.max(0, cap - (v + a));
        const ini = escapeHtml(initials(u.FirstName || u.first_name || u.UserName || ''));
        const name = escapeHtml(u.FirstName || u.first_name || u.UserName || 'Unknown');
        const blocks =
            Array.from({ length: v }).map(() => `<span class="blk"></span>`).join('') +
            Array.from({ length: a }).map(() => `<span class="blk a"></span>`).join('') +
            Array.from({ length: padCount }).map(() => `<span class="pad"></span>`).join('');
        return `
            <div class="queue-row">
                <div class="who"><span class="pic">${ini}</span><span>${name}</span></div>
                <div class="bar">${blocks}</div>
                <div class="tally">
                    <span class="v">V·${v}</span>
                    <span class="sep">/</span>
                    <span class="a">A·${a}</span>
                </div>
            </div>
        `;
    }).join('')}</div>`;
}
