import api from './api'

export const walletService = {
  async getAllWallets() {
    const response = await api.get('/wallets')
    return response.data
  },

  async getWallet(pubKeyHash) {
    const response = await api.get(`/wallets/${pubKeyHash}`)
    return response.data
  },

  async createWallet(wallet) {
    const response = await api.post('/wallets', wallet)
    return response.data
  },

  async updateWallet(pubKeyHash, wallet) {
    const response = await api.put(`/wallets/${pubKeyHash}`, wallet)
    return response.data
  },

  async deleteWallet(pubKeyHash) {
    await api.delete(`/wallets/${pubKeyHash}`)
  }
}

