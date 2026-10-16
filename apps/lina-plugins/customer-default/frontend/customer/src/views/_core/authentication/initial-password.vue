<script lang="ts" setup>
import type { VbenFormSchema } from '@vben/common-ui';
import type { Recordable } from '@vben/types';

import { computed, onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';

import { AuthenticationForgetPassword, z } from '@vben/common-ui';
import { preferences } from '@vben/preferences';

import { notification } from 'ant-design-vue';

import {
  changeInitialPassword,
  getInitialPasswordStatus,
} from '#/api/idc/customer-activation';
import { updateProfile } from '#/api/system/user';
import { $t } from '#/locales';

defineOptions({ name: 'InitialPassword' });

const loading = ref(false);
const router = useRouter();

onMounted(async () => {
  try {
    const status = await getInitialPasswordStatus();
    if (!status.required) {
      await router.replace(preferences.app.defaultHomePath);
    }
  } catch {
    await router.replace('/auth/login');
  }
});

const formSchema = computed((): VbenFormSchema[] => [
  {
    component: 'VbenInputPassword',
    componentProps: {
      passwordStrength: true,
      placeholder: $t('initialPassword.newPassword'),
    },
    fieldName: 'password',
    label: $t('initialPassword.newPassword'),
    renderComponentContent() {
      return { strengthText: () => $t('authentication.passwordStrength') };
    },
    rules: z.string().min(8, { message: $t('initialPassword.passwordTip') }),
  },
  {
    component: 'VbenInputPassword',
    componentProps: { placeholder: $t('initialPassword.confirmPassword') },
    dependencies: {
      rules(values) {
        return z
          .string({ required_error: $t('initialPassword.passwordTip') })
          .min(8, { message: $t('initialPassword.passwordTip') })
          .refine((value) => value === values.password, {
            message: $t('initialPassword.confirmPasswordTip'),
          });
      },
      triggerFields: ['password'],
    },
    fieldName: 'confirmPassword',
    label: $t('initialPassword.confirmPassword'),
  },
]);

async function handleSubmit(value: Recordable<any>) {
  if (loading.value) return;
  loading.value = true;
  try {
    const password = String(value.password ?? '');
    await updateProfile({ password });
    await changeInitialPassword(password);
    notification.success({
      description: $t('initialPassword.successDescription'),
      message: $t('initialPassword.successTitle'),
    });
    await router.replace(preferences.app.defaultHomePath);
  } finally {
    loading.value = false;
  }
}
</script>

<template>
  <AuthenticationForgetPassword
    :form-schema="formSchema"
    :loading="loading"
    :sub-title="$t('initialPassword.subtitle')"
    :submit-button-text="$t('initialPassword.submit')"
    :title="$t('initialPassword.title')"
    data-testid="initial-password-page"
    @submit="handleSubmit"
  />
</template>
