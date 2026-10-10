/**
 * 登录页 Live2D 控制器
 * ------------------------------------------------------------
 * 移植自 Pets-Live2D/preview.js 的自定义参数驱动方案：
 * 在 Cubism Core 的 beforeModelUpdate 前直接写入参数，动作由
 * motion-library.json 的关键帧插值得到，不走 pixi-live2d-display
 * 自带的 motion / 物理系统（模型加载后会停掉自带动作与自然摆动）。
 *
 * 模型设计空间为 625×559：猫、狗两个模型都保持原画坐标尺度，
 * 猫在左半幅、狗在右半幅，透明区域重叠在同一舞台上。
 */

const DESIGN_W = 625
const DESIGN_H = 559
const BASE = '/live2d'
/** 两只扩展模型共存所需的 Core 堆预留（参考 Pets-Live2D 经验值） */
const CORE_HEAP_BYTES = 64 * 1024 * 1024

export type PetKey = 'cat' | 'dog'
export type ActionType =
  | 'pet'
  | 'wink'
  | 'happy'
  | 'ears'
  | 'nod'
  | 'shake'
  | 'sleepy'
  | 'nuzzle'
  | 'stretch'
  | 'curious'

interface MotionSpec {
  duration: number
  text?: string
  mirrorTilt?: boolean
  frames: Array<[number, Record<string, number>]>
}

interface ActorEvent {
  type: ActionType
  at: number
  duration: number
}

interface ActorState {
  key: PetKey
  model: any
  /** 当前平滑后的水平转头（度） */
  turn: number
  /** 当前平滑后的纵向歪头（度） */
  tilt: number
  hovered: boolean
  event: ActorEvent | null
  /** 下一次随机眨眼的时刻（秒） */
  blinkAt: number
  /** 本次眨眼开始时刻（秒） */
  blinkStart: number
  /** 2.5s 内连续摸摸次数（≥3 升级为撒娇） */
  petCount: number
  lastPet: number
}

export interface ActionResult {
  /** 实际播放的动作（连摸三次会被替换为 nuzzle） */
  type: ActionType
  text: string
  keys: PetKey[]
  /** 气泡展示时长（毫秒） */
  duration: number
}

export interface Live2DController {
  action: (type: ActionType, keys?: PetKey[]) => ActionResult | null
  setPointer: (x: number, y: number) => void
  setHovered: (key: PetKey | null) => void
  /** 鼠标重新移动时把瞌睡动作立刻叫醒 */
  clearSleepy: () => void
  hasActiveEvent: () => boolean
}

interface ControllerImpl extends Live2DController {
  attach: (host: HTMLElement) => void
  destroy: () => void
}

/* ----------------------- vendor 运行库（单例） ----------------------- */

let vendorPromise: Promise<void> | null = null

function loadVendorScripts(): Promise<void> {
  if (vendorPromise) return vendorPromise
  const load = (src: string) =>
    new Promise<void>((resolve, reject) => {
      const s = document.createElement('script')
      s.src = src
      s.async = false
      s.onload = () => resolve()
      s.onerror = () => reject(new Error(`Live2D 运行库加载失败: ${src}`))
      document.head.appendChild(s)
    })
  vendorPromise = (async () => {
    await load(`${BASE}/vendor/live2dcubismcore.min.js`)
    await load(`${BASE}/vendor/pixi.min.js`)
    await load(`${BASE}/vendor/cubism4.min.js`)
  })()
  return vendorPromise
}

/* ----------------------- 动作插值工具 ----------------------- */

const clamp = (v: number, lo: number, hi: number) => Math.max(lo, Math.min(hi, v))
const smoothstep = (x: number) => x * x * (3 - 2 * x)

/** 内置短动作（无 motion-library 关键帧）之外的气泡文案 */
const FALLBACK_TEXT: Partial<Record<ActionType, string>> = {
  pet: '再摸摸嘛 ♡',
  wink: '收到啦 ♡',
  ears: '听到你啦！',
  happy: '有你在，好开心',
}

function poseAt(spec: MotionSpec, t: number, key: PetKey): Record<string, number> {
  const defaults: Record<string, number> = {
    ParamAngleX: 0,
    ParamAngleY: 0,
    ParamAngleZ: 0,
    ParamEyeLOpen: 1,
    ParamEyeROpen: 1,
    // 柯基原画就是张嘴微笑
    ParamMouthOpenY: key === 'dog' ? 1 : 0,
    ParamEarWiggle: 0,
    ParamBreath: 0,
  }
  const frames = spec.frames
  let index = 0
  while (index < frames.length - 2 && t > frames[index + 1][0]) index += 1
  const [a, av] = frames[index]
  const [b, bv] = frames[index + 1]
  const u = smoothstep(clamp((t - a) / (b - a), 0, 1))
  const result: Record<string, number> = {}
  for (const id of new Set([...Object.keys(av), ...Object.keys(bv)])) {
    result[id] = (av[id] ?? defaults[id]) * (1 - u) + (bv[id] ?? defaults[id]) * u
  }
  // 猫咪贴贴时歪头方向取反（建模约定）
  if (spec.mirrorTilt && key === 'cat' && result.ParamAngleZ !== undefined) {
    result.ParamAngleZ *= -1
  }
  return result
}

/* ----------------------- 控制器构造 ----------------------- */

async function createController(host: HTMLElement): Promise<ControllerImpl> {
  await loadVendorScripts()

  // Core 堆预留只在页面生命周期内做一次（重复初始化无意义且可能抛错）
  const memFlag = window as unknown as { __pmCubismHeapReady?: boolean }
  if (!memFlag.__pmCubismHeapReady) {
    Live2DCubismCore.Memory.initializeAmountOfMemory(CORE_HEAP_BYTES)
    memFlag.__pmCubismHeapReady = true
  }

  const reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches

  const app = new PIXI.Application({
    width: DESIGN_W,
    height: DESIGN_H,
    backgroundAlpha: 0,
    antialias: true,
    resolution: Math.min(window.devicePixelRatio || 1, 2),
    autoDensity: true,
  })
  host.appendChild(app.view)

  // 地面软投影：与模型同尺度，跟着舞台一起缩放
  const shadows = new PIXI.Graphics()
  shadows.beginFill(0xcf9460, 0.22)
  shadows.drawEllipse(161, 515, 125, 10)
  shadows.drawEllipse(432, 516, 128, 10)
  shadows.endFill()
  app.stage.addChild(shadows)

  const libraryResp = await fetch(`${BASE}/motion-library.json`)
  if (!libraryResp.ok) throw new Error('动作库加载失败')
  const library = (await libraryResp.json()) as Record<string, MotionSpec>

  let clock = 0
  const pointer = { x: 0, y: 0 }
  const actors = {} as Record<PetKey, ActorState>

  for (const key of ['cat', 'dog'] as PetKey[]) {
    const model = await PIXI.live2d.Live2DModel.from(`${BASE}/${key}/${key}.model3.json`, {
      autoInteract: false,
      autoUpdate: true,
    })
    model.internalModel.motionManager.stopAllMotions()
    model.internalModel.focusController.focus(0, 0, true)
    // 停掉适配层自带的自然摆动与视线，全部改由本控制器写参数
    model.internalModel.updateNaturalMovements = () => {}
    model.internalModel.updateFocus = () => {}
    model.scale.set(DESIGN_W / model.internalModel.width)
    app.stage.addChild(model)

    const state: ActorState = {
      key,
      model,
      turn: 0,
      tilt: 0,
      hovered: false,
      event: null,
      blinkAt: clock + (key === 'cat' ? 3.2 : 4.7),
      blinkStart: -100,
      petCount: 0,
      lastPet: -100,
    }
    actors[key] = state

    model.internalModel.on('beforeModelUpdate', () => {
      const cm = model.internalModel.coreModel
      let blink = 1
      if (!reduced) {
        const t = clock - state.blinkStart
        if (t >= 0 && t < 0.22) {
          blink = t < 0.07 ? 1 - t / 0.07 : (t - 0.07) / 0.15
        }
      }
      let left = blink
      let right = blink
      let ear = state.hovered ? Math.sin(clock * 7) * 0.28 : 0
      let mouth = key === 'dog' ? 1 : 0
      let turn = state.turn
      let nod = 0
      let tilt = state.tilt
      let breath = reduced ? 0 : (1 - Math.cos((clock * 2 * Math.PI) / 5.7)) * 0.35
      if (state.hovered) {
        // 被盯住时呼吸稍微明显一点
        breath = Math.max(breath, 0.22 + Math.sin(clock * 3.2) * 0.1)
      }

      if (state.event && library[state.event.type]) {
        const e = state.event
        const t = clock - e.at
        const spec = library[e.type]
        if (t > e.duration) {
          state.event = null
        } else {
          const pose = poseAt(spec, t, key)
          const fade = Math.min(1, t / 0.25, (e.duration - t) / 0.35)
          turn *= 1 - fade
          tilt *= 1 - fade
          turn += pose.ParamAngleX ?? 0
          nod = pose.ParamAngleY ?? 0
          tilt += pose.ParamAngleZ ?? 0
          left = pose.ParamEyeLOpen ?? blink
          right = pose.ParamEyeROpen ?? blink
          mouth = pose.ParamMouthOpenY ?? mouth
          ear = pose.ParamEarWiggle ?? 0
          breath = pose.ParamBreath ?? breath
        }
      } else if (state.event) {
        // 内置短动作：wink / pet / happy / ears
        const e = state.event
        const t = clock - e.at
        if (t > e.duration) {
          state.event = null
        } else {
          const pulse = Math.sin((Math.PI * t) / e.duration)
          if (e.type === 'wink') left = 1 - Math.min(1, pulse * 1.5)
          if (e.type === 'pet' || e.type === 'happy') {
            left = right = 1 - pulse * 0.8
            tilt += pulse * (key === 'cat' ? -2 : 2)
            mouth = key === 'cat' ? pulse * 0.45 : 1
          }
          if (e.type === 'ears' || e.type === 'pet') ear = Math.sin(t * 10) * pulse
        }
      }

      cm.setParameterValueById('ParamAngleX', clamp(turn, -8, 8))
      cm.setParameterValueById('ParamAngleY', clamp(nod, -8, 8))
      cm.setParameterValueById('ParamAngleZ', clamp(tilt, -6, 6))
      cm.setParameterValueById('ParamEyeLOpen', clamp(left, 0, 1))
      cm.setParameterValueById('ParamEyeROpen', clamp(right, 0, 1))
      cm.setParameterValueById('ParamMouthOpenY', clamp(mouth, 0, 1))
      cm.setParameterValueById('ParamBreath', clamp(breath, 0, 1))
      cm.setParameterValueById('ParamEarWiggle', clamp(ear, -1, 1))
    })
  }

  /* ----------------------- 帧驱动：平滑跟随 + 随机眨眼 ----------------------- */
  let last = performance.now()
  app.ticker.add(() => {
    const now = performance.now()
    const dt = Math.min((now - last) / 1000, 0.08)
    last = now
    clock += dt
    for (const s of Object.values(actors)) {
      const targetTurn = pointer.x * 7
      const targetTilt = pointer.y * 1.8
      const k = 1 - Math.exp(-dt * 8)
      s.turn += (targetTurn - s.turn) * k
      s.tilt += (targetTilt - s.tilt) * k
      if (!reduced && clock > s.blinkAt) {
        s.blinkStart = clock
        s.blinkAt = clock + 3.3 + Math.random() * 3.4
      }
    }
  })

  if (import.meta.env.DEV) {
    ;(window as unknown as { __pmLive2D?: unknown }).__pmLive2D = {
      actors,
      get clock() {
        return clock
      },
    }
  }

  const impl: ControllerImpl = {
    action(type, keys) {
      const list = (keys ? keys.map((k) => actors[k]) : Object.values(actors)).filter(Boolean)
      if (!list.length) return null
      let actual: ActionType = type
      for (const s of list) {
        let t: ActionType = type
        if (type === 'pet') {
          s.petCount = clock - s.lastPet < 2.5 ? s.petCount + 1 : 1
          s.lastPet = clock
          if (s.petCount >= 3) {
            t = 'nuzzle'
            actual = 'nuzzle'
          }
        }
        const spec = library[t]
        s.event = { type: t, at: clock, duration: spec?.duration ?? (t === 'wink' ? 0.8 : 1.5) }
      }
      const spec = library[actual]
      const durationMs = Math.max(1700, (spec?.duration ?? 1.6) * 1000)
      return {
        type: actual,
        text: spec?.text ?? FALLBACK_TEXT[actual] ?? '♡',
        keys: list.map((s) => s.key),
        duration: durationMs,
      }
    },
    setPointer(x, y) {
      pointer.x = x
      pointer.y = y
    },
    setHovered(key) {
      for (const s of Object.values(actors)) s.hovered = s.key === key
    },
    clearSleepy() {
      for (const s of Object.values(actors)) {
        if (s.event?.type === 'sleepy') s.event = null
      }
    },
    hasActiveEvent() {
      return Object.values(actors).some((s) => s.event !== null)
    },
    attach(nextHost) {
      if (app.view.parentElement !== nextHost) nextHost.appendChild(app.view)
    },
    destroy() {
      try {
        for (const s of Object.values(actors)) {
          // 解除 Ticker.shared 上的自动更新注册，避免销毁后仍有回调
          s.model.internalModel.autoUpdate = false
        }
        app.destroy(true, { children: true, texture: true, baseTexture: true })
      } catch (e) {
        // 销毁失败不阻塞页面
        console.error('Live2D 控制器销毁异常', e)
      }
    },
  }

  return impl
}

/* ----------------------- 单例 acquire / release -----------------------
 * React StrictMode（以及 HMR）会让 effect 经历 mount→cleanup→mount，
 * 复用同一个控制器可避免短期内 4 个模型并发加载撑爆 Core 堆。
 * -------------------------------------------------------------------- */

let active: ControllerImpl | null = null
let inflight: Promise<ControllerImpl> | null = null
let releaseTimer: number | null = null

export function acquireController(host: HTMLElement): Promise<Live2DController> {
  if (releaseTimer !== null) {
    window.clearTimeout(releaseTimer)
    releaseTimer = null
  }
  if (active) {
    active.attach(host)
    return Promise.resolve(active)
  }
  if (inflight) {
    return inflight.then((c) => {
      c.attach(host)
      return c
    })
  }
  inflight = createController(host).then((c) => {
    active = c
    inflight = null
    return c
  })
  return inflight
}

export function releaseController(): void {
  if (releaseTimer !== null) window.clearTimeout(releaseTimer)
  releaseTimer = window.setTimeout(() => {
    releaseTimer = null
    const c = active
    active = null
    c?.destroy()
  }, 800)
}
