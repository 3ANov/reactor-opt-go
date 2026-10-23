<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { api, type Definition, type State, type Task, type User } from './api'
import LoginView from './views/LoginView.vue'
import OptimizationView from './views/OptimizationView.vue'
import TaskLibraryView from './views/TaskLibraryView.vue'
import MethodLibraryView from './views/MethodLibraryView.vue'
import SettingsView from './views/SettingsView.vue'
import UsersView from './views/UsersView.vue'

type Tab = 'optimization' | 'tasks' | 'methods' | 'settings' | 'users'
const tabs: { id: Tab; name: string }[] = [
  { id: 'optimization', name: 'Оптимизация' },
  { id: 'tasks', name: 'Библиотека задач' },
  { id: 'methods', name: 'Библиотека методов' },
  { id: 'settings', name: 'Настройки' },
  { id: 'users', name: 'Пользователи' },
]
const tab = ref<Tab>('optimization')
const user = ref<User | null>(null)
const admin = computed(() => user.value?.role === 'admin')
const needsAdmin = ref(false)
const loading = ref(true)
const tasks = ref<Task[]>([])
const methods = ref<Definition[]>([])
const taskId = ref('')
const methodId = ref('nelder_mead')
const running = ref(false)
const notice = ref('')
const dark = ref(localStorage.getItem('reactoropt.theme') === 'dark')
const storedResolution = Number(localStorage.getItem('reactoropt.resolution'))
const resolution = ref([41, 71, 111].includes(storedResolution) ? storedResolution : 71)
let sessionVersion = 0
let sessionTimer: ReturnType<typeof setTimeout> | undefined

function errorText(reason: unknown) {
  return reason instanceof Error ? reason.message : String(reason)
}

function clearSession() {
  sessionVersion++
  clearTimeout(sessionTimer)
  user.value = null
  tasks.value = []
  methods.value = []
  running.value = false
  tab.value = 'optimization'
}

function checkSession() {
  clearTimeout(sessionTimer)
  if (!user.value) return
  const version = sessionVersion
  sessionTimer = setTimeout(async () => {
    try {
      const state = await api.Bootstrap()
      if (version !== sessionVersion) return
      if (!state.user) {
        sessionEnded()
        return
      }
      user.value = state.user
    } catch (reason) {
      if (version === sessionVersion) notice.value = errorText(reason)
    }
    if (version === sessionVersion) checkSession()
  }, 2000)
}

function authenticated(state: State) {
  sessionVersion++
  user.value = state.user
  needsAdmin.value = state.needs_admin
  tasks.value = state.tasks || []
  methods.value = state.methods || []
  if (!tasks.value.some((task) => task.id === taskId.value)) taskId.value = tasks.value[0]?.id || ''
  notice.value = ''
  checkSession()
}

function sessionEnded() {
  clearSession()
  notice.value = 'Сеанс завершён. Войдите снова.'
}

async function logout() {
  try {
    await api.Logout()
    clearSession()
    notice.value = ''
  } catch (reason) {
    notice.value = errorText(reason)
  }
}

async function refreshTasks() {
  if (!user.value) return
  const version = sessionVersion
  try {
    const updated = (await api.Tasks()) || []
    if (version !== sessionVersion) return
    tasks.value = updated
    if (!tasks.value.some((task) => task.id === taskId.value))
      taskId.value = tasks.value[0]?.id || ''
  } catch (reason) {
    if (version === sessionVersion) notice.value = errorText(reason)
  }
}

async function refreshMethods() {
  if (!user.value) return
  const version = sessionVersion
  try {
    const updated = (await api.Methods()) || []
    if (version === sessionVersion) methods.value = updated
  } catch (reason) {
    if (version === sessionVersion) notice.value = errorText(reason)
  }
}

function chooseTask(id: string) {
  if (running.value) return
  taskId.value = id
  tab.value = 'optimization'
}
function chooseMethod(id: string) {
  if (running.value) return
  methodId.value = id
  tab.value = 'optimization'
}

watch(
  dark,
  (value) => {
    document.documentElement.dataset.theme = value ? 'dark' : 'light'
    localStorage.setItem('reactoropt.theme', value ? 'dark' : 'light')
  },
  { immediate: true },
)
watch(resolution, (value) => localStorage.setItem('reactoropt.resolution', String(value)))
onMounted(async () => {
  try {
    authenticated(await api.Bootstrap())
  } catch (reason) {
    notice.value = errorText(reason)
  } finally {
    loading.value = false
  }
})
onBeforeUnmount(() => {
  sessionVersion++
  clearTimeout(sessionTimer)
})
</script>

<template>
  <p v-if="loading" class="boot">Загрузка ReactorOpt…</p>
  <template v-else-if="!user">
    <p v-if="notice" class="notice error" role="status">{{ notice }}</p>
    <LoginView :needs-admin="needsAdmin" @authenticated="authenticated" />
  </template>
  <template v-else>
    <header class="app-header">
      <h1>ReactorOpt</h1>
      <span v-if="running" class="muted">Выполняется расчёт</span>
      <div class="account">
        <span>{{ user.login }} — {{ admin ? 'администратор' : 'пользователь' }}</span>
        <button class="button logout-button" @click="logout">Выйти</button>
      </div>
    </header>
    <nav class="app-nav" aria-label="Разделы приложения">
      <template v-for="item in tabs" :key="item.id">
        <button
          v-if="item.id !== 'users' || admin"
          :class="{ active: tab === item.id }"
          @click="tab = item.id"
        >
          {{ item.name }}
        </button>
      </template>
    </nav>
    <main class="app-main">
      <p v-if="notice" class="notice error" role="status">{{ notice }}</p>
      <OptimizationView
        v-model:task-id="taskId"
        v-model:method-id="methodId"
        v-show="tab === 'optimization'"
        :tasks="tasks"
        :methods="methods"
        :admin="admin"
        :dark="dark"
        :resolution="resolution"
        @running="running = $event"
        @methods-changed="refreshMethods"
      />
      <TaskLibraryView
        v-show="tab === 'tasks'"
        :tasks="tasks"
        :admin="admin"
        :running="running"
        @changed="refreshTasks"
        @choose="chooseTask"
      />
      <MethodLibraryView
        v-if="tab === 'methods'"
        :methods="methods"
        :running="running"
        @choose="chooseMethod"
      />
      <SettingsView
        v-if="tab === 'settings'"
        v-model:dark="dark"
        v-model:resolution="resolution"
        @session-ended="sessionEnded"
      />
      <UsersView v-if="tab === 'users' && admin" :user="user" @session-ended="sessionEnded" />
    </main>
  </template>
</template>
