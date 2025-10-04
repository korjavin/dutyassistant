from playwright.sync_api import sync_playwright, Page, expect
import re
import http.server
import socketserver
import threading
import os

PORT = 8010  # Using a different port to avoid caching issues
# Serve from the project root directory
SERVE_DIR = "."

class Handler(http.server.SimpleHTTPRequestHandler):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, directory=SERVE_DIR, **kwargs)

def start_server():
    with socketserver.TCPServer(("", PORT), Handler) as httpd:
        print(f"Serving HTTP on port {PORT} from project root '{SERVE_DIR}'...")
        httpd.serve_forever()

def run_verification():
    server_thread = threading.Thread(target=start_server, daemon=True)
    server_thread.start()

    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        page = browser.new_page()

        mock_schedule = {
            "duties": [
                {"id": 1, "date": "2025-10-15T00:00:00Z", "title": "Morning Duty", "time": "08:00", "assignees": [{"id": 101, "name": "Alice"}]},
                {"id": 2, "date": "2025-10-20T00:00:00Z", "title": "Evening Duty", "time": "18:00", "assignees": [{"id": 123, "name": "Dev User"}]}
            ]
        }

        def handle_route(route):
            # The API call will now be relative to the web/ directory
            if re.search(r"api/v1/schedule", route.request.url):
                print(f"Mocking API request for: {route.request.url}")
                route.fulfill(status=200, json=mock_schedule)
            else:
                route.continue_()

        # We must mock the route before navigating
        page.route(re.compile(r".*"), handle_route)

        # The URL now points to the file within the web directory
        page_url = f"http://localhost:{PORT}/web/index.html"
        print(f"Navigating to {page_url}")
        page.goto(page_url, wait_until="networkidle")

        # Wait for the calendar to be rendered
        calendar_locator = page.locator('#calendar-container .vanilla-calendar')
        expect(calendar_locator).to_be_visible(timeout=10000)
        print("Calendar is visible.")

        # Click on a day with duties to open the modal
        day_with_duty_locator = page.locator('.vanilla-calendar-day__btn[data-calendar-day="2025-10-20"]')
        expect(day_with_duty_locator).to_be_visible()
        day_with_duty_locator.click()
        print("Clicked on October 20th.")

        # Wait for the modal to appear and verify its content
        modal_locator = page.locator('#duty-details-modal')
        expect(modal_locator).to_be_visible()
        expect(modal_locator.get_by_text("Duties for 2025-10-20")).to_be_visible()

        duty_card_locator = modal_locator.locator('.duty-card', has_text='Evening Duty')
        expect(duty_card_locator).to_be_visible()

        withdraw_button = duty_card_locator.get_by_role('button', name='Withdraw')
        expect(withdraw_button).to_be_visible()
        print("Modal content verified successfully.")

        # Take a screenshot
        screenshot_path = "jules-scratch/verification/verification.png"
        page.screenshot(path=screenshot_path)
        print(f"Screenshot saved to {screenshot_path}")

        browser.close()

if __name__ == "__main__":
    run_verification()