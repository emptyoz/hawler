<script setup>
defineProps({
  notifications: { type: Array, required: true },
  removeToast: { type: Function, required: true }
})
</script>

<template>
  <div class="toast-stack" v-if="notifications.length">
    <article
      v-for="toast in notifications"
      :key="toast.id"
      class="toast-item"
      :class="toast.type === 'error' ? 'toast-error' : 'toast-success'"
    >
      <p>{{ toast.message }}</p>
      <button class="toast-close" @click="removeToast(toast.id)">x</button>
    </article>
  </div>
</template>
