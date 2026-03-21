<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { useAppStore } from '../store/appStore'

const store = useAppStore()
const router = useRouter()
const route = useRoute()

const account = ref(typeof route.query.account === 'string' ? route.query.account : '')
const password = ref('')
const remember = ref(true)

const submit = async () => {
  const a = account.value.trim()
  const p = password.value
  if (!a || !p) {
    store.openDialog({ title: '登录失败', message: '请输入账号和密码' })
    return
  }
  try {
    await store.loginWithPassword({ account: a, password: p, remember: remember.value })
    store.showToast(remember.value ? '登录成功（已记住我）' : '登录成功')
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/dishes'
    await router.replace(redirect)
  } catch (e) {
    store.openErrorDialog('登录失败', e, '登录失败')
  }
}
</script>

<template>
  <div class="wrap">
    <header class="top">
      <div class="brand">
        <div class="badge">家</div>
        <div>
          <h1>家庭厨房</h1>
          <p>欢迎回来，先登录再决定今天吃什么</p>
        </div>
      </div>

      <div class="top-right">
        <div class="nav" aria-label="页面导航">
          <button type="button" class="chip" data-active="true">登录</button>
          <button type="button" class="chip" @click="router.push('/register')">注册</button>
        </div>
      </div>
    </header>

    <section class="auth-shell">
      <div class="panel auth-panel">
        <div class="panel-head">
          <h2>欢迎回来</h2>
          <div class="meta">登录后就能保存常做菜谱</div>
        </div>

        <div class="auth-body">
          <div class="auth-hero">
            <div class="auth-kicker">家里人都在等你开饭</div>
            <div class="auth-title">登录</div>
            <div class="auth-desc">输入账号密码即可进入应用。</div>
          </div>

          <form class="auth-form" @submit.prevent="submit">
            <label class="field">
              <div class="field-label">账号</div>
              <input
                v-model="account"
                class="field-input"
                placeholder="手机号 / 邮箱"
                autocomplete="username"
              />
            </label>

            <label class="field">
              <div class="field-label">密码</div>
              <input
                v-model="password"
                class="field-input"
                type="password"
                placeholder="请输入密码"
                autocomplete="current-password"
              />
            </label>

            <div class="field-row">
              <label class="check">
                <input v-model="remember" type="checkbox" />
                <span>记住我</span>
              </label>
              <button type="button" class="link" @click="store.openDialog({ title: '小提示', message: '暂不提供找回密码功能～' })">
                忘记密码？
              </button>
            </div>

            <div class="auth-actions">
              <button type="submit" class="btn primary">登录</button>
              <button type="button" class="btn ghost" @click="router.push('/register')">去注册</button>
            </div>
          </form>
        </div>
      </div>
    </section>
  </div>
</template>
