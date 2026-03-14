import { describe, it, expect, vi } from 'vitest';
import { createModal, escapeHtml } from '../web/js/ui/components.js';

describe('Modal component', () => {
    it('creates a modal and applies correct attributes', () => {
        const title = 'Test & Title';
        const content = '<p>Body</p>';
        const modalId = 'm1';

        const html = createModal(title, content, modalId);

        // Ensure that our HTML is properly escaped for the attributes
        expect(html).toContain('id="m1"');
        expect(html).toContain('id="modal-title">Test &amp; Title</h3>');
        expect(html).toContain('<p>Body</p>'); // Content is not escaped
    });
});
