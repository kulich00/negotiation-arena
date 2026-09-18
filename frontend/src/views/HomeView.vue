<script setup>
import { onMounted } from 'vue'
import { useNegotiationStore } from '../stores/negotiation'

const store = useNegotiationStore()
onMounted(() => store.loadScenarios())
</script>

<template>
  <section class="hero">
    <p class="eyebrow">Тренажёр делового общения</p>
    <h1>Попробуйте стратегию до реальных переговоров</h1>
    <p>Проходите сценарии, меняйте формулировки и получайте разбор решений.</p>
  </section>

  <p v-if="store.error" class="alert">{{ store.error }}</p>
  <section class="card-grid">
    <article v-for="scenario in store.scenarios" :key="scenario.id" class="card">
      <span class="badge">{{ scenario.difficulty }}</span>
      <h2>{{ scenario.title }}</h2>
      <p>{{ scenario.topic }}</p>
      <dl>
        <dt>Ваша цель</dt><dd>{{ scenario.playerGoal }}</dd>
        <dt>Собеседник</dt><dd>{{ scenario.opponentRole }}</dd>
      </dl>
      <RouterLink class="button" :to="`/scenario/${scenario.id}`">Начать</RouterLink>
    </article>
  </section>
</template>

