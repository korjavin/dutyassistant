import { getSchedule, getPrognosis, volunteerForDuty, withdrawFromDuty } from '../api.js';
import { getState, setState } from '../store.js';
import { escapeHtml, pad2, openModal, createLoadingSpinner, createErrorMessage } from './components.js';

const MONTHS = [
    "January", "February", "March", "April", "May", "June",
    "July", "August", "September", "October", "November", "December"
];
const DOW = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];
const WEEKDAY_LABELS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];

const calendarContainer = () => document.getElementById('calendar-container');
let scheduleLoadSeq = 0;
let currentDuties = {}; // dateKey -> array of duty objects (mixed real + prognosis)

function isCalendarDebugEnabled() {
    const params = new URLSearchParams(window.location.search);
    if (params.get('debugCalendar') === '1') return true;
    try { return window.localStorage?.getItem('debugCalendar') === '1'; } catch { return false; }
}

function calendarDebug(...args) {
    if (isCalendarDebugEnabled()) console.log('[CalendarDebug]', ...args);
}

function normalizeDateKey(value) {
    if (!value) return '';
    const raw = String(value);
    const m = raw.match(/^(\d{4}-\d{2}-\d{2})/);
    if (m) return m[1];
    const parsed = new Date(raw);
    if (Number.isNaN(parsed.getTime())) return '';
    return `${parsed.getFullYear()}-${pad2(parsed.getMonth() + 1)}-${pad2(parsed.getDate())}`;
}

function buildMonthCells(year, monthIdx /* 0-11 */) {
    const first = new Date(year, monthIdx, 1);
    const dow = (first.getDay() + 6) % 7; // 0 = Monday
    const daysInMonth = new Date(year, monthIdx + 1, 0).getDate();
    const prevDays = new Date(year, monthIdx, 0).getDate();
    const cells = [];
    for (let i = 0; i < dow; i++) {
        cells.push({ day: prevDays - dow + 1 + i, inMonth: false, monthIdx: monthIdx - 1, yearOffset: monthIdx === 0 ? -1 : 0 });
    }
    for (let d = 1; d <= daysInMonth; d++) {
        cells.push({ day: d, inMonth: true, monthIdx, yearOffset: 0, weekend: ((dow + d - 1) % 7) >= 5 });
    }
    let next = 1;
    while (cells.length < 42) {
        cells.push({ day: next++, inMonth: false, monthIdx: monthIdx + 1, yearOffset: monthIdx === 11 ? 1 : 0 });
    }
    return cells;
}

function classifyDuty(assignmentType) {
    if (assignmentType === 'voluntary') return 'voluntary';
    if (assignmentType === 'admin') return 'admin';
    if (assignmentType === 'round_robin') return 'round_robin';
    return 'prognosis';
}

function dutyClass(kind) {
    if (kind === 'voluntary') return 'vol';
    if (kind === 'admin' || kind === 'round_robin') return 'admin';
    return 'prog';
}

function dutyKindLabel(kind) {
    if (kind === 'voluntary') return 'Volunteered';
    if (kind === 'admin') return 'Admin Assigned';
    if (kind === 'round_robin') return 'Round-Robin · Auto';
    return 'Prognosis · Round-Robin';
}

function dutyLabel(duty) {
    const name = duty.user_name || '';
    return name.slice(0, 5).toUpperCase();
}

function countDutiesThisWeek(dutiesByDate, today) {
    const day = (today.getDay() + 6) % 7;
    const monday = new Date(today.getFullYear(), today.getMonth(), today.getDate() - day);
    const sunday = new Date(monday.getFullYear(), monday.getMonth(), monday.getDate() + 6);
    let count = 0;
    Object.entries(dutiesByDate).forEach(([key, duties]) => {
        const d = new Date(`${key}T00:00:00`);
        if (Number.isNaN(d.getTime())) return;
        if (d >= monday && d <= sunday) {
            count += duties.filter((x) => x.kind !== 'prognosis').length;
        }
    });
    return count;
}

async function loadAndDisplaySchedule() {
    const { currentYear, currentMonth } = getState();
    const loadSeq = ++scheduleLoadSeq;
    const container = calendarContainer();
    const hasGrid = container?.querySelector('.cal-grid');

    if (!hasGrid && container) {
        container.innerHTML = createLoadingSpinner();
    }
    calendarDebug('Loading month', { currentYear, currentMonth });

    try {
        const [scheduleData, prognosisData] = await Promise.all([
            getSchedule(currentYear, currentMonth),
            getPrognosis(currentYear, currentMonth),
        ]);

        if (loadSeq !== scheduleLoadSeq) {
            calendarDebug('Skip stale load result', { loadSeq, scheduleLoadSeq });
            return;
        }

        if (!scheduleData) {
            // Schedule is the primary data — if it fails, surface the error rather than
            // rendering an empty calendar from prognosis-only data.
            if (container && !hasGrid) container.innerHTML = createErrorMessage('Could not load schedule.');
            return;
        }

        setState({ schedule: { [`${currentYear}-${currentMonth}`]: scheduleData } });
        renderCalendar(scheduleData, prognosisData || {});
    } catch (error) {
        console.error('Error loading schedule:', error);
        if (container && !hasGrid) container.innerHTML = createErrorMessage('Error loading schedule. Please try again later.');
    }
}

function renderCalendar(scheduleData, prognosisData) {
    const { currentYear, currentMonth } = getState();
    const monthIdx = currentMonth - 1;
    const today = new Date();

    const dutiesByDate = {};
    if (Array.isArray(scheduleData.duties)) {
        scheduleData.duties.forEach((duty) => {
            const date = normalizeDateKey(duty.date);
            if (!date) return;
            if (!dutiesByDate[date]) dutiesByDate[date] = [];
            dutiesByDate[date].push({
                ...duty,
                kind: classifyDuty(duty.assignment_type),
                vq: duty.volunteer_queue_days || 0,
                aq: duty.admin_queue_days || 0,
            });
        });
    }
    if (Array.isArray(prognosisData.prognosis)) {
        prognosisData.prognosis.forEach((prog) => {
            const date = normalizeDateKey(prog.date);
            if (!date) return;
            if (!dutiesByDate[date]) dutiesByDate[date] = [];
            dutiesByDate[date].push({
                ...prog,
                kind: 'prognosis',
                vq: 0,
                aq: 0,
            });
        });
    }
    currentDuties = dutiesByDate;

    const dutiesThisWeek = countDutiesThisWeek(dutiesByDate, today);
    const statWeek = document.getElementById('stat-week');
    if (statWeek) statWeek.textContent = pad2(dutiesThisWeek);

    const cells = buildMonthCells(currentYear, monthIdx);

    const dowRow = DOW.map((d) => `<div class="cal-dow">${d}</div>`).join('');

    const cellsHtml = cells.map((cell) => {
        const cellYear = currentYear + (cell.yearOffset || 0);
        const cellMonthIdx = ((cell.monthIdx % 12) + 12) % 12;
        const dateKey = `${cellYear}-${pad2(cellMonthIdx + 1)}-${pad2(cell.day)}`;
        const duties = dutiesByDate[dateKey] || [];

        const isToday = cell.inMonth
            && today.getFullYear() === cellYear
            && today.getMonth() === cellMonthIdx
            && today.getDate() === cell.day;

        const classList = ['cal-day'];
        if (!cell.inMonth) classList.push('other');
        if (cell.weekend) classList.push('weekend');
        if (isToday) classList.push('today');
        if (cell.inMonth && duties.length === 0) classList.push('no-duty');

        const queueSum = duties.reduce((s, d) => s + (d.vq || 0) + (d.aq || 0), 0);
        const shown = duties.slice(0, 2);
        const overflow = duties.length - shown.length;

        const pills = cell.inMonth ? `
            <div class="pill-list">
                ${shown.map((d) => `<span class="pill ${dutyClass(d.kind)}">${escapeHtml(dutyLabel(d))}</span>`).join('')}
                ${overflow > 0 ? `<span class="pill more">+${overflow}</span>` : ''}
            </div>
        ` : '';

        const badge = cell.inMonth && queueSum > 0 ? `<span class="badge-q">·${queueSum}</span>` : '';

        return `
            <div class="${classList.join(' ')}" data-date="${dateKey}" data-clickable="${cell.inMonth && duties.length > 0 ? '1' : '0'}">
                <div class="num"><span>${pad2(cell.day)}</span>${badge}</div>
                ${pills}
            </div>
        `;
    }).join('');

    const html = `
        <div class="cal">
            <div class="cal-nav">
                <div class="month">${MONTHS[monthIdx]}<span class="yr">${currentYear}</span></div>
                <div class="arrows">
                    <button type="button" data-nav="prev" aria-label="Previous month">‹</button>
                    <button type="button" data-nav="today" class="today">Today</button>
                    <button type="button" data-nav="next" aria-label="Next month">›</button>
                </div>
            </div>
            <div class="cal-grid">${dowRow}${cellsHtml}</div>
        </div>
    `;

    const container = calendarContainer();
    if (!container) return;
    container.innerHTML = html;

    container.querySelectorAll('button[data-nav]').forEach((btn) => {
        btn.addEventListener('click', () => handleNav(btn.dataset.nav));
    });

    container.querySelectorAll('.cal-day[data-clickable="1"]').forEach((cell) => {
        cell.addEventListener('click', () => openDutyModal(cell.dataset.date));
    });
}

function handleNav(action) {
    const { currentYear, currentMonth } = getState();
    if (action === 'today') {
        const t = new Date();
        setState({ currentYear: t.getFullYear(), currentMonth: t.getMonth() + 1 });
    } else if (action === 'prev') {
        if (currentMonth === 1) setState({ currentYear: currentYear - 1, currentMonth: 12 });
        else setState({ currentMonth: currentMonth - 1 });
    } else if (action === 'next') {
        if (currentMonth === 12) setState({ currentYear: currentYear + 1, currentMonth: 1 });
        else setState({ currentMonth: currentMonth + 1 });
    }
    loadAndDisplaySchedule();
}

function openDutyModal(dateKey) {
    const duties = currentDuties[dateKey] || [];
    const [y, m, d] = dateKey.split('-').map((n) => parseInt(n, 10));
    const date = new Date(y, m - 1, d);
    const weekday = WEEKDAY_LABELS[date.getDay()];
    const dateLabel = `${weekday} · ${MONTHS[date.getMonth()]} ${pad2(date.getDate())} · ${date.getFullYear()}`;
    const { currentUser, currentUserInternalId } = getState();
    const currentFirstName = currentUser?.first_name || currentUser?.FirstName || '';

    function isMine(duty) {
        // currentUser.id is the Telegram user ID; duty.user_id is the internal store ID.
        // They live in different namespaces — only compare against currentUserInternalId,
        // resolved from /api/v1/users via TelegramUserID.
        if (duty.user_id != null && currentUserInternalId != null) {
            return Number(duty.user_id) === Number(currentUserInternalId);
        }
        if (duty.user_name && currentFirstName) {
            return String(duty.user_name).toLowerCase() === String(currentFirstName).toLowerCase();
        }
        return false;
    }

    const userOnDuty = duties.some((d) => d.kind !== 'prognosis' && isMine(d));

    const bodyDuties = duties.length === 0
        ? `<div class="empty">// no duties on this day</div>`
        : duties.map((duty, idx) => {
            const cls = dutyClass(duty.kind);
            const name = escapeHtml((duty.user_name || '').toLowerCase());
            const me = isMine(duty) ? ' · you' : '';
            const kindLabel = escapeHtml(dutyKindLabel(duty.kind));
            const qChip = (duty.vq > 0 || duty.aq > 0)
                ? `<div class="qchip">${duty.vq > 0 ? `V·${duty.vq}` : ''}${duty.vq > 0 && duty.aq > 0 ? ' / ' : ''}${duty.aq > 0 ? `A·${duty.aq}` : ''}</div>`
                : '';
            return `
                <div class="duty ${cls}" data-duty-index="${idx}">
                    <div>
                        <div class="name">@${name}${me}</div>
                        <div class="kind">${kindLabel}</div>
                    </div>
                    ${qChip}
                </div>
            `;
        }).join('');

    let actionsHtml = '';
    if (currentUser) {
        if (userOnDuty) {
            actionsHtml = `<button class="btn" data-action="withdraw">Withdraw</button>`;
        } else {
            actionsHtml = `<button class="btn primary" data-action="volunteer">Volunteer</button>`;
        }
    }
    actionsHtml += `<button class="btn ghost" data-action="close">Close</button>`;

    const bodyHtml = `
        ${bodyDuties}
        <div class="modal-actions">${actionsHtml}</div>
        <div class="error-container"></div>
    `;

    openModal({
        title: 'Duties',
        dateLabel,
        bodyHtml,
        onMount: (scrim, close) => {
            scrim.addEventListener('click', async (e) => {
                const target = e.target;
                if (!(target instanceof HTMLElement)) return;
                if (target.matches('button[data-action="close"]')) { close(); return; }

                const action = target.dataset.action;
                if (!action || (action !== 'volunteer' && action !== 'withdraw')) return;

                target.disabled = true;
                const originalText = target.textContent;
                target.textContent = 'Processing…';
                const errBox = scrim.querySelector('.error-container');
                if (errBox) errBox.innerHTML = '';

                try {
                    if (action === 'volunteer') await volunteerForDuty(dateKey);
                    else if (action === 'withdraw') await withdrawFromDuty(dateKey);
                    close();
                    loadAndDisplaySchedule();
                } catch (err) {
                    console.error(`Failed to ${action}:`, err);
                    target.disabled = false;
                    target.textContent = originalText;
                    if (errBox) errBox.innerHTML = createErrorMessage(`Failed to ${action}. Please try again.`);
                }
            });
        },
    });
}

export function initializeCalendar() {
    const today = new Date();
    setState({ currentYear: today.getFullYear(), currentMonth: today.getMonth() + 1 });
    loadAndDisplaySchedule();
}
