import { createRouter, createWebHistory } from 'vue-router'
import { useUserStore } from '../stores/user'

const routes = [
  {
    path: '/',
    redirect: '/chat'
  },
  {
    path: '/login',
    name: 'Login',
    component: () => import('../views/Login.vue')
  },
  {
    path: '/register',
    name: 'Register',
    component: () => import('../views/Register.vue')
  },
  {
    path: '/chat',
    name: 'Chat',
    component: () => import('../views/ChatRoom.vue'),
    meta: { requiresAuth: true }
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

// 路由守卫：未登录时重定向到登录页
router.beforeEach((to, from, next) => {
  if (to.meta.requiresAuth) {
    const userStore = useUserStore()
    const restoredUser = userStore.restoreCurrentUser?.()
    // 有效用户必须有 ID
    if (restoredUser?.ID != null) {
      next()
      return
    }

    // 内存中没有有效用户，尝试从存储中解析验证
    try {
      const saved = sessionStorage.getItem('ginchat_user') || localStorage.getItem('ginchat_user')
      if (saved) {
        const parsed = JSON.parse(saved)
        if (parsed && typeof parsed === 'object' && (parsed.ID != null || parsed.id != null)) {
          // 存储中有有效用户数据，尝试恢复到 store
          const restored = userStore.restoreCurrentUser?.()
          if (restored?.ID != null) {
            next()
            return
          }
        }
      }
    } catch {
      // JSON 解析失败，存储数据已损坏
    }

    // 没有有效用户，跳转到登录页
    next('/login')
  } else {
    next()
  }
})

export default router
