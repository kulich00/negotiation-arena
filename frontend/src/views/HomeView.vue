<script setup>
import { computed, ref, onMounted } from 'vue'
import { useNegotiationStore } from '../stores/negotiation'

const store = useNegotiationStore()

const searchQuery = ref('')
const selectedDifficulty = ref('all')
const selectedSphere = ref('all')

onMounted(() => {
  store.loadScenarios()
})

const availableSpheres = computed(() => {
  const spheres = new Set()
  store.scenarios.forEach((s) => {
    if (s.sphere) spheres.add(s.sphere)
  })
  return Array.from(spheres)
})

const filteredScenarios = computed(() => {
  return store.scenarios.filter((s) => {
    const query = searchQuery.value.toLowerCase().trim()
    const matchesSearch =
      !query ||
      s.title?.toLowerCase().includes(query) ||
      s.topic?.toLowerCase().includes(query) ||
      s.playerGoal?.toLowerCase().includes(query) ||
      s.opponentRole?.toLowerCase().includes(query)

    const matchesDifficulty =
      selectedDifficulty.value === 'all' || s.difficulty === selectedDifficulty.value

    const matchesSphere =
      selectedSphere.value === 'all' || s.sphere === selectedSphere.value

    return matchesSearch && matchesDifficulty && matchesSphere
  })
})

function translateDifficulty(diff) {
  switch (diff) {
    case 'easy':
      return { label: 'Легкий', class: 'badge-easy' }
    case 'medium':
      return { label: 'Средний', class: 'badge-medium' }
    case 'hard':
      return { label: 'Сложный', class: 'badge-hard' }
    default:
      return { label: diff || 'Нормальный', class: 'badge-default' }
  }
}
</script>

<template>
  <div class="home-page">
    <section class="hero">
      <p class="eyebrow">Тренажёр делового общения</p>
      <h1>Попробуйте стратегию до реальных переговоров</h1>
      <p class="hero-desc">
        Проходите интерактивные сценарии, проверяйте гипотезы, подбирайте верные формулировки и
        получайте экспертный разбор решений.
      </p>

      <div class="stats-pills">
        <div class="pill">
          <span class="pill-val">{{ store.scenarios.length }}</span>
          <span class="pill-lbl">Сценариев</span>
        </div>
        <div class="pill">
          <span class="pill-val">BATNA / SPIN</span>
          <span class="pill-lbl">Методологии</span>
        </div>
        <div class="pill">
          <span class="pill-val">ИИ Разбор</span>
          <span class="pill-lbl">Обратная связь</span>
        </div>
      </div>
    </section>

    <!-- Панель поиска и фильтрации -->
    <section class="filters-card">
      <div class="filters-grid">
        <div class="search-box">
          <label for="search-input" class="filter-label">Поиск сценария</label>
          <div class="input-wrapper">
            <svg class="search-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <circle cx="11" cy="11" r="8"></circle>
              <line x1="21" y1="21" x2="16.65" y2="16.65"></line>
            </svg>
            <input
              id="search-input"
              v-model="searchQuery"
              type="text"
              placeholder="Название, цель, тема или собеседник..."
            />
            <button v-if="searchQuery" class="clear-btn" @click="searchQuery = ''">✕</button>
          </div>
        </div>

        <div class="filter-group">
          <label for="difficulty-select" class="filter-label">Сложность</label>
          <select id="difficulty-select" v-model="selectedDifficulty">
            <option value="all">Все уровни</option>
            <option value="easy">Легкий</option>
            <option value="medium">Средний</option>
            <option value="hard">Сложный</option>
          </select>
        </div>

        <div class="filter-group" v-if="availableSpheres.length > 0">
          <label for="sphere-select" class="filter-label">Сфера</label>
          <select id="sphere-select" v-model="selectedSphere">
            <option value="all">Все сферы</option>
            <option v-for="sphere in availableSpheres" :key="sphere" :value="sphere">
              {{ sphere }}
            </option>
          </select>
        </div>
      </div>

      <div class="filter-meta" v-if="store.scenarios.length > 0">
        Найдено сценариев: <strong>{{ filteredScenarios.length }}</strong> из {{ store.scenarios.length }}
      </div>
    </section>

    <p v-if="store.error" class="alert">{{ store.error }}</p>

    <!-- Индикатор загрузки -->
    <div v-if="store.loading && !store.scenarios.length" class="loading-state">
      <div class="spinner"></div>
      <p>Загрузка сценариев...</p>
    </div>

    <!-- Список сценариев -->
    <section v-else-if="filteredScenarios.length > 0" class="card-grid">
      <article v-for="scenario in filteredScenarios" :key="scenario.id" class="card scenario-card">
        <div class="card-header">
          <span :class="['badge', translateDifficulty(scenario.difficulty).class]">
            {{ translateDifficulty(scenario.difficulty).label }}
          </span>
          <span v-if="scenario.sphere" class="sphere-tag">{{ scenario.sphere }}</span>
        </div>

        <h2 class="scenario-title">{{ scenario.title }}</h2>
        <p class="scenario-topic">{{ scenario.topic }}</p>

        <div class="scenario-details">
          <div class="detail-item">
            <span class="detail-label">Ваша цель:</span>
            <span class="detail-value">{{ scenario.playerGoal }}</span>
          </div>
          <div class="detail-item">
            <span class="detail-label">Собеседник:</span>
            <span class="detail-value">
              <strong>{{ scenario.opponentRole }}</strong>
              <em v-if="scenario.opponentTone"> (тон: {{ scenario.opponentTone }})</em>
            </span>
          </div>
        </div>

        <div class="card-footer">
          <RouterLink class="button start-btn" :to="`/scenario/${scenario.id}`">
            Начать переговоры
            <svg class="arrow-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M5 12h14M12 5l7 7-7 7" />
            </svg>
          </RouterLink>
        </div>
      </article>
    </section>

        <!-- История прошлых попыток пользователя -->
    <section v-if="store.history.length > 0" class="history-card">
      <div class="history-header">
        <div>
          <h2>📜 История ваших попыток ({{ store.history.length }})</h2>
          <p class="history-sub">Сравнивайте результаты и отслеживайте прогресс развития навыков</p>
        </div>
        <button class="button secondary small-btn" @click="store.clearHistory">Очистить историю</button>
      </div>

      <div class="history-grid">
        <div v-for="item in store.history" :key="item.id" class="history-item">
          <div class="h-main">
            <span class="h-title">{{ item.scenarioTitle }}</span>
            <span class="h-date">{{ item.date }} • {{ item.turnsCount }} ходов</span>
          </div>
          <div class="h-outcome">
            <span class="h-status">{{ item.outcome }}</span>
          </div>
          <div :class="['h-score', item.finalScore >= 70 ? 'score-high' : item.finalScore >= 40 ? 'score-mid' : 'score-low']">
            {{ item.finalScore }}/100
          </div>
        </div>
      </div>
    </section>

    <!-- Состояние пустых результатов -->
    <div v-else-if="!filteredScenarios.length" class="empty-state card">
      <h3>Сценарии не найдены</h3>
      <p>По вашему запросу не нашлось подходящих сценариев. Попробуйте сбросить фильтры.</p>
      <button class="button secondary" @click="searchQuery = ''; selectedDifficulty = 'all'; selectedSphere = 'all';">
        Сбросить фильтры
      </button>
    </div>
  </div>
</template>


