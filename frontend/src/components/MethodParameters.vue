<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { Parameters, Task } from '../api'

const parameters = defineModel<Parameters>({ required: true })
const props = defineProps<{ methodId: string; disabled: boolean; task?: Task }>()
const useStart = ref(Boolean(parameters.value.start))
const startX = ref(parameters.value.start?.[0] ?? 1)
const startY = ref(parameters.value.start?.[1] ?? 1)
const stoppingType = computed({
  get: () => {
    const criterion = parameters.value.stopping_criterion
    if (criterion === 'accuracy') return 'coordinates'
    if (criterion === 'iterations') return 'evaluations'
    return criterion
  },
  set: (value: string) => {
    parameters.value.stopping_criterion = value
  },
})

parameters.value.stopping_criterion = stoppingType.value

watch([useStart, startX, startY], () => {
  parameters.value.start =
    useStart.value && props.methodId !== 'nelder_mead' ? [startX.value, startY.value] : null
})

function randomSeed() {
  parameters.value.seed = crypto.getRandomValues(new Uint32Array(1))[0]
}
</script>

<template>
  <fieldset :disabled="disabled">
    <label>
      Тип остановки
      <select v-model="stoppingType">
        <option value="function">Точность по функции</option>
        <option value="coordinates">Точность по координатам</option>
        <option value="evaluations">Количество вычислений</option>
      </select>
    </label>
    <template v-if="stoppingType === 'function'">
      <label>
        Точность по функции ε
        <input
          v-model.number="parameters.function_precision"
          type="number"
          step="any"
          min="0"
          required
        />
      </label>
      <p class="hint">|F(xₖ₊₁) − F(xₖ)| ≤ ε</p>
    </template>
    <template v-else-if="stoppingType === 'coordinates'">
      <label>
        Точность по координатам δ
        <input v-model.number="parameters.precision" type="number" step="any" min="0" required />
      </label>
      <p class="hint">‖xₖ₊₁ − xₖ‖₂ ≤ δ. Сравниваются точки в исходных координатах.</p>
    </template>
    <label v-else>
      Лимит вычислений N
      <input
        v-model.number="parameters.max_evaluations"
        type="number"
        min="1"
        max="10000000"
        required
      />
    </label>
    <p v-if="stoppingType === 'evaluations'" class="hint">
      Учитываются все вычисления функции, в том числе при расчёте численного градиента.
    </p>
    <p v-else-if="methodId === 'nelder_mead'" class="hint">
      Условие проверяется для всех вершин симплекса на соседних итерациях.
    </p>
    <p v-else-if="methodId === 'bfgs'" class="hint">
      Сравниваются принятые шаги в исходных координатах.
    </p>
    <p v-else class="hint">Сравниваются лучшие точки соседних поколений.</p>
    <template v-if="methodId === 'nelder_mead'">
      <label>
        Начальный шаг
        <input v-model.number="parameters.initial_step" type="number" step="any" min="0" required />
      </label>
    </template>
    <template v-else>
      <template v-if="methodId === 'bfgs'">
        <label>
          Шаг численного градиента
          <input
            v-model.number="parameters.gradient_step"
            type="number"
            min="0"
            max="0.1"
            step="any"
            required
          />
        </label>
        <p class="hint">Вычисление градиента расходует общий бюджет функции.</p>
      </template>
      <template v-else>
        <label>
          Начальный масштаб
          <input
            v-model.number="parameters.initial_step"
            type="number"
            min="0"
            step="any"
            required
          />
        </label>
        <label>
          Размер популяции
          <input v-model.number="parameters.population" type="number" min="4" max="1024" required />
        </label>
        <label>
          Начальное число генератора
          <input v-model.number="parameters.seed" type="number" min="0" max="4294967295" required />
        </label>
        <button type="button" class="button small" @click="randomSeed">Случайное число</button>
        <p class="hint">Результат CMA-ES может меняться при другом начальном числе.</p>
      </template>
      <label class="check">
        <input v-model="useStart" type="checkbox" />
        Задать начальную точку
      </label>
      <div v-if="useStart" class="form-grid">
        <label>
          {{ task?.variables?.[0]?.display_name || 'x' }}
          <input v-model.number="startX" type="number" step="any" required />
        </label>
        <label>
          {{ task?.variables?.[1]?.display_name || 'y' }}
          <input v-model.number="startY" type="number" step="any" required />
        </label>
      </div>
    </template>
    <details>
      <summary>Дополнительные ограничения</summary>
      <label>
        Защитный лимит итераций
        <input
          v-model.number="parameters.max_iterations"
          type="number"
          min="1"
          max="1000000"
          required
        />
      </label>
      <label v-if="stoppingType !== 'evaluations'">
        Лимит вычислений
        <input
          v-model.number="parameters.max_evaluations"
          type="number"
          min="1"
          max="10000000"
          required
        />
      </label>
      <label v-if="methodId === 'bfgs'">
        Защитный порог градиента BFGS
        <input
          v-model.number="parameters.gradient_tolerance"
          type="number"
          min="0"
          step="any"
          required
        />
      </label>
      <p class="hint">
        Ограничения действуют на весь расчёт. Поиск может завершиться раньше, если метод не может
        продолжить вычисления.
      </p>
    </details>
    <label class="check">
      <input v-model="parameters.keep_history" type="checkbox" />
      Сохранять историю поиска
    </label>
  </fieldset>
</template>
