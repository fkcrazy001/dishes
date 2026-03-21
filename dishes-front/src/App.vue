<script setup lang="ts">
import { onBeforeUnmount, onMounted } from 'vue'

import { useAppStore } from './store/appStore'

const store = useAppStore()
store.bootstrap()

const onKeydown = (e: KeyboardEvent) => {
  if (e.key === 'Escape' && store.dialogShow.value) store.closeDialog()
}

onMounted(() => window.addEventListener('keydown', onKeydown))
onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))
</script>

<template>
  <router-view />
  <div class="toast" :data-show="String(store.toastShow)" role="status" aria-live="polite">{{ store.toastText }}</div>
  <div v-if="store.dialogShow.value" class="modal" role="dialog" aria-modal="true" @click.self="store.closeDialog()">
    <div class="modal-card alert-card" :data-tone="store.dialogTone.value">
      <div class="alert-head">
        <div class="alert-left">
          <div class="alert-icon" aria-hidden="true"></div>
          <div>
            <div class="alert-title">{{ store.dialogTitle.value }}</div>
            <div class="alert-sub">小提示：修好之后就能继续做饭啦</div>
          </div>
        </div>
        <button type="button" class="btn ghost" @click="store.closeDialog()">关闭</button>
      </div>
      <div class="alert-body">
        <div class="alert-message">{{ store.dialogMessage.value }}</div>
      </div>
      <div class="alert-actions">
        <button type="button" class="btn primary" @click="store.closeDialog()">知道了</button>
      </div>
    </div>
  </div>
</template>
