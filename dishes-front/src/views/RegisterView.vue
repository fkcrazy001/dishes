<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'

import { useAppStore } from '../store/appStore'

const store = useAppStore()
const router = useRouter()

const name = ref('')
const account = ref('')
const password = ref('')
const confirm = ref('')
const agree = ref(true)

const submit = async () => {
  const n = name.value.trim()
  const a = account.value.trim()
  const p = password.value
  const c = confirm.value

  if (!n || !a || !p || !c) {
    store.openDialog({ title: '注册信息不完整', message: '把昵称、账号和密码都填好再试试～', tone: 'error' })
    return
  }
  if (p.length < 6) {
    store.openDialog({ title: '密码有点短', message: '密码至少 6 位哦～', tone: 'error' })
    return
  }
  if (p !== c) {
    store.openDialog({ title: '两次密码不一致', message: '再确认一下两次输入是否一致～', tone: 'error' })
    return
  }
  if (!agree.value) {
    store.openDialog({ title: '还差一步', message: '请先勾选同意条款～', tone: 'error' })
    return
  }
  try {
    await store.register({ account: a, password: p, name: n })
    store.showToast('注册成功，已为你跳转登录')
    await router.replace({ path: '/login', query: { account: a } })
  } catch (e) {
    store.openErrorDialog('注册失败', e, '注册失败')
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
          <p>创建账号，把家的味道装进小本子</p>
        </div>
      </div>

      <div class="top-right">
        <div class="nav" aria-label="页面导航">
          <button type="button" class="chip" @click="router.push('/login')">登录</button>
          <button type="button" class="chip" data-active="true">注册</button>
        </div>
      </div>
    </header>

    <section class="auth-shell">
      <div class="panel auth-panel">
        <div class="panel-head">
          <h2>创建家庭账号</h2>
          <div class="meta">注册后可同步点菜单与偏好</div>
        </div>

        <div class="auth-body">
          <div class="auth-hero">
            <div class="auth-kicker">把家的味道装进小本子</div>
            <div class="auth-title">注册</div>
            <div class="auth-desc">用昵称、手机号/邮箱创建账号。</div>
          </div>

          <form class="auth-form" @submit.prevent="submit">
            <label class="field">
              <div class="field-label">昵称</div>
              <input v-model="name" class="field-input" placeholder="例如：小熊 / 妈妈 / 阿杰" autocomplete="name" />
            </label>

            <label class="field">
              <div class="field-label">账号</div>
              <input v-model="account" class="field-input" placeholder="手机号 / 邮箱" autocomplete="username" />
            </label>

            <div class="field-grid">
              <label class="field">
                <div class="field-label">密码</div>
                <input
                  v-model="password"
                  class="field-input"
                  type="password"
                  placeholder="至少 6 位"
                  autocomplete="new-password"
                />
              </label>
              <label class="field">
                <div class="field-label">确认密码</div>
                <input
                  v-model="confirm"
                  class="field-input"
                  type="password"
                  placeholder="再输一次"
                  autocomplete="new-password"
                />
              </label>
            </div>

            <div class="field-row">
              <label class="check">
                <input v-model="agree" type="checkbox" />
                <span>我同意家庭使用条款</span>
              </label>
              <button
                type="button"
                class="link"
                @click="store.openDialog({ title: '条款说明', message: '条款内容还在完善中～先把功能跑起来再慢慢补齐。' })"
              >
                查看条款
              </button>
            </div>

            <div class="auth-actions">
              <button type="submit" class="btn primary">注册</button>
              <button type="button" class="btn ghost" @click="router.push('/login')">去登录</button>
            </div>
          </form>
        </div>
      </div>
    </section>
  </div>
</template>
