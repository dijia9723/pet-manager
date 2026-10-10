/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** 应用标题 */
  readonly VITE_APP_TITLE: string
  /** 本地缓存命名空间前缀 */
  readonly VITE_APP_NAMESPACE: string
  /** 接口前缀 */
  readonly VITE_BASE_API: string
  /** dev 代理目标后端 */
  readonly VITE_PROXY_TARGET?: string
  /** dev server 端口 */
  readonly VITE_PORT?: string
  /** 资源公共路径 */
  readonly VITE_BASE?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}

/* 登录页 Live2D 场景全局运行库（public/live2d/vendor，本地自包含，非 npm 依赖） */
declare const PIXI: any
declare const Live2DCubismCore: any
