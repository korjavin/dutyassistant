import { initializeCalendar } from './ui/calendar.js';
import { displayQueueSummary } from './ui/queue.js';
import { displayPendingChores } from './ui/chores.js';
import { getUsers, getActiveChores } from './api.js';
import { setState } from './store.js';
import { pad2 } from './ui/components.js';

function setHeaderUser(user) {
    const el = document.getElementById('hdr-user-name');
    if (!el) return;
    const name = user?.username
        || user?.first_name
        || user?.FirstName
        || 'guest';
    el.textContent = `@${String(name).toLowerCase()}`;
}

function startClock() {
    const el = document.getElementById('foot-clock');
    if (!el) return;
    function tick() {
        const now = new Date();
        el.textContent = `↻ ${pad2(now.getHours())}:${pad2(now.getMinutes())}`;
    }
    tick();
    setInterval(tick, 30 * 1000);
}

function initializeApp() {
    if (window.Telegram && window.Telegram.WebApp) {
        window.Telegram.WebApp.ready();
        const user = window.Telegram.WebApp.initDataUnsafe?.user;
        if (user) {
            setState({ currentUser: user });
            setHeaderUser(user);
        } else {
            setHeaderUser(null);
        }
    } else {
        console.warn('Telegram Web App SDK not found. Running in standalone mode.');
        const devUser = { id: 123, first_name: 'Dev', last_name: 'User', username: 'devuser' };
        setState({ currentUser: devUser });
        setHeaderUser(devUser);
    }

    startClock();
    initializeCalendar();

    getUsers()
        .then((users) => displayQueueSummary(users))
        .catch((error) => console.error('Failed to fetch users:', error));

    getActiveChores()
        .then((chores) => displayPendingChores(chores))
        .catch((error) => console.error('Failed to fetch chores:', error));
}

document.addEventListener('DOMContentLoaded', initializeApp);
