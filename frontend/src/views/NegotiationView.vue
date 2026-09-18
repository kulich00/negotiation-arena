<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useNegotiationStore } from '../stores/negotiation'

const route = useRoute()
const store = useNegotiationStore()
const message = ref('')
const canSend = computed(() => message.value.trim() && !store.loading && !store.result)

onMounted(() => store.startSession(route.params.id))

async function sendMessage() {
  if (!canSend.value) return
  const content = message.value
  message.value = ''
  await store.sendMessage(content)
}
</script>

<template>
  <section class="negotiation-layout">
    <div class="panel">
      <p v-if="store.error" class="alert">{{ store.error }}</p>
      <h1>Раунд переговоров</h1>
      <div v-if="store.session" class="metrics">
        <span>Доверие: {{ store.session.trustScore }}</span>
        <span>Аргументация: {{ store.session.argumentScore }}</span>
        <span>Ход: {{ store.session.turn }}</span>
      </div>
      <div class="dialogue">
        <div v-for="(item, index) in store.messages" :key="index" :class="['message', item.sender]">
          {{ item.content }}
        </div>
      </div>
      <form v-if="!store.result" class="composer" @submit.prevent="sendMessage">
        <textarea v-model="message" placeholder="Введите свою реплику" rows="4" />
        <div class="actions">
          <button class="button" :disabled="!canSend">Отправить</button>
          <button class="button secondary" type="button" :disabled="!store.session || store.loading" @click="store.finish">Завершить</button>
        </div>
      </form>
    </div>

    <aside v-if="store.result" class="panel result">
      <p class="eyebrow">Результат</p>
      <h2>{{ store.result.outcome }}</h2>
      <div class="score">{{ store.result.finalScore }}/100</div>
      <h3>Сильные стороны</h3>
      <ul><li v-for="item in store.result.strengths" :key="item">{{ item }}</li></ul>
      <h3>Рекомендации</h3>
      <ul><li v-for="item in store.result.recommendations" :key="item">{{ item }}</li></ul>
      <button class="button" type="button" @click="store.restart(route.params.id)">Новая попытка</button>
    </aside>
  </section>
</template>

