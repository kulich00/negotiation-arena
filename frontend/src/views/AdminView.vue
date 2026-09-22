<script setup>
import { reactive, ref, onMounted } from 'vue'
import { api } from '../api/client'
import { useNegotiationStore } from '../stores/negotiation'

const store = useNegotiationStore()

const token = ref(localStorage.getItem('admin_token') || '')
const status = ref('')
const error = ref('')
const loading = ref(false)

const credentials = reactive({ email: 'admin@example.com', password: '' })

const form = reactive({
  title: '',
  sphere: '',
  topic: '',
  difficulty: 'medium',
  opponentRole: '',
  opponentTone: '',
  playerGoal: '',
  opponentGoal: '',
  initialMessage: '',
})

onMounted(() => {
  if (token.value) {
    store.loadScenarios()
  }
})

async function login() {
  loading.value = true
  status.value = ''
  error.value = ''
  try {
    const result = await api.login(credentials.email, credentials.password)
    token.value = result.token
    localStorage.setItem('admin_token', result.token)
    status.value = 'Авторизация успешна'
    store.loadScenarios()
  } catch (err) {
    error.value = err.message || 'Ошибка авторизации'
  } finally {
    loading.value = false
  }
}

function logout() {
  token.value = ''
  localStorage.removeItem('admin_token')
  status.value = 'Вы вышли из системы'
}

async function createScenario() {
  loading.value = true
  status.value = ''
  error.value = ''
  try {
    await api.createScenario(token.value, { ...form })
    status.value = `Сценарий "${form.title}" успешно создан!`
    // Сброс формы
    form.title = ''
    form.sphere = ''
    form.topic = ''
    form.difficulty = 'medium'
    form.opponentRole = ''
    form.opponentTone = ''
    form.playerGoal = ''
    form.opponentGoal = ''
    form.initialMessage = ''
    // Обновляем список сценариев
    await store.loadScenarios()
  } catch (err) {
    error.value = err.message || 'Ошибка создания сценария'
  } finally {
    loading.value = false
  }
}

// Популярные преднастроенные шаблоны для быстрого заполнения
const templates = [
  {
    name: 'Переговоры о повышении зарплаты',
    data: {
      title: 'Пересмотр компенсации разработчика',
      sphere: 'HR & IT',
      topic: 'Согласование роста оклада на 25% после успешного релизов',
      difficulty: 'medium',
      opponentRole: 'Технический директор (CTO)',
      opponentTone: 'Сдержанный, прагматичный, требовательный к метрикам',
      playerGoal: 'Аргументировать ценность для компании и добиться повышения оклада на 20-25%',
      opponentGoal: 'Сдержать рост ФОТ, предложить альтернативные бонусы или отложить повышение',
      initialMessage: 'Привет! Я изучил твою заявку на пересмотр оклада. Давай обсудим, почему мы должны сделать это прямо сейчас?',
    },
  },
  {
    name: 'Продажа B2B сервиса трудному клиенту',
    data: {
      title: 'Сделка по внедрению CRM в ритейле',
      sphere: 'Продажи',
      topic: 'Защита бюджета на интеграцию ПО перед коммерческим директором',
      difficulty: 'hard',
      opponentRole: 'Коммерческий директор',
      opponentTone: 'Скептичный, экономный, давящий на скидку',
      playerGoal: 'Заключить контракт без скидки более 5% и зафиксировать годовую подписку',
      opponentGoal: 'Выбить скидку 30% и отсрочку платежа на 90 дней',
      initialMessage: 'У вас хорошее предложение, но ваша цена завышена минимум на треть. Что предложите?',
    },
  },
]

function applyTemplate(tpl) {
  Object.assign(form, tpl.data)
  status.value = `Загружен шаблон: ${tpl.name}`
}
</script>

<template>
  <div class="admin-page">
    <section class="panel admin-panel">
      <div class="admin-header">
        <div>
          <p class="eyebrow">Контур администратора</p>
          <h1>Настройка симулятора</h1>
        </div>
        <button v-if="token" class="button secondary logout-btn" @click="logout">
          Выйти из панели
        </button>
      </div>

      <p v-if="status" class="success banner">{{ status }}</p>
      <p v-if="error" class="alert banner">{{ error }}</p>

      <!-- Форма входа -->
      <div v-if="!token" class="login-wrapper">
        <p class="login-desc">Для создания и редактирования сценариев требуется вход в систему.</p>
        <form class="form-grid login-form" @submit.prevent="login">
          <label>
            Email администратора
            <input v-model="credentials.email" type="email" required placeholder="admin@example.com" />
          </label>
          <label>
            Пароль
            <input v-model="credentials.password" type="password" required placeholder="••••••••" />
          </label>
          <button class="button wide" :disabled="loading">
            {{ loading ? 'Вход...' : 'Войти в систему' }}
          </button>
        </form>
      </div>

      <!-- Форма создания сценария -->
      <div v-else class="admin-content">
        <!-- Шаблоны для быстрого старта -->
        <div class="templates-section">
          <span class="templates-label">Быстрые шаблоны сценариев:</span>
          <div class="templates-buttons">
            <button
              v-for="tpl in templates"
              :key="tpl.name"
              type="button"
              class="template-chip"
              @click="applyTemplate(tpl)"
            >
              ⚡ {{ tpl.name }}
            </button>
          </div>
        </div>

        <form class="form-grid scenario-form" @submit.prevent="createScenario">
          <label>
            Название сценария *
            <input v-model="form.title" required placeholder="Например: Переговоры о повышении зарплаты" />
          </label>

          <label>
            Сфера / Индустрия *
            <input v-model="form.sphere" required placeholder="IT, HR, Продажи, Закупки..." />
          </label>

          <label class="wide">
            Предмет переговоров / Тема *
            <input v-model="form.topic" required placeholder="Краткое описание сущности конфликта или переговоров" />
          </label>

          <label>
            Уровень сложности *
            <select v-model="form.difficulty" required>
              <option value="easy">Легкий (Easy)</option>
              <option value="medium">Средний (Medium)</option>
              <option value="hard">Сложный (Hard)</option>
            </select>
          </label>

          <label>
            Роль собеседника (Виртуального оппонента) *
            <input v-model="form.opponentRole" required placeholder="Например: Технический директор" />
          </label>

          <label class="wide">
            Тон и стилистика оппонента *
            <input v-model="form.opponentTone" required placeholder="Например: Настойчивый, скептичный, вежливый" />
          </label>

          <label class="wide">
            Цель игрока (Пользователя) *
            <textarea v-model="form.playerGoal" rows="2" required placeholder="Что должен сэмулировать или добиться пользователь" />
          </label>

          <label class="wide">
            Цель собеседника (Скрытые мотивы) *
            <textarea v-model="form.opponentGoal" rows="2" required placeholder="К чему стремится оппонент и какие уступки допустимы" />
          </label>

          <label class="wide">
            Первая реплика оппонента (Старт диалога) *
            <textarea v-model="form.initialMessage" rows="3" required placeholder="Приветственное или провокационное высказывание оппонента..." />
          </label>

          <div class="form-actions wide">
            <button class="button" :disabled="loading">
              {{ loading ? 'Сохранение...' : 'Опубликовать сценарий' }}
            </button>
          </div>
        </form>

        <!-- Список созданных сценариев -->
        <div class="existing-scenarios" v-if="store.scenarios.length">
          <h3>Существующие сценарии в системе ({{ store.scenarios.length }})</h3>
          <div class="scenarios-table">
            <div v-for="s in store.scenarios" :key="s.id" class="scenario-row">
              <div class="s-info">
                <strong>{{ s.title }}</strong>
                <span>{{ s.sphere }} • {{ s.difficulty }}</span>
              </div>
              <div class="s-role">Роль оппонента: {{ s.opponentRole }}</div>
            </div>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>


