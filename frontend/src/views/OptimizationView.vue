<script setup lang="ts">
import { computed, defineAsyncComponent, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Events } from '@wailsio/runtime'
import {
  api,
  type Definition,
  type JobState,
  type Parameters,
  type PlotData,
  type Task,
} from '../api'
import MethodParameters from '../components/MethodParameters.vue'
import OptimizationResult from '../components/OptimizationResult.vue'

const OptimizationChart = defineAsyncComponent(() => import('../components/OptimizationChart.vue'))
const props = defineProps<{
  tasks: Task[]
  methods: Definition[]
  admin: boolean
  dark: boolean
  resolution: number
}>()
const taskId = defineModel<string>('taskId', { required: true })
const methodId = defineModel<string>('methodId', { required: true })
const emit = defineEmits<{ running: [value: boolean]; 'methods-changed': [] }>()
const task = computed(() => props.tasks.find((item) => item.id === taskId.value))
const method = computed(() => props.methods.find((item) => item.id === methodId.value))
const parameters = ref<Parameters>()
const remembered = new Map<string, Parameters>()
const direction = ref('max')
const emptyJob = (): JobState => ({ id: 0, status: 'idle', result: null, error: '' })
const job = ref<JobState>(emptyJob())
const running = computed(() => job.value.status === 'running')
const starting = ref(false)
const locked = computed(() => starting.value || running.value)
const result = computed(() => job.value.result)
const resultMethod = computed(
  () => props.methods.find((item) => item.id === result.value?.method)?.name || '',
)
const busy = ref(false)
const notice = ref('')
const isError = ref(false)
const plot = ref<PlotData | null>(null)
const plotBusy = ref(false)
const plotError = ref('')
const mode = ref('2d')
const feasible = ref(false)
const showDomain = ref(true)
const showHistory = ref(true)
let sampleVersion = 0
let pollTimer: ReturnType<typeof setTimeout> | undefined
let checkingJob = false
let disposed = false
let offEvent: (() => void) | undefined

const clone = (value: Parameters): Parameters => JSON.parse(JSON.stringify(value))
const format = (value: number | null | undefined) =>
  value == null ? '—' : new Intl.NumberFormat('ru-RU', { maximumFractionDigits: 6 }).format(value)

function setParameters() {
  const defaults = remembered.get(methodId.value) || method.value?.defaults
  parameters.value = defaults ? clone(defaults) : undefined
}

async function perform(action: () => Promise<void>, success = '') {
  if (busy.value) return
  busy.value = true
  notice.value = ''
  isError.value = false
  try {
    await action()
    if (success && !disposed) notice.value = success
  } catch (reason) {
    if (!disposed) {
      notice.value = reason instanceof Error ? reason.message : String(reason)
      isError.value = true
    }
  } finally {
    busy.value = false
  }
}

async function loadSample() {
  const version = ++sampleVersion
  plot.value = null
  plotError.value = ''
  if (!taskId.value) return
  plotBusy.value = true
  try {
    const data = await api.Sample(taskId.value, props.resolution)
    if (!disposed && version === sampleVersion) plot.value = data
  } catch (reason) {
    if (!disposed && version === sampleVersion) plotError.value = String(reason)
  } finally {
    if (!disposed && version === sampleVersion) plotBusy.value = false
  }
}

async function checkJob() {
  if (!running.value || checkingJob || disposed) return
  checkingJob = true
  clearTimeout(pollTimer)
  try {
    const state = await api.Job()
    if (!disposed) job.value = state
  } catch (reason) {
    if (!disposed) {
      notice.value = String(reason)
      isError.value = true
      await api.Cancel().catch(() => undefined)
      job.value = emptyJob()
    }
  } finally {
    checkingJob = false
    if (!disposed && running.value) pollTimer = setTimeout(checkJob, 300)
  }
}

async function start() {
  if (!parameters.value || locked.value || busy.value) return
  const settings = clone(parameters.value)
  const selectedTask = taskId.value
  const selectedMethod = methodId.value
  const selectedDirection = direction.value
  remembered.set(selectedMethod, settings)
  starting.value = true
  try {
    await perform(async () => {
      const state = await api.Start(selectedTask, selectedDirection, selectedMethod, settings)
      if (!disposed) {
        job.value = state
        void checkJob()
      }
    })
  } finally {
    starting.value = false
  }
}

async function cancel() {
  await perform(async () => {
    await api.Cancel()
  }, 'Запрошена остановка расчёта.')
}

async function saveDefaults() {
  if (!parameters.value) return
  await perform(async () => {
    await api.SaveMethodSettings(methodId.value, parameters.value!)
    emit('methods-changed')
  }, 'Настройки метода сохранены.')
}

watch(
  methodId,
  (_next, previous) => {
    if (parameters.value && previous) remembered.set(previous, clone(parameters.value))
    setParameters()
    if (!running.value) job.value = emptyJob()
  },
  { immediate: true },
)
watch(
  [taskId, () => JSON.stringify(task.value)],
  ([id], [previousId]) => {
    if (id !== previousId) direction.value = task.value?.direction || 'max'
    if (!running.value) job.value = emptyJob()
    void loadSample()
  },
  { immediate: true },
)
watch(direction, () => {
  if (!running.value) job.value = emptyJob()
})
watch(
  () => props.resolution,
  () => void loadSample(),
)
watch(locked, (value) => emit('running', value), { immediate: true, flush: 'sync' })
onMounted(() => {
  offEvent = Events.On('job:updated', () => void checkJob())
})
onBeforeUnmount(() => {
  disposed = true
  sampleVersion++
  clearTimeout(pollTimer)
  offEvent?.()
  emit('running', false)
})
</script>

<template>
  <section>
    <p v-if="notice" class="notice" :class="{ error: isError }" role="status">{{ notice }}</p>
    <div class="optimization-layout">
      <form v-if="parameters" class="panel controls" @submit.prevent="start">
        <h2>Параметры расчёта</h2>
        <label>
          Задача
          <select v-model="taskId" :disabled="locked">
            <option v-for="option in tasks" :key="option.id" :value="option.id">
              {{ option.name }}
            </option>
          </select>
        </label>
        <p class="hint">{{ task?.description }}</p>
        <details v-if="task">
          <summary>Формула и ограничения</summary>
          <p class="formula">{{ task.objective }}</p>
          <p v-for="variable in task.variables" :key="variable.name">
            {{ format(variable.min) }} ≤ {{ variable.display_name }} ≤ {{ format(variable.max) }}
            {{ variable.unit }}
          </p>
          <p v-for="(constraint, index) in task.constraints" :key="index">
            {{
              Object.entries(constraint.coefficients || {})
                .map(([name, value]) => format(value) + '·' + name)
                .join(' + ')
            }}
            {{ constraint.operator }} {{ format(constraint.value) }}
          </p>
          <dl>
            <template v-for="(value, name) in task.coefficients" :key="name">
              <dt>{{ name }}</dt>
              <dd>{{ format(value) }}</dd>
            </template>
          </dl>
        </details>
        <label>
          Направление
          <select v-model="direction" :disabled="locked">
            <option value="max">Максимум</option>
            <option value="min">Минимум</option>
          </select>
        </label>
        <label>
          Метод
          <select v-model="methodId" :disabled="locked">
            <option v-for="option in methods" :key="option.id" :value="option.id">
              {{ option.name }}
            </option>
          </select>
        </label>
        <MethodParameters
          :key="methodId"
          v-model="parameters"
          :method-id="methodId"
          :disabled="locked"
          :task="task"
        />
        <button v-if="!running" class="button primary wide" :disabled="busy || !task">
          Начать расчёт
        </button>
        <button v-else type="button" class="button danger wide" :disabled="busy" @click="cancel">
          Остановить расчёт
        </button>
        <button
          v-if="admin"
          type="button"
          class="text-button"
          :disabled="busy || running"
          @click="saveDefaults"
        >
          Сохранить настройки по умолчанию
        </button>
      </form>
      <div class="results">
        <section class="panel chart-panel">
          <div class="chart-toolbar">
            <h2>График функции</h2>
            <div class="segmented">
              <button :class="{ active: mode === '2d' }" @click="mode = '2d'">2D</button>
              <button :class="{ active: mode === '3d' }" @click="mode = '3d'">3D</button>
            </div>
          </div>
          <div class="toolbar">
            <label class="check">
              <input v-model="feasible" type="checkbox" />
              Только допустимая область
            </label>
            <label v-if="mode === '2d'" class="check">
              <input v-model="showDomain" type="checkbox" />
              Граница области
            </label>
            <label class="check">
              <input v-model="showHistory" type="checkbox" />
              История
            </label>
          </div>
          <p v-if="plotBusy" class="plot-area">Подготовка данных графика…</p>
          <div v-else-if="plotError" class="notice error">
            <p>{{ plotError }}</p>
            <button class="button" @click="loadSample">Повторить</button>
          </div>
          <OptimizationChart
            v-else-if="plot && task"
            :data="plot"
            :task="task"
            :result="result"
            :mode="mode"
            :dark="dark"
            :feasible="feasible"
            :domain="showDomain"
            :history="showHistory"
          />
        </section>
        <OptimizationResult
          :result="result"
          :task="task"
          :method-name="resultMethod"
          :running="running"
          :error="job.error"
        />
      </div>
    </div>
  </section>
</template>
