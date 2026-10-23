<script setup lang="ts">
import { reactive, ref } from 'vue'
import { api, type State } from '../api'

const props = defineProps<{ needsAdmin: boolean }>()
const emit = defineEmits<{ authenticated: [state: State] }>()
const credentials = reactive({ login: '', password: '', confirm: '' })
const busy = ref(false)
const error = ref('')

async function login() {
  if (busy.value) return
  error.value = ''
  if (props.needsAdmin && credentials.password !== credentials.confirm) {
    error.value = 'Пароли не совпадают.'
    return
  }
  busy.value = true
  try {
    const state = props.needsAdmin
      ? await api.Setup(credentials.login, credentials.password)
      : await api.Login(credentials.login, credentials.password)
    credentials.password = ''
    credentials.confirm = ''
    emit('authenticated', state)
  } catch (reason) {
    error.value = reason instanceof Error ? reason.message : String(reason)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <main class="auth-layout">
    <form class="panel auth-card" @submit.prevent="login">
      <h1>ReactorOpt</h1>
      <h2>{{ needsAdmin ? 'Создание администратора' : 'Вход' }}</h2>
      <p v-if="needsAdmin" class="hint">
        Создайте учётную запись для управления задачами и пользователями.
      </p>
      <label>
        Логин
        <input v-model="credentials.login" required maxlength="64" autocomplete="username" />
      </label>
      <label>
        Пароль
        <input
          v-model="credentials.password"
          type="password"
          required
          minlength="4"
          maxlength="1024"
          :autocomplete="needsAdmin ? 'new-password' : 'current-password'"
        />
      </label>
      <label v-if="needsAdmin">
        Повтор пароля
        <input
          v-model="credentials.confirm"
          type="password"
          required
          minlength="4"
          autocomplete="new-password"
        />
      </label>
      <p v-if="error" class="notice error" role="alert">{{ error }}</p>
      <button class="button primary wide" :disabled="busy">
        {{ needsAdmin ? 'Создать администратора' : 'Войти' }}
      </button>
    </form>
  </main>
</template>
