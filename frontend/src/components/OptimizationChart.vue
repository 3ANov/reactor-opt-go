<script setup lang="ts">
import { ref, watch, onMounted, onBeforeUnmount } from 'vue'
import Plotly from 'plotly.js-dist-min'
import type { Task, Result, PlotData } from '../api'

const props = defineProps<{
  data: PlotData
  task: Task
  result: Result | null
  mode: string
  dark: boolean
  feasible: boolean
  domain: boolean
  history: boolean
}>()
const root = ref<HTMLElement>()
const error = ref('')
let observer: ResizeObserver | undefined
let disposed = false
let revision = 0
let pending = Promise.resolve()
function axisLabel(axis: number) {
  const variable = props.task.variables?.[axis]
  return (
    (variable?.display_name || variable?.name || (axis ? 'A₂' : 'A₁')) +
    (variable?.unit ? ', ' + variable.unit : '')
  )
}
function draw(reset = false) {
  if (reset) revision++
  pending = pending.catch(() => undefined).then(renderPlot)
}
async function renderPlot() {
  if (!root.value || disposed) return
  error.value = ''
  const colors = getComputedStyle(document.documentElement)
  const textColor = colors.getPropertyValue('--text').trim()
  const background = colors.getPropertyValue('--surface').trim()
  const gridColor = colors.getPropertyValue('--border').trim()
  if (props.mode === '3d') {
    const canvas = document.createElement('canvas')
    if (!canvas.getContext('webgl2') && !canvas.getContext('webgl')) {
      error.value = 'WebGL недоступен. Выберите 2D для просмотра линий уровня.'
      return
    }
  }
  const data: Record<string, unknown>[] = []
  const result = props.result
  const zLabel =
    props.task.objective_label + (props.task.objective_unit ? ', ' + props.task.objective_unit : '')
  const surface = props.mode === '3d'
  data.push({
    type: surface ? 'surface' : 'contour',
    x: props.data.x,
    y: props.data.y,
    z: props.feasible ? props.data.feasible_z : props.data.z,
    colorscale: 'Viridis',
    connectgaps: false,
    contours: { coloring: 'heatmap', showlines: true },
    line: { width: 0.5, color: 'rgba(255,255,255,.4)' },
    colorbar: { title: { text: zLabel, side: 'right' }, thickness: 12, outlinewidth: 0 },
    hovertemplate:
      axisLabel(0) +
      ': %{x:.5g}<br>' +
      axisLabel(1) +
      ': %{y:.5g}<br>' +
      zLabel +
      ': %{z:.6g}<extra></extra>',
  })
  const polygon = props.data.polygon || []
  if (!surface && props.domain && polygon.length) {
    const closed = polygon.length > 2 ? [...polygon, polygon[0]] : polygon
    data.push({
      type: 'scatter',
      x: closed.map((p) => p[0]),
      y: closed.map((p) => p[1]),
      mode: polygon.length > 2 ? 'lines' : 'lines+markers',
      fill: polygon.length > 2 ? 'toself' : 'none',
      fillcolor: 'rgba(55,131,95,.10)',
      line: { color: '#f08a24', width: 2 },
      name: 'Допустимая область',
      hoverinfo: 'skip',
      showlegend: false,
    })
  }
  const history = result?.history || []
  if (props.history && history.length) {
    data.push({
      type: surface ? 'scatter3d' : 'scatter',
      mode: 'lines+markers',
      x: history.map((h) => h.point[0]),
      y: history.map((h) => h.point[1]),
      z: history.map((h) => h.value),
      line: { color: '#d28739', width: surface ? 3 : 1 },
      marker: { size: surface ? 2 : 4, color: '#d28739' },
      text: history.map((h) => 'Вычисление ' + h.evaluation),
      name: 'История',
      showlegend: false,
      hovertemplate: '%{text}<br>%{x:.5g}; %{y:.5g}<extra></extra>',
    })
  }
  if (result?.point && result.value !== null) {
    data.push({
      type: surface ? 'scatter3d' : 'scatter',
      mode: 'markers',
      x: [result.point[0]],
      y: [result.point[1]],
      z: [result.value],
      marker: {
        size: surface ? 7 : 13,
        color: '#e87753',
        symbol: 'diamond',
        line: { color: '#fff', width: 2 },
      },
      name: 'Лучший результат',
      showlegend: false,
      hovertemplate: 'Лучший результат<br>%{x:.6g}; %{y:.6g}<extra></extra>',
    })
  }
  const axis = {
    gridcolor: gridColor,
    zerolinecolor: gridColor,
    automargin: true,
  }
  try {
    await Plotly.react(
      root.value,
      data,
      {
        autosize: true,
        paper_bgcolor: background,
        plot_bgcolor: background,
        font: {
          family: 'Segoe UI, sans-serif',
          color: textColor,
          size: 12,
        },
        margin: { l: 60, r: 75, b: 55, t: 15 },
        separators: ', ',
        dragmode: surface ? 'orbit' : 'pan',
        uirevision: props.task.id + props.mode + revision,
        xaxis: { ...axis, title: { text: axisLabel(0) } },
        yaxis: { ...axis, title: { text: axisLabel(1) }, scaleanchor: 'x', scaleratio: 1 },
        scene: {
          bgcolor: background,
          xaxis: { ...axis, title: { text: axisLabel(0) } },
          yaxis: { ...axis, title: { text: axisLabel(1) } },
          zaxis: { ...axis, title: { text: zLabel } },
        },
      },
      {
        responsive: true,
        displayModeBar: false,
        displaylogo: false,
        scrollZoom: true,
        showTips: false,
      },
    )
  } catch {
    error.value = 'Не удалось построить график. Уменьшите разрешение или выберите 2D.'
  }
}
onMounted(() => {
  draw()
  observer = new ResizeObserver(() => {
    if (root.value && !disposed) void Plotly.Plots.resize(root.value).catch(() => undefined)
  })
  if (root.value) observer.observe(root.value)
})
watch(
  () => [
    props.data,
    props.task,
    props.result,
    props.mode,
    props.dark,
    props.feasible,
    props.domain,
    props.history,
  ],
  () => draw(),
)
onBeforeUnmount(() => {
  const element = root.value
  disposed = true
  observer?.disconnect()
  void pending.finally(() => {
    if (element) Plotly.purge(element)
  })
})
</script>

<template>
  <div class="chart-shell">
    <div v-show="!error" ref="root" class="plot" aria-label="График целевой функции"></div>
    <div v-if="error" class="empty chart-placeholder" role="alert">
      <strong>График недоступен</strong>
      <p>{{ error }}</p>
      <button class="button secondary" @click="draw(true)">Повторить</button>
    </div>
    <div class="chart-footer">
      <span>
        {{
          mode === '3d'
            ? 'Вращение — перетаскивание, масштаб — колесо мыши'
            : 'Масштаб — колесо мыши, перемещение — перетаскивание'
        }}
      </span>
      <button class="text-button" @click="draw(true)">Сбросить вид</button>
    </div>
  </div>
</template>
