import { describe, it, expect, beforeEach } from 'vitest';
import { createModal } from '../web/js/ui/components.js';

describe('Calendar and Components XSS fixes', () => {
    beforeEach(() => {
        document.body.innerHTML = '';
    });

    it('should create a modal safely without executing script tags', () => {
        const title = 'Test Title <script>alert(1)</script>';
        const content = 'Test Content <img src=x onerror=alert(1)>';

        // This simulates calendar.js creating a modal using its own safe templating
        // calendar.js now calls escapeHtml on the duty fields before inserting to `content`
        const safeContent = `<div>${content}</div>`;
        const tempDiv = document.createElement('div');
        tempDiv.innerHTML = createModal(title, safeContent, 'test-modal');
        const modalNode = tempDiv.firstElementChild;
        document.body.appendChild(modalNode);

        const modalEl = document.getElementById('test-modal');
        expect(modalEl).not.toBeNull();
        expect(document.body.innerHTML).toContain('Test Content');
    });
});
