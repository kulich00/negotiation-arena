<script setup>
<<<<<<< HEAD
import { computed, onBeforeUnmount, onMounted, ref, watch, nextTick } from 'vue'
=======
import { computed, onMounted, ref, watch, nextTick } from 'vue'
>>>>>>> 91bc0acbd98cf8d594617e2cd1a71e866976e702
import { useRoute, useRouter } from 'vue-router'
import { useNegotiationStore } from '../stores/negotiation'

const route = useRoute()
const router = useRouter()
const store = useNegotiationStore()

const message = ref('')
const dialogueContainer = ref(null)
<<<<<<< HEAD
const showActivePanels = ref(true)
const showResult = ref(false)
const showMethodologyHint = ref(false)
let resultRevealTimer = null
=======
>>>>>>> 91bc0acbd98cf8d594617e2cd1a71e866976e702

const currentScenario = computed(() => {
  if (!store.session) return null
  return store.scenarios.find((s) => s.id === store.session.scenarioId) || null
})

const canSend = computed(() => message.value.trim() && !store.loading && !store.result)

<<<<<<< HEAD
watch(
  () => store.result,
  (result, previousResult) => {
    if (result) {
      showResult.value = false
      showActivePanels.value = false
      if (resultRevealTimer) clearTimeout(resultRevealTimer)
      resultRevealTimer = setTimeout(() => {
        showResult.value = true
      }, 420)
      return
    }

    if (resultRevealTimer) clearTimeout(resultRevealTimer)
    showResult.value = false
    if (previousResult) {
      showActivePanels.value = false
      resultRevealTimer = setTimeout(() => {
        showActivePanels.value = true
      }, 420)
    } else {
      showActivePanels.value = true
    }
  },
  { immediate: true },
)

=======
>>>>>>> 91bc0acbd98cf8d594617e2cd1a71e866976e702
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

<<<<<<< HEAD
onBeforeUnmount(() => {
  if (resultRevealTimer) clearTimeout(resultRevealTimer)
})

// Автоскролл чата при новых сообщениях
watch(
  () => [store.messages.length, store.loading],
=======
// Автоскролл чата при новых сообщениях
watch(
  () => store.messages.length,
>>>>>>> 91bc0acbd98cf8d594617e2cd1a71e866976e702
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

<<<<<<< HEAD
function toggleMethodologyHint() {
  showMethodologyHint.value = !showMethodologyHint.value
}

=======
>>>>>>> 91bc0acbd98cf8d594617e2cd1a71e866976e702
function getScoreColorClass(score) {
  if (score >= 70) return 'score-high'
  if (score >= 40) return 'score-mid'
  return 'score-low'
}

<<<<<<< HEAD
function getMetricColor(value) {
  const score = Math.min(100, Math.max(0, Number(value) || 0))
  const hue = Math.round((score / 100) * 120)
  return `hsl(${hue} 72% 44%)`
}

=======
>>>>>>> 91bc0acbd98cf8d594617e2cd1a71e866976e702
function formatDelta(value) {
  return value > 0 ? `+${value}` : String(value)
}

async function retryFromTurn(turn) {
  await store.forkFromTurn(Math.max(0, turn))
  await nextTick()
  scrollToBottom()
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
<<<<<<< HEAD
    <Transition name="screen-fade" mode="out-in">
      <section v-if="showActivePanels" key="active" class="negotiation-grid">
=======
    <section class="negotiation-grid">
>>>>>>> 91bc0acbd98cf8d594617e2cd1a71e866976e702
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

          <div v-if="currentScenario.opponentGoal" class="ctx-group">
            <span class="ctx-label">Цель собеседника</span>
            <p class="ctx-val ctx-muted">{{ currentScenario.opponentGoal }}</p>
          </div>
        </div>

        <div v-else class="context-skeleton">
          <p>Загрузка контекста сценария...</p>
        </div>

        <div class="methodology-card">
<<<<<<< HEAD
          <button
            class="methodology-toggle"
            type="button"
            :aria-expanded="showMethodologyHint"
            @click="toggleMethodologyHint"
          >
            <span>💡 Подсказка по тактике</span>
            <span class="methodology-chevron" aria-hidden="true">{{ showMethodologyHint ? '−' : '+' }}</span>
          </button>
          <p v-if="showMethodologyHint">Используйте технику открытых вопросов (SPIN) и предлагайте варианты с учетом BATNA другой стороны.</p>
=======
          <h4>💡 Подсказка по тактике</h4>
          <p>Используйте технику открытых вопросов (SPIN) и предлагайте варианты с учетом BATNA другой стороны.</p>
>>>>>>> 91bc0acbd98cf8d594617e2cd1a71e866976e702
        </div>
      </aside>

      <!-- Центральная область: Диалог и окно ввода -->
      <div class="panel dialogue-panel">
        <!-- Шапка диалога с динамическими метриками -->
        <div class="dialogue-header">
          <div v-if="store.session" class="metrics-bar">
            <div class="metric-item">
              <span class="m-lbl">Доверие</span>
              <div class="progress-bg">
                <div
                  class="progress-fill trust"
<<<<<<< HEAD
                  :style="{ width: Math.min(100, Math.max(0, store.session.trustScore)) + '%', '--metric-color': getMetricColor(store.session.trustScore) }"
=======
                  :style="{ width: Math.min(100, Math.max(0, store.session.trustScore)) + '%' }"
>>>>>>> 91bc0acbd98cf8d594617e2cd1a71e866976e702
                ></div>
              </div>
              <span class="m-val">{{ store.session.trustScore }}</span>
            </div>

            <div class="metric-item">
              <span class="m-lbl">Аргументация</span>
              <div class="progress-bg">
                <div
                  class="progress-fill argument"
<<<<<<< HEAD
                  :style="{ width: Math.min(100, Math.max(0, store.session.argumentScore)) + '%', '--metric-color': getMetricColor(store.session.argumentScore) }"
=======
                  :style="{ width: Math.min(100, Math.max(0, store.session.argumentScore)) + '%' }"
>>>>>>> 91bc0acbd98cf8d594617e2cd1a71e866976e702
                ></div>
              </div>
              <span class="m-val">{{ store.session.argumentScore }}</span>
            </div>

            <div v-if="store.session.pressureScore !== undefined" class="metric-item">
              <span class="m-lbl">Давление</span>
              <div class="progress-bg">
                <div
                  class="progress-fill pressure"
<<<<<<< HEAD
                  :style="{ width: Math.min(100, Math.max(0, store.session.pressureScore)) + '%', '--metric-color': getMetricColor(store.session.pressureScore) }"
=======
                  :style="{ width: Math.min(100, Math.max(0, store.session.pressureScore)) + '%' }"
>>>>>>> 91bc0acbd98cf8d594617e2cd1a71e866976e702
                ></div>
              </div>
              <span class="m-val">{{ store.session.pressureScore }}</span>
            </div>

<<<<<<< HEAD
=======
            <div class="turn-pill">
              Ход: <strong>{{ store.session.turn }}</strong>
            </div>
            <div v-if="store.session.parentSessionId" class="branch-pill">
              Ветка с хода {{ store.session.forkedFromTurn }}
            </div>
>>>>>>> 91bc0acbd98cf8d594617e2cd1a71e866976e702
          </div>
        </div>

        <!-- Окно сообщений чата -->
        <div ref="dialogueContainer" class="dialogue-body">
<<<<<<< HEAD
          <TransitionGroup name="message" tag="div" class="message-list">
            <div
              v-for="(item, index) in store.messages"
              :key="`${item.sender}-${index}-${item.content}`"
              :class="['message-bubble', item.sender]"
            >
              <div class="bubble-sender">
                {{ item.sender === 'opponent' ? (currentScenario?.opponentRole || 'Собеседник') : 'Вы' }}
              </div>
              <div class="bubble-content">{{ item.content }}</div>
              <div v-if="item.sender === 'player' && item.analysis" class="turn-analysis">
=======
          <div
            v-for="(item, index) in store.messages"
            :key="index"
            :class="['message-bubble', item.sender]"
          >
            <div class="bubble-sender">
              {{ item.sender === 'opponent' ? (currentScenario?.opponentRole || 'Собеседник') : 'Вы' }}
            </div>
            <div class="bubble-content">{{ item.content }}</div>
            <div v-if="item.sender === 'player' && item.analysis" class="turn-analysis">
>>>>>>> 91bc0acbd98cf8d594617e2cd1a71e866976e702
              <div class="analysis-heading">
                <strong>{{ item.analysis.summary }}</strong>
                <span class="technique-code">{{ item.analysis.technique }}</span>
              </div>
              <div class="analysis-deltas">
                <span :class="{ positive: item.analysis.trustDelta > 0, negative: item.analysis.trustDelta < 0 }">
                  Доверие {{ formatDelta(item.analysis.trustDelta) }}
                </span>
                <span :class="{ positive: item.analysis.argumentDelta > 0 }">
                  Аргументы {{ formatDelta(item.analysis.argumentDelta) }}
                </span>
                <span :class="{ negative: item.analysis.pressureDelta > 0 }">
                  Давление {{ formatDelta(item.analysis.pressureDelta) }}
                </span>
              </div>
              <div v-if="item.analysis.errors?.length" class="turn-errors">
                <span
                  v-for="error in item.analysis.errors"
                  :key="error.code"
                  :class="['error-chip', `severity-${error.severity}`]"
                  :title="error.message"
                >
                  {{ error.label }}
                </span>
              </div>
              <p class="analysis-recommendation">{{ item.analysis.recommendation }}</p>
<<<<<<< HEAD
              </div>
            </div>
          </TransitionGroup>

          <!-- Анимация печати собеседника -->
          <Transition name="message">
            <div v-if="store.loading" class="message-bubble opponent typing">
              <div class="typing-dots">
                <span></span><span></span><span></span>
              </div>
              <span class="typing-text">Собеседник обдумывает ответ...</span>
            </div>
          </Transition>
=======
            </div>
          </div>

          <!-- Анимация печати собеседника -->
          <div v-if="store.loading" class="message-bubble opponent typing">
            <div class="typing-dots">
              <span></span><span></span><span></span>
            </div>
            <span class="typing-text">Собеседник обдумывает ответ...</span>
          </div>
>>>>>>> 91bc0acbd98cf8d594617e2cd1a71e866976e702
        </div>

        <!-- Поле ввода реплики -->
        <div v-if="!store.result" class="composer-area">
<<<<<<< HEAD
          <form class="composer-form" @submit.prevent="sendMessage">
            <textarea
              v-model="message"
              placeholder="Введите вашу реплику или выберите формулировку..."
              rows="3"
              @keydown="handleKeydown"
            />
            <div class="composer-actions">
              <div class="btn-group">
                <button
                  class="button secondary"
                  type="button"
                  :disabled="!store.session || store.loading"
                  @click="store.finish"
                >
                  Завершить переговоры
                </button>
                <button
                  class="send-button"
                  type="submit"
                  :disabled="!canSend"
                  aria-label="Отправить реплику"
                  title="Отправить реплику"
                >
                  <svg viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="1.8">
                    <path d="m3 5 18 7-18 7 3.5-7L3 5Z" stroke-linejoin="round" />
                    <path d="M6.5 12H21" />
                  </svg>
                </button>
              </div>
            </div>
          </form>
        </div>
      </div>

      <aside class="panel toolkit-panel">
        <div class="toolkit-heading">
          <p class="eyebrow">Навигация по сессии</p>
          <h2>Инструменты</h2>
        </div>
        <div v-if="store.session" class="turn-summary">
          <span class="turn-summary-label">Текущий ход</span>
          <strong>{{ store.session.turn }}</strong>
          <span>из переговоров</span>
        </div>
        <div v-if="store.session?.parentSessionId" class="branch-pill">
          Ветка с хода {{ store.session.forkedFromTurn }}
        </div>
        <div v-if="store.retryCheckpoints.length" class="toolkit-section">
          <span class="hints-title">Быстрый откат</span>
          <div class="toolkit-actions checkpoint-actions">
=======
          <div v-if="store.retryCheckpoints.length" class="retry-strip">
            <span>Быстрый откат:</span>
>>>>>>> 91bc0acbd98cf8d594617e2cd1a71e866976e702
            <button
              v-for="checkpoint in store.retryCheckpoints"
              :key="checkpoint.turn"
              type="button"
              class="checkpoint-button"
              :disabled="store.loading"
              @click="retryFromTurn(checkpoint.turn)"
            >
              {{ checkpoint.turn === 0 ? 'С начала' : `После хода ${checkpoint.turn}` }}
            </button>
          </div>
<<<<<<< HEAD
        </div>
        <div class="toolkit-section">
          <span class="hints-title">Быстрые формулировки</span>
          <div class="toolkit-actions">
=======
          <div class="hints-row">
            <span class="hints-title">Быстрые формулировки:</span>
>>>>>>> 91bc0acbd98cf8d594617e2cd1a71e866976e702
            <button
              v-for="hint in promptHints"
              :key="hint"
              class="hint-chip"
              @click="applyHint(hint)"
            >
              + {{ hint }}
            </button>
          </div>
<<<<<<< HEAD
        </div>
      </aside>

      </section>

      <section v-else-if="showResult" key="result" class="negotiation-grid has-result">
      <!-- Итоговый разбор результатов (если сессия завершена) -->
      <aside class="panel result-panel">
=======

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
>>>>>>> 91bc0acbd98cf8d594617e2cd1a71e866976e702
        <div class="result-header">
          <p class="eyebrow">Финальный разбор</p>
          <h2>{{ store.result.outcome }}</h2>
          <div :class="['final-score-badge', getScoreColorClass(store.result.finalScore)]">
            <span class="score-num">{{ store.result.finalScore }}</span>
            <span class="score-max">/100</span>
          </div>
        </div>

<<<<<<< HEAD
        <div v-if="store.result.strengths && store.result.strengths.length" class="result-section result-strengths">
=======
        <div v-if="store.result.strengths && store.result.strengths.length" class="result-section">
>>>>>>> 91bc0acbd98cf8d594617e2cd1a71e866976e702
          <h3>✅ Сильные стороны</h3>
          <ul class="result-list strengths">
            <li v-for="(item, i) in store.result.strengths" :key="i">{{ item }}</li>
          </ul>
        </div>

<<<<<<< HEAD
        <div v-if="store.result.mistakes && store.result.mistakes.length" class="result-section result-mistakes">
=======
        <div v-if="store.result.mistakes && store.result.mistakes.length" class="result-section">
>>>>>>> 91bc0acbd98cf8d594617e2cd1a71e866976e702
          <h3>⚠️ Ошибки и риски</h3>
          <ul class="result-list mistakes">
            <li v-for="(item, i) in store.result.mistakes" :key="i">{{ item }}</li>
          </ul>
        </div>

<<<<<<< HEAD
        <div v-if="store.result.recommendations && store.result.recommendations.length" class="result-section result-recommendations">
=======
        <div v-if="store.result.recommendations && store.result.recommendations.length" class="result-section">
>>>>>>> 91bc0acbd98cf8d594617e2cd1a71e866976e702
          <h3>💡 Рекомендации</h3>
          <ul class="result-list recommendations">
            <li v-for="(item, i) in store.result.recommendations" :key="i">{{ item }}</li>
          </ul>
        </div>

<<<<<<< HEAD
        <div v-if="store.result.analysis?.errorClasses?.length" class="result-section result-errors">
=======
        <div v-if="store.result.analysis?.errorClasses?.length" class="result-section">
>>>>>>> 91bc0acbd98cf8d594617e2cd1a71e866976e702
          <h3>🎯 Классы ошибок</h3>
          <div class="error-class-list">
            <article
              v-for="error in store.result.analysis.errorClasses"
              :key="error.code"
              :class="['error-class-card', `severity-${error.severity}`]"
            >
              <strong>{{ error.label }}</strong>
              <span>{{ error.count }} раз(а), ходы: {{ error.turns.join(', ') }}</span>
              <div class="error-retry-actions">
                <button
                  v-for="turn in error.turns"
                  :key="turn"
                  class="checkpoint-button"
                  :disabled="store.loading"
                  @click="retryFromTurn(turn - 1)"
                >
                  Переиграть ход {{ turn }}
                </button>
              </div>
            </article>
          </div>
        </div>

<<<<<<< HEAD
        <div v-if="store.result.analysis?.bestMove" class="result-section result-best-move">
=======
        <div v-if="store.result.analysis?.bestMove" class="result-section">
>>>>>>> 91bc0acbd98cf8d594617e2cd1a71e866976e702
          <h3>⭐ Лучший ход</h3>
          <p>
            Ход {{ store.result.analysis.bestMove.turn }}:
            {{ store.result.analysis.bestMove.summary }}
          </p>
        </div>

<<<<<<< HEAD
        <div v-if="store.result.achievements?.length" class="result-section result-achievements-section">
          <h3>🏆 Полученные достижения</h3>
          <div class="result-achievements">
            <article
              v-for="achievement in store.result.achievements"
              :key="achievement.code"
              :title="achievement.description || achievement.title"
            >
=======
        <div v-if="store.result.achievements?.length" class="result-section">
          <h3>🏆 Полученные достижения</h3>
          <div class="result-achievements">
            <article v-for="achievement in store.result.achievements" :key="achievement.code">
>>>>>>> 91bc0acbd98cf8d594617e2cd1a71e866976e702
              <strong>{{ achievement.title }}</strong>
              <span>{{ achievement.description }}</span>
            </article>
          </div>
        </div>

<<<<<<< HEAD
        <div v-if="store.retryCheckpoints.length" class="result-section result-retry">
=======
        <div v-if="store.retryCheckpoints.length" class="result-section">
>>>>>>> 91bc0acbd98cf8d594617e2cd1a71e866976e702
          <h3>↩ Быстрая работа над ошибками</h3>
          <div class="error-retry-actions">
            <button
              v-for="checkpoint in store.retryCheckpoints"
              :key="checkpoint.turn"
              class="checkpoint-button"
              :disabled="store.loading"
              @click="retryFromTurn(checkpoint.turn)"
            >
              {{ checkpoint.turn === 0 ? 'Начать заново' : `Продолжить после хода ${checkpoint.turn}` }}
            </button>
          </div>
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
<<<<<<< HEAD
      </section>
    </Transition>
=======
    </section>
>>>>>>> 91bc0acbd98cf8d594617e2cd1a71e866976e702
  </div>
</template>



