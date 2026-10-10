import { useCallback, useEffect, useRef, useState } from 'react'
import { AppstoreOutlined, SafetyCertificateOutlined, ShopOutlined } from '@ant-design/icons'
import { BrandLogo } from '@/components/BrandLogo'
import { FishboneMark, HeartMark, PawMark, SparkleMark } from './icons'
import {
  acquireController,
  releaseController,
  type ActionType,
  type Live2DController,
  type PetKey,
} from './live2dController'

/**
 * 小道具素材：本地透明 PNG（public/pets/），统一暖系绘本贴纸风。
 * 猫、狗已升级为真实 Live2D 模型（public/live2d/），鱼盘与骨头保留 CSS 贴纸。
 */
const FISH_IMG = '/pets/fish.png'
const BONE_IMG = '/pets/bone.png'

type PropActorId = 'fish' | 'bone'
type PetKind = 'paw' | 'heart'

interface PropReaction {
  actor: PropActorId
  /** 自增序号：连点同一道具时强制重播动画 */
  seq: number
}
interface BubbleState {
  seq: number
  text: string
  variant: PetKey | 'both'
}
interface Point {
  x: number
  y: number
}
interface PressState {
  key: PetKey
  start: number
  x: number
  y: number
  /** 长按是否已触发（触发后抬起不再执行点击） */
  fired: boolean
  timer: number
}

const FEATURES = [
  { icon: <ShopOutlined />, text: '租户 Schema 隔离，开通即得独立工作台' },
  { icon: <SafetyCertificateOutlined />, text: 'RBAC 角色权限，菜单与按钮级管控' },
  { icon: <AppstoreOutlined />, text: '繁育台账 · 工单引擎 · 标签打印一站式' },
]

/** 粒子 SVG（createElementNS 模板化，避免大段字符串拼接） */
const PARTICLE_SVG: Record<PetKind, string> = {
  heart:
    '<svg viewBox="0 0 64 64" fill="currentColor"><path d="M32 55C13.5 41.6 7.5 30.8 14.8 21.4 20.3 14.3 29 16.6 32 23c3-6.4 11.7-8.7 17.2-1.6C56.5 30.8 50.5 41.6 32 55Z"/></svg>',
  paw:
    '<svg viewBox="0 0 64 64" fill="currentColor"><ellipse cx="15.5" cy="25" rx="6.4" ry="7.4"/><ellipse cx="26.5" cy="17.5" rx="6.8" ry="7.8"/><ellipse cx="37.5" cy="17.5" rx="6.8" ry="7.8"/><ellipse cx="48.5" cy="25" rx="6.4" ry="7.4"/><path d="M32 29.5c-11.6 0-19.8 9.2-17.6 17.4 1.9 6.9 11.1 8.2 17.6 1.5 6.5 6.7 15.7 5.4 17.6-1.5C51.8 38.7 43.6 29.5 32 29.5Z"/></svg>',
}

const clampNum = (v: number, lo: number, hi: number) => Math.max(lo, Math.min(hi, v))

/**
 * 登录页左侧动态场景：
 * - 猫、狗为真实 Live2D 模型（同一 PIXI 舞台，保持原画坐标尺度）
 * - 鱼盘、骨头为 CSS 贴纸道具，各有点击反应
 *
 * 鼠标动作清单：
 * 1. 移动：猫狗浅转头 + 轻微歪头看向光标，分层视差
 * 2. 悬停：耳朵轻动、呼吸变明显
 * 3. 单击（摸摸头）：眯眼微笑、耳朵动、冒爱心；2.5s 内连摸三次 → 贴贴撒娇
 * 4. 双击：开心一下
 * 5. 右键：眨一只眼
 * 6. 长按（约 0.6s）：伸个懒腰
 * 7. 快速横向甩动鼠标：一起摇摇头
 * 8. 点击空白处：最近的小家伙好奇歪头 + 冒小粒子
 * 9. 进入场景：点头打招呼
 * 10. 鼠标静止约 8 秒：打个瞌睡，再次移动立刻叫醒
 */
export function PetScene() {
  const sceneRef = useRef<HTMLDivElement>(null)
  const hostRef = useRef<HTMLDivElement>(null)
  const particlesRef = useRef<HTMLDivElement>(null)
  const controllerRef = useRef<Live2DController | null>(null)
  const seqRef = useRef(0)
  const pressRef = useRef<PressState | null>(null)
  const lastClientRef = useRef<Point>({ x: 0, y: 0 })
  const lastMoveRef = useRef(0)
  const lastGreetRef = useRef(0)
  const lastFlickRef = useRef(0)
  const flickSamplesRef = useRef<Array<{ t: number; x: number }>>([])
  const bubbleTimerRef = useRef<number | null>(null)
  const [propReaction, setPropReaction] = useState<PropReaction | null>(null)
  const [bubble, setBubble] = useState<BubbleState | null>(null)

  /* ------------------------------- 粒子 ------------------------------- */
  const spawnParticles = useCallback((clientX: number, clientY: number, count: number, kind: PetKind | 'random') => {
    const host = particlesRef.current
    if (!host) return
    const rect = host.getBoundingClientRect()
    for (let i = 0; i < count; i += 1) {
      seqRef.current += 1
      const node = document.createElement('span')
      const finalKind: PetKind = kind === 'random' ? (Math.random() > 0.45 ? 'heart' : 'paw') : kind
      node.className = `pet-particle pet-particle-${finalKind}`
      node.style.left = `${clientX - rect.left}px`
      node.style.top = `${clientY - rect.top - i * 6}px`
      node.style.setProperty('--drift', `${Math.round((Math.random() - 0.5) * 70)}px`)
      node.style.animationDelay = `${i * 0.08}s`
      node.innerHTML = PARTICLE_SVG[finalKind]
      host.appendChild(node)
      window.setTimeout(() => node.remove(), 1500 + i * 80)
    }
  }, [])

  /* ------------------------- 动作 → 气泡 / 爱心 ------------------------- */
  const runAction = useCallback(
    (type: ActionType, keys?: PetKey[], point?: Point) => {
      const c = controllerRef.current
      if (!c) return
      const res = c.action(type, keys)
      if (!res) return
      const seq = ++seqRef.current
      setBubble({ seq, text: res.text, variant: res.keys.length === 2 ? 'both' : res.keys[0] })
      if (bubbleTimerRef.current) window.clearTimeout(bubbleTimerRef.current)
      bubbleTimerRef.current = window.setTimeout(() => {
        setBubble((b) => (b?.seq === seq ? null : b))
      }, res.duration)
      if (res.type === 'pet' || res.type === 'happy' || res.type === 'nuzzle') {
        const p = point ?? lastClientRef.current
        spawnParticles(p.x, p.y, res.type === 'nuzzle' ? 7 : 5, 'heart')
      }
    },
    [spawnParticles],
  )

  /* ------------------------- 生命周期：单例控制器 ------------------------- */
  useEffect(() => {
    let disposed = false
    lastMoveRef.current = performance.now()
    acquireController(hostRef.current as HTMLDivElement)
      .then((c) => {
        if (disposed) return
        controllerRef.current = c
      })
      .catch((err) => console.error('Live2D 初始化失败', err))

    // 闲置检测：8s 没有鼠标移动且没有正在播放的动作 → 瞌睡
    const idleTimer = window.setInterval(() => {
      const c = controllerRef.current
      if (!c || c.hasActiveEvent()) return
      if (performance.now() - lastMoveRef.current > 8000) runAction('sleepy')
    }, 1000)

    return () => {
      disposed = true
      window.clearInterval(idleTimer)
      if (bubbleTimerRef.current) window.clearTimeout(bubbleTimerRef.current)
      controllerRef.current = null
      releaseController()
    }
  }, [runAction])

  /* ---------------- 场景鼠标移动：视差 + 视线跟随 + 甩动检测 ---------------- */
  const handleSceneMouseMove = useCallback(
    (e: React.MouseEvent<HTMLDivElement>) => {
      const scene = sceneRef.current
      if (scene) {
        const rect = scene.getBoundingClientRect()
        const px = ((e.clientX - rect.left) / rect.width) * 2 - 1
        const py = ((e.clientY - rect.top) / rect.height) * 2 - 1
        scene.style.setProperty('--px', px.toFixed(3))
        scene.style.setProperty('--py', py.toFixed(3))
      }
      lastClientRef.current = { x: e.clientX, y: e.clientY }
      const now = performance.now()
      lastMoveRef.current = now

      const c = controllerRef.current
      const host = hostRef.current
      if (!c || !host) return
      c.clearSleepy()
      const rect = host.getBoundingClientRect()
      const nx = clampNum(((e.clientX - rect.left) / rect.width - 0.5) * 2, -1, 1)
      const ny = clampNum(((e.clientY - rect.top) / rect.height - 0.5) * 2, -1, 1)
      c.setPointer(nx, ny)

      // 快速横向甩动：取最近 140ms 内的归一化横向速度
      const samples = flickSamplesRef.current
      samples.push({ t: now, x: nx })
      while (samples.length > 6) samples.shift()
      const first = samples.find((s) => now - s.t <= 140)
      if (first && now - first.t > 30) {
        const speed = Math.abs(nx - first.x) / ((now - first.t) / 1000)
        if (speed > 3.2 && now - lastFlickRef.current > 2500) {
          lastFlickRef.current = now
          runAction('shake')
        }
      }
    },
    [runAction],
  )

  const handleSceneMouseEnter = useCallback(() => {
    const now = performance.now()
    if (now - lastGreetRef.current > 9000) {
      lastGreetRef.current = now
      runAction('nod')
    }
  }, [runAction])

  const handleSceneMouseLeave = useCallback(() => {
    sceneRef.current?.style.setProperty('--px', '0')
    sceneRef.current?.style.setProperty('--py', '0')
    controllerRef.current?.setPointer(0, 0)
    controllerRef.current?.setHovered(null)
  }, [])

  /** 点击空白处：冒粒子，最近的小家伙好奇歪头 */
  const handleSceneClick = useCallback(
    (e: React.MouseEvent<HTMLDivElement>) => {
      if ((e.target as HTMLElement).closest('.pet-zone')) return
      spawnParticles(e.clientX, e.clientY, 1, 'random')
      const host = hostRef.current
      if (host && controllerRef.current) {
        const r = host.getBoundingClientRect()
        const u = (e.clientX - r.left) / r.width
        runAction('curious', [u < 0.5 ? 'cat' : 'dog'], { x: e.clientX, y: e.clientY })
      }
    },
    [runAction, spawnParticles],
  )

  /* ------------ 猫/狗热区：单击 / 双击 / 右键 / 长按 / 悬停 ------------ */
  const handleZonePointerDown = useCallback(
    (key: PetKey, e: React.PointerEvent<HTMLDivElement>) => {
      if (e.button !== 0) return
      const entry: PressState = {
        key,
        start: performance.now(),
        x: e.clientX,
        y: e.clientY,
        fired: false,
        timer: 0,
      }
      entry.timer = window.setTimeout(() => {
        // 用身份比较确认这次按压没有被取消 / 替换
        if (pressRef.current !== entry) return
        entry.fired = true
        // 注意：此时 React 合成事件已回收，e.currentTarget 为 null；
        // 粒子位置直接复用按下时记录的 entry 坐标
        runAction('stretch', [key], { x: entry.x, y: entry.y })
      }, 580)
      pressRef.current = entry
    },
    [runAction],
  )

  const handleZonePointerUp = useCallback(
    (key: PetKey, e: React.PointerEvent<HTMLDivElement>) => {
      const p = pressRef.current
      if (!p || p.key !== key) return
      window.clearTimeout(p.timer)
      pressRef.current = null
      if (p.fired) return
      const dt = performance.now() - p.start
      const moved = Math.hypot(e.clientX - p.x, e.clientY - p.y)
      if (dt < 600 && moved < 12) runAction('pet', [key], { x: e.clientX, y: e.clientY })
    },
    [runAction],
  )

  /** 抬起前离开热区：未触发的按压取消；已触发的长按保留到 pointerup */
  const handleZonePointerCancel = useCallback(() => {
    const p = pressRef.current
    if (p && !p.fired) {
      window.clearTimeout(p.timer)
      pressRef.current = null
    }
  }, [])

  const handleZoneHover = useCallback((key: PetKey | null) => {
    controllerRef.current?.setHovered(key)
  }, [])

  const handleZoneDoubleClick = useCallback(
    (key: PetKey, e: React.MouseEvent<HTMLDivElement>) => {
      runAction('happy', [key], { x: e.clientX, y: e.clientY })
    },
    [runAction],
  )

  const handleZoneContextMenu = useCallback(
    (key: PetKey, e: React.MouseEvent<HTMLDivElement>) => {
      e.preventDefault()
      runAction('wink', [key], { x: e.clientX, y: e.clientY })
    },
    [runAction],
  )

  const bindZone = (key: PetKey) => ({
    onPointerDown: (e: React.PointerEvent<HTMLDivElement>) => handleZonePointerDown(key, e),
    onPointerUp: (e: React.PointerEvent<HTMLDivElement>) => handleZonePointerUp(key, e),
    onPointerLeave: handleZonePointerCancel,
    onPointerCancel: handleZonePointerCancel,
    onDoubleClick: (e: React.MouseEvent<HTMLDivElement>) => handleZoneDoubleClick(key, e),
    onContextMenu: (e: React.MouseEvent<HTMLDivElement>) => handleZoneContextMenu(key, e),
    onMouseEnter: () => handleZoneHover(key),
    onMouseLeave: () => handleZoneHover(null),
  })

  /* ---------------- 道具（鱼盘 / 骨头）点击反应 ---------------- */
  const triggerPropReaction = useCallback(
    (actor: PropActorId, e: React.MouseEvent<HTMLDivElement>) => {
      e.stopPropagation()
      const seq = ++seqRef.current
      setPropReaction(null)
      requestAnimationFrame(() => setPropReaction({ actor, seq }))
      spawnParticles(e.clientX, e.clientY, 2, 'paw')
    },
    [spawnParticles],
  )

  const propActorClass = (id: PropActorId) =>
    `pet-actor pet-${id}${propReaction?.actor === id ? ' is-react' : ''}`

  return (
    <div
      ref={sceneRef}
      className="pet-scene"
      onMouseMove={handleSceneMouseMove}
      onMouseEnter={handleSceneMouseEnter}
      onMouseLeave={handleSceneMouseLeave}
      onClick={handleSceneClick}
    >
      {/* 背景漂浮装饰（视差最快层） */}
      <div className="pet-deco pet-layer pet-layer-front">
        <PawMark className="pet-float pet-float-3" style={{ top: '46%', left: '1.5%', width: 36 }} />
        <PawMark className="pet-float pet-float-2" style={{ top: '30%', right: '4%', width: 30 }} />
        <PawMark className="pet-float pet-float-1" style={{ bottom: '30%', left: '3%', width: 24 }} />
        <HeartMark className="pet-heart" style={{ top: '24%', right: '15%', width: 34 }} />
        <FishboneMark className="pet-float pet-float-2" style={{ top: '44%', left: '52%', width: 36 }} />
        <SparkleMark className="pet-twinkle pet-twinkle-1" style={{ top: '16%', left: '38%', width: 20 }} />
        <SparkleMark className="pet-twinkle pet-twinkle-2" style={{ top: '38%', left: '13%', width: 14 }} />
        <SparkleMark className="pet-twinkle pet-twinkle-3" style={{ bottom: '34%', right: '10%', width: 16 }} />
        <SparkleMark className="pet-twinkle pet-twinkle-1" style={{ top: '58%', right: '22%', width: 14 }} />
      </div>

      {/* 地毯地面 */}
      <div className="pet-rug" />

      {/* Live2D 舞台（猫 + 狗） */}
      <div className="live2d-stage">
        <div ref={hostRef} className="live2d-canvas-host" />
        <div className="live2d-fx">
          <div className="pet-zone pet-zone-cat" role="button" aria-label="摸摸橘猫" {...bindZone('cat')} />
          <div className="pet-zone pet-zone-dog" role="button" aria-label="摸摸柯基" {...bindZone('dog')} />
          {bubble && (
            <span key={bubble.seq} className={`live2d-bubble live2d-bubble-${bubble.variant}`}>
              {bubble.text}
            </span>
          )}
        </div>
      </div>

      {/* CSS 贴纸道具层 */}
      <div className="pet-actors">
        {/* 鱼盘 */}
        <div
          className={propActorClass('fish')}
          style={{ ['--depth-x' as string]: '22px', ['--depth-y' as string]: '12px' }}
          onClick={(e) => triggerPropReaction('fish', e)}
        >
          <div className="pet-actor-tilt">
            <div
              key={`fish-${propReaction?.actor === 'fish' ? propReaction.seq : 0}`}
              className="pet-actor-sprite"
            >
              <img className="pet-art pet-art-float" src={FISH_IMG} alt="" draggable={false} />
            </div>
          </div>
        </div>

        {/* 骨头 */}
        <div
          className={propActorClass('bone')}
          style={{ ['--depth-x' as string]: '28px', ['--depth-y' as string]: '14px' }}
          onClick={(e) => triggerPropReaction('bone', e)}
        >
          <div className="pet-actor-tilt">
            <div
              key={`bone-${propReaction?.actor === 'bone' ? propReaction.seq : 0}`}
              className="pet-actor-sprite"
            >
              <img className="pet-art pet-art-float-spin" src={BONE_IMG} alt="" draggable={false} />
            </div>
          </div>
        </div>
      </div>

      {/* 点击粒子宿主 */}
      <div ref={particlesRef} className="pet-particles" />

      {/* 品牌信息 */}
      <div className="pet-brand pet-layer pet-layer-mid">
        <div className="pet-brand-title">
          <BrandLogo size={40} />
          <span>宠物商户 SaaS</span>
        </div>
        <div className="pet-brand-subtitle">多租户运营管理后台 · 宠物日常乐园</div>
        <ul className="pet-features">
          {FEATURES.map((f) => (
            <li key={f.text}>
              <span className="pet-feature-icon">{f.icon}</span>
              {f.text}
            </li>
          ))}
        </ul>
      </div>

      <div className="pet-hint">移动鼠标 · 点击 / 双击 / 右键毛孩子，有惊喜 ✨</div>
    </div>
  )
}
