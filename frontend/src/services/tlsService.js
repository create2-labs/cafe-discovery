import api from './api'

export const tlsService = {
  async scanEndpoint(url) {
    const response = await api.post('/discovery/scan/endpoints', { url })
    return response.data
  },

  async listScans(limit = 20, offset = 0) {
    const response = await api.get('/discovery/tls/scans', {
      params: { limit, offset }
    })
    return response.data
  }
}

