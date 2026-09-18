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
})

