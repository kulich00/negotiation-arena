<script setup>
import { computed, onMounted, ref, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useNegotiationStore } from '../stores/negotiation'

const route = useRoute()
const router = useRouter()
const store = useNegotiationStore()

const message = ref('')
const dialogueContainer = ref(null)

const currentScenario = computed(() => {
  if (!store.session) return null
  return store.scenarios.find((s) => s.id === store.session.scenarioId) || null
})

const canSend = computed(() => message.value.trim() && !store.loading && !store.result)

onMounted(async () => {
  if (!store.scenarios.length) {
    await store.loadScenarios()
  }
  try {
    await store.startSession(route.params.id)
    await nextTick()
    scrollToBottom()
  } catch (e) {
    console.error('Ошибка старта сессии:', e)
  }
})

// Автоскролл чата при новых сообщениях
watch(
  () => store.messages.length,
  async () => {
    await nextTick()
    scrollToBottom()
  }
)

function scrollToBottom() {
  if (dialogueContainer.value) {
    dialogueContainer.value.scrollTop = dialogueContainer.value.scrollHeight
  }
}

async function sendMessage() {
  if (!canSend.value) return
  const content = message.value
  message.value = ''
  await store.sendMessage(content)
}

function handleKeydown(event) {
  if (event.key === 'Enter' && (event.ctrlKey || event.metaKey)) {
    event.preventDefault()
    sendMessage()
  }
}

// Полезные фразы-подсказки для быстрой подстановки (техники переговоров)
const promptHints = [
  'Задать открытый вопрос (SPIN)',
  'Озвучить интересы и BATNA',
  'Предложить компромисс',
  'Уточнить критерии сделки'
]

function applyHint(hintText) {
  if (hintText.includes('SPIN')) {
    message.value = 'Какие ключевые показатели или результаты для вас сейчас наиболее критичны?'
  } else if (hintText.includes('BATNA')) {
    message.value = 'Давайте обсудим условия, при которых наше сотрудничество станет взаимно выгодным.'
  } else if (hintText.includes('компромисс')) {
    message.value = 'Если мы пойдем навстречу в вопросе сроков, готовы ли вы рассмотреть зафиксированный объем?'
  } else {
    message.value = 'Какие конкретные критерии позволят вам принять положительное решение?'
  }
}

function getScoreColorClass(score) {
  if (score >= 70) return 'score-high'
  if (score >= 40) return 'score-mid'
  return 'score-low'
}

function restartNegotiation() {
  store.restart(route.params.id)
}
</script>

<template>
  <div class="negotiation-page">
    <!-- Навигация назад -->
    <div class="page-header">
      <button class="back-link" @click="router.push('/')">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M19 12H5M12 19l-7-7 7-7" />
        </svg>
        К списку сценариев
      </button>

      <span v-if="currentScenario" class="scenario-badge">
        {{ currentScenario.title }}
      </span>
    </div>

    <p v-if="store.error" class="alert alert-banner">{{ store.error }}</p>

    <!-- Главная сетка: Боковая панель контекста + Диалог + Результаты -->
    <section class="negotiation-grid">
      <!-- Контекст и параметры сценария -->
      <aside class="panel context-panel">
        <div class="panel-header">
          <span class="eyebrow">Контекст симуляции</span>
          <h2>Условия & Цели</h2>
        </div>

        <div v-if="currentScenario" class="context-details">
          <div class="ctx-group">
            <span class="ctx-label">Тема переговоров</span>
            <p class="ctx-val">{{ currentScenario.topic }}</p>
          </div>

          <div class="ctx-group highlight">
            <span class="ctx-label">Ваша цель</span>
            <p class="ctx-val">{{ currentScenario.playerGoal }}</p>
          </div>

          <div class="ctx-group">
            <span class="ctx-label">Собеседник</span>
            <p class="ctx-val">
              <strong>{{ currentScenario.opponentRole }}</strong>
            </p>
            <p v-if="currentScenario.opponentTone" class="ctx-sub">
              Тон: {{ currentScenario.opponentTone }}
            </p>
          </div>

          <div class="ctx-group" v-if="currentScenario.opponentGoal">
            <span class="ctx-label">Цель собеседника</span>
            <p class="ctx-val ctx-muted">{{ currentScenario.opponentGoal }}</p>
          </div>
        </div>

        <div v-else class="context-skeleton">
          <p>Загрузка контекста сценария...</p>
        </div>

        <div class="methodology-card">
          <h4>💡 Подсказка по тактике</h4>
          <p>Используйте технику открытых вопросов (SPIN) и предлагайте варианты с учетом BATNA другой стороны.</p>
        </div>
      </aside>

      <!-- Центральная область: Диалог и окно ввода -->
      <div class="panel dialogue-panel">
        <!-- Шапка диалога с динамическими метриками -->
        <div class="dialogue-header">
          <div class="metrics-bar" v-if="store.session">
            <div class="metric-item">
              <span class="m-lbl">Доверие</span>
              <div class="progress-bg">
                <div
                  class="progress-fill trust"
                  :style="{ width: Math.min(100, Math.max(0, store.session.trustScore)) + '%' }"
                ></div>
              </div>
              <span class="m-val">{{ store.session.trustScore }}</span>
            </div>

            <div class="metric-item">
              <span class="m-lbl">Аргументация</span>
              <div class="progress-bg">
                <div
                  class="progress-fill argument"
                  :style="{ width: Math.min(100, Math.max(0, store.session.argumentScore)) + '%' }"
                ></div>
              </div>
              <span class="m-val">{{ store.session.argumentScore }}</span>
            </div>

            <div class="metric-item" v-if="store.session.pressureScore !== undefined">
              <span class="m-lbl">Давление</span>
              <div class="progress-bg">
                <div
                  class="progress-fill pressure"
                  :style="{ width: Math.min(100, Math.max(0, store.session.pressureScore)) + '%' }"
                ></div>
              </div>
              <span class="m-val">{{ store.session.pressureScore }}</span>
            </div>

            <div class="turn-pill">
              Ход: <strong>{{ store.session.turn }}</strong>
            </div>
          </div>
        </div>

        <!-- Окно сообщений чата -->
        <div ref="dialogueContainer" class="dialogue-body">
          <div
            v-for="(item, index) in store.messages"
            :key="index"
            :class="['message-bubble', item.sender]"
          >
            <div class="bubble-sender">
              {{ item.sender === 'opponent' ? (currentScenario?.opponentRole || 'Собеседник') : 'Вы' }}
            </div>
            <div class="bubble-content">{{ item.content }}</div>
          </div>

          <!-- Анимация печати собеседника -->
          <div v-if="store.loading" class="message-bubble opponent typing">
            <div class="typing-dots">
              <span></span><span></span><span></span>
            </div>
            <span class="typing-text">Собеседник обдумывает ответ...</span>
          </div>
        </div>

        <!-- Поле ввода реплики -->
        <div v-if="!store.result" class="composer-area">
          <div class="hints-row">
            <span class="hints-title">Быстрые формулировки:</span>
            <button
              v-for="hint in promptHints"
              :key="hint"
              class="hint-chip"
              @click="applyHint(hint)"
            >
              + {{ hint }}
            </button>
          </div>

          <form class="composer-form" @submit.prevent="sendMessage">
            <textarea
              v-model="message"
              placeholder="Введите вашу реплику или выберите формулировку... (Ctrl+Enter для отправки)"
              rows="3"
              @keydown="handleKeydown"
            />
            <div class="composer-actions">
              <span class="hotkey-tip">Ctrl + Enter для отправки</span>
              <div class="btn-group">
                <button
                  class="button secondary"
                  type="button"
                  :disabled="!store.session || store.loading"
                  @click="store.finish"
                >
                  Завершить переговоры
                </button>
                <button class="button" :disabled="!canSend">
                  Отправить
                </button>
              </div>
            </div>
          </form>
        </div>
      </div>

      <!-- Итоговый разбор результатов (если сессия завершена) -->
      <aside v-if="store.result" class="panel result-panel">
        <div class="result-header">
          <p class="eyebrow">Финальный разбор</p>
          <h2>{{ store.result.outcome }}</h2>
          <div :class="['final-score-badge', getScoreColorClass(store.result.finalScore)]">
            <span class="score-num">{{ store.result.finalScore }}</span>
            <span class="score-max">/100</span>
          </div>
        </div>

        <div class="result-section" v-if="store.result.strengths && store.result.strengths.length">
          <h3>✅ Сильные стороны</h3>
          <ul class="result-list strengths">
            <li v-for="(item, i) in store.result.strengths" :key="i">{{ item }}</li>
          </ul>
        </div>

        <div class="result-section" v-if="store.result.mistakes && store.result.mistakes.length">
          <h3>⚠️ Ошибки и риски</h3>
          <ul class="result-list mistakes">
            <li v-for="(item, i) in store.result.mistakes" :key="i">{{ item }}</li>
          </ul>
        </div>

        <div class="result-section" v-if="store.result.recommendations && store.result.recommendations.length">
          <h3>💡 Рекомендации</h3>
          <ul class="result-list recommendations">
            <li v-for="(item, i) in store.result.recommendations" :key="i">{{ item }}</li>
          </ul>
        </div>

        <div class="result-actions">
          <button class="button full-width" @click="restartNegotiation">
            Пройти снова с другой тактикой
          </button>
          <button class="button secondary full-width" @click="router.push('/')">
            Вернуться к сценариям
          </button>
        </div>
      </aside>
    </section>
  </div>
</template>



