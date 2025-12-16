<template>
  <Layout>
    <div class="space-y-6">
      <!-- Header -->
      <div class="flex justify-between items-center">
        <div>
          <h1 class="text-3xl font-bold text-gray-900">My Wallets</h1>
          <p class="text-gray-600 mt-1">Manage your saved wallet addresses</p>
        </div>
        <button
          @click="openCreateModal"
          class="btn btn-primary"
        >
          <span class="flex items-center">
            <svg class="w-5 h-5 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
            </svg>
            Add Wallet
          </span>
        </button>
      </div>

      <!-- Search -->
      <div class="card">
        <input
          v-model="searchQuery"
          type="text"
          placeholder="Search by label, pub key hash, or user pub key..."
          class="input"
        />
      </div>

      <!-- Loading State -->
      <div v-if="loading" class="card text-center py-12">
        <div class="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-primary-600"></div>
        <p class="mt-4 text-gray-600">Loading wallets...</p>
      </div>

      <!-- Empty State -->
      <div v-else-if="filteredWallets.length === 0" class="card text-center py-12">
        <svg class="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
        <h3 class="mt-4 text-lg font-medium text-gray-900">No wallets found</h3>
        <p class="mt-2 text-sm text-gray-500">
          {{ searchQuery ? 'Try adjusting your search query' : 'Get started by adding your first wallet' }}
        </p>
        <button
          v-if="!searchQuery"
          @click="openCreateModal"
          class="btn btn-primary mt-4"
        >
          Add Wallet
        </button>
      </div>

      <!-- Wallets Grid -->
      <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        <div
          v-for="wallet in filteredWallets"
          :key="wallet.pub_key_hash"
          class="card hover:shadow-lg transition-shadow cursor-pointer"
          @click="openEditModal(wallet)"
        >
          <div class="flex items-start justify-between">
            <div class="flex-1">
              <div class="flex items-center gap-2 mb-2">
                <div class="p-2 bg-primary-100 rounded-lg">
                  <svg class="w-5 h-5 text-primary-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                  </svg>
                </div>
                <h3 class="font-semibold text-gray-900">
                  {{ wallet.label || 'Unnamed Wallet' }}
                </h3>
              </div>
              <div class="space-y-1 text-sm text-gray-600">
                <div class="flex items-center gap-2">
                  <span class="font-medium">Hash:</span>
                  <code class="text-xs bg-gray-100 px-2 py-1 rounded">{{ truncateAddress(wallet.pub_key_hash) }}</code>
                </div>
                <div class="flex items-center gap-2">
                  <span class="font-medium">Pub Key:</span>
                  <code class="text-xs bg-gray-100 px-2 py-1 rounded">{{ truncateAddress(wallet.user_pub_key) }}</code>
                </div>
                <div class="text-xs text-gray-500 mt-2">
                  Added {{ formatDate(wallet.created_at) }}
                </div>
              </div>
            </div>
            <div class="flex gap-2">
              <button
                @click.stop="openEditModal(wallet)"
                class="p-2 text-gray-400 hover:text-primary-600 transition-colors"
                title="Edit"
              >
                <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                </svg>
              </button>
              <button
                @click.stop="confirmDelete(wallet)"
                class="p-2 text-gray-400 hover:text-red-600 transition-colors"
                title="Delete"
              >
                <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                </svg>
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- Create/Edit Modal -->
      <div
        v-if="showModal"
        class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4"
        @click.self="closeModal"
      >
        <div class="bg-white rounded-lg shadow-xl max-w-md w-full max-h-[90vh] overflow-y-auto">
          <div class="p-6">
            <div class="flex justify-between items-center mb-4">
              <h2 class="text-xl font-bold text-gray-900">
                {{ editingWallet ? 'Edit Wallet' : 'Add New Wallet' }}
              </h2>
              <button
                @click="closeModal"
                class="text-gray-400 hover:text-gray-600"
              >
                <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>

            <form @submit.prevent="saveWallet" class="space-y-4">
              <div>
                <label class="block text-sm font-medium text-gray-700 mb-1">
                  Label <span class="text-gray-500">(optional)</span>
                </label>
                <input
                  v-model="form.label"
                  type="text"
                  placeholder="My Main Wallet"
                  class="input"
                />
              </div>

              <div>
                <label class="block text-sm font-medium text-gray-700 mb-1">
                  Public Key Hash <span class="text-red-500">*</span>
                </label>
                <input
                  v-model="form.pub_key_hash"
                  type="text"
                  placeholder="0x..."
                  :disabled="editingWallet"
                  class="input"
                  :class="{ 'bg-gray-100 cursor-not-allowed': editingWallet }"
                  required
                />
                <p v-if="editingWallet" class="text-xs text-gray-500 mt-1">
                  Public key hash cannot be changed
                </p>
              </div>

              <div>
                <label class="block text-sm font-medium text-gray-700 mb-1">
                  User Public Key <span class="text-red-500">*</span>
                </label>
                <input
                  v-model="form.user_pub_key"
                  type="text"
                  placeholder="0x..."
                  class="input"
                  required
                />
              </div>

              <div v-if="error" class="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded">
                {{ error }}
              </div>

              <div class="flex gap-3 pt-4">
                <button
                  type="button"
                  @click="closeModal"
                  class="btn btn-secondary flex-1"
                  :disabled="saving"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  class="btn btn-primary flex-1"
                  :disabled="saving"
                >
                  <span v-if="saving" class="flex items-center justify-center">
                    <svg class="animate-spin -ml-1 mr-2 h-4 w-4" fill="none" viewBox="0 0 24 24">
                      <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                      <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                    </svg>
                    Saving...
                  </span>
                  <span v-else>{{ editingWallet ? 'Update' : 'Create' }}</span>
                </button>
              </div>
            </form>
          </div>
        </div>
      </div>

      <!-- Delete Confirmation Modal -->
      <div
        v-if="walletToDelete"
        class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4"
        @click.self="walletToDelete = null"
      >
        <div class="bg-white rounded-lg shadow-xl max-w-md w-full p-6">
          <h3 class="text-lg font-bold text-gray-900 mb-2">Delete Wallet?</h3>
          <p class="text-gray-600 mb-4">
            Are you sure you want to delete <strong>{{ walletToDelete.label || 'this wallet' }}</strong>? This action cannot be undone.
          </p>
          <div class="flex gap-3">
            <button
              @click="walletToDelete = null"
              class="btn btn-secondary flex-1"
              :disabled="deleting"
            >
              Cancel
            </button>
            <button
              @click="deleteWallet"
              class="btn btn-danger flex-1"
              :disabled="deleting"
            >
              <span v-if="deleting" class="flex items-center justify-center">
                <svg class="animate-spin -ml-1 mr-2 h-4 w-4" fill="none" viewBox="0 0 24 24">
                  <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                  <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
                Deleting...
              </span>
              <span v-else>Delete</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  </Layout>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import Layout from '@/components/Layout.vue'
import { walletService } from '@/services/walletService'

const loading = ref(false)
const wallets = ref([])
const searchQuery = ref('')
const showModal = ref(false)
const editingWallet = ref(null)
const walletToDelete = ref(null)
const saving = ref(false)
const deleting = ref(false)
const error = ref('')

const form = ref({
  pub_key_hash: '',
  user_pub_key: '',
  label: ''
})

const filteredWallets = computed(() => {
  if (!searchQuery.value) {
    return wallets.value
  }
  const query = searchQuery.value.toLowerCase()
  return wallets.value.filter(wallet =>
    wallet.label?.toLowerCase().includes(query) ||
    wallet.pub_key_hash?.toLowerCase().includes(query) ||
    wallet.user_pub_key?.toLowerCase().includes(query)
  )
})

const fetchWallets = async () => {
  loading.value = true
  error.value = ''
  try {
    const response = await walletService.getAllWallets()
    wallets.value = response.wallets || []
  } catch (err) {
    error.value = err.response?.data?.error || 'Failed to load wallets'
    console.error('Error fetching wallets:', err)
  } finally {
    loading.value = false
  }
}

const openCreateModal = () => {
  editingWallet.value = null
  form.value = {
    pub_key_hash: '',
    user_pub_key: '',
    label: ''
  }
  error.value = ''
  showModal.value = true
}

const openEditModal = (wallet) => {
  editingWallet.value = wallet
  form.value = {
    pub_key_hash: wallet.pub_key_hash,
    user_pub_key: wallet.user_pub_key,
    label: wallet.label || ''
  }
  error.value = ''
  showModal.value = true
}

const closeModal = () => {
  showModal.value = false
  editingWallet.value = null
  error.value = ''
}

const saveWallet = async () => {
  saving.value = true
  error.value = ''
  try {
    if (editingWallet.value) {
      await walletService.updateWallet(editingWallet.value.pub_key_hash, {
        user_pub_key: form.value.user_pub_key,
        label: form.value.label
      })
    } else {
      await walletService.createWallet({
        pub_key_hash: form.value.pub_key_hash,
        user_pub_key: form.value.user_pub_key,
        label: form.value.label
      })
    }
    await fetchWallets()
    closeModal()
  } catch (err) {
    error.value = err.response?.data?.error || 'Failed to save wallet'
    console.error('Error saving wallet:', err)
  } finally {
    saving.value = false
  }
}

const confirmDelete = (wallet) => {
  walletToDelete.value = wallet
}

const deleteWallet = async () => {
  if (!walletToDelete.value) return

  deleting.value = true
  error.value = ''
  try {
    await walletService.deleteWallet(walletToDelete.value.pub_key_hash)
    await fetchWallets()
    walletToDelete.value = null
  } catch (err) {
    error.value = err.response?.data?.error || 'Failed to delete wallet'
    console.error('Error deleting wallet:', err)
  } finally {
    deleting.value = false
  }
}

const truncateAddress = (address) => {
  if (!address) return ''
  if (address.length <= 12) return address
  return `${address.slice(0, 6)}...${address.slice(-6)}`
}

const formatDate = (dateString) => {
  if (!dateString) return ''
  const date = new Date(dateString)
  return date.toLocaleDateString('en-US', { year: 'numeric', month: 'short', day: 'numeric' })
}

onMounted(() => {
  fetchWallets()
})
</script>

<style scoped>
.btn-danger {
  @apply bg-red-600 text-white hover:bg-red-700;
}
</style>

