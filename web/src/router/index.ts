import { createRouter, createWebHistory } from 'vue-router'

import { useAuthStore } from '@/stores/auth'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: '/', name: 'dashboard', component: () => import('@/views/DashboardView.vue'), meta: { auth: true } },
    { path: '/services', name: 'services', component: () => import('@/views/ServicesView.vue'), meta: { auth: true } },
    { path: '/login', name: 'login', component: () => import('@/views/LoginView.vue') },
    { path: '/register', name: 'register', component: () => import('@/views/RegisterView.vue') },
  ],
})

// 登录态守卫：未登录访问面板 → 登录页（带 redirect）；已登录访问登录/注册 → 概览
router.beforeEach((to) => {
  const auth = useAuthStore()
  const isAuthPage = to.name === 'login' || to.name === 'register'
  if (!isAuthPage && !auth.isLoggedIn) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  if (isAuthPage && auth.isLoggedIn) {
    return { name: 'dashboard' }
  }
  return true
})

export default router
