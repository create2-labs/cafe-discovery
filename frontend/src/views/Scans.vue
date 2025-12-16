<template>
  <Layout>
    <div class="space-y-6">
      <!-- Header -->
      <div class="flex justify-between items-center">
        <div>
          <h1 class="text-3xl font-bold text-gray-900">Security Scans</h1>
          <p class="text-gray-600 mt-1">View and manage your wallet security scans</p>
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
              placeholder="Search by address..."
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
          <select v-model="typeFilter" class="input w-auto">
            <option value="">All Types</option>
            <option value="EOA">EOA</option>
            <option value="AA">AA</option>
            <option value="Contract">Contract</option>
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
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
        </svg>
        <h3 class="mt-2 text-sm font-medium text-gray-900">No scans found</h3>
        <p class="mt-1 text-sm text-gray-500">Get started by scanning a wallet address.</p>
      </div>

      <div v-else class="space-y-4">
        <div
          v-for="scan in paginatedScans"
          :key="scan.address + scan.first_seen"
          class="card hover:shadow-md transition-shadow cursor-pointer"
          @click="selectedScan = scan"
        >
          <div class="flex items-start justify-between">
            <div class="flex-1 min-w-0">
              <div class="flex items-center space-x-3 mb-3">
                <RiskBadge :risk-score="scan.risk_score" />
                <span class="badge" :class="getTypeBadgeClass(scan.type)">
                  {{ scan.type }}
                </span>
                <span v-if="scan.key_exposed" class="badge badge-high">
                  Key Exposed
                </span>
                <span v-if="scan.is_erc4337" class="badge badge-safe">
                  ERC-4337
                </span>
              </div>

              <div class="flex items-center space-x-2 mb-2">
                <code class="text-sm font-mono text-gray-900 bg-gray-100 px-2 py-1 rounded">
                  {{ scan.address }}
                </code>
                <button
                  @click.stop="copyAddress(scan.address)"
                  class="text-gray-400 hover:text-gray-600"
                  title="Copy address"
                >
                  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                  </svg>
                </button>
              </div>

              <div class="grid grid-cols-2 md:grid-cols-4 gap-4 mt-4 text-sm">
                <div>
                  <p class="text-gray-500">Algorithm</p>
                  <p class="font-medium">{{ scan.algorithm }}</p>
                </div>
                <div>
                  <p class="text-gray-500">NIST Level</p>
                  <p class="font-medium">Level {{ scan.nist_level }}</p>
                </div>
                <div>
                  <p class="text-gray-500">Networks</p>
                  <p class="font-medium">{{ scan.networks?.length || 0 }}</p>
                </div>
                <div>
                  <p class="text-gray-500">Scanned</p>
                  <p class="font-medium">{{ formatDate(scan.first_seen) }}</p>
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

    <!-- Scan Detail Modal -->
    <ScanDetailModal
      v-if="selectedScan"
      :scan="selectedScan"
      @close="selectedScan = null"
    />

    <!-- Scan Modal -->
    <FlexibleScanModal
      v-if="showScanModal"
      initial-type="wallet"
      @close="showScanModal = false"
      @scan-complete="handleScanComplete"
    />
  </Layout>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import Layout from '@/components/Layout.vue'
import FlexibleScanModal from '@/components/FlexibleScanModal.vue'
import ScanDetailModal from '@/components/ScanDetailModal.vue'
import RiskBadge from '@/components/RiskBadge.vue'
import { scanService } from '@/services/scanService'

const loading = ref(false)
const scans = ref([])
const searchQuery = ref('')
const riskFilter = ref('')
const typeFilter = ref('')
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
      scan.address.toLowerCase().includes(query)
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

  // Type filter
  if (typeFilter.value) {
    filtered = filtered.filter(scan => scan.type === typeFilter.value)
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

function getTypeBadgeClass(type) {
  switch (type) {
    case 'EOA': return 'bg-gray-100 text-gray-800'
    case 'AA': return 'bg-primary-100 text-primary-800'
    case 'Contract': return 'bg-blue-100 text-blue-800'
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

function copyAddress(address) {
  navigator.clipboard.writeText(address)
  // You could add a toast notification here
}

async function loadScans() {
  loading.value = true
  try {
    const response = await scanService.listScans(100, 0)
    scans.value = response.results || []
  } catch (error) {
    console.error('Failed to load scans:', error)
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

