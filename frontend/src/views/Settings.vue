<template>
  <Layout>
    <div class="space-y-6">
      <!-- Header -->
      <div>
        <h1 class="text-3xl font-bold text-gray-900">Settings</h1>
        <p class="text-gray-600 mt-1">Manage your account and subscription</p>
      </div>

      <!-- Loading State -->
      <div v-if="loading" class="card text-center py-12">
        <div class="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-primary-600"></div>
        <p class="mt-4 text-gray-600">Loading settings...</p>
      </div>

      <!-- Content -->
      <div v-else class="space-y-6">
        <!-- Your Plan and Billing -->
        <div class="card">
          <h2 class="text-xl font-bold text-gray-900 mb-4">Your Plan and Billing</h2>

          <div v-if="currentPlan" class="space-y-4">
            <div class="flex items-center justify-between p-4 bg-gray-50 rounded-lg">
              <div>
                <div class="flex items-center gap-3">
                  <span class="text-2xl font-bold text-gray-900">{{ currentPlan.name }}</span>
                  <span
                    class="px-3 py-1 rounded-full text-xs font-semibold"
                    :class="currentPlan.type === 'FREE'
                      ? 'bg-green-100 text-green-700'
                      : 'bg-primary-100 text-primary-700'"
                  >
                    {{ currentPlan.type }}
                  </span>
                </div>
                <p class="text-sm text-gray-600 mt-1">
                  {{ currentPlan.type === 'FREE' ? 'Free forever' : `$${currentPlan.price}/month` }}
                </p>
              </div>
              <button
                v-if="currentPlan.type === 'FREE'"
                @click="showUpgradeModal = true"
                class="btn btn-primary"
              >
                Upgrade Plan
              </button>
            </div>

            <!-- Usage Stats -->
            <div v-if="usage" class="grid grid-cols-1 md:grid-cols-2 gap-4 mt-6">
              <div class="p-4 border border-gray-200 rounded-lg">
                <div class="flex items-center justify-between mb-2">
                  <span class="text-sm font-medium text-gray-700">Wallet Scans</span>
                  <span class="text-sm text-gray-500">
                    {{ usage.wallet_scans_used }} / {{ usage.wallet_scan_limit === 0 ? '∞' : usage.wallet_scan_limit }}
                  </span>
                </div>
                <div class="w-full bg-gray-200 rounded-full h-2">
                  <div
                    class="bg-primary-600 h-2 rounded-full transition-all"
                    :style="{ width: usage.wallet_scan_limit === 0 ? '0%' : `${Math.min((usage.wallet_scans_used / usage.wallet_scan_limit) * 100, 100)}%` }"
                  ></div>
                </div>
                <p v-if="usage.wallet_scans_left >= 0" class="text-xs text-gray-500 mt-1">
                  {{ usage.wallet_scans_left }} scans remaining
                </p>
                <p v-else class="text-xs text-gray-500 mt-1">Unlimited</p>
              </div>

              <div class="p-4 border border-gray-200 rounded-lg">
                <div class="flex items-center justify-between mb-2">
                  <span class="text-sm font-medium text-gray-700">Endpoint Scans</span>
                  <span class="text-sm text-gray-500">
                    {{ usage.endpoint_scans_used }} / {{ usage.endpoint_scan_limit === 0 ? '∞' : usage.endpoint_scan_limit }}
                  </span>
                </div>
                <div class="w-full bg-gray-200 rounded-full h-2">
                  <div
                    class="bg-primary-600 h-2 rounded-full transition-all"
                    :style="{ width: usage.endpoint_scan_limit === 0 ? '0%' : `${Math.min((usage.endpoint_scans_used / usage.endpoint_scan_limit) * 100, 100)}%` }"
                  ></div>
                </div>
                <p v-if="usage.endpoint_scans_left >= 0" class="text-xs text-gray-500 mt-1">
                  {{ usage.endpoint_scans_left }} scans remaining
                </p>
                <p v-else class="text-xs text-gray-500 mt-1">Unlimited</p>
              </div>
            </div>
          </div>
        </div>

        <!-- Available Plans -->
        <div class="card">
          <h2 class="text-xl font-bold text-gray-900 mb-4">Available Plans</h2>

          <div v-if="plans.length > 0" class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div
              v-for="plan in plans"
              :key="plan.id"
              class="p-6 border-2 rounded-lg transition-all"
              :class="plan.type === currentPlan?.type
                ? 'border-primary-500 bg-primary-50'
                : 'border-gray-200 hover:border-gray-300'"
            >
              <div class="flex items-center justify-between mb-4">
                <h3 class="text-lg font-bold text-gray-900">{{ plan.name }}</h3>
                <span
                  v-if="plan.type === currentPlan?.type"
                  class="px-3 py-1 bg-primary-600 text-white text-xs font-semibold rounded-full"
                >
                  Current Plan
                </span>
                <span
                  v-else-if="!plan.is_active"
                  class="px-3 py-1 bg-yellow-100 text-yellow-700 text-xs font-semibold rounded-full"
                >
                  Coming Soon
                </span>
              </div>

              <div class="mb-4">
                <span class="text-3xl font-bold text-gray-900">
                  {{ plan.price === 0 ? 'Free' : `$${plan.price}` }}
                </span>
                <span v-if="plan.price > 0" class="text-gray-600">/month</span>
              </div>

              <ul class="space-y-2 mb-4">
                <li class="flex items-center gap-2 text-sm text-gray-700">
                  <svg class="w-5 h-5 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                  </svg>
                  {{ plan.wallet_scan_limit === 0 ? 'Unlimited' : `${plan.wallet_scan_limit}` }} Wallet Scans
                </li>
                <li class="flex items-center gap-2 text-sm text-gray-700">
                  <svg class="w-5 h-5 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                  </svg>
                  {{ plan.endpoint_scan_limit === 0 ? 'Unlimited' : `${plan.endpoint_scan_limit}` }} Endpoint Scans
                </li>
              </ul>

              <button
                v-if="plan.type !== currentPlan?.type && plan.is_active"
                @click="showUpgradeModal = true"
                class="w-full btn btn-primary"
              >
                {{ plan.type === 'FREE' ? 'Switch to Free' : 'Upgrade to Premium' }}
              </button>
              <button
                v-else-if="plan.type === currentPlan?.type"
                disabled
                class="w-full btn btn-secondary cursor-not-allowed opacity-50"
              >
                Current Plan
              </button>
              <button
                v-else
                disabled
                class="w-full btn btn-secondary cursor-not-allowed opacity-50"
              >
                Coming Soon
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- Upgrade Modal -->
      <div
        v-if="showUpgradeModal"
        class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4"
        @click.self="showUpgradeModal = false"
      >
        <div class="bg-white rounded-lg shadow-xl max-w-md w-full p-6">
          <h3 class="text-lg font-bold text-gray-900 mb-2">Upgrade Your Plan</h3>
          <p class="text-gray-600 mb-4">
            Premium plan features are coming soon! Stay tuned for unlimited scans and advanced features.
          </p>
          <button
            @click="showUpgradeModal = false"
            class="btn btn-primary w-full"
          >
            Got it
          </button>
        </div>
      </div>
    </div>
  </Layout>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import Layout from '@/components/Layout.vue'
import { planService } from '@/services/planService'

const loading = ref(false)
const currentPlan = ref(null)
const plans = ref([])
const usage = ref(null)
const showUpgradeModal = ref(false)

const fetchData = async () => {
  loading.value = true
  try {
    const [planData, plansData, usageData] = await Promise.all([
      planService.getCurrentPlan(),
      planService.getAllPlans(),
      planService.getPlanUsage()
    ])
    currentPlan.value = planData
    plans.value = plansData.plans || []
    usage.value = usageData
  } catch (err) {
    console.error('Error fetching settings:', err)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchData()
})
</script>

