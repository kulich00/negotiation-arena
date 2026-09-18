<script setup>
import { RouterLink, RouterView } from 'vue-router'
</script>

<template>
  <div class="app-shell">
    <header class="topbar">
      <RouterLink class="brand" to="/">Арена переговоров</RouterLink>
      <nav>
        <RouterLink to="/">Сценарии</RouterLink>
        <RouterLink to="/admin">Администратор</RouterLink>
      </nav>
    </header>
    <main class="container">
      <RouterView />
    </main>
  </div>
</template>

