async function request(path, options = {}) {
  const response = await fetch(path, {
    headers: { 'Content-Type': 'application/json', ...(options.headers || {}) },
    ...options,
  })

  const payload = await response.json().catch(() => ({}))
  if (!response.ok) {
    throw new Error(payload.error || `HTTP ${response.status}`)
  }
  return payload
}

export const api = {
  listScenarios: () => request('/api/v1/scenarios'),
  startSession: (scenarioId) => request('/api/v1/sessions', {
    method: 'POST',
    body: JSON.stringify({ scenarioId }),
  }),
  sendMessage: (sessionId, content) => request(`/api/v1/sessions/${sessionId}/messages`, {
    method: 'POST',
    body: JSON.stringify({ content }),
  }),
  finishSession: (sessionId) => request(`/api/v1/sessions/${sessionId}/finish`, {
    method: 'POST',
  }),
  login: (email, password) => request('/api/v1/admin/login', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  }),
  createScenario: (token, scenario) => request('/api/v1/admin/scenarios', {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}` },
    body: JSON.stringify(scenario),
  }),
}

