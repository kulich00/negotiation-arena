import { defineStore } from 'pinia'
import { api } from '../api/client'

export const useNegotiationStore = defineStore('negotiation', {
  state: () => ({
    scenarios: [],
    session: null,
    messages: [],
    result: null,
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
    },
  },
})

