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
})

