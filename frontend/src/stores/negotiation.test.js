import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useNegotiationStore } from './negotiation'
import { api } from '../api/client'

vi.mock('../api/client', () => ({
  api: {
    listScenarios: vi.fn(),
    listAchievements: vi.fn(),
    createPlayer: vi.fn(),
    getPlayer: vi.fn(),
    getSession: vi.fn(),
    getMessages: vi.fn(),
    getCheckpoints: vi.fn(),
    getResult: vi.fn(),
    startSession: vi.fn(),
    sendMessage: vi.fn(),
    finishSession: vi.fn(),
    forkSession: vi.fn(),
  },
}))

describe('negotiation store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    globalThis.localStorage.clear()
    globalThis.sessionStorage.clear()
    vi.clearAllMocks()
    api.listScenarios.mockResolvedValue([])
    api.listAchievements.mockResolvedValue([])
    api.getCheckpoints.mockResolvedValue([])
  })

  it('creates one local player and stores its id', async () => {
    api.createPlayer.mockResolvedValue({ id: 'player-1', unlockedDifficulty: 'easy' })

    const store = useNegotiationStore()
    await store.ensurePlayer()
    await store.ensurePlayer()

    expect(api.createPlayer).toHaveBeenCalledOnce()
    expect(globalThis.localStorage.getItem('negotiation_player_id')).toBe('player-1')
    expect(store.isDifficultyUnlocked('easy')).toBe(true)
    expect(store.isDifficultyUnlocked('medium')).toBe(false)
  })

  it('refreshes a cached player when the home page is initialized', async () => {
    api.getPlayer.mockResolvedValue({ id: 'player-1', unlockedDifficulty: 'medium' })
    const store = useNegotiationStore()
    store.player = { id: 'player-1', unlockedDifficulty: 'easy' }

    await store.initializeHome()

    expect(api.getPlayer).toHaveBeenCalledWith('player-1')
    expect(store.player.unlockedDifficulty).toBe('medium')
  })

  it('restores the persisted dialogue, checkpoints and result after a page reload', async () => {
    globalThis.localStorage.setItem('negotiation_player_id', 'player-1')
    globalThis.sessionStorage.setItem('negotiation-session:salary', 'saved-session')
    api.getPlayer.mockResolvedValue({ id: 'player-1', unlockedDifficulty: 'medium' })
    api.getSession.mockResolvedValue({
      id: 'saved-session', scenarioId: 'salary', playerId: 'player-1', status: 'finished', turn: 2,
    })
    api.getMessages.mockResolvedValue([{ sender: 'player', content: 'Условия?' }])
    api.getCheckpoints.mockResolvedValue([{ turn: 0 }, { turn: 1 }, { turn: 2 }])
    api.getResult.mockResolvedValue({ finalScore: 70 })

    const store = useNegotiationStore()
    await store.startSession('salary')

    expect(api.startSession).not.toHaveBeenCalled()
    expect(store.messages).toEqual([{ sender: 'player', content: 'Условия?' }])
    expect(store.checkpoints).toHaveLength(3)
    expect(store.result.finalScore).toBe(70)
  })

  it('stores per-turn analysis and refreshes checkpoints', async () => {
    const analysis = { intent: 'pressure', trustDelta: -2, errors: [{ code: 'pressure_tactic' }] }
    api.sendMessage.mockResolvedValue({
      reply: 'Вернёмся к фактам.',
      session: { id: 'session-1', scenarioId: 'easy', status: 'active', turn: 1, trustScore: 48 },
      analysis,
    })
    api.getCheckpoints.mockResolvedValue([{ turn: 0 }, { turn: 1 }])
    const store = useNegotiationStore()
    store.session = { id: 'session-1', scenarioId: 'easy', status: 'active', turn: 0 }

    await store.sendMessage('Делайте, как я сказал')

    expect(store.messages[0].analysis).toEqual(analysis)
    expect(store.messages[1]).toEqual({ sender: 'opponent', content: 'Вернёмся к фактам.' })
    expect(store.session.trustScore).toBe(48)
    expect(store.checkpoints).toHaveLength(2)
  })

  it('switches the active dialogue to a forked session', async () => {
    const child = {
      id: 'child-1', scenarioId: 'easy', parentSessionId: 'parent-1', forkedFromTurn: 1,
      status: 'active', turn: 1,
    }
    api.forkSession.mockResolvedValue(child)
    api.getMessages.mockResolvedValue([{ sender: 'opponent', content: 'Продолжим' }])
    api.getCheckpoints.mockResolvedValue([{ turn: 0 }, { turn: 1 }])
    const store = useNegotiationStore()
    store.session = { id: 'parent-1', scenarioId: 'easy', status: 'finished', turn: 3 }
    store.result = { finalScore: 20 }

    await store.forkFromTurn(1)

    expect(api.forkSession).toHaveBeenCalledWith('parent-1', 1)
    expect(store.session).toEqual(child)
    expect(store.result).toBeNull()
    expect(globalThis.sessionStorage.getItem('negotiation-session:easy')).toBe('child-1')
  })
})
