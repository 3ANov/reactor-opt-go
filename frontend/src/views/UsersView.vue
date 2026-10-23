<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { api, type User } from '../api'

const props = defineProps<{ user: User }>()
const emit = defineEmits<{ 'session-ended': [] }>()
const users = ref<User[]>([])
const busy = ref(false)
const notice = ref('')
const isError = ref(false)
const newUser = reactive({ login: '', password: '', role: 'user' })
const resetTarget = ref<User | null>(null)
const resetPassword = ref('')
const resetConfirm = ref('')

async function perform(action: () => Promise<void>, success = '') {
  if (busy.value) return
  busy.value = true
  notice.value = ''
  isError.value = false
  try {
    await action()
    notice.value = success
  } catch (error) {
    notice.value = error instanceof Error ? error.message : String(error)
    isError.value = true
  } finally {
    busy.value = false
  }
}

async function loadUsers() {
  users.value = (await api.Users()) || []
}

async function createUser() {
  await perform(async () => {
    await api.CreateUser(newUser.login, newUser.password, newUser.role)
    Object.assign(newUser, { login: '', password: '', role: 'user' })
    await loadUsers()
  }, 'Пользователь создан.')
}

async function updateUser(target: User, role: string, active: boolean) {
  await perform(async () => {
    await api.UpdateUser(target.id, role, active)
    if (target.id === props.user.id) {
      emit('session-ended')
      return
    }
    await loadUsers()
  }, 'Права пользователя изменены. Его прежние сеансы завершены.')
}

function changeRole(target: User, event: Event) {
  const select = event.target as HTMLSelectElement
  const role = select.value
  select.value = target.role
  if (role !== target.role) void updateUser(target, role, target.active)
}

function openReset(target: User) {
  if (target.id === props.user.id) return
  resetTarget.value = target
  resetPassword.value = ''
  resetConfirm.value = ''
  notice.value = ''
}

function closeReset() {
  resetTarget.value = null
  resetPassword.value = ''
  resetConfirm.value = ''
}

async function submitReset() {
  const target = resetTarget.value
  if (!target || target.id === props.user.id || busy.value) return
  if (resetPassword.value !== resetConfirm.value) {
    notice.value = 'Пароли не совпадают.'
    isError.value = true
    return
  }
  await perform(async () => {
    await api.ResetPassword(target.id, resetPassword.value)
    closeReset()
  }, 'Пароль сброшен. Прежние сеансы пользователя завершены.')
}

onMounted(() => void perform(loadUsers))
</script>

<template>
  <p v-if="notice" class="notice" :class="{ error: isError }" :role="isError ? 'alert' : 'status'">
    {{ notice }}
  </p>
  <section class="panel">
    <div class="toolbar">
      <h2>Учётные записи</h2>
      <button class="button" :disabled="busy" @click="perform(loadUsers)">Обновить</button>
    </div>
    <p>
      Смена роли, блокировка и сброс пароля завершают сеансы пользователя. Последнего активного
      администратора отключить нельзя.
    </p>
    <div class="table-scroll">
      <table class="table">
        <thead>
          <tr>
            <th>Логин</th>
            <th>Роль</th>
            <th>Доступ</th>
            <th>Действия</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="account in users" :key="account.id">
            <td>{{ account.login }}{{ account.id === user.id ? ' (вы)' : '' }}</td>
            <td>
              <select
                :value="account.role"
                :disabled="busy"
                :aria-label="'Роль пользователя ' + account.login"
                @change="changeRole(account, $event)"
              >
                <option value="user">Пользователь</option>
                <option value="admin">Администратор</option>
              </select>
            </td>
            <td>{{ account.active ? 'Активен' : 'Заблокирован' }}</td>
            <td>
              <div class="toolbar">
                <button
                  class="button"
                  :disabled="busy"
                  @click="updateUser(account, account.role, !account.active)"
                >
                  {{ account.active ? 'Заблокировать' : 'Разблокировать' }}
                </button>
                <button
                  class="button"
                  :disabled="busy || account.id === user.id"
                  @click="openReset(account)"
                >
                  Сбросить пароль
                </button>
              </div>
            </td>
          </tr>
          <tr v-if="!users.length">
            <td colspan="4">{{ busy ? 'Загрузка…' : 'Список пользователей пуст.' }}</td>
          </tr>
        </tbody>
      </table>
    </div>
    <p>Свой пароль можно изменить в настройках.</p>
  </section>

  <form v-if="resetTarget" class="panel" @submit.prevent="submitReset">
    <h2>Сброс пароля: {{ resetTarget.login }}</h2>
    <div class="form-grid">
      <label>
        Новый пароль
        <input
          v-model="resetPassword"
          type="password"
          autocomplete="new-password"
          minlength="4"
          maxlength="1024"
          required
          :disabled="busy"
        />
      </label>
      <label>
        Повторите пароль
        <input
          v-model="resetConfirm"
          type="password"
          autocomplete="new-password"
          minlength="4"
          maxlength="1024"
          required
          :disabled="busy"
        />
      </label>
    </div>
    <div class="toolbar">
      <button class="button" :disabled="busy">Сбросить пароль</button>
      <button type="button" class="button" :disabled="busy" @click="closeReset">Отмена</button>
    </div>
  </form>

  <form class="panel" @submit.prevent="createUser">
    <h2>Новый пользователь</h2>
    <div class="form-grid">
      <label>
        Логин
        <input
          v-model="newUser.login"
          autocomplete="off"
          maxlength="64"
          required
          :disabled="busy"
        />
      </label>
      <label>
        Пароль
        <input
          v-model="newUser.password"
          type="password"
          autocomplete="new-password"
          minlength="4"
          maxlength="1024"
          required
          :disabled="busy"
        />
      </label>
      <label>
        Роль
        <select v-model="newUser.role" :disabled="busy">
          <option value="user">Пользователь</option>
          <option value="admin">Администратор</option>
        </select>
      </label>
    </div>
    <button class="button" :disabled="busy">Создать пользователя</button>
  </form>
</template>
