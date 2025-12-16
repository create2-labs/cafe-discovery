<template>
  <div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4" @click.self="$emit('close')">
    <div class="bg-white rounded-lg shadow-xl max-w-2xl w-full max-h-[90vh] overflow-y-auto">
      <div class="p-6">
        <div class="flex justify-between items-center mb-6">
          <h2 class="text-2xl font-semibold">Scan Details</h2>
          <button @click="$emit('close')" class="text-gray-400 hover:text-gray-600">
            <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <div v-if="scan" class="space-y-6">
          <!-- Header Info -->
          <div class="flex items-center space-x-4 pb-4 border-b">
            <RiskBadge :risk-score="scan.risk_score" />
            <span class="badge" :class="getTypeBadgeClass(scan.type)">
              {{ scan.type }}
            </span>
            <span v-if="scan.key_exposed" class="badge badge-high">
              Key Exposed
            </span>
            <span v-if="scan.is_erc4337" class="badge badge-safe">
              ERC-4337 Compatible
            </span>
          </div>

          <!-- Address -->
          <div>
            <label class="block text-sm font-medium text-gray-700 mb-1">Wallet Address</label>
            <div class="flex items-center space-x-2">
              <code class="flex-1 text-sm font-mono text-gray-900 bg-gray-100 px-3 py-2 rounded">
                {{ scan.address }}
              </code>
              <button
                @click="copyAddress(scan.address)"
                class="btn btn-secondary text-sm"
              >
                Copy
              </button>
            </div>
          </div>

          <!-- Public Key (if recovered) -->
          <div v-if="scan.public_key">
            <label class="block text-sm font-medium text-gray-700 mb-1">
              Recovered Public Key
              <span class="ml-2 text-xs text-success-600 font-normal">✓ Found</span>
            </label>
            <div class="flex items-center space-x-2">
              <code class="flex-1 text-xs font-mono text-gray-900 bg-gray-100 px-3 py-2 rounded break-all">
                {{ scan.public_key }}
              </code>
              <button
                @click="copyAddress(scan.public_key)"
                class="btn btn-secondary text-sm"
                title="Copy public key"
              >
                Copy
              </button>
            </div>
            <p class="text-xs text-gray-500 mt-1">
              This public key was recovered from transaction signatures on the blockchain.
            </p>
          </div>

          <!-- Security Details Grid -->
          <div class="grid grid-cols-2 gap-4">
            <div class="card">
              <p class="text-sm text-gray-600 mb-1">Algorithm</p>
              <p class="font-semibold">{{ scan.algorithm }}</p>
            </div>
            <div class="card">
              <p class="text-sm text-gray-600 mb-1">NIST Security Level</p>
              <p class="font-semibold">Level {{ scan.nist_level }}</p>
              <p class="text-xs text-gray-500 mt-1">
                {{ getNISTDescription(scan.nist_level) }}
              </p>
            </div>
            <div class="card">
              <p class="text-sm text-gray-600 mb-1">Account Type</p>
              <p class="font-semibold">{{ scan.type }}</p>
              <p class="text-xs text-gray-500 mt-1">
                {{ scan.is_eoa ? 'Externally Owned Account' : 'Smart Contract' }}
              </p>
            </div>
            <div class="card">
              <p class="text-sm text-gray-600 mb-1">Risk Score</p>
              <p class="font-semibold">{{ (scan.risk_score * 100).toFixed(1) }}%</p>
              <div class="mt-2 w-full bg-gray-200 rounded-full h-2">
                <div
                  :class="getRiskBarColor(scan.risk_score)"
                  class="h-2 rounded-full"
                  :style="{ width: `${scan.risk_score * 100}%` }"
                ></div>
              </div>
            </div>
          </div>

          <!-- Networks -->
          <div v-if="scan.networks && scan.networks.length > 0">
            <label class="block text-sm font-medium text-gray-700 mb-2">Exposed Networks</label>
            <div class="flex flex-wrap gap-2">
              <span
                v-for="network in scan.networks"
                :key="network"
                class="badge bg-primary-100 text-primary-800"
              >
                {{ network }}
              </span>
            </div>
          </div>

          <!-- Timestamps -->
          <div class="grid grid-cols-2 gap-4 text-sm">
            <div>
              <p class="text-gray-600">First Seen</p>
              <p class="font-medium">{{ formatDate(scan.first_seen) }}</p>
            </div>
            <div>
              <p class="text-gray-600">Last Scanned</p>
              <p class="font-medium">{{ formatDate(scan.last_seen) }}</p>
            </div>
          </div>

          <!-- Security Recommendations -->
          <div class="border-t pt-4">
            <h3 class="font-semibold mb-3">Security Recommendations</h3>
            <div class="space-y-2">
              <div
                v-for="recommendation in getRecommendations(scan)"
                :key="recommendation"
                class="flex items-start space-x-2 text-sm"
              >
                <svg class="w-5 h-5 text-warning-600 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                </svg>
                <p class="text-gray-700">{{ recommendation }}</p>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import RiskBadge from '@/components/RiskBadge.vue'

const props = defineProps({
  scan: {
    type: Object,
    required: true
  }
})

function getRiskBarColor(score) {
  if (score >= 0.7) return 'bg-danger-600'
  if (score >= 0.4) return 'bg-warning-600'
  if (score >= 0.1) return 'bg-success-600'
  return 'bg-primary-600'
}

function getTypeBadgeClass(type) {
  switch (type) {
    case 'EOA': return 'bg-gray-100 text-gray-800'
    case 'AA': return 'bg-primary-100 text-primary-800'
    case 'Contract': return 'bg-blue-100 text-blue-800'
    default: return 'bg-gray-100 text-gray-800'
  }
}

function getNISTDescription(level) {
  const descriptions = {
    1: 'Quantum-broken - Vulnerable to quantum attacks',
    2: 'Low quantum resistance',
    3: 'Moderate quantum resistance',
    4: 'High quantum resistance',
    5: 'PQC-ready - Post-quantum cryptography ready'
  }
  return descriptions[level] || 'Unknown'
}

function getRecommendations(scan) {
  const recommendations = []

  if (scan.nist_level === 1) {
    recommendations.push('Upgrade to post-quantum cryptography (NIST Level 5) to protect against quantum attacks')
  }

  if (scan.key_exposed) {
    recommendations.push('Private key has been exposed. Consider migrating to a new wallet address')
  }

  if (scan.risk_score >= 0.7) {
    recommendations.push('High risk detected. Review security practices and consider wallet migration')
  }

  if (scan.type === 'EOA' && !scan.is_erc4337) {
    recommendations.push('Consider upgrading to Account Abstraction (ERC-4337) for better security and flexibility')
  }

  if (scan.networks && scan.networks.length > 1) {
    recommendations.push(`Key exposed on ${scan.networks.length} networks. Review activity across all networks`)
  }

  if (recommendations.length === 0) {
    recommendations.push('Wallet security appears good. Continue monitoring for changes')
  }

  return recommendations
}

function formatDate(dateString) {
  if (!dateString) return 'N/A'
  const date = new Date(dateString)
  return date.toLocaleDateString('en-US', {
    month: 'long',
    day: 'numeric',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  })
}

function copyAddress(address) {
  navigator.clipboard.writeText(address)
}
</script>

