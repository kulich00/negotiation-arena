<script setup>
import { onMounted, onUnmounted, ref } from 'vue'
import { RouterLink, RouterView } from 'vue-router'

const THEME_STORAGE_KEY = 'negotiation-arena-theme'
const TEXT_SCALE_STORAGE_KEY = 'negotiation-arena-text-scale'
const storedTheme = globalThis.localStorage?.getItem(THEME_STORAGE_KEY)
const isDark = ref(storedTheme !== 'light')
if (typeof document !== 'undefined') {
  document.documentElement.dataset.theme = isDark.value ? 'dark' : 'light'
}
const textScale = ref(100)
const backgroundRipples = ref([])
const isBackgroundDragging = ref(false)
const appBackground = ref(null)
let rippleId = 0
let backgroundPointerId = null
let lastWaveAt = 0
let ambientFrame = null

function applyTheme(dark) {
  isDark.value = dark
  document.documentElement.dataset.theme = dark ? 'dark' : 'light'
  localStorage.setItem(THEME_STORAGE_KEY, dark ? 'dark' : 'light')
}

function toggleTheme() {
  applyTheme(!isDark.value)
}

function applyTextScale(scale) {
  textScale.value = Math.min(115, Math.max(90, scale))
  document.documentElement.style.fontSize = `${textScale.value}%`
  localStorage.setItem(TEXT_SCALE_STORAGE_KEY, String(textScale.value))
}

function changeTextScale(delta) {
  applyTextScale(textScale.value + delta)
}

function isBackgroundTarget(target) {
  if (!(target instanceof Element)) return false
  return !target.closest('button, a, input, textarea, select, .card, .panel, .filters-card, .history-card, .topbar')
}

function addBackgroundRipple(event, type = 'ripple') {
  const id = ++rippleId
  backgroundRipples.value.push({
    id,
    type,
    x: event.clientX,
    y: event.clientY,
    size: type === 'wave' ? 132 : 210,
  })
}

function removeBackgroundRipple(id) {
  backgroundRipples.value = backgroundRipples.value.filter((ripple) => ripple.id !== id)
}

function handleBackgroundPointerDown(event) {
  if (!isBackgroundTarget(event.target)) return
  backgroundPointerId = event.pointerId
  isBackgroundDragging.value = true
  lastWaveAt = performance.now()
  addBackgroundRipple(event)
}

function handleBackgroundPointerMove(event) {
  if (event.pointerId !== backgroundPointerId || !isBackgroundTarget(event.target)) return
  const now = performance.now()
  if (now - lastWaveAt < 90) return
  lastWaveAt = now
  addBackgroundRipple(event, 'wave')
}

function stopBackgroundDrag(event) {
  if (event.pointerId === backgroundPointerId) {
    backgroundPointerId = null
    isBackgroundDragging.value = false
  }
}

function animateAmbientBackground(time) {
  if (appBackground.value) {
    const seconds = time / 1000
    const driftX = Math.sin(seconds * 0.16) * 10
    const driftY = Math.cos(seconds * 0.12) * 7
    const scale = 1.055 + Math.sin(seconds * 0.1) * 0.022
    appBackground.value.style.transform = `translate3d(${driftX}%, ${driftY}%, 0) scale(${scale})`
    appBackground.value.style.backgroundPosition = `${50 + driftX}% ${50 + driftY}%, ${50 - driftX}% ${50 + driftY * 1.4}%, ${50 + driftX * .6}% ${50 - driftY}%`
  }
  ambientFrame = requestAnimationFrame(animateAmbientBackground)
}

onMounted(() => {
  applyTheme(localStorage.getItem(THEME_STORAGE_KEY) !== 'light')
  applyTextScale(Number(localStorage.getItem(TEXT_SCALE_STORAGE_KEY)) || 100)
  ambientFrame = requestAnimationFrame(animateAmbientBackground)
})

onUnmounted(() => {
  if (ambientFrame) cancelAnimationFrame(ambientFrame)
})
</script>

<template>
  <div
    :class="['app-shell', { 'is-background-dragging': isBackgroundDragging }]"
    @pointerdown="handleBackgroundPointerDown"
    @pointermove="handleBackgroundPointerMove"
    @pointerup="stopBackgroundDrag"
    @pointercancel="stopBackgroundDrag"
    @pointerleave="stopBackgroundDrag"
  >
    <div ref="appBackground" class="app-background" aria-hidden="true"></div>
    <span
      v-for="ripple in backgroundRipples"
      :key="ripple.id"
      :class="['background-ripple', ripple.type]"
      :style="{ left: `${ripple.x}px`, top: `${ripple.y}px`, width: `${ripple.size}px`, height: `${ripple.size}px` }"
      aria-hidden="true"
      @animationend="removeBackgroundRipple(ripple.id)"
    ></span>
    <header class="topbar">
      <RouterLink class="brand" to="/">Арена переговоров</RouterLink>
      <nav>
        <RouterLink class="nav-link" to="/">Сценарии</RouterLink>
        <RouterLink class="nav-link" to="/admin">Администратор</RouterLink>
        <div class="text-scale-control" aria-label="Масштаб текста">
          <button
            type="button"
            title="Уменьшить текст"
            aria-label="Уменьшить масштаб текста"
            :disabled="textScale <= 90"
            @click="changeTextScale(-5)"
          >
            A-
          </button>
          <button
            type="button"
            title="Увеличить текст"
            aria-label="Увеличить масштаб текста"
            :disabled="textScale >= 115"
            @click="changeTextScale(5)"
          >
            A+
          </button>
        </div>
        <button
          class="theme-toggle"
          type="button"
          :aria-label="isDark ? 'Включить светлую тему' : 'Включить темную тему'"
          :aria-pressed="isDark"
          @click="toggleTheme"
        >
          <span class="theme-toggle-icon" aria-hidden="true">{{ isDark ? '☀' : '◐' }}</span>
          <span>{{ isDark ? 'Светлая' : 'Темная' }}</span>
        </button>
      </nav>
    </header>
    <main class="container">
      <RouterView />
    </main>
  </div>
</template>

