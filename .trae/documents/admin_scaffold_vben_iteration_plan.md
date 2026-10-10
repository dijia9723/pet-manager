# Admin 前端脚手架对标 Vben Pro 迭代计划

## 一、调研结论（现状 vs Vben 5）

技术路线决策：**保留 React19 + Vite6 + Antd5 + ProComponents + zustand + react-query 技术栈**，用 React 生态的等价实现对标 [Vben Pro](https://doc.vben.pro/) 的架构理念与体验（项目已定稿 React，W6+ 小程序亦为 React/Taro，不切换 Vue）。

### 已具备（保留，不重复造轮子）
- axios 拦截器链（可插拔）、401 静默 refresh + 单飞、统一错误提示、统一响应体解包
- JWT 双 token + zustand persist、后端菜单 `/auth/menus` 下发 + 本地路由表兜底
- 路由守卫（登录态 + 权限点/anyOf）、按钮级权限 PermButton
- 平台端/租户端双身份登录与落地页分流

### 差距（Vben 有、当前没有）
1. **单包 2.37MB**：所有页面静态 import，零 code-splitting；无构建分包
2. **无环境变量体系**：API 前缀 `/api/v1`、代理目标 `:8088`、标题均硬编码
3. **无多标签页**：Vben 最标志性能力（打开/关闭/关闭其他/刷新/固定/持久化/实例缓存）
4. **无偏好设置中心**：布局模式、主题色、暗色模式、水印、面包屑/页签开关均不可配
5. **顶栏无 widgets**：全屏、主题切换、页面刷新、偏好入口
6. **无路由懒加载/切换进度条/动态页面标题**
7. **无 ESLint/Prettier 工程规范**（Vben：eslint/oxlint/prettier 体系）
8. 登录页为简单居中卡片，与 Vben 品牌分屏登录页差距明显

### 明确不对标（与本项目不适配，不做）
- pnpm monorepo + turbo（单 admin 应用，server/admin 已并列管理）
- Nitro Mock（已有真实 Gin 后端）
- i18n 多语言（纯中文产品）
- Tailwind + shadcn（已选 Antd，不混用样式体系；暗色走 antd `darkAlgorithm`）
- 完全后端动态路由（后端菜单契约仅 key/title/path/icon；当前"本地路由表 + 后端菜单过滤"等同 Vben **mixed 模式**，本次只强化路由 meta，不改后端契约）
- PWA、锁屏、全局搜索、通知中心、生产期 `_app-config.js`（无业务/运维诉求）

### 前置风险（执行前必须处理）
- IDE 中 `admin/src/router/routes.tsx` 有**未保存改动**（新增 `@/pages/tenant/packages` 套餐路由，但该页面目录为空、磁盘版无此引用）。磁盘代码 `tsc`/`vite build` 均通过。
  - **执行前请先在 IDE 中保存或撤销该文件的未保存改动**；重构后新增套餐页面只需新建 1 个路由 module + 1 个页面文件（新结构下有现成范式）。

---

## 二、目标目录结构（仅列新增/改动）

```
admin/
  .env                              # 新增：VITE_APP_TITLE / VITE_APP_NAMESPACE
  .env.development                  # 新增：VITE_PORT / VITE_BASE_API / VITE_PROXY_TARGET
  .env.production                   # 新增：VITE_BASE_API / VITE_BASE
  eslint.config.js                  # 新增：ESLint9 flat config
  .prettierrc.json / .prettierignore# 新增
  vite.config.ts                    # 改：env 驱动 port/proxy/base + manualChunks 分包
  index.html                        # 改：%VITE_APP_TITLE% + 首屏 loading
  src/
    config/index.ts                 # 新增：import.meta.env 类型化出口（title/namespace/baseApi/proxyTarget）
    constants/index.ts              # 新增：store 存储键前缀、固定 tab 等常量
    styles/global.css               # 新增：全局基线、nprogress 配色、暗色微调、登录页响应式、切页动画
    api/client.ts                   # 改：baseURL 取 config
    utils/token.ts                  # 改：localStorage 键名加 namespace 前缀
    store/
      auth.ts                       # 改：persist key 加 namespace 前缀（一次性失效需重登）
      preferences.ts                # 新增：布局/主题/页签/面包屑/水印偏好（persist）
      tabs.ts                       # 新增：多标签页状态与动作（persist）
    router/
      types.ts                      # 新增：AppRouteModule（含 title/icon/perm/anyOf/affixTab/keepAlive/order/hideInMenu）
      helpers.ts                    # 新增：聚合路由→菜单树、按 path 查 meta
      routes/
        modules/dashboard.tsx       # 新增（由现 routes.tsx 拆出，lazy loader）
        modules/tenant.tsx          # 新增（users/roles）
        modules/platform.tsx        # 新增（tenants/audit/permissions）
        index.ts                    # 新增：汇总 + 排序
      index.tsx                     # 改：统一 lazy + Suspense + 进度条 + 动态标题
      guard.tsx                     # 保留（HomeRedirect）
    components/
      PageLoading.tsx               # 新增：居中 Spin（Suspense fallback）
      KeepAliveOutlet.tsx           # 新增：按 path 缓存页面实例，非活动页 display:none
      RouteTracker.tsx              # 新增：页面挂载后结束进度条 + 写 document.title
      ChunkErrorBoundary.tsx        # 新增：发版后旧 chunk 失效时自动刷新一次
    layouts/
      MainLayout.tsx                # 改：接 preferences 响应式布局/暗色/水印 + 挂 TabBar/HeaderActions
      components/
        TabsBar.tsx                 # 新增：antd Tabs 页签条（右键菜单：关闭/左/右/其他/全部 + 刷新）
        HeaderActions.tsx           # 新增：刷新页、全屏、明暗切换、偏好设置
        PreferencesDrawer.tsx       # 新增：偏好设置抽屉（含恢复默认）
    hooks/useFullscreen.ts          # 新增
    pages/login/index.tsx           # 改：Vben 风格左品牌右表单分屏，响应式
    main.tsx                        # 改：挂 global.css、ConfigProvider 联动 preferences（算法/主色）
  doc/architecture.md               # 改：同步前端工程结构与偏好/页签机制说明
```

## 三、实施步骤（依赖排序）

1. **环境与构建基座**：三个 `.env` + `src/config/index.ts` + `vite-env.d.ts` 扩 `ImportMetaEnv`；`vite.config.ts` 改为 `loadEnv` 驱动（端口/代理/base），`manualChunks` 拆 react-vendor / antd-vendor / icons 三块；`index.html` 标题与首屏 loading。
2. **存储命名空间收敛**：`constants` 定义前缀；改 `utils/token.ts`、`store/auth.ts` 持久化键；`api/client.ts` baseURL 取 config。
3. **路由 meta 化 + 懒加载**：新增 router/types/helpers 与 routes/modules 拆分（页面全部 `React.lazy`，带 chunk 失效重试）；`router/index.tsx` 用统一 Suspense + PageLoading；现有 routes.tsx 内容迁入 modules 后删除。
4. **进度条与动态标题**：引入 `nprogress`，RouteTracker + 位置 effect 控制 start/done；按 meta 写 `document.title`（`页面名 · 应用名`）。
5. **preferences store + 主题联动**：zustand persist；main.tsx 订阅后下发 ConfigProvider（`darkAlgorithm`/colorPrimary/圆角），ProLayout 响应 layout/navTheme/水印。
6. **tabs store + TabsBar**：路由变化自动开签；关闭/关闭左侧/右侧/其他/全部、刷新（remount key）、固定首页不可关、刷新持久化；右键上下文菜单。
7. **KeepAliveOutlet**：活动路由渲染、已开 keepAlive 页面以 `display:none` 保活；关签清缓存；meta `keepAlive:false` 可逐页关闭（未来详情页用）。含已知限制说明（见风险）。
8. **HeaderActions + PreferencesDrawer**：刷新页、全屏（原生 Fullscreen API）、明暗一键切换、偏好抽屉（布局 side/mix/top、主色 ColorPicker、暗色、水印、页签开关、面包屑开关、恢复默认）。
9. **登录页分屏改版**：左品牌区（产品名+能力点+渐变，暗色自适应，窄屏隐藏）右表单，登录逻辑不动。
10. **ESLint9 + Prettier**：flat config（typescript-eslint、react-hooks、react-refresh、prettier 兼容），加 lint/format 脚本，全量 lint:fix 并收尾手工问题。
11. **文档同步**：更新 `doc/architecture.md` 前端结构、环境变量、偏好/页签/缓存机制。

## 四、依赖与考虑

- 新增运行时依赖：`nprogress`；devDeps：`@types/nprogress`、`eslint@9`、`typescript-eslint`、`eslint-plugin-react-hooks`、`eslint-plugin-react-refresh`、`eslint-config-prettier`、`prettier`。全部装在 admin 本地（npm，Node v22.23 / npm 10.9 已探测）。
- 不改任何后端接口与契约；业务页面逻辑零改动，仅改其挂载方式（lazy）与外层布局。
- persist 键改名后浏览器需重新登录一次（开发期可接受）。
- 水印用 ProLayout 原生 `waterMarkProps`（用户名+租户名），零新依赖。
- 页面样式体系仍为 antd token，global.css 只放基线与少量覆盖，不引入 Tailwind。

## 五、验证

- `npx tsc --noEmit`、`npx eslint .`、`npm run build` 全绿；build 产物由 1 个 2.37MB 包变为 1 个入口小 chunk + 页面级 chunks + vendor 分包。
- 浏览器冒烟（平台端 + 租户端各登录一次）：
  - 懒加载：Network 可见独立页面 chunk；进度条与标题随切换变化
  - 页签：开/关/关其他/右键菜单/刷新/固定首页；在列表筛选或表单输入后切走再切回，**状态保留**；刷新页面后已开页签恢复
  - 偏好：明暗模式、主色、三种布局、水印即时生效，刷新后保持；恢复默认可用
  - 全屏、退出登录、403/404、暗色下登录页显示正常
- 改 `.env.development` 代理目标后能连通 Gin（验证 env 链路，默认仍 127.0.0.1:8088）。

## 六、风险与兜底

- **KeepAlive 限制**：React 无原生 keep-alive，display:none 保活下，被缓存组件若直接读 router 的 location/params，读到的是当前 URL。当前全部为无参列表页（ProTable 内部状态驱动），不受影响；代码注释中写明"未来带 `:id` 的详情页设 `keepAlive:false`"。若实测出现异常页面，逐页关闭缓存即可，不影响页签 UI。
- **IDE 未保存缓冲**：见前置风险，执行前需用户保存/撤销 routes.tsx；否则重构会与缓冲冲突。
- **ESLint 规则摩擦**：以"警告不阻断、错误全修"为原则配置，避免规则噪音影响 W2 业务开发节奏。
