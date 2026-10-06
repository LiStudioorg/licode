<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

/**
 * 登录页。
 *
 * 必须是原生表单提交（method=post action=/login）：后端用 Cookie + 302
 * 处理登录，没有 JSON 登录接口。用 fetch + JSON 会拿不到 Set-Cookie 之后的
 * 跳转语义，也会因 Accept 头不同被 auth 中间件当成 API 请求回 401/302。
 *
 * 所以这里刻意不用 @submit.prevent，让浏览器原生提交。
 */
const username = ref('')
const password = ref('')
const showPw = ref(false)
const usernameEl = ref<HTMLInputElement | null>(null)
const submitting = ref(false)

const canSubmit = computed(() => username.value.trim().length > 0 && password.value.length > 0)

/** 后端在鉴权失败时 302 回 /login?error=1，这里据此给出提示 */
const failed = ref(false)

onMounted(() => {
  const p = new URLSearchParams(location.search)
  failed.value = p.has('error')
  usernameEl.value?.focus()
})

function onSubmit(e: Event) {
  if (!canSubmit.value) {
    e.preventDefault()
    return
  }
  // 提交后按钮进入等待态：原生提交期间页面不重绘，避免用户重复点击
  submitting.value = true
}
</script>

<template>
  <div class="grid min-h-dvh place-items-center bg-canvas px-4 text-ink-100">
    <div class="w-full max-w-[360px]">
      <div class="mb-5 flex flex-col items-center gap-2">
        <span
          class="grid size-9 place-items-center rounded-[10px] bg-accent font-mono text-[15px] text-[var(--color-accent-ink)]"
          aria-hidden="true"
          >L</span
        >
        <h1 class="text-ink-100">登录 Licode</h1>
        <p class="text-ink-500">请输入访问凭据</p>
      </div>

      <form method="post" action="/login" class="card" @submit="onSubmit">
        <p
          v-if="failed"
          class="mb-3 rounded-lg border border-danger/40 bg-danger/5 px-2.5 py-1.5 text-danger"
          role="alert"
        >
          用户名或密码不正确
        </p>

        <div class="field">
          <label for="username">用户名</label>
          <input
            id="username"
            ref="usernameEl"
            v-model="username"
            name="username"
            type="text"
            autocomplete="username"
            required
            autofocus
            class="input"
          />
        </div>

        <div class="field">
          <label for="password">密码</label>
          <div class="relative">
            <input
              id="password"
              v-model="password"
              name="password"
              :type="showPw ? 'text' : 'password'"
              autocomplete="current-password"
              required
              class="input pr-8"
            />
            <button
              type="button"
              class="absolute right-1.5 top-1/2 grid size-6 -translate-y-1/2 place-items-center rounded text-ink-500 transition-colors hover:text-ink-200"
              :aria-label="showPw ? '隐藏密码' : '显示密码'"
              @click="showPw = !showPw"
            >
              <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.3" class="size-3.5">
                <path v-if="!showPw" d="M1.5 8S4 3.5 8 3.5 14.5 8 14.5 8 12 12.5 8 12.5 1.5 8 1.5 8Z" />
                <circle v-if="!showPw" cx="8" cy="8" r="2" />
                <path v-else d="M2 2l12 12M6.5 6.6A2 2 0 0 0 8 10a2 2 0 0 0 1.5-.6M1.5 8S4 3.5 8 3.5c1 0 1.9.3 2.7.7M14.5 8s-1 1.8-2.8 3" />
              </svg>
            </button>
          </div>
        </div>

        <button
          type="submit"
          class="mt-1 h-9 w-full rounded-lg bg-accent text-[var(--color-accent-ink)] transition-opacity hover:opacity-90 active:scale-[0.99] disabled:opacity-50"
          :disabled="!canSubmit || submitting"
        >
          {{ submitting ? '登录中…' : '登录' }}
        </button>
      </form>

      <p class="mt-3 text-center text-ink-600">凭据由服务端 --username/--password 或 LICODE_PASSWORD 决定</p>
    </div>
  </div>
</template>

<style scoped>
.card {
  border: 1px solid var(--color-line);
  border-radius: var(--radius-card);
  background: var(--color-surface);
  padding: 16px;
}
.field {
  margin-bottom: 10px;
}
.field label {
  display: block;
  margin-bottom: 4px;
  color: var(--color-ink-400);
}
.input {
  height: 34px;
  width: 100%;
  border: 1px solid var(--color-line);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.02);
  padding: 0 10px;
  color: var(--color-ink-100);
  outline: none;
}
.input:focus {
  border-color: var(--color-accent-line);
}
</style>
