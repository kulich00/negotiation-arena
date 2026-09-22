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
        const key = `negotiation-session:${scenarioId}`
        const savedId = globalThis.sessionStorage?.getItem(key)
        if (savedId) {
          try {
            this.session = await api.getSession(savedId)
            this.messages = await api.getMessages(savedId)
            if (this.session.status === 'finished') this.result = await api.getResult(savedId)
            return
          } catch (error) {
            if (error.message !== 'session not found') throw error
            globalThis.sessionStorage?.removeItem(key)
          }
        }
        this.session = await api.startSession(scenarioId)
        globalThis.sessionStorage?.setItem(key, this.session.id)
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
          if (this.history.length > 20) {
            this.history.pop()
          }
          localStorage.setItem(HISTORY_STORAGE_KEY, JSON.stringify(this.history))
        }
      } catch (error) {
        this.error = error.message
      }
    },
    async restart(scenarioId) {
      globalThis.sessionStorage?.removeItem(`negotiation-session:${scenarioId}`)
      await this.startSession(scenarioId)
    },
    clearHistory() {
      this.history = []
      localStorage.removeItem(HISTORY_STORAGE_KEY)
    },
  },
})
    },
  },
})


