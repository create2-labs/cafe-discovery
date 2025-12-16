<template>
  <Layout>
    <div class="space-y-6">
      <!-- Header -->
      <div class="flex justify-between items-center">
        <div>
          <h1 class="text-3xl font-bold text-gray-900">TLS Security Scans</h1>
          <p class="text-gray-600 mt-1">View and manage your TLS endpoint security scans</p>
        </div>
        <button
          @click="showScanModal = true"
          class="btn btn-primary"
        >
          <span class="flex items-center">
            <svg class="w-5 h-5 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
            </svg>
            New Scan
          </span>
        </button>
      </div>

      <!-- Filters -->
      <div class="card">
        <div class="flex flex-wrap items-center gap-4">
          <div class="flex-1 min-w-[200px]">
            <input
              v-model="searchQuery"
              type="text"
              placeholder="Search by URL..."
              class="input"
            />
          </div>
          <select v-model="riskFilter" class="input w-auto">
            <option value="">All Risk Levels</option>
            <option value="high">High Risk</option>
            <option value="medium">Medium Risk</option>
            <option value="low">Low Risk</option>
            <option value="safe">Safe</option>
          </select>
          <select v-model="pqcRiskFilter" class="input w-auto">
            <option value="">All PQC Risks</option>
            <option value="critical">Critical</option>
            <option value="warning">Warning</option>
            <option value="safe">Safe</option>
          </select>
        </div>
      </div>

      <!-- Scans List -->
      <div v-if="loading" class="card text-center py-12">
        <div class="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-primary-600"></div>
        <p class="mt-4 text-gray-600">Loading scans...</p>
      </div>

      <div v-else-if="filteredScans.length === 0" class="card text-center py-12">
        <svg class="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
        </svg>
        <h3 class="mt-2 text-sm font-medium text-gray-900">No TLS scans found</h3>
        <p class="mt-1 text-sm text-gray-500">Get started by scanning a TLS endpoint.</p>
      </div>

      <div v-else class="space-y-4">
        <div
          v-for="scan in paginatedScans"
          :key="scan.url + scan.scanned_at"
          class="card hover:shadow-md transition-shadow cursor-pointer"
          @click="selectedScan = scan"
        >
          <div class="flex items-start justify-between">
            <div class="flex-1 min-w-0">
              <div class="flex items-center space-x-3 mb-3">
                <RiskBadge :risk-score="scan.risk_score" />
                <span class="badge" :class="getPQCRiskBadgeClass(scan.pqc_risk)">
                  {{ scan.pqc_risk.toUpperCase() }}
                </span>
                <span v-if="scan.certificate.is_pqc_ready" class="badge badge-safe">
                  PQC Ready
                </span>
              </div>

              <div class="flex items-center space-x-2 mb-2">
                <code class="text-sm font-mono text-gray-900 bg-gray-100 px-2 py-1 rounded">
                  {{ scan.url }}
                </code>
                <button
                  @click.stop="copyURL(scan.url)"
                  class="text-gray-400 hover:text-gray-600"
                  title="Copy URL"
                >
                  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                  </svg>
                </button>
              </div>

              <div class="grid grid-cols-2 md:grid-cols-4 gap-4 mt-4 text-sm">
                <div>
                  <p class="text-gray-500">Protocol</p>
                  <p class="font-medium">{{ scan.protocol_version }}</p>
                </div>
                <div>
                  <p class="text-gray-500">NIST Level</p>
                  <p class="font-medium">Level {{ scan.nist_level }}</p>
                </div>
                <div>
                  <p class="text-gray-500">Cipher Suites</p>
                  <p class="font-medium">{{ scan.cipher_suites?.length || 0 }}</p>
                </div>
                <div>
                  <p class="text-gray-500">Scanned</p>
                  <p class="font-medium">{{ formatDate(scan.scanned_at) }}</p>
                </div>
              </div>

              <!-- Risk Score Bar -->
              <div class="mt-4">
                <div class="flex justify-between text-xs text-gray-600 mb-1">
                  <span>Risk Score</span>
                  <span>{{ (scan.risk_score * 100).toFixed(0) }}%</span>
                </div>
                <div class="w-full bg-gray-200 rounded-full h-2">
                  <div
                    :class="getRiskBarColor(scan.risk_score)"
                    class="h-2 rounded-full transition-all"
                    :style="{ width: `${scan.risk_score * 100}%` }"
                  ></div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Pagination -->
        <div v-if="totalPages > 1" class="flex justify-center items-center space-x-2">
          <button
            @click="currentPage--"
            :disabled="currentPage === 1"
            class="btn btn-secondary"
            :class="{ 'opacity-50 cursor-not-allowed': currentPage === 1 }"
          >
            Previous
          </button>
          <span class="text-sm text-gray-600">
            Page {{ currentPage }} of {{ totalPages }}
          </span>
          <button
            @click="currentPage++"
            :disabled="currentPage === totalPages"
            class="btn btn-secondary"
            :class="{ 'opacity-50 cursor-not-allowed': currentPage === totalPages }"
          >
            Next
          </button>
        </div>
      </div>
    </div>

    <!-- TLS Scan Detail Modal -->
    <TLSScanDetailModal
      v-if="selectedScan"
      :scan="selectedScan"
      @close="selectedScan = null"
    />

    <!-- Scan Modal -->
    <FlexibleScanModal
      v-if="showScanModal"
      initial-type="tls"
      @close="showScanModal = false"
      @scan-complete="handleScanComplete"
    />
  </Layout>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import Layout from '@/components/Layout.vue'
import FlexibleScanModal from '@/components/FlexibleScanModal.vue'
import TLSScanDetailModal from '@/components/TLSScanDetailModal.vue'
import RiskBadge from '@/components/RiskBadge.vue'
import { tlsService } from '@/services/tlsService'

const loading = ref(false)
const scans = ref([])
const searchQuery = ref('')
const riskFilter = ref('')
const pqcRiskFilter = ref('')
const currentPage = ref(1)
const itemsPerPage = 10
const showScanModal = ref(false)
const selectedScan = ref(null)

const filteredScans = computed(() => {
  let filtered = scans.value

  // Search filter
  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    filtered = filtered.filter(scan =>
      scan.url.toLowerCase().includes(query) ||
      scan.host.toLowerCase().includes(query)
    )
  }

  // Risk filter
  if (riskFilter.value) {
    filtered = filtered.filter(scan => {
      const score = scan.risk_score
      switch (riskFilter.value) {
        case 'high': return score >= 0.7
        case 'medium': return score >= 0.4 && score < 0.7
        case 'low': return score >= 0.1 && score < 0.4
        case 'safe': return score < 0.1
        default: return true
      }
    })
  }

  // PQC Risk filter
  if (pqcRiskFilter.value) {
    filtered = filtered.filter(scan => scan.pqc_risk === pqcRiskFilter.value)
  }

  return filtered
})

const totalPages = computed(() => {
  return Math.ceil(filteredScans.value.length / itemsPerPage)
})

const paginatedScans = computed(() => {
  const start = (currentPage.value - 1) * itemsPerPage
  const end = start + itemsPerPage
  return filteredScans.value.slice(start, end)
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

function formatDate(dateString) {
  if (!dateString) return 'N/A'
  const date = new Date(dateString)
  return date.toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  })
}

function copyURL(url) {
  navigator.clipboard.writeText(url)
}

async function loadScans() {
  loading.value = true
  try {
    const response = await tlsService.listScans(100, 0)
    scans.value = response.results || []
  } catch (error) {
    console.error('Failed to load TLS scans:', error)
  } finally {
    loading.value = false
  }
}

function handleScanComplete() {
  showScanModal.value = false
  loadScans()
}

onMounted(() => {
  loadScans()
})
</script>

