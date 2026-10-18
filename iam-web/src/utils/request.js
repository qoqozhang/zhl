import axios from 'axios'

const TOKEN_KEY = 'access_token'
const REFRESH_TOKEN_KEY = 'refresh_token'

let isRefreshing = false
let pendingRequests = []

const request = axios.create({
  baseURL: '/api',
  timeout: 15000,
})

const getAccessToken = () => localStorage.getItem(TOKEN_KEY)
const getRefreshToken = () => localStorage.getItem(REFRESH_TOKEN_KEY)

const setTokens = (accessToken, refreshToken) => {
  localStorage.setItem(TOKEN_KEY, accessToken)
  if (refreshToken) {
    localStorage.setItem(REFRESH_TOKEN_KEY, refreshToken)
  }
}

const clearTokens = () => {
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(REFRESH_TOKEN_KEY)
}

request.interceptors.request.use(
  (config) => {
    const token = getAccessToken()
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (error) => Promise.reject(error),
)

request.interceptors.response.use(
  (response) => response.data,
  async (error) => {
    const originalRequest = error.config

    if (error.response?.status === 401 && !originalRequest._retry) {
      if (isRefreshing) {
        return new Promise((resolve, reject) => {
          pendingRequests.push({ resolve, reject, config: originalRequest })
        })
      }

      originalRequest._retry = true
      isRefreshing = true

      try {
        const refreshToken = getRefreshToken()
        if (!refreshToken) {
          clearTokens()
          return Promise.reject(error)
        }

        const response = await axios.post('/api/users/refresh', {
          refresh_token: refreshToken,
        })

        const { access_token, refresh_token } = response.data?.data || response.data
        setTokens(access_token, refresh_token)

        request.defaults.headers.Authorization = `Bearer ${access_token}`

        pendingRequests.forEach(({ resolve, reject, config }) => {
          config.headers.Authorization = `Bearer ${access_token}`
          resolve(request(config))
        })
        pendingRequests = []

        originalRequest.headers.Authorization = `Bearer ${access_token}`
        return request(originalRequest)
      } catch (refreshError) {
        clearTokens()
        pendingRequests.forEach(({ reject }) => reject(refreshError))
        pendingRequests = []
        return Promise.reject(refreshError)
      } finally {
        isRefreshing = false
      }
    }

    return Promise.reject(error)
  },
)

export { setTokens, clearTokens }
export default request