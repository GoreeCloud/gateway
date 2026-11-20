// Browser acceptance uses a synthetic loopback HTTP server and fake API data.
// It never contacts Caddy, protected DNS, production hosts or credentials.
import { test, expect } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'
import { createServer } from 'node:http'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const distUrl = new URL('../dist/', import.meta.url)
const token = 'fixture-test-operator-token-32-chars'
let server
let baseURL

function serveFixture(request, response) {
  const name = request.url || '/'
  const authorized = request.headers.authorization === 'Bearer ' + token
  const common = { 'Cache-Control': 'no-store', 'X-Content-Type-Options': 'nosniff' }
  if (name === '/api/status' || name === '/api/sites') {
    response.writeHead(authorized ? 200 : 401, { ...common, 'Content-Type': 'application/json; charset=utf-8' })
    if (!authorized) return response.end('{"error":"Unauthorized"}')
    if (name === '/api/status') return response.end(JSON.stringify({
      name: 'GoreeCloud Gateway', engine: 'Caddy v2', version: 'synthetic-fixture',
      siteCount: 1, running: false, writable: true, management: 'loopback',
    }))
    if (request.method === 'POST') {
      response.statusCode = 201
      return response.end('{"host":"added.example.test","upstream":"http://127.0.0.1:3031"}')
    }
    return response.end('{"sites":[{"host":"fixture.example.test","upstream":"http://127.0.0.1:3030"}]}')
  }
  const assets = {
    '/': ['index.html', 'text/html; charset=utf-8'],
    '/index.html': ['index.html', 'text/html; charset=utf-8'],
    '/assets/app.js': ['assets/app.js', 'text/javascript'],
    '/assets/app.css': ['assets/app.css', 'text/css'],
  }
  if (!(name in assets) || request.method !== 'GET') {
    response.writeHead(404, { ...common, 'Content-Type': 'text/plain' })
    return response.end('not found')
  }
  const [filename, contentType] = assets[name]
  try {
    const bytes = readFileSync(fileURLToPath(new URL(filename, distUrl)))
    response.writeHead(200, { ...common, 'Content-Type': contentType })
    response.end(bytes)
  } catch {
    response.writeHead(404, { ...common, 'Content-Type': 'text/plain' })
    response.end('not found')
  }
}

test.beforeAll(async () => {
  server = createServer(serveFixture)
  await new Promise((resolve, reject) => {
    server.once('error', reject)
    server.listen(0, '127.0.0.1', resolve)
  })
  baseURL = 'http://127.0.0.1:' + server.address().port + '/'
})

test.afterAll(async () => {
  if (server) await new Promise(resolve => server.close(resolve))
})

async function unlock(page, secret = token) {
  await page.goto(baseURL, { waitUntil: 'networkidle' })
  await page.getByLabel('Management token').fill(secret)
  await page.getByRole('button', { name: /Unlock console/ }).click()
}

test('keyboard focus stays inside the route dialog and returns to its trigger', async ({ page }) => {
  const failures = []
  page.on('pageerror', error => failures.push(error.message))
  await unlock(page)
  await expect(page.getByRole('heading', { name: /Gateway overview/ })).toBeVisible()
  await page.getByRole('button', { name: 'Sites & routes' }).click()
  const addRoute = page.getByRole('button', { name: 'Add site' }).first()
  await addRoute.click()

  const dialog = page.getByRole('dialog', { name: 'Add a new site' })
  await expect(dialog).toBeVisible()
  await expect(page.locator('#main-content')).toHaveAttribute('inert', '')
  await expect(page.locator('aside.sidebar')).toHaveAttribute('inert', '')
  await expect(dialog.getByRole('textbox', { name: 'Public hostname' })).toBeFocused()
  await expect(dialog.getByRole('textbox', { name: 'Public hostname' }))
    .toHaveAttribute('aria-describedby', 'hostname-help')

  await page.keyboard.press('Shift+Tab')
  await expect(dialog.getByRole('button', { name: 'Close dialog' })).toBeFocused()
  await page.keyboard.press('Shift+Tab')
  await expect(dialog.getByRole('button', { name: 'Create site' })).toBeFocused()
  await page.keyboard.press('Tab')
  await expect(dialog.getByRole('button', { name: 'Close dialog' })).toBeFocused()
  await page.keyboard.press('Escape')
  await expect(dialog).toHaveCount(0)
  await expect(page.locator('#main-content')).not.toHaveAttribute('inert', '')
  await expect(addRoute).toBeFocused()
  expect(failures).toEqual([])
})

test('mobile navigation is hidden when closed and restores keyboard focus', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 800 })
  await unlock(page)
  await expect(page.getByRole('heading', { name: /Gateway overview/ })).toBeVisible()
  const sidebar = page.locator('aside.sidebar')
  await expect(sidebar).toHaveCSS('visibility', 'hidden')
  const trigger = page.getByRole('button', { name: 'Open navigation' })
  await trigger.click()
  await expect(sidebar).toHaveCSS('visibility', 'visible')
  await expect(page.locator('#main-content')).toHaveAttribute('inert', '')
  await page.keyboard.press('Escape')
  await expect(sidebar).toHaveCSS('visibility', 'hidden')
  await expect(trigger).toBeFocused()
  await expect(page.locator('#main-content')).not.toHaveAttribute('inert', '')
})

test('successful synthetic route save restores focus when busy clears', async ({ page }) => {
  await unlock(page)
  await page.getByRole('button', { name: 'Sites & routes' }).click()
  const addRoute = page.getByRole('button', { name: 'Add site' }).first()
  await addRoute.click()
  const dialog = page.getByRole('dialog', { name: 'Add a new site' })
  await dialog.getByRole('textbox', { name: 'Public hostname' }).fill('added.example.test')
  await dialog.getByRole('textbox', { name: 'Upstream address' }).fill('http://127.0.0.1:3031')
  await dialog.getByRole('button', { name: 'Create site' }).click()
  await expect(dialog).toHaveCount(0)
  await expect(addRoute).toBeFocused()
  await expect(page.getByRole('status')).toContainText('Route created')
})

test('invalid local fixture token does not unlock the UI or store credentials', async ({ page }) => {
  await unlock(page, 'wrong-operator-token-32-characters')
  await expect(page.getByRole('alert')).toContainText('Unauthorized')
  await expect(page.getByRole('heading', { name: /Gateway overview/ })).toHaveCount(0)
  expect(await page.evaluate(() => Object.keys(localStorage))).toEqual([])
})

test('automated WCAG A/AA audit of synthetic login and console states', async ({ page }) => {
  async function audit(label) {
    const results = await new AxeBuilder({ page })
      .withTags(['wcag2a', 'wcag2aa', 'wcag21aa', 'wcag22aa'])
      .analyze()
    const violations = results.violations.map(v => ({
      id: v.id, impact: v.impact,
      nodes: v.nodes.slice(0, 3).map(n => ({ target: n.target, summary: n.failureSummary })),
    }))
    expect(violations, label + ': automated WCAG A/AA audit violations').toEqual([])
  }
  await page.goto(baseURL)
  await audit('login')
  await unlock(page)
  await expect(page.getByRole('heading', { name: /Gateway overview/ })).toBeVisible()
  await audit('overview')
  await page.getByRole('button', { name: 'Sites & routes' }).click()
  await audit('routes')
  await page.getByRole('button', { name: 'Add site' }).first().click()
  await audit('route dialog')
  await page.getByRole('dialog').getByRole('button', { name: 'Cancel' }).click()
  await page.getByRole('button', { name: 'Security' }).click()
  await audit('security')
  await page.getByRole('button', { name: 'Settings' }).click()
  await audit('settings')
  await page.getByRole('button', { name: 'Toggle color scheme' }).click()
  await audit('settings in light theme')
  await page.getByRole('button', { name: 'Sites & routes' }).click()
  await audit('routes in light theme')
  await page.setViewportSize({ width: 390, height: 800 })
  await audit('mobile routes in light theme')
  await page.getByRole('button', { name: 'Open navigation' }).click()
  await audit('mobile navigation open')
})

test('canonical Iris roles render dark/light correctly with compact viewport containment', async ({ page }) => {
  await unlock(page)
  await expect(page.getByRole('heading', { name: /Gateway overview/ })).toBeVisible()
  const roles = () => page.evaluate(() => {
    const root = document.documentElement
    const css = getComputedStyle(root)
    return {
      sourceTheme: root.dataset.gcTheme,
      localTheme: root.dataset.theme,
      sourceBrand: css.getPropertyValue('--gc-brand').trim(),
      localBrand: css.getPropertyValue('--brand').trim(),
      sourceCanvas: css.getPropertyValue('--gc-canvas').trim(),
      localCanvas: css.getPropertyValue('--canvas').trim(),
      scheme: css.colorScheme,
      clientWidth: root.clientWidth, documentWidth: root.scrollWidth,
    }
  })
  const dark = await roles()
  expect(dark.sourceTheme).toBe('dark')
  expect(dark.localTheme).toBe('dark')
  expect(dark.sourceBrand).toBe('#BFB8FF')
  expect(dark.localBrand).toBe(dark.sourceBrand)
  expect(dark.sourceCanvas).toBe('#0D121D')
  expect(dark.localCanvas).toBe(dark.sourceCanvas)
  expect(dark.scheme).toContain('dark')

  await page.getByRole('button', { name: 'Toggle color scheme' }).click()
  await expect(page.locator('html')).toHaveAttribute('data-gc-theme', 'light')
  const light = await roles()
  expect(light.sourceBrand).toBe('#5146C8')
  expect(light.localBrand).toBe(light.sourceBrand)
  expect(light.sourceCanvas).toBe('#F6F7FB')
  expect(light.localCanvas).toBe(light.sourceCanvas)
  expect(light.scheme).toContain('light')

  for (const width of [390, 320]) {
    await page.setViewportSize({ width, height: 760 })
    const compact = await roles()
    expect(compact.documentWidth, width + 'px viewport must not overflow').toBeLessThanOrEqual(width + 1)
    await page.getByRole('button', { name: 'Open navigation' }).click()
    const withMenu = await roles()
    expect(withMenu.documentWidth, width + 'px open menu must not overflow').toBeLessThanOrEqual(width + 1)
    await page.keyboard.press('Escape')
    await expect(page.locator('aside.sidebar')).toHaveCSS('visibility', 'hidden')
  }
})

test('opt-in screenshot acceptance of synthetic Gateway desktop/mobile and explicit themes', async ({ page }, info) => {
  const folder = process.env.GC_QA_SCREENSHOTS_DIR
  test.skip(!folder, 'Visual evidence capture is opt-in, not an external service')
  // Run only when an already-existing absolute destination was selected;
  // never publish screenshots of actual sites, certificates or user accounts.
  if (!folder.startsWith('/')) throw Error('GC_QA_SCREENSHOTS_DIR must be absolute')
  const { statSync } = await import('node:fs')
  const { join } = await import('node:path')
  if (!statSync(folder).isDirectory()) throw Error('Screenshot directory does not exist')

  await page.setViewportSize({ width: 900, height: 760 })
  await unlock(page)
  await expect(page.getByRole('heading', { name: /Gateway overview/ })).toBeVisible()
  await page.screenshot({ path: join(folder, info.project.name + '-gateway-desktop-dark.png'), fullPage: true })

  await page.getByRole('button', { name: 'Toggle color scheme' }).click()
  await expect(page.locator('html')).toHaveAttribute('data-gc-theme', 'light')
  await page.getByRole('button', { name: 'Sites & routes' }).click()
  await page.screenshot({ path: join(folder, info.project.name + '-gateway-desktop-light-sites.png'), fullPage: true })

  await page.getByRole('button', { name: 'Toggle color scheme' }).click()
  await page.setViewportSize({ width: 390, height: 844 })
  await page.getByRole('button', { name: 'Open navigation' }).click()
  await page.waitForFunction(() => {
    const sidebar = document.querySelector('aside.sidebar')
    if (!sidebar) return false
    const bounds = sidebar.getBoundingClientRect()
    return Math.abs(bounds.left) <= 0.5 && bounds.right <= innerWidth + 0.5
  })
  await page.screenshot({ path: join(folder, info.project.name + '-gateway-mobile-dark-menu.png'), fullPage: true })
  await page.keyboard.press('Escape')
  await expect(page.locator('aside.sidebar')).toHaveCSS('visibility', 'hidden')

  await page.getByRole('button', { name: 'Toggle color scheme' }).click()
  await page.setViewportSize({ width: 320, height: 760 })
  await page.screenshot({ path: join(folder, info.project.name + '-gateway-compact-light-sites.png'), fullPage: true })
})
