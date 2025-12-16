<template>
  <Layout>
    <div class="space-y-6">
      <!-- Header -->
      <div class="flex justify-between items-center">
        <div>
          <h1 class="text-3xl font-bold text-gray-900">Dashboard</h1>
          <p class="text-gray-600 mt-1">Monitor your wallet security scans</p>
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

      <!-- Stats Cards -->
      <div class="grid grid-cols-1 md:grid-cols-4 gap-4">
        <div class="card">
          <div class="flex items-center justify-between">
            <div>
              <p class="text-sm font-medium text-gray-600">Total Scans</p>
              <p class="text-2xl font-bold text-gray-900 mt-1">{{ stats.total }}</p>
              <p class="text-xs text-gray-500 mt-1">{{ stats.walletTotal }} wallet + {{ stats.tlsTotal }} TLS</p>
            </div>
            <div class="p-3 bg-primary-100 rounded-lg">
              <svg class="w-6 h-6 text-primary-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            </div>
          </div>
        </div>

        <div class="card">
          <div class="flex items-center justify-between">
            <div>
              <p class="text-sm font-medium text-gray-600">High Risk</p>
              <p class="text-2xl font-bold text-danger-600 mt-1">{{ stats.highRisk }}</p>
            </div>
            <div class="p-3 bg-danger-100 rounded-lg">
              <svg class="w-6 h-6 text-danger-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
              </svg>
            </div>
          </div>
        </div>

        <div class="card">
          <div class="flex items-center justify-between">
            <div>
              <p class="text-sm font-medium text-gray-600">Medium Risk</p>
              <p class="text-2xl font-bold text-warning-600 mt-1">{{ stats.mediumRisk }}</p>
            </div>
            <div class="p-3 bg-warning-100 rounded-lg">
              <svg class="w-6 h-6 text-warning-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
              </svg>
            </div>
          </div>
        </div>

        <div class="card">
          <div class="flex items-center justify-between">
            <div>
              <p class="text-sm font-medium text-gray-600">Safe</p>
              <p class="text-2xl font-bold text-success-600 mt-1">{{ stats.safe }}</p>
            </div>
            <div class="p-3 bg-success-100 rounded-lg">
              <svg class="w-6 h-6 text-success-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            </div>
          </div>
        </div>
      </div>

      <!-- Recent Scans -->
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <!-- Recent Wallet Scans -->
        <div class="card">
          <div class="flex justify-between items-center mb-4">
            <h2 class="text-xl font-semibold">Recent Wallet Scans</h2>
            <router-link to="/scans" class="text-sm text-primary-600 hover:text-primary-700">
              View all →
            </router-link>
          </div>
          <div v-if="loading" class="text-center py-8">
            <div class="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-primary-600"></div>
          </div>
          <div v-else-if="scans.length === 0" class="text-center py-8 text-gray-500">
            No wallet scans yet.
          </div>
          <div v-else class="space-y-3">
            <div
              v-for="scan in scans.slice(0, 5)"
              :key="scan.address"
              class="flex items-center justify-between p-4 bg-gray-50 rounded-lg hover:bg-gray-100 transition-colors cursor-pointer"
              @click="$router.push('/scans')"
            >
              <div class="flex items-center space-x-4 flex-1">
                <div class="flex-shrink-0">
                  <RiskBadge :risk-score="scan.risk_score" />
                </div>
                <div class="flex-1 min-w-0">
                  <p class="text-sm font-medium text-gray-900 truncate font-mono">{{ scan.address }}</p>
                  <p class="text-sm text-gray-500">
                    {{ scan.type }} • NIST Level {{ scan.nist_level }}
                  </p>
                </div>
              </div>
              <svg class="w-5 h-5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
              </svg>
            </div>
          </div>
        </div>

        <!-- Recent TLS Scans -->
        <div class="card">
          <div class="flex justify-between items-center mb-4">
            <h2 class="text-xl font-semibold">Recent TLS Scans</h2>
            <router-link to="/tls-scans" class="text-sm text-primary-600 hover:text-primary-700">
              View all →
            </router-link>
          </div>
          <div v-if="loading" class="text-center py-8">
            <div class="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-primary-600"></div>
          </div>
          <div v-else-if="tlsScans.length === 0" class="text-center py-8 text-gray-500">
            No TLS scans yet.
          </div>
          <div v-else class="space-y-3">
            <div
              v-for="scan in tlsScans.slice(0, 5)"
              :key="scan.url"
              class="flex items-center justify-between p-4 bg-gray-50 rounded-lg hover:bg-gray-100 transition-colors cursor-pointer"
              @click="$router.push('/tls-scans')"
            >
              <div class="flex items-center space-x-4 flex-1">
                <div class="flex-shrink-0">
                  <RiskBadge :risk-score="scan.risk_score" />
                </div>
                <div class="flex-1 min-w-0">
                  <p class="text-sm font-medium text-gray-900 truncate">{{ scan.url }}</p>
                  <p class="text-sm text-gray-500">
                    {{ scan.protocol_version }} • {{ scan.pqc_risk }} • NIST Level {{ scan.nist_level }}
                  </p>
                </div>
              </div>
              <svg class="w-5 h-5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
              </svg>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Scan Modal -->
    <FlexibleScanModal
      v-if="showScanModal"
      @close="showScanModal = false"
      @scan-complete="handleScanComplete"
    />
  </Layout>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import Layout from '@/components/Layout.vue'
import FlexibleScanModal from '@/components/FlexibleScanModal.vue'
import RiskBadge from '@/components/RiskBadge.vue'
import { scanService } from '@/services/scanService'
import { tlsService } from '@/services/tlsService'

const loading = ref(false)
const scans = ref([])
const tlsScans = ref([])
const showScanModal = ref(false)

const stats = computed(() => {
  const walletTotal = scans.value.length
  const tlsTotal = tlsScans.value.length
  const total = walletTotal + tlsTotal

  const walletHighRisk = scans.value.filter(s => s.risk_score >= 0.7).length
  const tlsHighRisk = tlsScans.value.filter(s => s.risk_score >= 0.7 || s.pqc_risk === 'critical').length
  const highRisk = walletHighRisk + tlsHighRisk

  const walletMediumRisk = scans.value.filter(s => s.risk_score >= 0.4 && s.risk_score < 0.7).length
  const tlsMediumRisk = tlsScans.value.filter(s => s.risk_score >= 0.4 && s.risk_score < 0.7 || s.pqc_risk === 'warning').length
  const mediumRisk = walletMediumRisk + tlsMediumRisk

  const walletSafe = scans.value.filter(s => s.risk_score < 0.4).length
  const tlsSafe = tlsScans.value.filter(s => s.risk_score < 0.4 && s.pqc_risk === 'safe').length
  const safe = walletSafe + tlsSafe

  return { total, highRisk, mediumRisk, safe, walletTotal, tlsTotal }
})

const recentScans = computed(() => {
  return scans.value.slice(0, 5)
})

async function loadScans() {
  loading.value = true
  try {
    const [walletResponse, tlsResponse] = await Promise.all([
      scanService.listScans(5, 0),
      tlsService.listScans(5, 0)
    ])
    scans.value = walletResponse.results || []
    tlsScans.value = tlsResponse.results || []
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

function formatDate(dateString) {
  if (!dateString) return 'N/A'
  const date = new Date(dateString)
  return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })
}

onMounted(() => {
  loadScans()
})
</script>

