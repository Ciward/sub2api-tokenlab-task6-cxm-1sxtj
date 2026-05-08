import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import PlatformTypeBadge from '../PlatformTypeBadge.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

// Helper to create a test JWT
function makeTestJWT(payload: Record<string, unknown>): string {
  const header = { alg: 'HS256', typ: 'JWT' }
  const enc = (s: string) => btoa(s).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/g, '')
  return enc(JSON.stringify(header)) + '.' + enc(JSON.stringify(payload)) + '.signature'
}

describe('PlatformTypeBadge', () => {
  describe('subscription expiration display', () => {
    it('should NOT show expiration for free plan', () => {
      const wrapper = mount(PlatformTypeBadge, {
        props: {
          platform: 'openai',
          type: 'oauth',
          planType: 'free',
          subscriptionExpiresAt: '2026-12-31T23:59:59Z'
        },
        global: {
          stubs: {
            PlatformIcon: true,
            Icon: true
          }
        }
      })

      // Free plan should not show expiration
      expect(wrapper.text()).not.toContain('subscriptionExpires')
    })

    it('should NOT show expiration when no plan type', () => {
      const wrapper = mount(PlatformTypeBadge, {
        props: {
          platform: 'openai',
          type: 'oauth',
          subscriptionExpiresAt: '2026-12-31T23:59:59Z'
        },
        global: {
          stubs: {
            PlatformIcon: true,
            Icon: true
          }
        }
      })

      expect(wrapper.text()).not.toContain('subscriptionExpires')
    })

    it('should NOT show expiration for invalid date', () => {
      const wrapper = mount(PlatformTypeBadge, {
        props: {
          platform: 'openai',
          type: 'oauth',
          planType: 'plus',
          subscriptionExpiresAt: 'not-a-valid-date'
        },
        global: {
          stubs: {
            PlatformIcon: true,
            Icon: true
          }
        }
      })

      expect(wrapper.text()).not.toContain('subscriptionExpires')
    })

    it('should NOT show expiration when no expiration date', () => {
      const wrapper = mount(PlatformTypeBadge, {
        props: {
          platform: 'openai',
          type: 'oauth',
          planType: 'plus'
        },
        global: {
          stubs: {
            PlatformIcon: true,
            Icon: true
          }
        }
      })

      expect(wrapper.text()).not.toContain('subscriptionExpires')
    })

    it('should show expiration in MM-DD format for paid plan with credentials.expires_at', () => {
      const wrapper = mount(PlatformTypeBadge, {
        props: {
          platform: 'openai',
          type: 'oauth',
          planType: 'plus',
          subscriptionExpiresAt: '2026-12-31T23:59:59Z'
        },
        global: {
          stubs: {
            PlatformIcon: true,
            Icon: true
          }
        }
      })

      expect(wrapper.text()).toContain('subscriptionExpires')
      expect(wrapper.text()).toContain('12-31') // MM-DD format
      // Should NOT show full date format
      expect(wrapper.text()).not.toContain('2026-12-31')
    })

    it('should use id_token as fallback when credentials.expires_at is not set', () => {
      // Create id_token with subscription_expires_at in OpenAI auth claim
      const idToken = makeTestJWT({
        sub: 'user123',
        'https://api.openai.com/auth': {
          chatgpt_plan_type: 'plus',
          subscription_expires_at: '2026-05-15T12:00:00Z'
        }
      })

      const wrapper = mount(PlatformTypeBadge, {
        props: {
          platform: 'openai',
          type: 'oauth',
          planType: 'plus',
          // Note: no subscriptionExpiresAt from credentials
          idToken
        },
        global: {
          stubs: {
            PlatformIcon: true,
            Icon: true
          }
        }
      })

      expect(wrapper.text()).toContain('subscriptionExpires')
      expect(wrapper.text()).toContain('05-15') // MM-DD format
    })

    it('should prefer credentials.subscription_expires_at over id_token', () => {
      const idToken = makeTestJWT({
        sub: 'user123',
        'https://api.openai.com/auth': {
          subscription_expires_at: '2026-01-01T00:00:00Z' // id_token has Jan 1
        }
      })

      const wrapper = mount(PlatformTypeBadge, {
        props: {
          platform: 'openai',
          type: 'oauth',
          planType: 'plus',
          subscriptionExpiresAt: '2026-12-31T23:59:59Z', // credentials has Dec 31
          idToken
        },
        global: {
          stubs: {
            PlatformIcon: true,
            Icon: true
          }
        }
      })

      expect(wrapper.text()).toContain('12-31') // Should prefer credentials
      expect(wrapper.text()).not.toContain('01-01')
    })

    it('should not crash on invalid id_token', () => {
      const invalidTokens = [
        '',
        '   ',
        'not-a-jwt',
        'only.two',
        'a.!@#$.c', // invalid base64
        makeTestJWT({}) // valid JWT but no OpenAI auth claim
      ]

      for (const token of invalidTokens) {
        const wrapper = mount(PlatformTypeBadge, {
          props: {
            platform: 'openai',
            type: 'oauth',
            planType: 'plus',
            idToken: token
          },
          global: {
            stubs: {
              PlatformIcon: true,
              Icon: true
            }
          }
        })
        // Should not crash, and should not show expiration (since no valid expiry)
        expect(wrapper.text()).not.toContain('subscriptionExpires')
      }
    })

    it('should not crash on id_token with invalid expiry format', () => {
      const idToken = makeTestJWT({
        sub: 'user123',
        'https://api.openai.com/auth': {
          subscription_expires_at: 'not-a-date'
        }
      })

      const wrapper = mount(PlatformTypeBadge, {
        props: {
          platform: 'openai',
          type: 'oauth',
          planType: 'plus',
          idToken
        },
        global: {
          stubs: {
            PlatformIcon: true,
            Icon: true
          }
        }
      })

      expect(wrapper.text()).not.toContain('subscriptionExpires')
    })

    it('should set title attribute with full expiration date', () => {
      const wrapper = mount(PlatformTypeBadge, {
        props: {
          platform: 'openai',
          type: 'oauth',
          planType: 'plus',
          subscriptionExpiresAt: '2026-06-15T10:30:00+09:00'
        },
        global: {
          stubs: {
            PlatformIcon: true,
            Icon: true
          }
        }
      })

      const el = wrapper.find('.text-\\[10px\\]')
      expect(el.exists()).toBe(true)
      expect(el.attributes('title')).toBe('2026-06-15T10:30:00+09:00')
    })

    it('should support team plan type', () => {
      const wrapper = mount(PlatformTypeBadge, {
        props: {
          platform: 'openai',
          type: 'oauth',
          planType: 'team',
          subscriptionExpiresAt: '2026-03-20T23:59:59Z'
        },
        global: {
          stubs: {
            PlatformIcon: true,
            Icon: true
          }
        }
      })

      expect(wrapper.text()).toContain('Team')
      expect(wrapper.text()).toContain('03-20')
    })

    it('should support pro plan type', () => {
      const wrapper = mount(PlatformTypeBadge, {
        props: {
          platform: 'openai',
          type: 'oauth',
          planType: 'pro',
          subscriptionExpiresAt: '2026-07-04T23:59:59Z'
        },
        global: {
          stubs: {
            PlatformIcon: true,
            Icon: true
          }
        }
      })

      expect(wrapper.text()).toContain('Pro')
      expect(wrapper.text()).toContain('07-04')
    })
  })
})
