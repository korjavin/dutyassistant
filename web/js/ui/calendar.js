import VanillaCalendar from '/vendor/vanilla-calendar/vanilla-calendar.min.js';
import { getSchedule, getPrognosis, volunteerForDuty, withdrawFromDuty } from '../api.js';
import { getState, setState } from '../store.js';
import { createDutyCard, createModal, showModal, createLoadingSpinner, createErrorMessage, hideModal } from './components.js';

const calendarContainer = document.getElementById('calendar-container');
let calendar;

/**
 * Fetches and displays the schedule for the current month.
 */
async function loadAndDisplaySchedule() {
    const { currentYear, currentMonth } = getState();
    calendarContainer.innerHTML = createLoadingSpinner();

    try {
        const [scheduleData, prognosisData] = await Promise.all([
            getSchedule(currentYear, currentMonth),
            getPrognosis(currentYear, currentMonth)
        ]);

        if (scheduleData) {
            setState({ schedule: { [`${currentYear}-${currentMonth}`]: scheduleData } });
            renderCalendar(scheduleData, prognosisData);
        } else {
            calendarContainer.innerHTML = createErrorMessage('Could not load schedule.');
        }
    } catch (error) {
        console.error('Error loading schedule:', error);
        calendarContainer.innerHTML = createErrorMessage('Error loading schedule. Please try again later.');
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
            const date = duty.date.split('T')[0];
            if (!dutiesByDate[date]) {
                dutiesByDate[date] = [];
            }
            // Add user name and assignment type style
            duty.displayName = duty.user_name || 'Unassigned';
            duty.typeClass = duty.assignment_type === 'voluntary' ? 'text-green-600' :
                            duty.assignment_type === 'admin' ? 'text-blue-600' : 'text-gray-600';
            duty.isPrognosis = false;
            dutiesByDate[date].push(duty);
        });
    }

    // Add prognosis for unassigned days
    if (prognosisData.prognosis) {
        prognosisData.prognosis.forEach(prog => {
            if (!dutiesByDate[prog.date]) {
                dutiesByDate[prog.date] = [];
            }
            dutiesByDate[prog.date].push({
                displayName: prog.user_name,
                typeClass: 'text-gray-400 italic',
                assignment_type: 'prognosis (round-robin)',
                isPrognosis: true,
                date: prog.date
            });
        });
    }

    const dates = Object.keys(dutiesByDate).map(dateStr => ({
        date: dateStr,
        CSSClasses: ['has-duty'],
    }));

    const options = {
        type: 'default',
        settings: {
            lang: 'en',
            iso8601: true,
            selection: { day: 'single' },
            visibility: { theme: 'light', weekend: true, today: true },
            selected: { dates: dates.map(d => d.date) },
        },
        actions: {
            clickDay(event, self) {
                const date = self.selectedDates[0];
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
            arrowPrev() {
                const { currentYear, currentMonth } = getState();
                const newDate = new Date(currentYear, currentMonth - 2);
                setState({ currentYear: newDate.getFullYear(), currentMonth: newDate.getMonth() + 1 });
                loadAndDisplaySchedule();
            },
            arrowNext() {
                const { currentYear, currentMonth } = getState();
                const newDate = new Date(currentYear, currentMonth);
                setState({ currentYear: newDate.getFullYear(), currentMonth: newDate.getMonth() + 1 });
                loadAndDisplaySchedule();
            },
        },
        popups: {},
    };

    Object.keys(dutiesByDate).forEach(date => {
        options.popups[date] = {
            html: `${dutiesByDate[date].length} duties`,
        };
    });

    if (calendar) {
        calendar.options = options;
        calendar.update();
    } else {
        calendar = new VanillaCalendar(calendarContainer, options);
        calendar.init();
    }
}

/**
 * Initializes the calendar view.
 */
export function initializeCalendar() {
    const today = new Date();
    setState({
        currentYear: today.getFullYear(),
        currentMonth: today.getMonth() + 1,
    });

    if (!document.getElementById('calendar')) {
        calendarContainer.innerHTML = '<div id="calendar"></div>';
    }

    loadAndDisplaySchedule();
}