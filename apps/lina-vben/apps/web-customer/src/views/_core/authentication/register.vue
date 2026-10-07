<script lang="ts" setup>
import type { VbenFormSchema } from '@vben/common-ui';
import type { Recordable } from '@vben/types';

import { computed, h, onMounted, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import { AuthenticationRegister, z } from '@vben/common-ui';
import { $t } from '@vben/locales';
import { preferences } from '@vben/preferences';

import { Input, notification } from 'ant-design-vue';

import { registerApi } from '#/api/core/auth';
import {
  completeOnboardingHumanChallenge,
  createOnboardingHumanChallenge,
  createVerifiedOnboardingIntent,
  sendOnboardingVerificationCode,
  type OnboardingBinding,
} from '#/api/idc/customer-onboarding';
import { publicFrontendSettings } from '#/runtime/public-frontend';
import VbenModalCompat from '#/components/vben-modal-compat.vue';

defineOptions({ name: 'Register' });

const router = useRouter();
const route = useRoute();
const loading = ref(false);
const privacyOpen = ref(false);
const termsOpen = ref(false);
const humanChallengeOpen = ref(false);
const verificationCodeOpen = ref(false);
const humanAnswer = ref('');
const verificationCode = ref('');
const challengePrompt = ref('');
const maskedDestination = ref('');
const pendingRegistration = ref<Recordable<any> | null>(null);
const pendingBinding = ref<OnboardingBinding | null>(null);
const challengeNo = ref('');
const verificationNo = ref('');
const verificationIdempotencyKey = ref('');
const intentIdempotencyKey = ref('');

const onboardingTokenKey = 'linapro.customer.onboarding.token';
const onboardingBrowserKey = 'linapro.customer.onboarding.browser';
const onboardingUserKey = 'linapro.customer.onboarding.user';
const onboardingExpiryKey = 'linapro.customer.onboarding.expires';

const registerEnabled = computed(
  () => publicFrontendSettings.auth.registerEnabled !== false,
);

const activationRedirect = computed(() => {
  if (typeof route.query.redirect !== 'string') return '';
  try {
    const redirect = decodeURIComponent(route.query.redirect);
    return redirect === '/auth/customer-activation' ? redirect : '';
  } catch {
    return '';
  }
});

const privacyPolicyContent = computed(
  () =>
    `${publicFrontendSettings.auth.privacyPolicy || $t('authentication.privacyPolicyDefaultBody')}\n\n${$t('customerOnboarding.abusePreventionNotice')}`,
);

const termsOfServiceContent = computed(
  () =>
    publicFrontendSettings.auth.termsOfService ||
    $t('authentication.termsOfServiceDefaultBody'),
);

function redirectWhenDisabled() {
  if (!registerEnabled.value) {
    void router.replace('/auth/login');
  }
}

function openPrivacyPolicy(event: Event) {
  event.preventDefault();
  event.stopPropagation();
  privacyOpen.value = true;
}

function openTermsOfService(event: Event) {
  event.preventDefault();
  event.stopPropagation();
  termsOpen.value = true;
}

onMounted(redirectWhenDisabled);
watch(registerEnabled, redirectWhenDisabled);

const formSchema = computed((): VbenFormSchema[] => {
  const schema: VbenFormSchema[] = [
    {
      component: 'VbenInput',
      componentProps: {
        placeholder: $t('authentication.usernameTip'),
      },
      fieldName: 'username',
      label: $t('authentication.username'),
      rules: z.string().min(2, { message: $t('authentication.usernameTip') }),
    },
    {
      component: 'VbenInput',
      componentProps: {
        placeholder: 'example@example.com',
      },
      fieldName: 'email',
      label: $t('authentication.email'),
      rules: z
        .string()
        .min(1, { message: $t('authentication.emailTip') })
        .email($t('authentication.emailValidErrorTip')),
    },
    {
      component: 'VbenInputPassword',
      componentProps: {
        passwordStrength: true,
        placeholder: $t('authentication.password'),
      },
      fieldName: 'password',
      label: $t('authentication.password'),
      renderComponentContent() {
        return {
          strengthText: () => $t('authentication.passwordStrength'),
        };
      },
      rules: z.string().min(6, { message: $t('authentication.passwordTip') }),
    },
    {
      component: 'VbenInputPassword',
      componentProps: {
        placeholder: $t('authentication.confirmPassword'),
      },
      dependencies: {
        rules(values) {
          const { password } = values;
          return z
            .string({ required_error: $t('authentication.passwordTip') })
            .min(6, { message: $t('authentication.passwordTip') })
            .refine((value) => value === password, {
              message: $t('authentication.confirmPasswordTip'),
            });
        },
        triggerFields: ['password'],
      },
      fieldName: 'confirmPassword',
      label: $t('authentication.confirmPassword'),
    },
    {
      component: 'VbenCheckbox',
      fieldName: 'agreePolicy',
      renderComponentContent: () => ({
        default: () =>
          h('span', { class: 'text-sm' }, [
            $t('authentication.agree'),
            h(
              'a',
              {
                class: 'vben-link ml-1',
                href: 'javascript:void(0)',
                'data-testid': 'register-privacy-link',
                onClick: openPrivacyPolicy,
              },
              $t('authentication.privacyPolicy'),
            ),
            h('span', { class: 'mx-1' }, ' & '),
            h(
              'a',
              {
                class: 'vben-link',
                href: 'javascript:void(0)',
                'data-testid': 'register-terms-link',
                onClick: openTermsOfService,
              },
              $t('authentication.terms'),
            ),
          ]),
      }),
      rules: z.boolean().refine((value) => !!value, {
        message: $t('authentication.agreeTip'),
      }),
    },
  ];
  return schema;
});

async function handleSubmit(value: Recordable<any>) {
  loading.value = true;
  try {
    if (activationRedirect.value) {
      await registerApi({
        email: String(value.email ?? ''),
        password: String(value.password ?? ''),
        username: String(value.username ?? ''),
      });
      notification.success({
        description: $t('authentication.registerSuccessDesc'),
        duration: 3,
        message: $t('authentication.registerSuccessTitle'),
      });
      await router.push({
        path: '/auth/login',
        query: { redirect: encodeURIComponent(activationRedirect.value) },
      });
      return;
    }
    const browserNonce = randomReference('browser');
    const binding: OnboardingBinding = {
      browserNonce,
      customerType: 'individual',
      destination: String(value.email ?? '').trim(),
      destinationKind: 'email',
      locale: preferences.app.locale === 'en-US' ? 'en-US' : 'zh-CN',
      returnPath: '/account',
      username: String(value.username ?? '').trim(),
    };
    const challenge = await createOnboardingHumanChallenge(binding);
    pendingRegistration.value = { ...value };
    pendingBinding.value = binding;
    challengeNo.value = challenge.challengeNo;
    challengePrompt.value = challenge.accessiblePrompt || '';
    humanAnswer.value = '';
    humanChallengeOpen.value = true;
  } catch {
    // requestClient already surfaces the localized API error toast.
  } finally {
    loading.value = false;
  }
}

function randomReference(prefix: string) {
  const bytes = new Uint8Array(18);
  crypto.getRandomValues(bytes);
  const value = Array.from(bytes, (item) =>
    item.toString(16).padStart(2, '0'),
  ).join('');
  return `${prefix}-${value}`;
}

async function confirmHumanChallenge() {
  if (!pendingBinding.value || !humanAnswer.value.trim()) return;
  loading.value = true;
  try {
    const proof = await completeOnboardingHumanChallenge({
      ...pendingBinding.value,
      answer: humanAnswer.value.trim(),
      challengeNo: challengeNo.value,
    });
    verificationIdempotencyKey.value = randomReference('code');
    const verification = await sendOnboardingVerificationCode({
      ...pendingBinding.value,
      idempotencyKey: verificationIdempotencyKey.value,
      proofToken: proof.proofToken,
    });
    verificationNo.value = verification.verificationNo;
    maskedDestination.value = verification.maskedDestination;
    verificationCode.value = '';
    humanChallengeOpen.value = false;
    verificationCodeOpen.value = true;
  } catch {
    // requestClient already displays the localized reason.
  } finally {
    loading.value = false;
  }
}

async function confirmVerificationCode() {
  const binding = pendingBinding.value;
  const registration = pendingRegistration.value;
  if (!binding || !registration || !/^\d{6}$/u.test(verificationCode.value)) {
    return;
  }
  loading.value = true;
  try {
    intentIdempotencyKey.value ||= randomReference('intent');
    const intent = await createVerifiedOnboardingIntent({
      ...binding,
      code: verificationCode.value,
      idempotencyKey: intentIdempotencyKey.value,
      verificationNo: verificationNo.value,
    });
    sessionStorage.setItem(onboardingTokenKey, intent.token);
    sessionStorage.setItem(onboardingBrowserKey, binding.browserNonce);
    sessionStorage.setItem(onboardingExpiryKey, String(intent.expiresAt));
    const registered = await registerApi({
      email: String(registration.email ?? ''),
      password: String(registration.password ?? ''),
      username: String(registration.username ?? ''),
    });
    sessionStorage.setItem(onboardingUserKey, String(registered.userId));
    verificationCodeOpen.value = false;
    pendingRegistration.value = null;
    notification.success({
      description: $t('customerOnboarding.registerSuccessDescription'),
      duration: 5,
      message: $t('authentication.registerSuccessTitle'),
    });
    await router.push({
      path: '/auth/login',
      query: {
        redirect: encodeURIComponent('/auth/customer-onboarding-complete'),
      },
    });
  } catch {
    notification.info({
      description: $t('customerOnboarding.intentPreservedDescription'),
      duration: 6,
      message: $t('customerOnboarding.intentPreservedTitle'),
    });
  } finally {
    loading.value = false;
  }
}
</script>

<template>
  <div>
    <AuthenticationRegister
      v-if="registerEnabled"
      :form-schema="formSchema"
      :loading="loading"
      :sub-title="$t('authentication.signUpSubtitle')"
      data-testid="register-page"
      @submit="handleSubmit"
    />

    <VbenModalCompat
      v-model:open="privacyOpen"
      :footer="null"
      :title="$t('authentication.privacyPolicy')"
      destroy-on-close
      width="640px"
    >
      <div
        class="max-h-[60vh] overflow-y-auto whitespace-pre-wrap text-sm leading-6 text-foreground"
        data-testid="register-privacy-modal-body"
      >
        {{ privacyPolicyContent }}
      </div>
    </VbenModalCompat>

    <VbenModalCompat
      v-model:open="termsOpen"
      :footer="null"
      :title="$t('authentication.terms')"
      destroy-on-close
      width="640px"
    >
      <div
        class="max-h-[60vh] overflow-y-auto whitespace-pre-wrap text-sm leading-6 text-foreground"
        data-testid="register-terms-modal-body"
      >
        {{ termsOfServiceContent }}
      </div>
    </VbenModalCompat>

    <VbenModalCompat
      v-model:open="humanChallengeOpen"
      :confirm-loading="loading"
      :ok-text="$t('customerOnboarding.continue')"
      :title="$t('customerOnboarding.humanTitle')"
      data-testid="customer-onboarding-human-modal"
      @ok="confirmHumanChallenge"
    >
      <p class="mb-3 text-sm text-muted-foreground">
        {{ $t('customerOnboarding.humanDescription') }}
      </p>
      <p class="mb-3 font-medium" data-testid="customer-onboarding-prompt">
        {{ challengePrompt }}
      </p>
      <Input
        v-model:value="humanAnswer"
        :placeholder="$t('customerOnboarding.humanAnswer')"
        autocomplete="off"
        data-testid="customer-onboarding-human-answer"
        @press-enter="confirmHumanChallenge"
      />
    </VbenModalCompat>

    <VbenModalCompat
      v-model:open="verificationCodeOpen"
      :confirm-loading="loading"
      :ok-text="$t('customerOnboarding.createAccount')"
      :title="$t('customerOnboarding.codeTitle')"
      data-testid="customer-onboarding-code-modal"
      @ok="confirmVerificationCode"
    >
      <p class="mb-3 text-sm text-muted-foreground">
        {{
          $t('customerOnboarding.codeDescription', {
            destination: maskedDestination,
          })
        }}
      </p>
      <Input
        v-model:value="verificationCode"
        :maxlength="6"
        :placeholder="$t('customerOnboarding.codePlaceholder')"
        autocomplete="one-time-code"
        data-testid="customer-onboarding-code"
        inputmode="numeric"
        @press-enter="confirmVerificationCode"
      />
    </VbenModalCompat>
  </div>
</template>
