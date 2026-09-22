import { defineStore } from 'pinia'
import { api } from '../api/client'

const HISTORY_STORAGE_KEY = 'negotiation_attempts_history'

export const useNegotiationStore = defineStore('negotiation', {
  state: () => ({
    scenarios: [],
    session: null,
    messages: [],
    result: null,
    history: JSON.parse(localStorage.getItem(HISTORY_STORAGE_KEY) || '[]'),
    loading: false,
    error: '',
  }),
  actions: {
    async loadScenarios() {
      this.error = ''
      try {
        this.scenarios = await api.listScenarios()
      } catch (error) {
        this.error = error.message
      }
    },
    async startSession(scenarioId) {
      this.loading = true
      this.error = ''
      this.result = null
      try {
        this.session = await api.startSession(scenarioId)
        this.messages = [{ sender: 'opponent', content: this.session.initialMessage }]
      } catch (error) {
        this.error = error.message
        throw error
      } finally {
        this.loading = false
      }
    },
    async sendMessage(content) {
      if (!this.session || !content.trim()) return
      this.loading = true
      this.messages.push({ sender: 'player', content })
      try {
        const turn = await api.sendMessage(this.session.id, content)
        this.messages.push({ sender: 'opponent', content: turn.reply })
        this.session = { ...this.session, ...turn.session }
      } catch (error) {
        this.error = error.message
      } finally {
        this.loading = false
      }
    },
    async finish() {
      if (!this.session) return
      this.result = await api.finishSession(this.session.id)
      
      // Сохраняем попытку в историю
      if (this.result) {
        const scenario = this.scenarios.find((s) => s.id === this.session.scenarioId)
        const attempt = {
          id: Date.now().toString(),
          scenarioId: this.session.scenarioId,
          scenarioTitle: scenario ? scenario.title : 'Сценарий переговоров',
          date: new Date().toLocaleString('ru-RU', {
            day: 'numeric',
            month: 'short',
            hour: '2-digit',
            minute: '2-digit',
          }),
          turnsCount: this.session.turn,
          finalScore: this.result.finalScore,
          outcome: this.result.outcome,
          trustScore: this.session.trustScore,
          argumentScore: this.session.argumentScore,
        }

        this.history.unshift(attempt)
        // Храним последние 20 попыток
        if (this.history.length > 20) {
          this.history.pop()
        }
        localStorage.setItem(HISTORY_STORAGE_KEY, JSON.stringify(this.history))
      }
    },
    clearHistory() {
      this.history = []
      localStorage.removeItem(HISTORY_STORAGE_KEY)
    },
  },
})


