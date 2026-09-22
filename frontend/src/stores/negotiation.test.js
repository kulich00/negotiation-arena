import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useNegotiationStore } from './negotiation'
import { api } from '../api/client'

vi.mock('../api/client', () => ({
  api: {
    getSession: vi.fn(),
    getMessages: vi.fn(),
    getResult: vi.fn(),
    startSession: vi.fn(),
  },
}))

describe('negotiation session restoration', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    globalThis.sessionStorage.clear()
    vi.clearAllMocks()
  })

  it('restores the persisted dialogue and result after a page reload', async () => {
    globalThis.sessionStorage.setItem('negotiation-session:salary', 'saved-session')
    api.getSession.mockResolvedValue({ id: 'saved-session', status: 'finished' })
    api.getMessages.mockResolvedValue([{ sender: 'player', content: 'Условия?' }])
    api.getResult.mockResolvedValue({ finalScore: 70 })

    const store = useNegotiationStore()
    await store.startSession('salary')

    expect(api.startSession).not.toHaveBeenCalled()
    expect(store.messages).toEqual([{ sender: 'player', content: 'Условия?' }])
    expect(store.result.finalScore).toBe(70)
  })
})
