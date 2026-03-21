import { createRouter, createWebHistory } from 'vue-router'

import { useAppStore } from './store/appStore'
import DishesView from './views/DishesView.vue'
import LoginView from './views/LoginView.vue'
import RegisterView from './views/RegisterView.vue'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/login' },
    { path: '/login', name: 'login', component: LoginView, meta: { public: true } },
    { path: '/register', name: 'register', component: RegisterView, meta: { public: true } },
    { path: '/dishes', name: 'dishes', component: DishesView, meta: { requiresAuth: true } },
  ],
})

router.beforeEach((to) => {
  const store = useAppStore()
  if (to.meta.requiresAuth && !store.auth.loggedIn) return { path: '/login', query: { redirect: to.fullPath } }
  if (to.meta.public && store.auth.loggedIn) return { path: '/dishes' }
  return true
})

