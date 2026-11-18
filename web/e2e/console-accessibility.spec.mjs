// Browser acceptance uses a synthetic loopback HTTP server and fake API data.
// It never contacts Caddy, protected DNS, production hosts or credentials.
import { test, expect } from '@playwright/test'
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
