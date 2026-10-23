<script setup lang="ts">
import type { Result, Task } from '../api'

defineProps<{
  result: Result | null
  task?: Task
  methodName: string
  running: boolean
  error: string
}>()

const statusNames: Record<string, string> = {
  converged: 'Поиск завершён по условию сходимости',
  budget_exhausted: 'Бюджет исчерпан',
  iteration_limit: 'Достигнут лимит итераций',
  cancelled: 'Расчёт отменён',
  stalled: 'Поиск остановился без сходимости',
  failed: 'Ошибка расчёта',
  no_feasible_region: 'Допустимая область пуста',
  no_feasible_point: 'Нет допустимого результата',
}
const sourceNames: Record<string, string> = {
  vertex: 'Проверенная вершина',
  nelder_mead: 'Симплексный поиск',
  bfgs_interior: 'Внутренний поиск BFGS',
  bfgs_boundary: 'Поиск BFGS на ребре',
  cma_es_interior: 'Внутренний поиск CMA-ES',
  cma_es_boundary: 'Поиск CMA-ES на ребре',
}
function format(value: number | null) {
  return value == null
    ? '—'
    : new Intl.NumberFormat('ru-RU', { maximumFractionDigits: 6 }).format(value)
}

function formatChange(value: number) {
  return new Intl.NumberFormat('ru-RU', {
    maximumSignificantDigits: 6,
    notation: value !== 0 && Math.abs(value) < 0.0001 ? 'scientific' : 'standard',
  }).format(value)
}
</script>

<template>
  <section class="panel result-panel" aria-live="polite">
    <h2>Результат</h2>
    <p v-if="error" class="notice error">{{ error }}</p>
    <template v-if="result">
      <p class="result-status">
        <strong>{{ statusNames[result.status] || 'Расчёт завершён' }}</strong>
      </p>
      <p>{{ result.reason }}</p>
      <table v-if="result.point && result.value !== null" class="table result-values">
        <thead>
          <tr>
            <th>Показатель</th>
            <th>Значение</th>
            <th>Единицы</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(coordinate, axis) in result.point" :key="axis">
            <td>{{ task?.variables?.[axis]?.display_name }}</td>
            <td>{{ format(coordinate) }}</td>
            <td>{{ task?.variables?.[axis]?.unit }}</td>
          </tr>
          <tr>
            <td>{{ task?.objective_label }}</td>
            <td>{{ format(result.value) }}</td>
            <td>{{ task?.objective_unit }}</td>
          </tr>
          <tr v-for="metric in result.metrics" :key="metric.id">
            <td :title="metric.name">{{ metric.symbol || metric.name }}</td>
            <td>{{ format(metric.value) }}</td>
            <td>{{ metric.unit }}</td>
          </tr>
        </tbody>
      </table>
      <p>
        {{ methodName }}. Итераций: {{ result.iterations }}, вычислений функции:
        {{ result.evaluations }}.
      </p>
      <p class="hint">
        Показан лучший проверенный допустимый результат. Глобальная оптимальность не гарантируется.
      </p>
      <details>
        <summary>Подробности расчёта</summary>
        <dl>
          <dt>Источник результата</dt>
          <dd>{{ sourceNames[result.source] || '—' }}</dd>
          <dt>Точек истории</dt>
          <dd>{{ result.history?.length || 0 }}</dd>
          <template v-if="result.function_change != null">
            <dt>Изменение функции ΔF</dt>
            <dd>{{ formatChange(result.function_change) }}</dd>
          </template>
          <template v-if="result.coordinate_change != null">
            <dt>Перемещение Δx</dt>
            <dd>{{ formatChange(result.coordinate_change) }}</dd>
          </template>
        </dl>
        <p v-if="result.history_truncated" class="hint">
          История ограничена 2000 записями; счётчики учитывают все вычисления.
        </p>
      </details>
    </template>
    <p v-else>{{ running ? 'Выполняется расчёт…' : 'Выберите задачу и запустите расчёт.' }}</p>
  </section>
</template>
