<template>
  <div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4" @click.self="$emit('close')">
    <div class="bg-white rounded-lg shadow-xl max-w-md w-full p-6">
      <div class="flex items-center gap-3 mb-4">
        <div class="flex-shrink-0">
          <div class="w-12 h-12 bg-yellow-100 rounded-full flex items-center justify-center">
            <svg class="w-6 h-6 text-yellow-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
            </svg>
          </div>
        </div>
        <div class="flex-1">
          <h3 class="text-lg font-bold text-gray-900">Scan Limit Reached</h3>
          <p class="text-sm text-gray-600 mt-1">{{ message }}</p>
        </div>
      </div>

      <div class="bg-gray-50 rounded-lg p-4 mb-4">
        <p class="text-sm text-gray-700">
          You've reached your plan's scan limit. Upgrade to Premium for unlimited scans and access to advanced features.
        </p>
      </div>

      <div class="flex gap-3">
        <button
          @click="$emit('close')"
          class="btn btn-secondary flex-1"
        >
          Cancel
        </button>
        <router-link
          to="/settings"
          @click="$emit('close')"
          class="btn btn-primary flex-1 text-center"
        >
          View Plans
        </router-link>
      </div>
    </div>
  </div>
</template>

<script setup>
defineProps({
  message: {
    type: String,
    required: true
  }
})

defineEmits(['close'])
</script>

