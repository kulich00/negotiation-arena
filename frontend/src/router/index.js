import { createRouter, createWebHistory } from 'vue-router'
import HomeView from '../views/HomeView.vue'
import NegotiationView from '../views/NegotiationView.vue'
import AdminView from '../views/AdminView.vue'

export default createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', component: HomeView },
    { path: '/scenario/:id', component: NegotiationView },
    { path: '/admin', component: AdminView },
  ],
})

