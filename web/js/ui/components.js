/**
 * Reusable UI helpers for the Roster Bot frontend (Monospace Web).
 */

export function escapeHtml(unsafe) {
    if (unsafe === null || unsafe === undefined) return '';
    return String(unsafe)
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;")
        .replace(/'/g, "&#039;");
}

export function pad2(n) {
    return String(n).padStart(2, "0");
}

export function createLoadingSpinner() {
    return `<div class="spinner">Loading…</div>`;
}

export function createErrorMessage(message) {
    return `<div class="err"><span class="lbl">ERR</span>${escapeHtml(message)}</div>`;
}

/**
 * Opens a monospace-style modal with the given title and inner HTML.
 * Returns the modal root element (already attached to document.body).
 * Closes on Escape, on backdrop click, and on the × button.
 */
export function openModal({ title, dateLabel, bodyHtml, onMount }) {
    const existing = document.getElementById('duty-modal');
    if (existing) existing.remove();

    const scrim = document.createElement('div');
    scrim.className = 'scrim';
    scrim.id = 'duty-modal';
    scrim.innerHTML = `
        <div class="modal" role="dialog" aria-modal="true">
            <div class="modal-hdr">
                <div class="title">▍ ${escapeHtml(title)}${dateLabel ? `<span class="date">${escapeHtml(dateLabel)}</span>` : ''}</div>
                <button class="x" aria-label="Close">×</button>
            </div>
            <div class="modal-body">${bodyHtml}</div>
        </div>
    `;

    function close() {
        scrim.remove();
        document.removeEventListener('keydown', onKey);
    }
    function onKey(e) { if (e.key === 'Escape') close(); }

    scrim.addEventListener('click', (e) => { if (e.target === scrim) close(); });
    scrim.querySelector('.x').addEventListener('click', close);
    document.addEventListener('keydown', onKey);

    document.body.appendChild(scrim);

    if (typeof onMount === 'function') {
        onMount(scrim, close);
    }

    return { root: scrim, close };
}
