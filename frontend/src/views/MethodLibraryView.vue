<script setup lang="ts">
import type { Definition } from '../api'
import MethodHelp from '../components/MethodHelp.vue'

defineProps<{ methods: Definition[]; running: boolean }>()
const emit = defineEmits<{ choose: [id: string] }>()
</script>

<template>
  <section class="panel">
    <h2>Методы оптимизации</h2>
    <div class="method-list">
      <article v-for="method in methods" :key="method.id" class="method-item">
        <div class="method-heading">
          <h3>{{ method.name }}</h3>
          <MethodHelp :method-id="method.id" :method-name="method.name" />
        </div>
        <p>{{ method.description }}</p>
        <button class="button" :disabled="running" @click="emit('choose', method.id)">
          Выбрать метод
        </button>
      </article>
    </div>
    <p class="hint">
      Поддерживаются две переменные с границами и линейными ограничениями. Локальная сходимость не
      гарантирует глобальный оптимум.
    </p>
  </section>
</template>

<style scoped>
.method-heading {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}
.method-heading h3 {
  min-width: 0;
  margin: 0;
}
</style>
