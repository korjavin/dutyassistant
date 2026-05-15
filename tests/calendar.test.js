import { describe, it, expect, afterEach } from 'vitest';
import { openModal, escapeHtml } from '../web/js/ui/components.js';

describe('Modal XSS safety', () => {
    afterEach(() => {
        document.body.innerHTML = '';
    });

    it('escapes the title to neutralize injected scripts', () => {
        const title = 'Test Title <script>alert(1)</script>';
        const { root } = openModal({ title, bodyHtml: '<div class="probe">safe</div>' });

        expect(root.querySelector('script')).toBeNull();
        expect(root.querySelector('.title').innerHTML).toContain(escapeHtml(title));
    });
});
