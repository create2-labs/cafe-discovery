<template>
  <div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4" @click.self="$emit('close')">
    <div class="bg-white rounded-lg shadow-xl max-w-md w-full">
      <div class="p-6">
        <div class="flex justify-between items-center mb-4">
          <h2 class="text-xl font-semibold">Scan Wallet Address</h2>
          <button @click="$emit('close')" class="text-gray-400 hover:text-gray-600">
            <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <form @submit.prevent="handleScan" class="space-y-4">
          <div>
            <label for="address" class="block text-sm font-medium text-gray-700 mb-1">
              Ethereum Address
            </label>
            <input
              id="address"
              v-model="address"
              type="text"
              required
              class="input font-mono"
              placeholder="0x742d35Cc6634C0532925a3b844Bc454e4438f44e"
            />
            <p class="mt-1 text-xs text-gray-500">Enter a valid Ethereum wallet address</p>
          </div>

          <div v-if="error" class="bg-danger-50 border border-danger-200 text-danger-800 px-4 py-3 rounded-lg text-sm">
            {{ error }}
          </div>

          <div v-if="scanning" class="text-center py-4">
            <div class="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-primary-600 mb-2"></div>
            <p class="text-sm text-gray-600">Scanning wallet across networks...</p>
          </div>

          <div v-if="scanResult" class="space-y-4">
            <div class="bg-gray-50 rounded-lg p-4">
              <h3 class="font-medium mb-2">Scan Complete</h3>
              <div class="space-y-2 text-sm">
                <div class="flex justify-between">
                  <span class="text-gray-600">Risk Score:</span>
                  <RiskBadge :risk-score="scanResult.risk_score" />
                </div>
                <div class="flex justify-between">
                  <span class="text-gray-600">Type:</span>
                  <span class="font-medium">{{ scanResult.type }}</span>
                </div>
                <div class="flex justify-between">
                  <span class="text-gray-600">NIST Level:</span>
                  <span class="font-medium">{{ scanResult.nist_level }}</span>
                </div>
                <div class="flex justify-between">
                  <span class="text-gray-600">Key Exposed:</span>
                  <span :class="scanResult.key_exposed ? 'text-danger-600' : 'text-success-600'">
                    {{ scanResult.key_exposed ? 'Yes' : 'No' }}
                  </span>
                </div>
              </div>
            </div>
            <button
              type="button"
              @click="handleClose"
              class="btn btn-primary w-full"
            >
              View Details
            </button>
          </div>

          <div v-else class="flex space-x-3">
            <button
              type="button"
              @click="$emit('close')"
              class="btn btn-secondary flex-1"
              :disabled="scanning"
            >
              Cancel
            </button>
            <button
              type="submit"
              :disabled="scanning"
              class="btn btn-primary flex-1"
            >
              Scan
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { scanService } from '@/services/scanService'
import RiskBadge from '@/components/RiskBadge.vue'

const router = useRouter()

const address = ref('')
const scanning = ref(false)
const error = ref('')
const scanResult = ref(null)

async function handleScan() {
  scanning.value = true
  error.value = ''
  scanResult.value = null

  try {
    const result = await scanService.scanWallet(address.value)
    scanResult.value = result
  } catch (err) {
    error.value = err.response?.data?.error || 'Failed to scan wallet. Please try again.'
  } finally {
    scanning.value = false
  }
}

function handleClose() {
  emit('close')
  emit('scan-complete')
  router.push('/scans')
}

const emit = defineEmits(['close', 'scan-complete'])
</script>

