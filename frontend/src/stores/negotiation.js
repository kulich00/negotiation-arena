import { defineStore } from 'pinia'
import { api } from '../api/client'

const HISTORY_STORAGE_KEY = 'negotiation_attempts_history'
const PLAYER_STORAGE_KEY = 'negotiation_player_id'
const DIFFICULTY_RANK = { easy: 1, medium: 2, hard: 3 }

function readHistory() {
  try {
    return JSON.parse(globalThis.localStorage?.getItem(HISTORY_STORAGE_KEY) || '[]')
  } catch {
    return []
  }
}

export const useNegotiationStore = defineStore('negotiation', {
  state: () => ({
    scenarios: [],
    achievementCatalog: [],
    player: null,
    session: null,
    messages: [],
    checkpoints: [],
    result: null,
    history: readHistory(),
    loading: false,
    error: '',
  }),
  getters: {
    isDifficultyUnlocked: (state) => (difficulty) => {
      if (!state.player) return difficulty === 'easy'
      return (DIFFICULTY_RANK[difficulty] || 0) <= (DIFFICULTY_RANK[state.player.unlockedDifficulty] || 1)
    },
    retryCheckpoints: (state) => state.checkpoints.filter(
      (checkpoint) => checkpoint.turn < (state.session?.turn || 0),
    ),
  },
  actions: {
    async initializeHome() {
      this.loading = true
      this.error = ''
      try {
        const [scenarios, achievements] = await Promise.all([
          api.listScenarios(),
          api.listAchievements(),
        ])
        this.scenarios = scenarios
        this.achievementCatalog = achievements
        if (this.player?.id) {
          await this.refreshPlayer()
        } else {
          await this.ensurePlayer()
        }
      } catch (error) {
        this.error = error.message
      } finally {
        this.loading = false
      }
    },
    async loadScenarios() {
      this.error = ''
      try {
        this.scenarios = await api.listScenarios()
      } catch (error) {
        this.error = error.message
      }
    },
    async ensurePlayer() {
      if (this.player) return this.player
      const storedId = globalThis.localStorage?.getItem(PLAYER_STORAGE_KEY)
      if (storedId) {
        try {
          this.player = await api.getPlayer(storedId)
          return this.player
        } catch (error) {
          if (error.message !== 'player not found') throw error
          globalThis.localStorage?.removeItem(PLAYER_STORAGE_KEY)
        }
      }
      this.player = await api.createPlayer('Игрок')
      globalThis.localStorage?.setItem(PLAYER_STORAGE_KEY, this.player.id)
      return this.player
    },
    async refreshPlayer() {
      if (!this.player?.id) return
      this.player = await api.getPlayer(this.player.id)
    },
    async startSession(scenarioId) {
      this.loading = true
      this.error = ''
      this.result = null
      try {
        await this.ensurePlayer()
        const key = `negotiation-session:${scenarioId}`
        const savedId = globalThis.sessionStorage?.getItem(key)
        if (savedId) {
          try {
            const savedSession = await api.getSession(savedId)
            if (savedSession.playerId === this.player.id) {
              this.session = savedSession
              this.messages = await api.getMessages(savedId)
              await this.loadCheckpoints(savedId)
              if (['finished', 'abandoned'].includes(this.session.status)) {
                this.result = await api.getResult(savedId)
                await this.refreshPlayer()
              }
              return
            }
            globalThis.sessionStorage?.removeItem(key)
          } catch (error) {
            if (error.message !== 'session not found') throw error
            globalThis.sessionStorage?.removeItem(key)
          }
        }
        this.session = await api.startSession(scenarioId, this.player.id)
        globalThis.sessionStorage?.setItem(key, this.session.id)
        this.messages = [{ sender: 'opponent', content: this.session.initialMessage }]
        await this.loadCheckpoints(this.session.id)
      } catch (error) {
        this.error = error.message
        this.session = null
        this.messages = []
        this.checkpoints = []
      } finally {
        this.loading = false
      }
    },
    async loadCheckpoints(sessionId = this.session?.id) {
      if (!sessionId) {
        this.checkpoints = []
        return
      }
      this.checkpoints = await api.getCheckpoints(sessionId)
    },
    async sendMessage(content) {
      if (!this.session || !content.trim()) return
      this.loading = true
      this.error = ''
      const playerMessage = { sender: 'player', content }
      this.messages.push(playerMessage)
      try {
        const turn = await api.sendMessage(this.session.id, content)
        playerMessage.analysis = turn.analysis
        this.messages.push({ sender: 'opponent', content: turn.reply })
        this.session = { ...this.session, ...turn.session }
        await this.loadCheckpoints(this.session.id)
        if (turn.result) {
          this.result = turn.result
          await this.recordCompletedSession()
        }
      } catch (error) {
        const pendingIndex = this.messages.indexOf(playerMessage)
        if (pendingIndex >= 0) this.messages.splice(pendingIndex, 1)
        this.error = error.message
      } finally {
        this.loading = false
      }
    },
    async finish() {
      if (!this.session) return
      this.loading = true
      this.error = ''
      try {
        this.result = await api.finishSession(this.session.id)
        this.session.status = 'finished'
        await this.recordCompletedSession()
      } catch (error) {
        this.error = error.message
      } finally {
        this.loading = false
      }
    },
    async recordCompletedSession() {
      if (!this.result || !this.session) return
      const scenario = this.scenarios.find((item) => item.id === this.session.scenarioId)
      const attempt = {
        id: this.session.id,
        scenarioId: this.session.scenarioId,
        scenarioTitle: scenario?.title || 'Сценарий переговоров',
        date: new Date().toLocaleString('ru-RU', {
          day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit',
        }),
        turnsCount: this.session.turn,
        finalScore: this.result.finalScore,
        outcome: this.result.outcome,
        trustScore: this.session.trustScore,
        argumentScore: this.session.argumentScore,
      }
      this.history = [attempt, ...this.history.filter((item) => item.id !== attempt.id)].slice(0, 20)
      globalThis.localStorage?.setItem(HISTORY_STORAGE_KEY, JSON.stringify(this.history))
      await this.refreshPlayer()
    },
    async forkFromTurn(turn) {
      if (!this.session || this.loading) return
      this.loading = true
      this.error = ''
      try {
        const child = await api.forkSession(this.session.id, turn)
        this.session = child
        this.result = null
        globalThis.sessionStorage?.setItem(`negotiation-session:${child.scenarioId}`, child.id)
        const [messages, checkpoints] = await Promise.all([
          api.getMessages(child.id),
          api.getCheckpoints(child.id),
        ])
        this.messages = messages
        this.checkpoints = checkpoints
      } catch (error) {
        this.error = error.message
      } finally {
        this.loading = false
      }
    },
    async restart(scenarioId) {
      globalThis.sessionStorage?.removeItem(`negotiation-session:${scenarioId}`)
      this.checkpoints = []
      await this.startSession(scenarioId)
    },
    clearHistory() {
      this.history = []
      globalThis.localStorage?.removeItem(HISTORY_STORAGE_KEY)
    },
  },
})
