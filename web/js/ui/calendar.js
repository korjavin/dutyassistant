import VanillaCalendar from '/vendor/vanilla-calendar/vanilla-calendar.min.js';
import { getSchedule, getPrognosis, volunteerForDuty, withdrawFromDuty } from '../api.js';
import { getState, setState } from '../store.js';
import { createDutyCard, createModal, showModal, createLoadingSpinner, createErrorMessage, hideModal } from './components.js';

const calendarContainer = document.getElementById('calendar-container');
let calendar;
let scheduleLoadSeq = 0;
const calendarElementId = 'calendar';

function ensureCalendarElement() {
    let element = document.getElementById(calendarElementId);
    if (!element) {
        calendarContainer.innerHTML = `<div id="${calendarElementId}"></div>`;
        element = document.getElementById(calendarElementId);
    }
    return element;
}

function isCalendarDebugEnabled() {
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
 * Normalizes incoming date values to YYYY-MM-DD keys used in calendar maps.
 * @param {string} value
 * @returns {string}
 */
function normalizeDateKey(value) {
    if (!value) {
        return '';
    }

    const raw = String(value);
    const directMatch = raw.match(/^(\d{4}-\d{2}-\d{2})/);
    if (directMatch) {
        return directMatch[1];
    }

    const parsed = new Date(raw);
    if (Number.isNaN(parsed.getTime())) {
        return '';
    }

    return `${parsed.getFullYear()}-${String(parsed.getMonth() + 1).padStart(2, '0')}-${String(parsed.getDate()).padStart(2, '0')}`;
}

/**
 * Fetches and displays the schedule for the current month.
 */
async function loadAndDisplaySchedule() {
    const { currentYear, currentMonth } = getState();
    const loadSeq = ++scheduleLoadSeq;
    const hasCalendarInstance = Boolean(calendar);

    // Keep previous calendar visible while next month loads.
    if (!hasCalendarInstance) {
        calendarContainer.innerHTML = createLoadingSpinner();
    }
    calendarDebug('Loading month', { currentYear, currentMonth });

    try {
        const [scheduleData, prognosisData] = await Promise.all([
            getSchedule(currentYear, currentMonth),
            getPrognosis(currentYear, currentMonth)
        ]);

        // Ignore stale async responses from older month loads.
        if (loadSeq !== scheduleLoadSeq) {
            calendarDebug('Skip stale load result', { loadSeq, scheduleLoadSeq, currentYear, currentMonth });
            return;
        }

        if (scheduleData) {
            calendarDebug('Month payload ready', {
                currentYear,
                currentMonth,
                dutiesCount: Array.isArray(scheduleData?.duties) ? scheduleData.duties.length : 0,
                prognosisCount: Array.isArray(prognosisData?.prognosis) ? prognosisData.prognosis.length : 0,
            });
            setState({ schedule: { [`${currentYear}-${currentMonth}`]: scheduleData } });
            renderCalendar(scheduleData, prognosisData);
        } else {
            if (!hasCalendarInstance) {
                calendarContainer.innerHTML = createErrorMessage('Could not load schedule.');
            }
        }
    } catch (error) {
        console.error('Error loading schedule:', error);
        if (!hasCalendarInstance) {
            calendarContainer.innerHTML = createErrorMessage('Error loading schedule. Please try again later.');
        }
    }
}

/**
 * Renders the calendar with the given schedule data.
 * @param {object} scheduleData - The schedule data for the current month.
 * @param {object} prognosisData - The prognosis data for unassigned days.
 */
function renderCalendar(scheduleData = {}, prognosisData = {}) {
    const { currentYear, currentMonth, currentUser } = getState();
    const dutiesByDate = {};

    // Add actual duties
    if (scheduleData.duties) {
        scheduleData.duties.forEach(duty => {
            const date = normalizeDateKey(duty.date);
            if (!date) {
                return;
            }
            if (!dutiesByDate[date]) {
                dutiesByDate[date] = [];
            }
            // Add user name and assignment type style
            let displayName = duty.user_name || 'Unassigned';

            // Add queue counts to display name if present
            const queueParts = [];
            if (duty.volunteer_queue_days > 0) {
                queueParts.push(`V:${duty.volunteer_queue_days}`);
            }
            if (duty.admin_queue_days > 0) {
                queueParts.push(`A:${duty.admin_queue_days}`);
            }
            if (queueParts.length > 0) {
                displayName += ` (${queueParts.join(' ')})`;
            }

            duty.displayName = displayName;
            duty.typeClass = duty.assignment_type === 'voluntary' ? 'text-green-600' :
                            duty.assignment_type === 'admin' ? 'text-blue-600' : 'text-gray-600';
            duty.isPrognosis = false;
            dutiesByDate[date].push(duty);
        });
    }

    // Add prognosis for unassigned days
    if (prognosisData.prognosis) {
        prognosisData.prognosis.forEach(prog => {
            const date = normalizeDateKey(prog.date);
            if (!date) {
                return;
            }
            if (!dutiesByDate[date]) {
                dutiesByDate[date] = [];
            }
            dutiesByDate[date].push({
                displayName: prog.user_name,
                typeClass: 'text-gray-400 italic',
                assignment_type: 'prognosis (round-robin)',
                isPrognosis: true,
                date
            });
        });
    }

    const dates = Object.keys(dutiesByDate).map(dateStr => ({
        date: dateStr,
        CSSClasses: ['has-duty'],
    }));
    const monthPrefix = `${currentYear}-${String(currentMonth).padStart(2, '0')}-`;
    const datesInCurrentMonth = dates.map(d => d.date).filter(d => d.startsWith(monthPrefix));
    calendarDebug('Render month', {
        currentYear,
        currentMonth,
        mappedDatesTotal: dates.length,
        mappedDatesInCurrentMonth: datesInCurrentMonth.length,
        sampleInCurrentMonth: datesInCurrentMonth.slice(0, 10),
        sampleAllDates: dates.map(d => d.date).slice(0, 10),
    });

    const options = {
        type: 'default',
        settings: {
            lang: 'en',
            iso8601: true,
            selection: { day: 'single' },
            visibility: { theme: 'light', weekend: true, today: true },
            selected: {
                dates: dates.map(d => d.date),
                month: currentMonth - 1,
                year: currentYear
            },
        },
        actions: {
            clickDay(event, self) {
                const date = normalizeDateKey(self.selectedDates?.[0]);
                if (!date) {
                    return;
                }
                if (dutiesByDate[date]) {
                    const duties = dutiesByDate[date];
                    const content = duties.map(duty => `
                        <div class="p-3 mb-2 border rounded ${duty.typeClass}">
                            <div class="font-bold">${duty.displayName}</div>
                            <div class="text-sm text-gray-600">${duty.assignment_type}</div>
                        </div>
                    `).join('');
                    const modalId = 'duty-details-modal';

                    const existingModal = document.getElementById(modalId);
                    if (existingModal) existingModal.remove();

                    document.body.insertAdjacentHTML('beforeend', createModal(`Duties for ${date}`, content, modalId));
                    showModal(modalId);

                    const modalElement = document.getElementById(modalId);
                    modalElement.addEventListener('click', async (e) => {
                        const target = e.target;
                        if (target.tagName === 'BUTTON' && target.dataset.dutyId) {
                            const dutyId = target.dataset.dutyId;
                            const action = target.dataset.action;

                            const errorContainer = modalElement.querySelector('.error-container');
                            if(errorContainer) errorContainer.remove();

                            try {
                                target.disabled = true;
                                target.textContent = 'Processing...';

                                if (action === 'volunteer') {
                                    await volunteerForDuty(dutyId);
                                } else if (action === 'withdraw') {
                                    await withdrawFromDuty(dutyId);
                                }

                                hideModal(modalId);
                                loadAndDisplaySchedule();
                            } catch (error) {
                                console.error(`Failed to ${action}:`, error);
                                target.disabled = false;
                                target.textContent = action.charAt(0).toUpperCase() + action.slice(1);

                                const msg = createErrorMessage(`Failed to ${action}. Please try again.`);
                                target.insertAdjacentHTML('afterend', `<div class="error-container mt-2">${msg}</div>`);
                            }
                        }
                    });
                }
            },
            clickArrow(event, self) {
                // VanillaCalendar updates selectedMonth/selectedYear first, then calls this callback.
                const { currentYear, currentMonth } = getState();
                const nextYear = self.selectedYear;
                const nextMonth = self.selectedMonth + 1;
                if (currentYear === nextYear && currentMonth === nextMonth) {
                    // Prevent duplicate reload when calendar emits duplicate arrow callbacks.
                    return;
                }
                setState({
                    currentYear: nextYear,
                    currentMonth: nextMonth,
                });
                loadAndDisplaySchedule();
            },
            getDays(day, date, HTMLElement, HTMLButtonElement, self) {
                const dateStr = normalizeDateKey(date) || `${self.selectedYear}-${String(self.selectedMonth + 1).padStart(2, '0')}-${String(day).padStart(2, '0')}`;
                if (dutiesByDate[dateStr]) {
                    const duties = dutiesByDate[dateStr];
                    const namesHTML = duties.map(duty => {
                        const bgColor = duty.isPrognosis ? 'bg-gray-200' :
                                       duty.assignment_type === 'voluntary' ? 'bg-green-100' :
                                       duty.assignment_type === 'admin' ? 'bg-blue-100' : 'bg-gray-100';
                        const textColor = duty.isPrognosis ? 'text-gray-500' : 'text-gray-800';
                        const shortName = duty.displayName.substring(0, 3);
                        return `<span class="${bgColor} ${textColor} px-1 rounded text-[10px]">${shortName}</span>`;
                    }).join(' ');
                    HTMLButtonElement.innerHTML = `<span>${day}</span><div style="font-size:10px;margin-top:2px;">${namesHTML}</div>`;
                }
            },
        },
    };

    // Recreate calendar instance on every render to avoid stale closures
    // and duplicated internal handlers from month-to-month updates.
    if (calendar) {
        calendar.destroy();
        calendar = null;
    }

    const calendarElement = ensureCalendarElement();
    calendar = new VanillaCalendar(calendarElement, options);
    calendar.init();
}

/**
 * Displays the queue summary for all users with pending queues.
 * @param {Array} users - Array of user objects with queue information
 */
/**
 * Initializes the calendar view.
 */
export function initializeCalendar() {
    const today = new Date();
    setState({
        currentYear: today.getFullYear(),
        currentMonth: today.getMonth() + 1,
    });

    ensureCalendarElement();

    loadAndDisplaySchedule();
}
