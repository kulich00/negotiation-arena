import { describe, expect, it, vi } from 'vitest'
import { api } from './client'

describe('api client', () => {
  it('loads scenarios', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: async () => [{ id: 'salary-negotiation' }],
    }))

    await expect(api.listScenarios()).resolves.toEqual([{ id: 'salary-negotiation' }])
  })

  it('sends the admin token when logging out', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({}),
    })
    vi.stubGlobal('fetch', fetchMock)

    await api.logout('session-token')

    expect(fetchMock).toHaveBeenCalledWith('/api/v1/admin/logout', expect.objectContaining({
      method: 'POST',
      headers: expect.objectContaining({
        Authorization: 'Bearer session-token',
        'Content-Type': 'application/json',
      }),
    }))
  })

  it('starts a session for the local player', async () => {
    const fetchMock = vi.fn().mockResolvedValue({ ok: true, json: async () => ({ id: 'session-1' }) })
    vi.stubGlobal('fetch', fetchMock)

    await api.startSession('vendor-introduction', 'player-1')

    expect(fetchMock).toHaveBeenCalledWith('/api/v1/sessions', expect.objectContaining({
      method: 'POST',
      body: JSON.stringify({ scenarioId: 'vendor-introduction', playerId: 'player-1' }),
    }))
  })

  it('creates a branch from a checkpoint', async () => {
    const fetchMock = vi.fn().mockResolvedValue({ ok: true, json: async () => ({ id: 'child-1' }) })
    vi.stubGlobal('fetch', fetchMock)

    await api.forkSession('parent-1', 2)

    expect(fetchMock).toHaveBeenCalledWith('/api/v1/sessions/parent-1/fork', expect.objectContaining({
      method: 'POST',
      body: JSON.stringify({ turn: 2 }),
    }))
  })
})

