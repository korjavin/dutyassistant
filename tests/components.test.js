import { describe, it, expect, afterEach } from 'vitest';
import { escapeHtml, openModal, pad2 } from '../web/js/ui/components.js';

describe('escapeHtml', () => {
    it('escapes HTML special characters', () => {
        expect(escapeHtml('Test & Title')).toBe('Test &amp; Title');
        expect(escapeHtml('<script>alert(1)</script>')).toBe('&lt;script&gt;alert(1)&lt;/script&gt;');
        expect(escapeHtml('"quoted" \'single\'')).toBe('&quot;quoted&quot; &#039;single&#039;');
    });

    it('returns empty string for null/undefined', () => {
        expect(escapeHtml(null)).toBe('');
        expect(escapeHtml(undefined)).toBe('');
    });
});

describe('pad2', () => {
    it('left-pads numbers with zero to width 2', () => {
        expect(pad2(0)).toBe('00');
        expect(pad2(9)).toBe('09');
        expect(pad2(15)).toBe('15');
    });
});

describe('openModal', () => {
    afterEach(() => {
        document.body.innerHTML = '';
    });

    it('mounts a modal with the given title and body, escaping the title', () => {
        const { root, close } = openModal({
            title: 'Test & <Title>',
            dateLabel: 'Mon · May 15 · 2026',
            bodyHtml: '<div class="probe">body</div>',
        });

        expect(root).not.toBeNull();
        expect(document.getElementById('duty-modal')).toBe(root);
        expect(root.querySelector('.modal .title').innerHTML).toContain('Test &amp; &lt;Title&gt;');
        expect(root.querySelector('.probe')).not.toBeNull();
        close();
        expect(document.getElementById('duty-modal')).toBeNull();
    });

    it('closes when the × button is clicked', () => {
        const { root } = openModal({ title: 'Duties', bodyHtml: '' });
        root.querySelector('.x').click();
        expect(document.getElementById('duty-modal')).toBeNull();
    });

    it('closes when Escape is pressed', () => {
        openModal({ title: 'Duties', bodyHtml: '' });
        document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
        expect(document.getElementById('duty-modal')).toBeNull();
    });
});
