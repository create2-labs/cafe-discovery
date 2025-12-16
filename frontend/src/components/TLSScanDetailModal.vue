<template>
  <div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4" @click.self="$emit('close')">
    <div class="bg-white rounded-lg shadow-xl max-w-3xl w-full max-h-[90vh] overflow-y-auto">
      <div class="p-6">
        <div class="flex justify-between items-center mb-6">
          <h2 class="text-2xl font-semibold">TLS Scan Details</h2>
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
            <span class="badge" :class="getPQCRiskBadgeClass(scan.pqc_risk)">
              {{ scan.pqc_risk.toUpperCase() }}
            </span>
            <span v-if="scan.certificate.is_pqc_ready" class="badge badge-safe">
              PQC Ready
            </span>
          </div>

          <!-- URL -->
          <div>
            <label class="block text-sm font-medium text-gray-700 mb-1">Endpoint URL</label>
            <div class="flex items-center space-x-2">
              <code class="flex-1 text-sm font-mono text-gray-900 bg-gray-100 px-3 py-2 rounded break-all">
                {{ scan.url }}
              </code>
              <button
                @click="copyURL(scan.url)"
                class="btn btn-secondary text-sm"
              >
                Copy
              </button>
            </div>
          </div>

          <!-- Security Details Grid -->
          <div class="grid grid-cols-2 gap-4">
            <div class="card">
              <p class="text-sm text-gray-600 mb-1">Protocol Version</p>
              <p class="font-semibold">{{ scan.protocol_version }}</p>
            </div>
            <div class="card">
              <p class="text-sm text-gray-600 mb-1">NIST Security Level</p>
              <p class="font-semibold">Level {{ scan.nist_level }}</p>
              <p class="text-xs text-gray-500 mt-1">
                {{ getNISTDescription(scan.nist_level) }}
              </p>
            </div>
            <div class="card">
              <p class="text-sm text-gray-600 mb-1">Host</p>
              <p class="font-semibold">{{ scan.host }}</p>
            </div>
            <div class="card">
              <p class="text-sm text-gray-600 mb-1">Port</p>
              <p class="font-semibold">{{ scan.port }}</p>
            </div>
          </div>

          <!-- Certificate Information -->
          <div class="card">
            <h3 class="font-semibold mb-3">Certificate Information</h3>
            <div class="grid grid-cols-2 gap-4 text-sm">
              <div>
                <p class="text-gray-600">Subject</p>
                <p class="font-medium break-all">{{ scan.certificate.subject }}</p>
              </div>
              <div>
                <p class="text-gray-600">Issuer</p>
                <p class="font-medium break-all">{{ scan.certificate.issuer }}</p>
              </div>
              <div>
                <p class="text-gray-600">Signature Algorithm</p>
                <p class="font-medium">{{ scan.certificate.signature_algorithm }}</p>
              </div>
              <div>
                <p class="text-gray-600">Public Key Algorithm</p>
                <p class="font-medium">{{ scan.certificate.public_key_algorithm }}</p>
                <p v-if="scan.certificate.key_size > 0" class="text-xs text-gray-500">
                  Key Size: {{ scan.certificate.key_size }} bits
                </p>
              </div>
              <div>
                <p class="text-gray-600">Valid From</p>
                <p class="font-medium">{{ formatDate(scan.certificate.not_before) }}</p>
              </div>
              <div>
                <p class="text-gray-600">Valid Until</p>
                <p class="font-medium">{{ formatDate(scan.certificate.not_after) }}</p>
              </div>
              <div>
                <p class="text-gray-600">Serial Number</p>
                <p class="font-mono text-xs">{{ scan.certificate.serial_number }}</p>
              </div>
              <div>
                <p class="text-gray-600">NIST Level</p>
                <p class="font-medium">Level {{ scan.certificate.nist_level }}</p>
                <span v-if="scan.certificate.is_pqc_ready" class="badge badge-safe text-xs mt-1">
                  PQC Ready
                </span>
              </div>
            </div>
          </div>

          <!-- Cipher Suites -->
          <div v-if="scan.cipher_suites && scan.cipher_suites.length > 0" class="card">
            <h3 class="font-semibold mb-3">Cipher Suites ({{ scan.cipher_suites.length }})</h3>
            <div class="space-y-2 max-h-64 overflow-y-auto">
              <div
                v-for="suite in scan.cipher_suites"
                :key="suite.id"
                class="flex items-center justify-between p-3 bg-gray-50 rounded-lg"
              >
                <div class="flex-1">
                  <p class="font-medium text-sm">{{ suite.name }}</p>
                  <p class="text-xs text-gray-500">
                    {{ suite.key_exchange }} • {{ suite.encryption }} • {{ suite.mac }}
                  </p>
                </div>
                <div class="flex items-center space-x-2">
                  <span class="text-xs text-gray-600">NIST {{ suite.nist_level }}</span>
                  <span v-if="suite.is_pqc_ready" class="badge badge-safe text-xs">
                    PQC
                  </span>
                </div>
              </div>
            </div>
          </div>

          <!-- Supported PQC Algorithms -->
          <div v-if="scan.supported_pqc && scan.supported_pqc.length > 0" class="card">
            <h3 class="font-semibold mb-3">Supported PQC Algorithms</h3>
            <div class="flex flex-wrap gap-2">
              <span
                v-for="algo in scan.supported_pqc"
                :key="algo"
                class="badge badge-safe"
              >
                {{ algo }}
              </span>
            </div>
          </div>

          <!-- Risk Score -->
          <div class="card">
            <div class="flex justify-between items-center mb-2">
              <p class="text-sm font-medium text-gray-700">Risk Score</p>
              <p class="text-lg font-bold">{{ (scan.risk_score * 100).toFixed(1) }}%</p>
            </div>
            <div class="w-full bg-gray-200 rounded-full h-3">
              <div
                :class="getRiskBarColor(scan.risk_score)"
                class="h-3 rounded-full transition-all"
                :style="{ width: `${scan.risk_score * 100}%` }"
              ></div>
            </div>
          </div>

          <!-- Recommendations -->
          <div v-if="scan.recommendations && scan.recommendations.length > 0" class="border-t pt-4">
            <h3 class="font-semibold mb-3">Security Recommendations</h3>
            <div class="space-y-2">
              <div
                v-for="(recommendation, index) in scan.recommendations"
                :key="index"
                class="flex items-start space-x-2 text-sm"
              >
                <svg class="w-5 h-5 text-warning-600 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                </svg>
                <p class="text-gray-700">{{ recommendation }}</p>
              </div>
            </div>
          </div>

          <!-- Timestamp -->
          <div class="text-sm text-gray-500 border-t pt-4">
            Scanned at: {{ formatDate(scan.scanned_at) }}
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
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

function getPQCRiskBadgeClass(risk) {
  switch (risk) {
    case 'critical': return 'badge-high'
    case 'warning': return 'badge-medium'
    case 'safe': return 'badge-safe'
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

function copyURL(url) {
  navigator.clipboard.writeText(url)
}
</script>

