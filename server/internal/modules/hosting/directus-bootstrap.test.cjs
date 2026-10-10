const assert = require('node:assert/strict')
const { readFileSync } = require('node:fs')
const { join } = require('node:path')
const vm = require('node:vm')
const { test } = require('node:test')
const script = readFileSync(join(__dirname, 'directus-bootstrap.js'), 'utf8')

async function exercise(existing, failLogin = false) {
  const calls = []
  const process = { env: { ADMIN_EMAIL: 'test@example.com', ADMIN_PASSWORD: 'secret-password' }, exitCode: 0 }
  const logs = []
  await vm.runInNewContext(script, {
    process,
    AbortSignal,
    console: { log: (value) => logs.push(value), error: (value) => logs.push(value) },
    fetch: async (url, init) => {
      assert.ok(url.startsWith('http://127.0.0.1:8055/'))
      calls.push({ path: url.replace('http://127.0.0.1:8055', ''), init })
      if (url.endsWith('/auth/login')) return { ok: !failLogin, json: async () => ({ data: { access_token: 'local-instance-token' } }) }
      assert.equal(init.headers.Authorization, 'Bearer local-instance-token')
      if (init.method === 'GET') return { ok: true, json: async () => ({ data: existing.map((collection) => ({ collection })) }) }
      return { ok: true, json: async () => ({ data: JSON.parse(init.body) }) }
    },
  })
  return { calls, process, logs }
}
test('initializes four content collections with draft defaults and reuses Directus editors', async () => {
  const { calls, process } = await exercise([])
  assert.equal(process.exitCode, 0)
  const writes = calls.filter((call) => call.init.method === 'POST' && call.path !== '/auth/login')
  assert.equal(writes.length, 4)
  for (const write of writes) {
    assert.equal(write.path, '/collections')
    const collection = JSON.parse(write.init.body)
    assert.ok(collection.fields.some((field) => field.field === 'id' && field.schema.is_primary_key))
    if (collection.collection !== 'site_settings') assert.equal(collection.fields.find((field) => field.field === 'status').schema.default_value, 'draft')
  }
  assert.equal(JSON.parse(writes[0].init.body).fields.find((field) => field.field === 'content').meta.interface, 'input-rich-text-html')
  assert.ok(!calls.some((call) => call.path.includes('permissions')))
})
test('retries create only missing collections and preserve existing tenant schemas', async () => {
  const { calls } = await exercise(['site_articles', 'site_pages', 'site_settings'])
  const writes = calls.filter((call) => call.path === '/collections' && call.init.method === 'POST')
  assert.equal(writes.length, 1)
  assert.equal(JSON.parse(writes[0].init.body).collection, 'site_navigation')
})
test('failed authentication stops initialization and never logs credentials', async () => {
  const { calls, process, logs } = await exercise([], true)
  assert.equal(process.exitCode, 1)
  assert.equal(calls.length, 1)
  assert.ok(!logs.join('').includes('secret-password'))
})
