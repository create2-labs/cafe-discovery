import api from './api'

export const planService = {
  async getCurrentPlan() {
    const response = await api.get('/plans/current')
    return response.data
  },

  async getAllPlans() {
    const response = await api.get('/plans')
    return response.data
  },

  async getPlanUsage() {
    const response = await api.get('/plans/usage')
    return response.data
  }
}

