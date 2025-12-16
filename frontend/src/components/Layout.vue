<template>
  <div class="min-h-screen bg-gray-50 flex">
    <!-- Sidebar -->
    <aside class="w-64 bg-gradient-to-b from-slate-900 via-slate-800 to-slate-900 border-r border-slate-700 flex flex-col fixed h-screen shadow-xl">
      <!-- Logo -->
      <div class="p-6 border-b border-slate-700/50">
        <router-link to="/dashboard" class="flex items-center gap-3 hover:opacity-80 transition-opacity">
          <img :src="logo" alt="Cafe Discovery" class="h-10 w-10 object-contain" />
          <h1 class="text-xl font-bold text-white">Cafe Discovery</h1>
        </router-link>
      </div>

      <!-- Navigation Menu -->
      <nav class="flex-1 overflow-y-auto py-4">
        <div class="px-3 space-y-1">
          <router-link
            to="/dashboard"
            class="flex items-center gap-3 px-3 py-2 rounded-lg text-sm font-medium transition-all duration-200"
            :class="$route.name === 'Dashboard'
              ? 'bg-primary-600 text-white shadow-lg shadow-primary-500/50'
              : 'text-slate-300 hover:bg-slate-700/50 hover:text-white'"
          >
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6" />
            </svg>
            Dashboard
          </router-link>

          <router-link
            to="/scans"
            class="flex items-center gap-3 px-3 py-2 rounded-lg text-sm font-medium transition-all duration-200"
            :class="$route.name === 'Scans'
              ? 'bg-primary-600 text-white shadow-lg shadow-primary-500/50'
              : 'text-slate-300 hover:bg-slate-700/50 hover:text-white'"
          >
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            Scans
          </router-link>

          <router-link
            to="/tls-scans"
            class="flex items-center gap-3 px-3 py-2 rounded-lg text-sm font-medium transition-all duration-200"
            :class="$route.name === 'TLSScans'
              ? 'bg-primary-600 text-white shadow-lg shadow-primary-500/50'
              : 'text-slate-300 hover:bg-slate-700/50 hover:text-white'"
          >
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
            </svg>
            TLS Scans
          </router-link>

          <router-link
            to="/wallets"
            class="flex items-center gap-3 px-3 py-2 rounded-lg text-sm font-medium transition-all duration-200"
            :class="$route.name === 'Wallets'
              ? 'bg-primary-600 text-white shadow-lg shadow-primary-500/50'
              : 'text-slate-300 hover:bg-slate-700/50 hover:text-white'"
          >
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            Wallets
          </router-link>

          <router-link
            to="/chains"
            class="flex items-center gap-3 px-3 py-2 rounded-lg text-sm font-medium transition-all duration-200"
            :class="$route.name === 'Chains'
              ? 'bg-primary-600 text-white shadow-lg shadow-primary-500/50'
              : 'text-slate-300 hover:bg-slate-700/50 hover:text-white'"
          >
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9" />
            </svg>
            Networks
          </router-link>

          <router-link
            to="/settings"
            class="flex items-center gap-3 px-3 py-2 rounded-lg text-sm font-medium transition-all duration-200"
            :class="$route.name === 'Settings'
              ? 'bg-primary-600 text-white shadow-lg shadow-primary-500/50'
              : 'text-slate-300 hover:bg-slate-700/50 hover:text-white'"
          >
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
            </svg>
            Settings
          </router-link>
        </div>
      </nav>

      <!-- User Section -->
      <div class="p-4 border-t border-slate-700/50">
        <div class="flex items-center gap-3 mb-3">
          <div class="flex-shrink-0">
            <div class="h-8 w-8 rounded-full bg-gradient-to-br from-primary-500 to-primary-600 flex items-center justify-center shadow-lg">
              <span class="text-white text-sm font-medium">
                {{ authStore.user?.email?.charAt(0).toUpperCase() || 'U' }}
              </span>
            </div>
          </div>
          <div class="flex-1 min-w-0">
            <p class="text-sm font-medium text-white truncate">
              {{ authStore.user?.email || 'User' }}
            </p>
          </div>
        </div>
        <button
          @click="handleSignOut"
          class="w-full flex items-center gap-2 px-3 py-2 text-sm font-medium text-slate-300 hover:bg-slate-700/50 hover:text-white rounded-lg transition-all duration-200"
        >
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
          </svg>
          Sign Out
        </button>
      </div>
    </aside>

    <!-- Main Content Area -->
    <div class="flex-1 ml-64 min-h-screen">
      <!-- Main Content -->
      <main class="p-6">
        <slot />
      </main>
    </div>
  </div>
</template>

<script setup>
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import logo from '@/assets/logo.png'

const router = useRouter()
const authStore = useAuthStore()

function handleSignOut() {
  authStore.clearAuth()
  router.push('/signin')
}
</script>

