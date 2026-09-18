<script setup>
import { reactive, ref } from 'vue'
import { api } from '../api/client'

const token = ref('')
const status = ref('')
const credentials = reactive({ email: 'admin@example.com', password: '' })
const form = reactive({
  title: '', sphere: '', topic: '', difficulty: 'medium',
  opponentRole: '', opponentTone: '', playerGoal: '', opponentGoal: '', initialMessage: '',
})

async function login() {
  const result = await api.login(credentials.email, credentials.password)
  token.value = result.token
  status.value = 'Вход выполнен'
}

async function createScenario() {
  await api.createScenario(token.value, form)
  status.value = 'Сценарий создан'
}
</script>

<template>
  <section class="panel admin">
    <p class="eyebrow">Контур администратора</p>
    <h1>Настройка симуляции</h1>
    <p v-if="status" class="success">{{ status }}</p>

    <form v-if="!token" class="form-grid" @submit.prevent="login">
      <label>Email<input v-model="credentials.email" type="email" required /></label>
      <label>Пароль<input v-model="credentials.password" type="password" required /></label>
      <button class="button">Войти</button>
    </form>

    <form v-else class="form-grid" @submit.prevent="createScenario">
      <label>Название<input v-model="form.title" required /></label>
      <label>Сфера<input v-model="form.sphere" required /></label>
      <label>Тема<input v-model="form.topic" required /></label>
      <label>Сложность<select v-model="form.difficulty"><option>easy</option><option>medium</option><option>hard</option></select></label>
      <label>Роль собеседника<input v-model="form.opponentRole" required /></label>
      <label>Тон<input v-model="form.opponentTone" required /></label>
      <label>Цель игрока<textarea v-model="form.playerGoal" required /></label>
      <label>Цель собеседника<textarea v-model="form.opponentGoal" required /></label>
      <label class="wide">Первая реплика<textarea v-model="form.initialMessage" required /></label>
      <button class="button">Создать сценарий</button>
    </form>
  </section>
</template>

