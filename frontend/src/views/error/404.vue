<template>
  <div class="error-page">
    <div class="error-content">
      <h1>404</h1>
      <p>{{ t('common.error.notFoundTitle') }}</p>
      <el-button type="primary" @click="$router.push('/dashboard')">{{ t('common.error.backHome') }}</el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
</script>

<style scoped>
.error-page {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 60vh;
}
.error-content {
  text-align: center;
}
.error-content h1 {
  font-size: 80px;
  font-weight: 700;
  color: var(--theme-default, #5a67f5);
  margin: 0 0 8px;
}
.error-content p {
  font-size: 16px;
  color: #909399;
  margin: 0 0 24px;
}
</style>