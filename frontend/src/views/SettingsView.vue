<script setup lang="ts">
import { reactive, ref } from 'vue'
import { api } from '../api'

defineProps<{ dark: boolean; resolution: number }>()
const emit = defineEmits<{
  'update:dark': [value: boolean]
  'update:resolution': [value: number]
  'session-ended': []
}>()

const password = reactive({ current: '', next: '', confirm: '' })
const busy = ref(false)
const notice = ref('')
const isError = ref(false)

async function changePassword() {
  if (busy.value) return
  notice.value = ''
  isError.value = false
  if (password.next !== password.confirm) {
    notice.value = 'Новые пароли не совпадают.'
    isError.value = true
    return
  }
  busy.value = true
  try {
    await api.ChangePassword(password.current, password.next)
    Object.assign(password, { current: '', next: '', confirm: '' })
    notice.value = 'Пароль изменён. Войдите снова.'
    emit('session-ended')
  } catch (error) {
    notice.value = error instanceof Error ? error.message : String(error)
    isError.value = true
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <section class="panel">
    <h2>Оформление и графики</h2>
    <div class="form-grid">
      <label>
        Тема
        <select
          :value="dark ? 'dark' : 'light'"
          @change="emit('update:dark', ($event.target as HTMLSelectElement).value === 'dark')"
        >
          <option value="light">Светлая</option>
          <option value="dark">Тёмная</option>
        </select>
      </label>
      <label>
        Разрешение графика
        <select
          :value="resolution"
          @change="emit('update:resolution', Number(($event.target as HTMLSelectElement).value))"
        >
          <option :value="41">41 × 41 — быстро</option>
          <option :value="71">71 × 71 — стандартное</option>
          <option :value="111">111 × 111 — подробно</option>
        </select>
      </label>
    </div>
    <p>Разрешение задаёт сетку графика и не влияет на точность оптимизации.</p>
  </section>

  <form class="panel" @submit.prevent="changePassword">
    <h2>Изменение пароля</h2>
    <div class="form-grid">
      <label>
        Текущий пароль
        <input
          v-model="password.current"
          type="password"
          autocomplete="current-password"
          minlength="4"
          maxlength="1024"
          required
          :disabled="busy"
        />
      </label>
      <label>
        Новый пароль
        <input
          v-model="password.next"
          type="password"
          autocomplete="new-password"
          minlength="4"
          maxlength="1024"
          required
          :disabled="busy"
        />
      </label>
      <label>
        Повторите новый пароль
        <input
          v-model="password.confirm"
          type="password"
          autocomplete="new-password"
          minlength="4"
          maxlength="1024"
          required
          :disabled="busy"
        />
      </label>
    </div>
    <p>Пароль должен содержать от 4 до 1024 символов. После изменения потребуется войти снова.</p>
    <p
      v-if="notice"
      class="notice"
      :class="{ error: isError }"
      :role="isError ? 'alert' : 'status'"
    >
      {{ notice }}
    </p>
    <button class="button" :disabled="busy">
      {{ busy ? 'Сохранение…' : 'Изменить пароль' }}
    </button>
  </form>
</template>
