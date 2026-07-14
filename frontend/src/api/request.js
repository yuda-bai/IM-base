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
