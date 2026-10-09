"""Check the mobile navigation's keyboard close behavior in the local preview."""

from playwright.sync_api import sync_playwright

URL = 'http://127.0.0.1:8765/sysc/docs/start/install/'

with sync_playwright() as playwright:
    browser = playwright.chromium.launch(headless=True, executable_path='/usr/bin/chromium')
    page = browser.new_page(viewport={'width': 390, 'height': 844})
    page.goto(URL, wait_until='networkidle')

    trigger = page.locator('.site-sidebar-trigger')
    assert trigger.get_attribute('aria-expanded') == 'false'
    trigger.focus()
    page.keyboard.press('Enter')
    assert page.locator('#nd-sidebar-mobile').get_attribute('data-state') == 'open'
    assert trigger.get_attribute('aria-expanded') == 'true'
    assert trigger.get_attribute('aria-label') == 'Close navigation'

    page.keyboard.press('Escape')
    page.wait_for_timeout(100)
    state = page.locator('#nd-sidebar-mobile').get_attribute('data-state')
    assert state != 'open', f'Escape left mobile navigation open (state={state})'
    assert trigger.get_attribute('aria-expanded') == 'false'
    assert trigger.get_attribute('aria-label') == 'Open navigation'
    assert trigger.evaluate('(button) => document.activeElement === button')

    browser.close()
