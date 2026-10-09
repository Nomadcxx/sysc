"""Check documentation identity and heading hierarchy in the static preview."""

from pathlib import Path

from playwright.sync_api import sync_playwright

BASE_URL = 'http://127.0.0.1:8766/sysc'
REPO_ROOT = Path(__file__).resolve().parents[3]
CONTENT_ROOT = REPO_ROOT / 'docs-site' / 'content' / 'docs'
ARTICLES = (
    '/docs/components/sysc-shell/',
    '/docs/components/sysc-clipboard/',
    '/docs/components/sysc-metrics/',
    '/docs/guides/wallpapers/',
    '/docs/start/install/',
    '/docs/plugins/ui/',
    '/docs/plugins/manifest/',
    '/docs/plugins/test/',
    '/docs/reference/sources/',
    '/docs/troubleshooting/installer/',
)


def article_route(path: Path) -> str:
    relative = path.relative_to(CONTENT_ROOT)
    if relative.name == 'index.mdx':
        section = relative.parent.as_posix()
        return '/docs/' if section == '.' else f'/docs/{section}/'
    return f'/docs/{relative.with_suffix("").as_posix()}/'


ALL_ARTICLES = tuple(article_route(path) for path in sorted(CONTENT_ROOT.rglob('*.mdx')))

with sync_playwright() as playwright:
    browser = playwright.chromium.launch(headless=True, executable_path='/usr/bin/chromium')
    page = browser.new_page(viewport={'width': 390, 'height': 844})

    for route in ALL_ARTICLES:
        page.goto(f'{BASE_URL}{route}', wait_until='networkidle')
        headings = page.locator('article[role="main"] .prose > :is(h2,h3)').evaluate_all(
            "elements => elements.map(element => ({text: element.innerText.trim(), "
            "height: element.clientHeight, lineHeight: parseFloat(getComputedStyle(element).lineHeight)}))"
        )
        for heading in headings:
            assert heading['height'] <= heading['lineHeight'] * 1.2, (
                f'{route} at 390px: section heading wraps: {heading["text"]!r}'
            )
        assert page.evaluate('document.documentElement.scrollWidth <= innerWidth'), (
            f'{route} overflows at 390px'
        )

    page.goto(f'{BASE_URL}/docs/components/sysc-shell/', wait_until='networkidle')
    mobile_navigation = page.locator('.site-sidebar-trigger')
    assert mobile_navigation.get_attribute('aria-label') == 'Open navigation'
    mobile_navigation.focus()
    page.keyboard.press('Enter')
    assert mobile_navigation.get_attribute('aria-expanded') == 'true'
    assert page.locator('#nd-sidebar-mobile').is_visible()
    page.keyboard.press('Escape')
    assert mobile_navigation.get_attribute('aria-expanded') == 'false'

    for width in (390, 720, 1440):
        page.set_viewport_size({'width': width, 'height': 1000})
        page.goto(f'{BASE_URL}/docs/', wait_until='networkidle')
        home_title = page.locator('.docs-home-heading h1')
        home_logo = home_title.locator('img')
        assert home_logo.count() == 1 and home_logo.get_attribute('src').endswith('/sysc-logo.png')
        assert home_logo.get_attribute('alt') == 'SYSC'
        assert home_title.locator('span').inner_text().strip() == 'Documentation'
        assert 'desktop' not in home_title.inner_text().lower()

        for route in ARTICLES:
            page.goto(f'{BASE_URL}{route}', wait_until='networkidle')
            article = page.locator('article[role="main"]')
            sizes = article.evaluate(
                "element => [element.querySelector(':scope > h1'), "
                "element.querySelector('.prose > h2')].map((heading) => "
                "parseFloat(getComputedStyle(heading).fontSize))"
            )
            assert sizes[0] >= sizes[1] * 1.35, (
                f'{route} at {width}px: page title {sizes[0]}px, section {sizes[1]}px'
            )
            frame = article.locator('.prose > h2').first.evaluate(
                "heading => {const style = getComputedStyle(heading.querySelector('a[data-card]'), '::before'); "
                "return [style.content, parseFloat(style.fontSize), Number(style.fontWeight)]}"
            )
            assert '/' in frame[0] and frame[1] <= sizes[1] * 0.5 and frame[2] <= 400, (
                f'{route} at {width}px: slash frame should stay visible and lighter ({frame})'
            )
            assert page.evaluate('document.documentElement.scrollWidth <= innerWidth'), (
                f'{route} overflows at {width}px'
            )

            brand = page.locator('.site-brand')
            logo = brand.locator('img')
            assert logo.count() == 1 and logo.get_attribute('src').endswith('/sysc-logo.png')
            label = brand.locator('.sysc-brand-label')
            assert label.inner_text().strip() == 'Documentation'
            assert label.is_visible(), f'Documentation label hidden at {width}px'
            assert 'desktop' not in brand.inner_text().lower()

        if width == 390:
            page.goto(f'{BASE_URL}/docs/plugins/ui/', wait_until='networkidle')
            section = page.locator('article[role="main"] .prose > h2').first
            text = ' '.join(section.inner_text().split())
            assert text == 'ACCESSIBLE CONTROLS', f'unexpected section label: {text!r}'
            height, line_height = section.evaluate(
                "element => [element.clientHeight, parseFloat(getComputedStyle(element).lineHeight)]"
            )
            assert height <= line_height * 1.2, (
                f'plugin UI section wraps at 390px: {height}px / {line_height}px'
            )

    captures = Path(__file__).parent
    for name, route, width, height in (
        ('home-desktop.png', '/docs/', 1440, 1000),
        ('home-tablet.png', '/docs/', 720, 1000),
        ('home-mobile.png', '/docs/', 390, 844),
        ('sysc-shell-desktop.png', '/docs/components/sysc-shell/', 1440, 1000),
        ('sysc-shell-tablet.png', '/docs/components/sysc-shell/', 720, 1000),
        ('sysc-shell-mobile.png', '/docs/components/sysc-shell/', 390, 844),
        ('plugin-ui-desktop.png', '/docs/plugins/ui/', 1440, 1000),
        ('plugin-ui-tablet.png', '/docs/plugins/ui/', 720, 1000),
        ('plugin-ui-mobile.png', '/docs/plugins/ui/', 390, 844),
    ):
        page.set_viewport_size({'width': width, 'height': height})
        page.goto(f'{BASE_URL}{route}', wait_until='networkidle')
        page.screenshot(path=str(captures / name), full_page=True)

    browser.close()
