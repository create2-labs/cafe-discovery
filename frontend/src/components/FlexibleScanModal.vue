<template>
  <div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4" @click.self="$emit('close')">
    <div class="bg-white rounded-lg shadow-xl max-w-md w-full">
      <div class="p-6">
        <div class="flex justify-between items-center mb-4">
          <h2 class="text-xl font-semibold">
            {{ scanType === 'wallet' ? 'Scan Wallet Address' : 'Scan TLS Endpoint' }}
          </h2>
          <button @click="$emit('close')" class="text-gray-400 hover:text-gray-600">
            <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <!-- Scan Type Selector -->
        <div v-if="!scanType" class="mb-4">
          <label class="block text-sm font-medium text-gray-700 mb-2">Scan Type</label>
          <div class="grid grid-cols-2 gap-3">
            <button
              @click="scanType = 'wallet'"
              class="p-4 border-2 border-gray-200 rounded-lg hover:border-primary-500 hover:bg-primary-50 transition-colors text-left"
            >
              <div class="flex items-center space-x-2">
                <svg class="w-5 h-5 text-primary-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                <div>
                  <p class="font-medium text-gray-900">Wallet</p>
                  <p class="text-xs text-gray-500">Ethereum address</p>
                </div>
              </div>
            </button>
            <button
              @click="scanType = 'tls'"
              class="p-4 border-2 border-gray-200 rounded-lg hover:border-primary-500 hover:bg-primary-50 transition-colors text-left"
            >
              <div class="flex items-center space-x-2">
                <svg class="w-5 h-5 text-primary-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
                </svg>
                <div>
                  <p class="font-medium text-gray-900">TLS Endpoint</p>
                  <p class="text-xs text-gray-500">HTTPS URL</p>
                </div>
              </div>
            </button>
          </div>
        </div>

        <!-- Wallet Scan Form -->
        <form v-else-if="scanType === 'wallet'" @submit.prevent="handleWalletScan" class="space-y-4">
          <div>
            <label for="address" class="block text-sm font-medium text-gray-700 mb-1">
              Ethereum Address
            </label>
            <input
              id="address"
              v-model="walletAddress"
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
                <div v-if="scanResult.public_key" class="pt-2 border-t">
                  <div class="flex items-center justify-between mb-1">
                    <span class="text-gray-600">Public Key:</span>
                    <span class="text-xs text-success-600 font-medium">✓ Recovered</span>
                  </div>
                  <code class="block text-xs font-mono text-gray-900 bg-white px-2 py-1 rounded break-all mt-1">
                    {{ scanResult.public_key }}
                  </code>
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
              @click="resetForm"
              class="btn btn-secondary flex-1"
              :disabled="scanning"
            >
              Back
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

        <!-- TLS Scan Form -->
        <form v-else-if="scanType === 'tls'" @submit.prevent="handleTLSScan" class="space-y-4">
          <div>
            <label for="url" class="block text-sm font-medium text-gray-700 mb-1">
              HTTPS URL
            </label>
            <input
              id="url"
              v-model="tlsURL"
              type="url"
              required
              class="input"
              placeholder="https://example.com"
            />
            <p class="mt-1 text-xs text-gray-500">Enter a valid HTTPS URL to scan</p>
          </div>

          <div v-if="error" class="bg-danger-50 border border-danger-200 text-danger-800 px-4 py-3 rounded-lg text-sm">
            {{ error }}
          </div>

          <div v-if="scanning" class="text-center py-4">
            <div class="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-primary-600 mb-2"></div>
            <p class="text-sm text-gray-600">Scanning TLS configuration...</p>
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
                  <span class="text-gray-600">PQC Risk:</span>
                  <span :class="getPQCRiskClass(scanResult.pqc_risk)" class="font-medium">
                    {{ scanResult.pqc_risk }}
                  </span>
                </div>
                <div class="flex justify-between">
                  <span class="text-gray-600">NIST Level:</span>
                  <span class="font-medium">{{ scanResult.nist_level }}</span>
                </div>
                <div class="flex justify-between">
                  <span class="text-gray-600">Protocol:</span>
                  <span class="font-medium">{{ scanResult.protocol_version }}</span>
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
              @click="resetForm"
              class="btn btn-secondary flex-1"
              :disabled="scanning"
            >
              Back
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

      <!-- Limit Reached Modal -->
      <LimitReachedModal
        v-if="showLimitModal"
        :message="limitMessage"
        @close="showLimitModal = false"
      />
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { scanService } from '@/services/scanService'
import { tlsService } from '@/services/tlsService'
import RiskBadge from '@/components/RiskBadge.vue'
import LimitReachedModal from '@/components/LimitReachedModal.vue'

const props = defineProps({
  initialType: {
    type: String,
    default: null,
    validator: (value) => !value || ['wallet', 'tls'].includes(value)
  }
})

const router = useRouter()
const emit = defineEmits(['close', 'scan-complete'])

const scanType = ref(props.initialType)
const walletAddress = ref('')
const tlsURL = ref('')
const scanning = ref(false)
const error = ref('')
const scanResult = ref(null)
const showLimitModal = ref(false)
const limitMessage = ref('')

function resetForm() {
  scanType.value = null
  walletAddress.value = ''
  tlsURL.value = ''
  error.value = ''
  scanResult.value = null
}

function getPQCRiskClass(risk) {
  switch (risk) {
    case 'safe': return 'text-success-600'
    case 'warning': return 'text-warning-600'
    case 'critical': return 'text-danger-600'
    default: return 'text-gray-600'
  }
}

async function handleWalletScan() {
  scanning.value = true
  error.value = ''
  scanResult.value = null

  try {
    const result = await scanService.scanWallet(walletAddress.value)
    scanResult.value = result
  } catch (err) {
    const errorMsg = err.response?.data?.error || 'Failed to scan wallet. Please try again.'
    // Check if it's a limit error
    if (errorMsg.toLowerCase().includes('limit reached')) {
      limitMessage.value = errorMsg
      showLimitModal.value = true
    } else {
      error.value = errorMsg
    }
  } finally {
    scanning.value = false
  }
}

async function handleTLSScan() {
  scanning.value = true
  error.value = ''
  scanResult.value = null

  try {
    const result = await tlsService.scanEndpoint(tlsURL.value)
    scanResult.value = result
  } catch (err) {
    const errorMsg = err.response?.data?.error || 'Failed to scan TLS endpoint. Please try again.'
    // Check if it's a limit error
    if (errorMsg.toLowerCase().includes('limit reached')) {
      limitMessage.value = errorMsg
      showLimitModal.value = true
    } else {
      error.value = errorMsg
    }
  } finally {
    scanning.value = false
  }
}

function handleClose() {
  emit('close')
  emit('scan-complete')
  if (scanType.value === 'wallet') {
    router.push('/scans')
  } else {
    router.push('/tls-scans')
  }
}
</script>

