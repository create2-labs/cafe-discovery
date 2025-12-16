import api from './api'

export const authService = {
  async signIn(email, password) {
    const response = await api.post('/auth/signin', { email, password })
    return response.data
  },

  async signUp(email, password, confirmPassword) {
    const response = await api.post('/auth/signup', {
      email,
      password,
      confirm_password: confirmPassword
    })
    return response.data
  }
}

