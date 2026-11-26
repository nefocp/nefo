<script setup>
import { useI18n } from 'vue-i18n'
import { LOCALES, setLocale } from '../i18n.js'

const { t, locale } = useI18n()
</script>

<template>
  <label class="lang">
    <span class="sr-only">{{ t('common.language') }}</span>
    <select class="input input-sm" :value="locale" @change="setLocale($event.target.value)">
      <option v-for="l in LOCALES" :key="l.code" :value="l.code">{{ l.name }}</option>
    </select>
  </label>
</template>
