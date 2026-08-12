import axios from 'axios'

const request = axios.create({
  // 开发环境：'/' → Vite proxy 代理到后端
  // 生产环境：'/' → 前后端同源
  baseURL: import.meta.env.VITE_API_BASE_URL || '/',
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json'
  }
})

request.interceptors.request.use((config) => {
  try {
    const saved = sessionStorage.getItem('ginchat_user') || localStorage.getItem('ginchat_user')
    const user = saved ? JSON.parse(saved) : null
    const identity = user?.Identity || user?.identity || sessionStorage.getItem('ginchat_user_identity') || localStorage.getItem('ginchat_user_identity') || ''
    if (identity) {
      config.headers = config.headers || {}
      config.headers.Authorization = `Bearer ${identity}`
    }
  } catch {
    // 存储数据损坏时由服务端返回未授权，避免阻断其他请求。
  }
  return config
})

// 响应拦截器
request.interceptors.response.use(
  (response) => {
    return response.data
  },
  (error) => {
    console.error('请求失败:', error)
    return Promise.reject(error)
  }
)

export default request
