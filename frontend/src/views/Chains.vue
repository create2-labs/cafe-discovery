<template>
  <Layout>
    <div class="space-y-6">
      <!-- Header -->
      <div>
        <h1 class="text-3xl font-bold text-gray-900">Supported Blockchains</h1>
        <p class="text-gray-600 mt-1">View all blockchain networks managed by Cafe Discovery</p>
      </div>

      <!-- Stats Card -->
      <div class="card">
        <div class="flex items-center justify-between">
          <div>
            <p class="text-sm font-medium text-gray-600">Total Networks</p>
            <p class="text-3xl font-bold text-primary-600 mt-1">{{ chains.length }}</p>
          </div>
          <div class="p-4 bg-primary-100 rounded-lg">
            <svg class="w-8 h-8 text-primary-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
            </svg>
          </div>
        </div>
      </div>

      <!-- Loading State -->
      <div v-if="loading" class="card text-center py-12">
        <div class="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-primary-600"></div>
        <p class="mt-4 text-gray-600">Loading blockchain networks...</p>
      </div>

      <!-- Error State -->
      <div v-else-if="error" class="card bg-danger-50 border border-danger-200">
        <div class="flex items-center space-x-2">
          <svg class="w-5 h-5 text-danger-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <p class="text-danger-800">{{ error }}</p>
        </div>
      </div>

      <!-- Empty State -->
      <div v-else-if="chains.length === 0" class="card text-center py-12">
        <svg class="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4" />
        </svg>
        <h3 class="mt-2 text-sm font-medium text-gray-900">No networks configured</h3>
        <p class="mt-1 text-sm text-gray-500">No blockchain networks are currently configured.</p>
      </div>

      <!-- Chains Grid -->
      <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        <div
          v-for="chain in chains"
          :key="chain.name"
          class="card hover:shadow-md transition-shadow"
        >
          <div class="flex items-start justify-between mb-4">
            <div class="flex items-center space-x-3">
              <div class="p-2 bg-primary-100 rounded-lg">
                <svg class="w-6 h-6 text-primary-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
                </svg>
              </div>
              <div>
                <h3 class="text-lg font-semibold text-gray-900">{{ chain.name }}</h3>
                <p class="text-sm text-gray-500">Blockchain Network</p>
              </div>
            </div>
            <span class="badge badge-safe">Active</span>
          </div>

          <div class="space-y-2">
            <div>
              <p class="text-xs font-medium text-gray-500 uppercase tracking-wide mb-1">RPC Endpoint</p>
              <div class="flex items-center space-x-2">
                <code class="flex-1 text-sm font-mono text-gray-900 bg-gray-100 px-3 py-2 rounded break-all">
                  {{ chain.rpc }}
                </code>
                <button
                  @click="copyRPC(chain.rpc)"
                  class="text-gray-400 hover:text-gray-600 transition-colors"
                  title="Copy RPC URL"
                >
                  <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                  </svg>
                </button>
              </div>
            </div>

            <div class="pt-2 border-t border-gray-200">
              <div class="flex items-center justify-between text-sm">
                <span class="text-gray-600">Status</span>
                <div class="flex items-center space-x-1">
                  <span class="w-2 h-2 bg-success-500 rounded-full animate-pulse"></span>
                  <span class="text-success-600 font-medium">Connected</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Info Card -->
      <div class="card bg-primary-50 border border-primary-200">
        <div class="flex items-start space-x-3">
          <svg class="w-5 h-5 text-primary-600 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <div>
            <h3 class="text-sm font-medium text-primary-900 mb-1">About Supported Networks</h3>
            <p class="text-sm text-primary-700">
              Cafe Discovery scans wallets across all configured blockchain networks. Each network is monitored for
              security vulnerabilities, key exposure, and quantum-resistant cryptography status. Scans are performed
              in parallel across all networks to provide comprehensive security analysis.
            </p>
          </div>
        </div>
      </div>
    </div>
  </Layout>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import Layout from '@/components/Layout.vue'
import { scanService } from '@/services/scanService'

const loading = ref(false)
const error = ref('')
const chains = ref([])

async function loadChains() {
  loading.value = true
  error.value = ''

  try {
    const response = await scanService.listRPCs()
    chains.value = response.blockchains || []
  } catch (err) {
    error.value = err.response?.data?.error || 'Failed to load blockchain networks'
    console.error('Failed to load chains:', err)
  } finally {
    loading.value = false
  }
}

function copyRPC(rpc) {
  navigator.clipboard.writeText(rpc)
  // You could add a toast notification here
}

onMounted(() => {
  loadChains()
})
</script>

