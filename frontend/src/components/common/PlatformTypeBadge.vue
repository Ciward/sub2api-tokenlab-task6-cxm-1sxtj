<template>
  <div class="inline-flex flex-col gap-0.5 text-xs font-medium">
    <!-- Row 1: Platform + Type -->
    <div class="inline-flex items-center overflow-hidden rounded-md">
      <span :class="['inline-flex items-center gap-1 px-2 py-1', platformClass]">
        <PlatformIcon :platform="platform" size="xs" />
        <span>{{ platformLabel }}</span>
      </span>
      <span :class="['inline-flex items-center gap-1 px-1.5 py-1', typeClass]">
        <!-- OAuth icon -->
        <svg
          v-if="type === 'oauth'"
          class="h-3 w-3"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          stroke-width="2"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z"
          />
        </svg>
        <!-- Setup Token icon -->
        <Icon v-else-if="type === 'setup-token'" name="shield" size="xs" />
        <!-- API Key icon -->
        <Icon v-else name="key" size="xs" />
        <span>{{ typeLabel }}</span>
      </span>
    </div>
    <!-- Row 2: Plan type + Privacy mode (only if either exists) -->
    <div v-if="planLabel || privacyBadge" class="inline-flex items-center overflow-hidden rounded-md">
      <span v-if="planLabel" :class="['inline-flex items-center gap-1 px-1.5 py-1', planBadgeClass]">
        <span>{{ planLabel }}</span>
      </span>
      <span
        v-if="privacyBadge"
        :class="['inline-flex items-center gap-1 px-1.5 py-1', privacyBadge.class]"
        :title="privacyBadge.title"
      >
        <svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" :d="privacyBadge.icon" />
        </svg>
        <span>{{ privacyBadge.label }}</span>
      </span>
    </div>
    <!-- Row 3: Subscription expiration (non-free paid accounts only) -->
    <div v-if="expiresLabel" class="text-[10px] leading-tight text-gray-400 dark:text-gray-500 pl-0.5" :title="effectiveExpiresAt">
      {{ expiresLabel }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AccountPlatform, AccountType } from '@/types'
import PlatformIcon from './PlatformIcon.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()

interface Props {
  platform: AccountPlatform
  type: AccountType
  planType?: string
  privacyMode?: string
  subscriptionExpiresAt?: string
  idToken?: string
}

const props = defineProps<Props>()

/**
 * Decodes base64url (RFC 4648 §5) with missing padding.
 * Safe for all inputs - returns null on any error.
 */
function base64UrlDecode(input: string): string | null {
  if (!input) return null
  try {
    // Convert base64url to base64
    let s = input.replace(/-/g, '+').replace(/_/g, '/')
    // Add padding
    while (s.length % 4) {
      s += '='
    }
    return atob(s)
  } catch {
    return null
  }
}

/**
 * Extracts subscription_expires_at from the OpenAI auth claim in an id_token JWT.
 * Returns empty string if not present or invalid. Never throws.
 *
 * The claim is located at:
 *   payload['https://api.openai.com/auth']['subscription_expires_at']
 *
 * Supports:
 * - RFC3339 string: "2026-05-02T20:32:12+00:00"
 * - Unix timestamp (number)
 */
function extractSubscriptionExpiresFromIdToken(idToken: string | undefined): string {
  if (!idToken) return ''
  const trimmed = idToken.trim()
  if (!trimmed) return ''

  const parts = trimmed.split('.')
  if (parts.length !== 3) return ''

  const payloadJson = base64UrlDecode(parts[1])
  if (!payloadJson) return ''

  try {
    const payload = JSON.parse(payloadJson) as Record<string, unknown>
    const openaiAuth = payload['https://api.openai.com/auth']
    if (!openaiAuth || typeof openaiAuth !== 'object') return ''

    const authObj = openaiAuth as Record<string, unknown>
    const raw = authObj.subscription_expires_at
    if (raw == null) return ''

    // RFC3339 string
    if (typeof raw === 'string') {
      const s = raw.trim()
      if (!s) return ''
      // Validate parseable date
      const d = new Date(s)
      if (!isNaN(d.getTime())) return s
      // Try parsing as Unix timestamp string
      const ts = parseInt(s, 10)
      if (!isNaN(ts) && ts > 0) {
        return new Date(ts * 1000).toISOString()
      }
      return ''
    }

    // Unix timestamp number
    if (typeof raw === 'number' && isFinite(raw) && raw > 0) {
      // Determine if seconds or milliseconds (> 1e12 suggests ms)
      const ts = raw > 1e12 ? raw : raw * 1000
      const d = new Date(ts)
      if (!isNaN(d.getTime())) return d.toISOString()
    }

    return ''
  } catch {
    return ''
  }
}

// The actual expiration time used for display:
// 1. First try credentials.subscription_expires_at
// 2. Fall back to id_token's OpenAI auth claim
const effectiveExpiresAt = computed(() => {
  if (props.subscriptionExpiresAt) {
    return props.subscriptionExpiresAt
  }
  return extractSubscriptionExpiresFromIdToken(props.idToken)
})

const platformLabel = computed(() => {
  if (props.platform === 'anthropic') return 'Anthropic'
  if (props.platform === 'openai') return 'OpenAI'
  if (props.platform === 'antigravity') return 'Antigravity'
  return 'Gemini'
})

const typeLabel = computed(() => {
  switch (props.type) {
    case 'oauth':
      return 'OAuth'
    case 'setup-token':
      return 'Token'
    case 'apikey':
      return 'Key'
    case 'bedrock':
      return 'AWS'
    default:
      return props.type
  }
})

const planLabel = computed(() => {
  if (!props.planType) return ''
  const lower = props.planType.toLowerCase()
  switch (lower) {
    case 'plus':
      return 'Plus'
    case 'team':
      return 'Team'
    case 'chatgptpro':
    case 'pro':
      return 'Pro'
    case 'free':
      return 'Free'
    case 'abnormal':
      return t('admin.accounts.subscriptionAbnormal')
    default:
      return props.planType
  }
})

const platformClass = computed(() => {
  if (props.platform === 'anthropic') {
    return 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400'
  }
  if (props.platform === 'openai') {
    return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400'
  }
  if (props.platform === 'antigravity') {
    return 'bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400'
  }
  return 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400'
})

const typeClass = computed(() => {
  if (props.platform === 'anthropic') {
    return 'bg-orange-100 text-orange-600 dark:bg-orange-900/30 dark:text-orange-400'
  }
  if (props.platform === 'openai') {
    return 'bg-emerald-100 text-emerald-600 dark:bg-emerald-900/30 dark:text-emerald-400'
  }
  if (props.platform === 'antigravity') {
    return 'bg-purple-100 text-purple-600 dark:bg-purple-900/30 dark:text-purple-400'
  }
  return 'bg-blue-100 text-blue-600 dark:bg-blue-900/30 dark:text-blue-400'
})

const planBadgeClass = computed(() => {
  if (props.planType && props.planType.toLowerCase() === 'abnormal') {
    return 'bg-red-100 text-red-600 dark:bg-red-900/30 dark:text-red-400'
  }
  return typeClass.value
})

// Subscription expiration label (non-free only)
const expiresLabel = computed(() => {
  const expiry = effectiveExpiresAt.value
  if (!expiry || !props.planType) return ''
  if (props.planType.toLowerCase() === 'free') return ''
  try {
    const d = new Date(expiry)
    if (isNaN(d.getTime())) return ''
    // Use UTC date to ensure consistency across timezones
    const mm = String(d.getUTCMonth() + 1).padStart(2, '0')
    const dd = String(d.getUTCDate()).padStart(2, '0')
    return `${t('admin.accounts.subscriptionExpires')} ${mm}-${dd}`
  } catch {
    return ''
  }
})

// Privacy badge — shows different states for OpenAI/Antigravity OAuth privacy setting
const privacyBadge = computed(() => {
  if (props.type !== 'oauth' || !props.privacyMode) return null
  // 支持 OpenAI 和 Antigravity 平台
  if (props.platform !== 'openai' && props.platform !== 'antigravity') return null

  const shieldCheck = 'M9 12.75L11.25 15 15 9.75m-3-7.036A11.959 11.959 0 013.598 6 11.99 11.99 0 003 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285z'
  const shieldX = 'M12 9v3.75m0-10.036A11.959 11.959 0 013.598 6 11.99 11.99 0 003 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285zM12 18h.008v.008H12V18z'
  switch (props.privacyMode) {
    // OpenAI states
    case 'training_off':
      return { label: 'Private', icon: shieldCheck, title: t('admin.accounts.privacyTrainingOff'), class: 'bg-green-100 text-green-600 dark:bg-green-900/30 dark:text-green-400' }
    case 'training_set_cf_blocked':
      return { label: 'CF', icon: shieldX, title: t('admin.accounts.privacyCfBlocked'), class: 'bg-yellow-100 text-yellow-600 dark:bg-yellow-900/30 dark:text-yellow-400' }
    case 'training_set_failed':
      return { label: 'Fail', icon: shieldX, title: t('admin.accounts.privacyFailed'), class: 'bg-red-100 text-red-600 dark:bg-red-900/30 dark:text-red-400' }
    // Antigravity states
    case 'privacy_set':
      return { label: 'Private', icon: shieldCheck, title: t('admin.accounts.privacyAntigravitySet'), class: 'bg-green-100 text-green-600 dark:bg-green-900/30 dark:text-green-400' }
    case 'privacy_set_failed':
      return { label: 'Fail', icon: shieldX, title: t('admin.accounts.privacyAntigravityFailed'), class: 'bg-red-100 text-red-600 dark:bg-red-900/30 dark:text-red-400' }
    default:
      return null
  }
})
</script>
