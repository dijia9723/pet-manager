// Runs inside this site's Directus container, using its API and Studio editors.
async function bootstrap() {
  let token
  async function api(route, method = 'GET', body) {
    const response = await fetch('http://127.0.0.1:8055' + route, {
      method,
      headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: 'Bearer ' + token } : {}) },
      ...(body === undefined ? {} : { body: JSON.stringify(body) }),
      signal: AbortSignal.timeout(15000),
    })
    if (!response.ok) throw new Error('CMS initialization failed')
    const result = await response.json()
    return result.data
  }
  const auth = await api('/auth/login', 'POST', { email: process.env.ADMIN_EMAIL, password: process.env.ADMIN_PASSWORD })
  token = auth.access_token
  const collections = new Set((await api('/collections')).map((item) => item.collection))
  const id = { field: 'id', type: 'integer', schema: { is_primary_key: true, has_auto_increment: true }, meta: { hidden: true, interface: 'input', readonly: true } }
  const text = (field, label, extra = {}) => ({
    field, type: 'string', schema: { max_length: 255, ...extra },
    meta: { interface: 'input', translations: [{ language: 'zh-CN', translation: label }] },
  })
  const status = {
    field: 'status', type: 'string', schema: { default_value: 'draft', max_length: 32 },
    meta: { interface: 'select-dropdown', options: { choices: [{ text: '草稿', value: 'draft' }, { text: '已发布', value: 'published' }, { text: '归档', value: 'archived' }] } },
  }
  const content = { field: 'content', type: 'text', meta: { interface: 'input-rich-text-html', translations: [{ language: 'zh-CN', translation: '正文' }] } }
  const definitions = [
    {
      collection: 'site_articles', schema: {},
      meta: { icon: 'article', display_template: '{{title}}', archive_field: 'status', archive_value: 'archived', unarchive_value: 'draft', translations: [{ language: 'zh-CN', translation: '文章' }] },
      fields: [id, status, text('title', '标题', { is_nullable: false }), text('slug', '唯一标识', { is_unique: true }), text('summary', '摘要'), content,
        { field: 'published_at', type: 'timestamp', meta: { interface: 'datetime' } }],
    },
    {
      collection: 'site_pages', schema: {},
      meta: { icon: 'web', display_template: '{{title}}', archive_field: 'status', archive_value: 'archived', unarchive_value: 'draft', translations: [{ language: 'zh-CN', translation: '页面' }] },
      fields: [id, status, text('title', '标题', { is_nullable: false }), text('slug', '唯一标识', { is_unique: true }), content],
    },
    {
      collection: 'site_navigation', schema: {},
      meta: { icon: 'menu', display_template: '{{label}}', sort_field: 'sort', translations: [{ language: 'zh-CN', translation: '导航' }] },
      fields: [id, status, text('label', '名称'), text('url', '链接'),
        { field: 'sort', type: 'integer', schema: { default_value: 0 }, meta: { interface: 'input', hidden: true } }],
    },
    {
      collection: 'site_settings', schema: {},
      meta: { icon: 'settings', singleton: true, translations: [{ language: 'zh-CN', translation: '站点设置' }] },
      fields: [id, text('name', '网站名称'), text('description', '描述'), text('contact_email', '联系邮箱'), text('seo_title', 'SEO 标题'), text('seo_description', 'SEO 描述')],
    },
  ]
  for (const definition of definitions) {
    if (!collections.has(definition.collection)) await api('/collections', 'POST', definition)
  }
  // Public permissions stay closed; tenant decides which published content to expose.
  console.log('CMS base collections initialized')
}
bootstrap().catch(() => { console.error('CMS initialization failed; check instance authentication and schema permissions'); process.exitCode = 1 })
