<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { api, type Task } from '../api'

const props = defineProps<{ tasks: Task[]; admin: boolean; running: boolean }>()
const emit = defineEmits<{ changed: []; choose: [id: string] }>()
const busy = ref(false)
const notice = ref('')
const isError = ref(false)
const editorOpen = ref(false)
const originalID = ref('')
const builtin = ref(false)
const deleteTarget = ref<Task>()
const importOpen = ref(false)
const importText = ref('')
const exported = ref('')
const exportName = ref('')
const readOnly = computed(() => builtin.value || !props.admin)
const draft = reactive({
  id: '',
  name: '',
  description: '',
  objective: '',
  direction: 'min',
  objective_label: 'C',
  objective_unit: '',
  variables: '',
  coefficients: '',
  constraints: '',
  derived_metrics: '',
})

async function perform(action: () => Promise<void>, success = '') {
  if (busy.value) return
  busy.value = true
  notice.value = ''
  isError.value = false
  try {
    await action()
    if (success) notice.value = success
  } catch (error) {
    notice.value = error instanceof Error ? error.message : String(error)
    isError.value = true
  } finally {
    busy.value = false
  }
}

function openEditor(existing?: Task) {
  const task: Task = existing || {
    id: 'task-' + crypto.randomUUID(),
    name: 'Новая задача',
    description: '',
    objective: '(x-2)**2+(y-3)**2',
    direction: 'min',
    variables: [
      { name: 'x', display_name: 'A₁', unit: 'м³/ч', min: 0, max: 6 },
      { name: 'y', display_name: 'A₂', unit: 'м³/ч', min: 0, max: 6 },
    ],
    coefficients: {},
    constraints: [],
    derived_metrics: [],
    builtin: false,
    objective_label: 'C',
    objective_unit: '',
  }
  Object.assign(draft, {
    ...task,
    variables: JSON.stringify(task.variables || [], null, 2),
    coefficients: JSON.stringify(task.coefficients || {}, null, 2),
    constraints: JSON.stringify(task.constraints || [], null, 2),
    derived_metrics: JSON.stringify(task.derived_metrics || [], null, 2),
  })
  originalID.value = existing?.id || ''
  builtin.value = task.builtin
  editorOpen.value = true
  importOpen.value = false
  exported.value = ''
  deleteTarget.value = undefined
  notice.value = ''
}

function parseField<T>(value: string, name: string): T {
  try {
    return JSON.parse(value) as T
  } catch {
    throw new Error(`Поле «${name}» содержит некорректный JSON.`)
  }
}

async function saveTask() {
  if (readOnly.value || props.running) return
  await perform(async () => {
    const task: Task = {
      ...draft,
      builtin: false,
      variables: parseField<Task['variables']>(draft.variables, 'Переменные'),
      coefficients: parseField<Task['coefficients']>(draft.coefficients, 'Параметры'),
      constraints: parseField<Task['constraints']>(draft.constraints, 'Линейные ограничения'),
      derived_metrics: parseField<Task['derived_metrics']>(
        draft.derived_metrics,
        'Расчётные показатели',
      ),
    }
    await api.SaveTask(task)
    editorOpen.value = false
    emit('changed')
  }, 'Задача сохранена.')
}

async function copyTask(task: Task) {
  if (!props.admin || props.running) return
  await perform(async () => {
    const copy = await api.CopyTask(task.id)
    emit('changed')
    openEditor(copy)
  }, 'Копия создана. Можно изменить её параметры.')
}

async function deleteTask() {
  const task = deleteTarget.value
  if (!task || task.builtin || !props.admin || props.running) return
  await perform(async () => {
    await api.DeleteTask(task.id)
    deleteTarget.value = undefined
    if (originalID.value === task.id) editorOpen.value = false
    emit('changed')
  }, 'Задача удалена.')
}

function openImport() {
  importText.value = ''
  importOpen.value = true
  editorOpen.value = false
  exported.value = ''
  deleteTarget.value = undefined
  notice.value = ''
}

async function readImport(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  await perform(async () => {
    if (file.size > 65536) throw new Error('Файл должен занимать до 64 КБ.')
    importText.value = await file.text()
  })
  input.value = ''
}

async function importTasks() {
  if (!props.admin || props.running) return
  await perform(async () => {
    if (new TextEncoder().encode(importText.value).length > 65536)
      throw new Error('Содержимое JSON должно занимать до 64 КБ.')
    const added = await api.ImportTasks(importText.value)
    emit('changed')
    importOpen.value = false
    notice.value = `Импортировано задач: ${added?.length || 0}.`
  })
}

async function exportTask(task: Task) {
  await perform(async () => {
    exported.value = await api.ExportTask(task.id)
    exportName.value = task.id
    editorOpen.value = false
    importOpen.value = false
    deleteTarget.value = undefined
  })
}

function downloadExport() {
  const url = URL.createObjectURL(new Blob([exported.value], { type: 'application/json' }))
  const link = document.createElement('a')
  link.href = url
  link.download = exportName.value + '.json'
  link.click()
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}
</script>

<template>
  <section>
    <div v-if="admin" class="toolbar">
      <button class="button" :disabled="running || busy" @click="openImport">Импорт JSON</button>
      <button class="button primary" :disabled="running || busy" @click="openEditor()">
        Новая задача
      </button>
    </div>

    <p v-if="notice" class="notice" :class="{ error: isError }" role="status">{{ notice }}</p>

    <section v-if="deleteTarget" class="panel" aria-labelledby="delete-title">
      <h2 id="delete-title">Удаление задачи</h2>
      <p>Удалить задачу «{{ deleteTarget.name }}» из библиотеки?</p>
      <div class="task-actions">
        <button class="button danger" :disabled="running || busy" @click="deleteTask">
          Удалить
        </button>
        <button class="button" :disabled="busy" @click="deleteTarget = undefined">Отмена</button>
      </div>
    </section>

    <section v-if="editorOpen" class="panel" aria-labelledby="editor-title">
      <h2 id="editor-title">{{ readOnly ? 'Просмотр задачи' : 'Редактор задачи' }}</h2>
      <p v-if="builtin">
        Встроенная задача защищена от изменений. Администратор может создать её копию.
      </p>
      <p v-else-if="!admin">Изменение задач доступно администратору.</p>
      <form @submit.prevent="saveTask">
        <fieldset :disabled="readOnly || running || busy">
          <div class="form-grid">
            <label>
              Постоянный идентификатор
              <input v-model="draft.id" required :readonly="Boolean(originalID)" />
            </label>
            <label>
              Название
              <input v-model="draft.name" required maxlength="200" />
            </label>
          </div>
          <label>
            Описание
            <textarea v-model="draft.description" rows="2" maxlength="4000"></textarea>
          </label>
          <label>
            Целевая функция
            <textarea
              v-model="draft.objective"
              class="code"
              rows="2"
              required
              maxlength="2048"
            ></textarea>
          </label>
          <p class="hint">
            Степень — **. Функции: sin, cos, tan, sqrt, exp, log, log10, abs, min, max. Константы:
            pi, e.
          </p>
          <div class="form-grid">
            <label>
              Направление поиска
              <select v-model="draft.direction">
                <option value="min">Минимум</option>
                <option value="max">Максимум</option>
              </select>
            </label>
            <label>
              Обозначение функции
              <input v-model="draft.objective_label" required maxlength="32" />
            </label>
            <label>
              Единица функции
              <input v-model="draft.objective_unit" maxlength="32" />
            </label>
          </div>
          <div class="form-grid">
            <label>
              Переменные · JSON
              <textarea v-model="draft.variables" class="code" rows="10" required></textarea>
            </label>
            <label>
              Параметры · JSON
              <textarea v-model="draft.coefficients" class="code" rows="10" required></textarea>
            </label>
            <label>
              Линейные ограничения · JSON
              <textarea v-model="draft.constraints" class="code" rows="7" required></textarea>
            </label>
            <label>
              Расчётные показатели · JSON
              <textarea v-model="draft.derived_metrics" class="code" rows="7" required></textarea>
            </label>
          </div>
        </fieldset>
        <div class="task-actions">
          <button v-if="!readOnly" class="button primary" :disabled="running || busy">
            Сохранить задачу
          </button>
          <button type="button" class="button" :disabled="busy" @click="editorOpen = false">
            Закрыть
          </button>
        </div>
      </form>
    </section>

    <section v-if="importOpen && admin" class="panel" aria-labelledby="import-title">
      <h2 id="import-title">Импорт задач</h2>
      <p>
        Выберите файл или вставьте объект задачи либо список задач в формате JSON, до 64 КБ. Для
        копии встроенной задачи измените её идентификатор.
      </p>
      <label>
        JSON-файл
        <input
          type="file"
          accept=".json,application/json"
          :disabled="running || busy"
          @change="readImport"
        />
      </label>
      <label>
        Содержимое JSON
        <textarea
          v-model="importText"
          class="code"
          rows="12"
          :disabled="running || busy"
        ></textarea>
      </label>
      <div class="task-actions">
        <button
          class="button primary"
          :disabled="running || busy || !importText.trim()"
          @click="importTasks"
        >
          Импортировать
        </button>
        <button class="button" :disabled="busy" @click="importOpen = false">Закрыть</button>
      </div>
    </section>

    <section v-if="exported" class="panel" aria-labelledby="export-title">
      <h2 id="export-title">Экспорт задачи</h2>
      <label>
        Содержимое JSON
        <textarea :value="exported" class="code" rows="12" readonly></textarea>
      </label>
      <div class="task-actions">
        <button class="button primary" @click="downloadExport">Скачать JSON</button>
        <button class="button" @click="exported = ''">Закрыть</button>
      </div>
    </section>

    <div class="task-list">
      <article v-for="task in tasks" :key="task.id" class="panel task-item">
        <h2>{{ task.name }}</h2>
        <p v-if="task.description">{{ task.description }}</p>
        <p class="hint">
          {{ task.builtin ? 'Встроенная задача' : 'Пользовательская задача' }} ·
          {{ task.direction === 'max' ? 'Максимум' : 'Минимум' }}
        </p>
        <code class="formula">{{ task.objective_label }} = {{ task.objective }}</code>
        <div class="task-actions">
          <button
            class="button primary"
            :disabled="running || busy"
            @click="emit('choose', task.id)"
          >
            Рассчитать
          </button>
          <button class="button" :disabled="busy" @click="openEditor(task)">
            {{ task.builtin || !admin ? 'Просмотр' : 'Изменить' }}
          </button>
          <button v-if="admin" class="button" :disabled="running || busy" @click="copyTask(task)">
            Копировать
          </button>
          <button class="button" :disabled="busy" @click="exportTask(task)">Экспорт JSON</button>
          <button
            v-if="admin && !task.builtin"
            class="button danger"
            :disabled="running || busy"
            @click="deleteTarget = task"
          >
            Удалить
          </button>
        </div>
      </article>
      <p v-if="!tasks.length" class="panel">В библиотеке пока нет задач.</p>
    </div>
  </section>
</template>
