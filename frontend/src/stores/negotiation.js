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
        const key = `negotiation-session:${scenarioId}`
        const savedId = globalThis.sessionStorage.getItem(key)
        if (savedId) {
          try {
            this.session = await api.getSession(savedId)
            this.messages = await api.getMessages(savedId)
            if (this.session.status === 'finished') this.result = await api.getResult(savedId)
            return
          } catch (error) {
            if (error.message !== 'session not found') throw error
            globalThis.sessionStorage.removeItem(key)
          }
        }
        this.session = await api.startSession(scenarioId)
        globalThis.sessionStorage.setItem(key, this.session.id)
        this.messages = [{ sender: 'opponent', content: this.session.initialMessage }]
      } catch (error) {
        this.error = error.message
        this.session = null
        this.messages = []
      } finally {
        this.loading = false
      }
    },
    async sendMessage(content) {
      if (!this.session || !content.trim()) return
      this.loading = true
      this.error = ''
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
      this.error = ''
      try {
        this.result = await api.finishSession(this.session.id)
        this.session.status = 'finished'
      } catch (error) {
        this.messages.pop()
        this.error = error.message
      }
    },
    async restart(scenarioId) {
      globalThis.sessionStorage.removeItem(`negotiation-session:${scenarioId}`)
      await this.startSession(scenarioId)
    },
  },
})

