/**
 * Reusable UI components for the Roster Bot frontend.
 */

/**
 * Creates a user badge component.
 * @param {object} user - The user object.
 * @param {string} user.name - The user's name.
 * @param {string} [user.avatarUrl] - URL for the user's avatar.
 * @returns {string} The HTML string for the user badge.
 */
export function createUserBadge(user) {
  return `
    <div class="inline-flex items-center bg-gray-200 rounded-full px-3 py-1 text-sm font-semibold text-gray-700 mr-2 mb-2">
      ${user.avatarUrl ? `<img src="${user.avatarUrl}" class="w-6 h-6 rounded-full mr-2" alt="${user.name}">` : ''}
      <span>${user.name}</span>
    </div>
  `;
}

/**
 * Creates a duty card component.
 * @param {object} duty - The duty object.
 * @param {string} duty.title - The title of the duty.
 * @param {string} duty.time - The time of the duty.
 * @param {Array<object>} duty.assignees - Users assigned to the duty.
 * @returns {string} The HTML string for the duty card.
 */
export function createDutyCard(duty, currentUser) {
    const isAssigned = duty.assignees.some(assignee => assignee.id === currentUser?.id);
    const canVolunteer = !isAssigned; // Basic logic, can be expanded

    let actionButton = '';
    if (currentUser) {
        if (isAssigned) {
            actionButton = `<button data-duty-id="${duty.id}" data-action="withdraw" class="mt-2 px-4 py-2 bg-red-500 text-white rounded hover:bg-red-600">Withdraw</button>`;
        } else if (canVolunteer) {
            actionButton = `<button data-duty-id="${duty.id}" data-action="volunteer" class="mt-2 px-4 py-2 bg-green-500 text-white rounded hover:bg-green-600">Volunteer</button>`;
        }
    }

    return `
    <div class="duty-card bg-white p-4 rounded-lg shadow mb-4" data-duty-id="${duty.id}">
      <h3 class="font-bold text-lg">${duty.title}</h3>
      <p class="text-gray-600">${duty.time}</p>
      <div class="mt-2">
        ${duty.assignees.map(user => createUserBadge(user)).join('')}
      </div>
      ${actionButton}
    </div>
  `;
}

/**
 * Creates a modal component.
 * @param {string} title - The title of the modal.
 * @param {string} content - The HTML content of the modal.
 * @param {string} modalId - The ID for the modal element.
 * @returns {string} The HTML string for the modal.
 */
export function createModal(title, content, modalId) {
  return `
    <div id="${modalId}" class="fixed inset-0 bg-gray-600 bg-opacity-50 overflow-y-auto h-full w-full hidden" aria-labelledby="modal-title" role="dialog" aria-modal="true">
      <div class="relative top-20 mx-auto p-5 border w-96 shadow-lg rounded-md bg-white">
        <div class="mt-3 text-center">
          <h3 class="text-lg leading-6 font-medium text-gray-900" id="modal-title">${title}</h3>
          <div class="mt-2 px-7 py-3">
            ${content}
          </div>
          <div class="items-center px-4 py-3">
            <button id="close-${modalId}" class="px-4 py-2 bg-gray-500 text-white text-base font-medium rounded-md w-full shadow-sm hover:bg-gray-700 focus:outline-none focus:ring-2 focus:ring-gray-300">
              Close
            </button>
          </div>
        </div>
      </div>
    </div>
  `;
}

/**
 * Shows a modal by its ID.
 * @param {string} modalId - The ID of the modal to show.
 */
export function showModal(modalId) {
    const modal = document.getElementById(modalId);
    if (modal) {
        modal.classList.remove('hidden');
        document.getElementById(`close-${modalId}`).onclick = () => hideModal(modalId);
    }
}

/**
 * Hides a modal by its ID.
 * @param {string} modalId - The ID of the modal to hide.
 */
export function hideModal(modalId) {
    const modal = document.getElementById(modalId);
    if (modal) {
        modal.classList.add('hidden');
    }
}

/**
 * Creates a loading spinner component.
 * @returns {string} The HTML string for the loading spinner.
 */
export function createLoadingSpinner() {
    return `
        <div class="flex justify-center items-center p-8">
            <div class="animate-spin rounded-full h-16 w-16 border-t-2 border-b-2 border-blue-500"></div>
        </div>
    `;
}

/**
 * Creates an error message component.
 * @param {string} message - The error message to display.
 * @returns {string} The HTML string for the error message.
 */
export function createErrorMessage(message) {
    return `
        <div class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded relative" role="alert">
            <strong class="font-bold">Error:</strong>
            <span class="block sm:inline">${message}</span>
        </div>
    `;
}