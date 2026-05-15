import { escapeHtml, pad2 } from './components.js';

function formatAge(assignedAt) {
    if (!assignedAt) return '';
    const t = new Date(assignedAt);
    if (Number.isNaN(t.getTime())) return '';
    const diffMs = Date.now() - t.getTime();
    if (diffMs < 0) return '';
    const hours = Math.floor(diffMs / (60 * 60 * 1000));
    if (hours < 1) {
        const minutes = Math.floor(diffMs / (60 * 1000));
        return `${minutes}m ago`;
    }
    if (hours < 24) return `${hours}h ago`;
    const days = Math.floor(hours / 24);
    const rem = hours % 24;
    return `${days}d ${rem}h`;
}

function formatDateTime(value) {
    if (!value) return '';
    const d = new Date(value);
    if (Number.isNaN(d.getTime())) return escapeHtml(value);
    const y = d.getFullYear();
    const m = pad2(d.getMonth() + 1);
    const day = pad2(d.getDate());
    const hh = pad2(d.getHours());
    const mm = pad2(d.getMinutes());
    return `${y}-${m}-${day} · ${hh}:${mm}`;
}

/**
 * Renders pending (assigned but not yet completed) chores.
 * @param {{chores?: Array}|null} choresData
 */
export function displayPendingChores(choresData) {
    const list = document.getElementById('pending-chores-list');
    const counter = document.getElementById('chores-count');
    if (!list) return;

    const chores = Array.isArray(choresData?.chores) ? choresData.chores : [];

    if (counter) counter.textContent = `[${pad2(chores.length)}]`;
    const statTotal = document.getElementById('stat-chores');
    if (statTotal) statTotal.textContent = pad2(chores.length);

    if (chores.length === 0) {
        list.innerHTML = `<div class="empty">// no chores awaiting confirmation</div>`;
        return;
    }

    list.innerHTML = chores.map((chore) => {
        const desc = escapeHtml(chore.description || 'No description');
        const assignee = escapeHtml((chore.user_name || 'unknown').toLowerCase());
        const assignedAt = formatDateTime(chore.assigned_at);
        const age = escapeHtml(formatAge(chore.assigned_at));
        return `
            <div class="chore">
                <span class="mark">▢</span>
                <div class="body">
                    <div class="desc">${desc}</div>
                    <div class="meta">
                        <span class="assignee">@${assignee}</span>
                        <span class="dot-sep">·</span>
                        <span>${assignedAt}</span>
                    </div>
                </div>
                <div class="age tn">${age}</div>
            </div>
        `;
    }).join('');
}
