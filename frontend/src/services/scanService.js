import api from './api'

export const scanService = {
  async scanWallet(address) {
    const response = await api.post('/discovery/scan/wallet', { address })
    return response.data
  },

  async listScans(limit = 20, offset = 0) {
    const response = await api.get('/discovery/scans', {
      params: { limit, offset }
    })
    return response.data
  },

  async listRPCs() {
    const response = await api.get('/discovery/rpcs')
    return response.data
  }
}

