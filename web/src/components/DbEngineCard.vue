<script setup>
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from './Icon.vue'
import AppLogo from './AppLogo.vue'
import SoftwareCard from './SoftwareCard.vue'

// DbEngineCard is the Software page's database server: a server has
// MariaDB or MySQL 8.4, never both. While neither is installed it offers the
// choice (MariaDB recommended); once one is, it is that item's ordinary
// card. Where Oracle has no MySQL packages for the server, MySQL's item
// says why (not_offered) and only MariaDB is offered.
const props = defineProps({
  mariadb: { type: Object, required: true },
  mysql: { type: Object, default: null },
  busy: { type: Boolean, default: false },
  working: { type: Boolean, default: false },
  supported: { type: Boolean, default: true },
})
const emit = defineEmits(['install', 'remove'])
const { t, te } = useI18n()

const installed = computed(() => (props.mysql?.installed ? props.mysql : props.mariadb.installed ? props.mariadb : null))
const mysqlOffered = computed(() => !!props.mysql && !props.mysql.not_offered)
const choice = ref('mariadb')
const chosen = computed(() => (choice.value === 'mysql' && mysqlOffered.value ? props.mysql : props.mariadb))
const whyNot = computed(() => {
  const code = props.mysql?.not_offered
  if (!code) return ''
  const key = `errors.${code}`
  return te(key) ? t(key, { detail: props.mysql.not_offered_detail ?? '' }) : code
})
</script>

<template>
  <SoftwareCard
    v-if="installed"
    :item="installed"
    icon="database"
    :busy="busy"
    :working="working"
    :supported="supported"
    @remove="(i) => emit('remove', i)"
  >
    <RouterLink class="btn btn-sm" :to="{ name: 'databases' }">{{ t('software.open_databases') }}</RouterLink>
  </SoftwareCard>

  <section v-else class="db-card" :aria-busy="working">
    <div class="db-head">
      <AppLogo :name="choice === 'mysql' ? 'mysql' : 'mariadb'" icon="database" dim />
      <div class="db-name">
        <h3>{{ t('software.db_engine') }}</h3>
        <div class="muted small">{{ t('software.db_engine_sub') }}</div>
      </div>
      <span v-if="working" class="status-pill warn"><span class="spinner" aria-hidden="true"></span>{{ t('software.working') }}</span>
      <span v-else class="status-pill">{{ t('software.not_installed') }}</span>
    </div>
    <div class="db-opts" role="radiogroup" :aria-label="t('software.db_engine')">
      <label class="db-opt" :class="{ on: choice === 'mariadb' }">
        <input v-model="choice" type="radio" name="db-engine" value="mariadb" />
        <span class="db-opt-text">
          <span class="db-opt-title">{{ t('software.mariadb') }} <span class="tag tag-ok">{{ t('software.db_recommended') }}</span></span>
          <span class="muted small">{{ t('software.db_choice_mariadb', { mb: mariadb.ram_mb }) }}</span>
        </span>
      </label>
      <label v-if="mysqlOffered" class="db-opt" :class="{ on: choice === 'mysql' }">
        <input v-model="choice" type="radio" name="db-engine" value="mysql" />
        <span class="db-opt-text">
          <span class="db-opt-title">{{ t('software.mysql') }}</span>
          <span class="muted small">{{ t('software.db_choice_mysql', { mb: mysql.ram_mb }) }}</span>
        </span>
      </label>
    </div>
    <p v-if="whyNot" class="muted small db-note"><Icon name="info" :size="14" />{{ whyNot }}</p>
    <p class="muted small db-note">{{ t('software.db_switch_note') }}</p>
    <div class="db-actions">
      <button class="btn btn-primary btn-sm" :disabled="busy || !supported" @click="emit('install', chosen)">
        <Icon name="download" :size="15" />{{ t('software.install_engine', { name: t(`software.${chosen.kind}`) }) }}
      </button>
    </div>
  </section>
</template>

<style scoped>
.db-card {
  display: flex; flex-direction: column; gap: 10px; padding: 16px; min-width: 0;
  background: var(--surface); border: 1px dashed var(--border-strong); border-radius: var(--radius);
}
.db-head { display: flex; align-items: center; gap: 12px; }
.db-name { flex: 1; min-width: 0; }
.db-name h3 { margin: 0; font-size: 15px; }
.db-opts { display: flex; flex-direction: column; gap: 8px; }
.db-opt {
  display: flex; align-items: flex-start; gap: 10px; padding: 10px 12px; cursor: pointer;
  border: 1px solid var(--border); border-radius: 10px;
}
.db-opt.on { border-color: var(--accent); box-shadow: 0 0 0 1px var(--accent) inset; }
.db-opt input { margin-top: 3px; flex: none; }
.db-opt-text { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.db-opt-title { font-weight: 600; display: inline-flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.db-note { display: flex; align-items: flex-start; gap: 6px; margin: 0; }
.db-note .icon { flex: none; margin-top: 2px; }
.db-actions { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; margin-top: auto; padding-top: 4px; }
</style>
