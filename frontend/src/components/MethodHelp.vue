<script setup lang="ts">
import { computed, ref } from 'vue'
import { methodGuides } from './methodGuides'

const props = defineProps<{ methodId: string; methodName: string }>()
const dialog = ref<HTMLDialogElement>()
const guide = computed(() => methodGuides[props.methodId])
const helpID = computed(() => 'method-help-' + props.methodId)
</script>

<template>
  <template v-if="guide">
    <button
      type="button"
      class="help-button"
      :aria-label="'Как работает ' + methodName"
      :title="'Как работает ' + methodName"
      aria-haspopup="dialog"
      :aria-controls="helpID"
      @click="dialog?.showModal()"
    >
      ?
    </button>
    <dialog
      ref="dialog"
      :id="helpID"
      class="method-help-dialog"
      :aria-labelledby="helpID + '-title'"
    >
      <div class="help-heading">
        <h2 :id="helpID + '-title'">{{ methodName }}</h2>
        <button type="button" class="button" @click="dialog?.close()">Закрыть</button>
      </div>
      <p>{{ guide.idea }}</p>
      <h3>Как работает</h3>
      <ol>
        <li v-for="step in guide.steps" :key="step">{{ step }}</li>
      </ol>
      <h3>Основные параметры</h3>
      <dl>
        <template v-for="parameter in guide.parameters" :key="parameter.name">
          <dt>{{ parameter.name }}</dt>
          <dd>{{ parameter.text }}</dd>
        </template>
      </dl>
      <h3>Как используется в приложении</h3>
      <p>{{ guide.application }}</p>
      <p class="hint">{{ guide.limitation }}</p>
    </dialog>
  </template>
</template>

<style scoped>
.help-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  width: 28px;
  height: 28px;
  padding: 0;
  color: var(--primary);
  background: var(--surface);
  border: 1px solid var(--primary);
  border-radius: 50%;
  font-size: 16px;
  font-weight: 600;
}
.help-button:hover {
  background: var(--tint);
}
.method-help-dialog {
  width: min(760px, calc(100vw - 32px));
  max-height: calc(100dvh - 48px);
  padding: 20px;
  color: var(--text);
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 4px;
  overflow: auto;
  overflow-wrap: anywhere;
}
.method-help-dialog::backdrop {
  background: rgb(0 0 0 / 45%);
}
.help-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}
.help-heading h2 {
  margin: 0;
}
.help-heading .button {
  flex-shrink: 0;
}
ol {
  margin: 0 0 18px;
  padding-left: 22px;
}
li + li {
  margin-top: 8px;
}
dl {
  margin: 0 0 18px;
}
dt {
  font-weight: 600;
}
dd {
  margin: 3px 0 12px;
}
.hint {
  margin-bottom: 0;
}
</style>
