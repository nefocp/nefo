<script setup lang="ts">
import { computed, ref } from 'vue'
import { useData, withBase } from 'vitepress'
import Icon from './Icon.vue'

const { lang, isDark } = useData()

const INSTALL = 'curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/install.sh | sudo bash'
const GITHUB = 'https://github.com/nefocp/nefo'

const EN = {
  docs: '/docs/',
  badge: 'Open source · AGPLv3',
  title: ['A light, free control panel', 'for your Linux server'],
  lead: 'Websites, SSL, databases, apps, backups and security from one web interface. One small program, every feature free, and nothing phones home.',
  install: 'Install Nefo',
  readDocs: 'Read the docs',
  copy: 'Copy',
  copied: 'Copied',
  os: 'Ubuntu 22.04 · 24.04 · 26.04 and Debian 12 · 13, on x86-64 or ARM64. Run as root on a fresh server.',
  shotsAlt: 'The Nefo panel',
  tabs: { dashboard: 'Dashboard', websites: 'Websites', apps: 'App store', security: 'Security' },
  stats: [
    ['≈30 MB', 'of RAM for the panel itself, measured on a live server'],
    ['1 file', 'a single static program with the web interface built in'],
    ['0', 'telemetry, licence checks or sign-up accounts'],
    ['27 apps', 'in the app store, each installed in one click'],
  ],
  featuresTitle: 'Everything a VPS needs, nothing it doesn’t',
  featuresLead: 'Install only what you use. The menu hides what isn’t installed, and every component can be removed again.',
  features: [
    ['globe', 'Websites', 'PHP, static, Node.js and Python sites on Nginx, Apache or OpenLiteSpeed. One Linux user per site, with SFTP locked to its folder.', 'websites'],
    ['lock', 'Free SSL', 'Let’s Encrypt certificates that renew on their own, wildcard certificates through Cloudflare DNS, or your own.', 'ssl'],
    ['package', 'App store', 'WordPress, Nextcloud, Ghost, n8n, Uptime Kuma, Vaultwarden, Gitea and more, each behind its own website.', 'apps'],
    ['database', 'Databases', 'MariaDB or MySQL tuned for small servers, PostgreSQL, remote access per database, a slow query log, Adminer, phpMyAdmin and a Redis browser.', 'databases'],
    ['archive', 'Backups', 'Scheduled backups of sites, databases and folders to S3, R2, B2, Wasabi, MinIO or SFTP. Restore in one click.', 'backups'],
    ['shieldCheck', 'Website security', 'A web application firewall with the OWASP rules, rate limits, blocked addresses and a malware scanner.', 'security'],
    ['git', 'Deploy from Git', 'GitHub, GitLab, Gitea or any Git server, on every push, with a deploy key per site and a signed webhook.', 'git-deploy'],
    ['wordpress', 'WordPress toolkit', 'Updates with a database backup first, staging copies, maintenance mode and admin password reset.', 'wordpress'],
    ['box', 'Docker', 'Containers, images and Compose projects, checked for privileged containers and public ports.', 'docker'],
    ['folder', 'Files and terminal', 'A file manager with an editor and a 30-day trash, and a root terminal in your browser.', 'files'],
    ['activity', 'Monitoring and alerts', 'Live load with 7 days of history, uptime checks for each website, and alerts by Telegram, e-mail, Discord, Slack or a webhook when something needs you.', 'mail-alerts'],
    ['server', 'Server tools', 'Cron jobs, services, background programs, firewall, fail2ban and SSH settings with lock-out guards.', 'system'],
  ],
  secTitle: 'Secure from the first start',
  secLead: 'A control panel has root on your server, so Nefo treats security as the first feature, not an add-on.',
  sec: [
    ['eyeOff', 'A hidden sign-in page', 'The panel answers only on a secret path. Every other address returns 404, so scanners can’t even find the login page.'],
    ['lock', 'HTTPS from the first minute', 'Encrypted from the first start, and a free certificate for the panel’s own domain when you add one.'],
    ['key', 'Two-factor sign-in', 'Authenticator codes, rate-limited sign-ins and an audit log of who did what.'],
    ['shieldCheck', 'Signed updates', 'Every release is checksummed and signed with Ed25519, and the panel checks it before it installs.'],
    ['code', 'No shell in between', 'System programs run directly, never through a shell, and every input is checked against an allow-list.'],
    ['ban', 'Lock-out guards', 'Firewall and SSH changes that would cut off your own access are refused before they apply.'],
  ],
  freeTitle: 'Free means free',
  free: [
    ['No paid tier', 'Every feature is free for everyone, including businesses.'],
    ['No account', 'No sign-up, no licence server, nothing to renew.'],
    ['No telemetry', 'Your server’s data stays on your server.'],
    ['Open source', 'AGPLv3. Read, change and share the code.'],
  ],
  ctaTitle: 'Try it on a fresh server',
  ctaLead: 'One command installs the panel and prints its address and a one-time password.',
  github: 'Source on GitHub',
}

const VI: typeof EN = {
  docs: '/vi/docs/',
  badge: 'Mã nguồn mở · AGPLv3',
  title: ['Bảng điều khiển nhẹ, miễn phí', 'cho máy chủ Linux của bạn'],
  lead: 'Website, SSL, cơ sở dữ liệu, ứng dụng, sao lưu và bảo mật trong một giao diện web. Một chương trình nhỏ, mọi tính năng đều miễn phí, không gửi dữ liệu đi đâu.',
  install: 'Cài Nefo',
  readDocs: 'Đọc tài liệu',
  copy: 'Chép',
  copied: 'Đã chép',
  os: 'Ubuntu 22.04 · 24.04 · 26.04 và Debian 12 · 13, chip x86-64 hoặc ARM64. Chạy bằng root trên máy chủ mới.',
  shotsAlt: 'Giao diện Nefo',
  tabs: { dashboard: 'Tổng quan', websites: 'Website', apps: 'Kho ứng dụng', security: 'Bảo mật' },
  stats: [
    ['≈30 MB', 'RAM cho chính panel, đo trên máy chủ đang chạy thật'],
    ['1 tệp', 'một chương trình duy nhất, đã kèm sẵn giao diện web'],
    ['0', 'thu thập dữ liệu, kiểm tra bản quyền hay tài khoản đăng ký'],
    ['27 ứng dụng', 'trong kho, mỗi ứng dụng cài bằng một cú bấm'],
  ],
  featuresTitle: 'Đủ mọi thứ VPS cần, không thừa thứ gì',
  featuresLead: 'Chỉ cài những gì bạn dùng. Menu ẩn những phần chưa cài, và mọi thành phần đều gỡ ra được.',
  features: [
    ['globe', 'Website', 'Website PHP, tĩnh, Node.js và Python trên Nginx, Apache hoặc OpenLiteSpeed. Mỗi website một user Linux riêng, SFTP chỉ vào được thư mục của nó.', 'websites'],
    ['lock', 'SSL miễn phí', 'Chứng chỉ Let’s Encrypt tự gia hạn, chứng chỉ wildcard qua DNS Cloudflare, hoặc chứng chỉ của bạn.', 'ssl'],
    ['package', 'Kho ứng dụng', 'WordPress, Nextcloud, Ghost, n8n, Uptime Kuma, Vaultwarden, Gitea và nhiều ứng dụng khác, mỗi cái có website riêng.', 'apps'],
    ['database', 'Cơ sở dữ liệu', 'MariaDB hoặc MySQL tối ưu cho máy chủ nhỏ, PostgreSQL, truy cập từ xa theo từng cơ sở dữ liệu, nhật ký truy vấn chậm, Adminer, phpMyAdmin và trình xem Redis.', 'databases'],
    ['archive', 'Sao lưu', 'Sao lưu website, cơ sở dữ liệu và thư mục theo lịch lên S3, R2, B2, Wasabi, MinIO hoặc SFTP. Khôi phục bằng một cú bấm.', 'backups'],
    ['shieldCheck', 'Bảo vệ website', 'Tường lửa ứng dụng web với bộ luật OWASP, giới hạn truy cập, chặn địa chỉ IP và quét mã độc.', 'security'],
    ['git', 'Triển khai từ Git', 'GitHub, GitLab, Gitea hoặc máy chủ Git bất kỳ, tự chạy mỗi lần push, khoá deploy riêng cho từng website và webhook có chữ ký.', 'git-deploy'],
    ['wordpress', 'Công cụ WordPress', 'Cập nhật kèm sao lưu cơ sở dữ liệu trước, bản staging, chế độ bảo trì và đặt lại mật khẩu admin.', 'wordpress'],
    ['box', 'Docker', 'Container, image và dự án Compose, được kiểm tra để chặn container đặc quyền và cổng mở ra ngoài.', 'docker'],
    ['folder', 'Tệp và terminal', 'Trình quản lý tệp có trình soạn thảo và thùng rác giữ 30 ngày, cùng terminal root ngay trong trình duyệt.', 'files'],
    ['activity', 'Giám sát và cảnh báo', 'Tải máy chủ theo thời gian thực kèm lịch sử 7 ngày, theo dõi từng website, cảnh báo qua Telegram, e-mail, Discord, Slack hoặc webhook khi cần bạn xử lý.', 'mail-alerts'],
    ['server', 'Công cụ máy chủ', 'Cron, dịch vụ, chương trình chạy nền, tường lửa, fail2ban và cài đặt SSH có cơ chế chống tự khoá mình.', 'system'],
  ],
  secTitle: 'An toàn ngay từ lần chạy đầu',
  secLead: 'Panel có quyền root trên máy chủ, nên với Nefo bảo mật là tính năng đầu tiên chứ không phải phần thêm vào sau.',
  sec: [
    ['eyeOff', 'Trang đăng nhập ẩn', 'Panel chỉ trả lời ở một đường dẫn bí mật. Mọi địa chỉ khác đều trả về 404, nên máy quét không tìm thấy trang đăng nhập.'],
    ['lock', 'HTTPS từ phút đầu tiên', 'Mã hoá ngay khi chạy lần đầu, và có chứng chỉ miễn phí cho tên miền riêng của panel khi bạn thêm vào.'],
    ['key', 'Xác thực hai lớp', 'Mã từ ứng dụng xác thực, giới hạn số lần đăng nhập, và nhật ký ghi lại ai đã làm gì.'],
    ['shieldCheck', 'Bản cập nhật có chữ ký', 'Mỗi bản phát hành đều có mã kiểm tra và chữ ký Ed25519, panel kiểm tra trước khi cài.'],
    ['code', 'Không qua shell', 'Các chương trình hệ thống được gọi trực tiếp, không bao giờ qua shell, và mọi dữ liệu nhập vào đều được kiểm tra theo danh sách cho phép.'],
    ['ban', 'Chống tự khoá mình', 'Thay đổi tường lửa hay SSH nào làm bạn mất quyền truy cập sẽ bị từ chối trước khi áp dụng.'],
  ],
  freeTitle: 'Miễn phí là miễn phí thật',
  free: [
    ['Không có bản trả phí', 'Mọi tính năng miễn phí cho tất cả, kể cả doanh nghiệp.'],
    ['Không cần tài khoản', 'Không đăng ký, không máy chủ bản quyền, không phải gia hạn gì.'],
    ['Không thu thập dữ liệu', 'Dữ liệu trên máy chủ của bạn ở lại trên máy chủ của bạn.'],
    ['Mã nguồn mở', 'AGPLv3. Đọc, sửa và chia sẻ mã nguồn thoải mái.'],
  ],
  ctaTitle: 'Thử ngay trên một máy chủ mới',
  ctaLead: 'Một lệnh duy nhất cài panel, rồi in ra địa chỉ và mật khẩu dùng một lần.',
  github: 'Mã nguồn trên GitHub',
}

const t = computed(() => (lang.value === 'vi' ? VI : EN))

type Tab = 'dashboard' | 'websites' | 'apps' | 'security'
const TABS: Tab[] = ['dashboard', 'websites', 'apps', 'security']
const tab = ref<Tab>('dashboard')
const shot = computed(() =>
  withBase(`/screens/${lang.value === 'vi' ? 'vi' : 'en'}/${tab.value}-${isDark.value ? 'dark' : 'light'}.webp`),
)

const copied = ref('')
async function copy(where: string) {
  try {
    await navigator.clipboard.writeText(INSTALL)
    copied.value = where
    setTimeout(() => (copied.value = ''), 1600)
  } catch {
    // no clipboard (old browser or not allowed): the command stays selectable
  }
}

const doc = (page: string) => withBase(t.value.docs + page)
</script>

<template>
  <div class="nf-home">
    <!-- hero -->
    <section class="hero">
      <div class="wrap">
        <p class="badge"><span class="dot" />{{ t.badge }}</p>
        <h1>
          {{ t.title[0] }}{{ ' ' }}<br class="br" />
          <span class="accent">{{ t.title[1] }}</span>
        </h1>
        <p class="lead">{{ t.lead }}</p>
        <div class="actions">
          <a class="btn brand" :href="doc('install')">{{ t.install }}</a>
          <a class="btn alt" :href="withBase(t.docs)">{{ t.readDocs }}</a>
        </div>
        <div class="cmd" role="group">
          <span class="prompt" aria-hidden="true">#</span>
          <code>{{ INSTALL }}</code>
          <button type="button" class="copy" @click="copy('hero')">
            <Icon :name="copied === 'hero' ? 'check' : 'copy'" :size="16" />
            <span>{{ copied === 'hero' ? t.copied : t.copy }}</span>
          </button>
        </div>
        <p class="os">{{ t.os }}</p>
      </div>
    </section>

    <!-- screenshots -->
    <section class="shots">
      <div class="wrap">
        <div class="tabs" role="tablist">
          <button
            v-for="k in TABS"
            :key="k"
            type="button"
            role="tab"
            :aria-selected="tab === k"
            :class="{ on: tab === k }"
            @click="tab = k"
          >
            {{ t.tabs[k] }}
          </button>
        </div>
        <div class="frame">
          <div class="bar" aria-hidden="true"><i /><i /><i /></div>
          <img :src="shot" :alt="t.shotsAlt + ': ' + t.tabs[tab]" width="1600" height="1000" loading="eager" />
        </div>
      </div>
    </section>

    <!-- numbers -->
    <section class="stats">
      <div class="wrap grid4">
        <div v-for="s in t.stats" :key="s[0]" class="stat">
          <strong>{{ s[0] }}</strong>
          <span>{{ s[1] }}</span>
        </div>
      </div>
    </section>

    <!-- features -->
    <section class="block">
      <div class="wrap">
        <h2>{{ t.featuresTitle }}</h2>
        <p class="sub">{{ t.featuresLead }}</p>
        <div class="cards">
          <a v-for="f in t.features" :key="f[1]" class="card" :href="doc(f[3])">
            <span class="ico"><Icon :name="f[0]" :size="20" /></span>
            <h3>{{ f[1] }}</h3>
            <p>{{ f[2] }}</p>
          </a>
        </div>
      </div>
    </section>

    <!-- security -->
    <section class="block ink">
      <div class="wrap">
        <h2>{{ t.secTitle }}</h2>
        <p class="sub">{{ t.secLead }}</p>
        <div class="secgrid">
          <div v-for="s in t.sec" :key="s[1]" class="secitem">
            <span class="ico"><Icon :name="s[0]" :size="20" /></span>
            <div>
              <h3>{{ s[1] }}</h3>
              <p>{{ s[2] }}</p>
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- free -->
    <section class="block">
      <div class="wrap">
        <h2>{{ t.freeTitle }}</h2>
        <div class="free grid4">
          <div v-for="f in t.free" :key="f[0]" class="freeitem">
            <Icon name="checkCircle" :size="20" />
            <h3>{{ f[0] }}</h3>
            <p>{{ f[1] }}</p>
          </div>
        </div>
      </div>
    </section>

    <!-- call to action -->
    <section class="block cta">
      <div class="wrap">
        <h2>{{ t.ctaTitle }}</h2>
        <p class="sub">{{ t.ctaLead }}</p>
        <div class="cmd">
          <span class="prompt" aria-hidden="true">#</span>
          <code>{{ INSTALL }}</code>
          <button type="button" class="copy" @click="copy('cta')">
            <Icon :name="copied === 'cta' ? 'check' : 'copy'" :size="16" />
            <span>{{ copied === 'cta' ? t.copied : t.copy }}</span>
          </button>
        </div>
        <div class="actions center">
          <a class="btn brand" :href="doc('install')">{{ t.install }}</a>
          <a class="btn alt" :href="GITHUB">{{ t.github }}</a>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.nf-home {
  --w: 1152px;
  color: var(--nf-text);
}
.wrap {
  max-width: var(--w);
  margin: 0 auto;
  padding: 0 24px;
}
@media (max-width: 640px) {
  .wrap {
    padding: 0 16px;
  }
}

/* hero */
.hero {
  padding: 72px 0 40px;
  text-align: center;
}
.badge {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  margin: 0 0 22px;
  padding: 5px 12px;
  border: 1px solid var(--nf-border);
  border-radius: 999px;
  background: var(--nf-surface);
  color: var(--nf-text-2);
  font-size: 13px;
  font-weight: 500;
}
.badge .dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--nf-bright);
  box-shadow: 0 0 0 3px rgb(45 212 191 / 0.2);
}
h1 {
  margin: 0 auto;
  max-width: 880px;
  font-size: clamp(34px, 5.6vw, 60px);
  line-height: 1.08;
  font-weight: 700;
  letter-spacing: -0.03em;
}
h1 .accent {
  color: var(--nf-accent);
}
.lead {
  margin: 22px auto 0;
  max-width: 680px;
  color: var(--nf-text-2);
  font-size: clamp(16px, 2vw, 19px);
  line-height: 1.6;
}
.actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 12px;
  margin-top: 30px;
}
.btn {
  display: inline-flex;
  align-items: center;
  height: 44px;
  padding: 0 22px;
  border-radius: 10px;
  font-size: 15px;
  font-weight: 600;
  text-decoration: none;
  transition: background-color 0.15s, border-color 0.15s, color 0.15s;
}
.btn.brand {
  background: var(--vp-button-brand-bg);
  color: var(--vp-button-brand-text);
}
.btn.brand:hover {
  background: var(--vp-button-brand-hover-bg);
}
.btn.alt {
  border: 1px solid var(--nf-border);
  background: var(--nf-surface);
  color: var(--nf-text);
}
.btn.alt:hover {
  border-color: var(--nf-accent);
  color: var(--nf-accent);
}

/* the install command */
.cmd {
  display: flex;
  align-items: center;
  gap: 12px;
  max-width: 960px;
  border: 1px solid transparent;
  margin: 28px auto 0;
  padding: 10px 10px 10px 18px;
  border-radius: 12px;
  background: var(--nf-ink);
  color: var(--nf-ink-text);
  text-align: left;
  box-shadow: var(--nf-shadow);
}
.cmd .prompt {
  color: var(--nf-bright);
  font-family: var(--vp-font-family-mono);
  font-size: 14px;
}
.cmd code {
  flex: 1;
  min-width: 0;
  font-family: var(--vp-font-family-mono);
  font-size: 13.5px;
  line-height: 1.6;
  overflow-wrap: anywhere;
}
@media (max-width: 760px) {
  .cmd {
    align-items: flex-start;
  }
  .cmd code {
    font-size: 12.5px;
  }
}
.dark .cmd {
  border-color: var(--nf-border);
  background: var(--nf-ink-2);
}
.copy {
  display: inline-flex;
  flex: none;
  align-items: center;
  gap: 6px;
  height: 34px;
  padding: 0 12px;
  border-radius: 8px;
  background: var(--nf-ink-3);
  color: var(--nf-ink-text);
  font-size: 13px;
  font-weight: 500;
}
.copy:hover {
  background: #24324c;
}
.os {
  margin: 14px auto 0;
  color: var(--nf-muted);
  font-size: 13.5px;
}

/* screenshots */
.shots {
  padding: 16px 0 24px;
}
.tabs {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 6px;
  margin-bottom: 18px;
}
.tabs button {
  height: 34px;
  padding: 0 14px;
  border-radius: 999px;
  color: var(--nf-text-2);
  font-size: 14px;
  font-weight: 500;
}
.tabs button:hover {
  color: var(--nf-text);
}
.tabs button.on {
  background: var(--nf-accent-soft);
  color: var(--nf-accent);
}
.frame {
  overflow: hidden;
  border: 1px solid var(--nf-border);
  border-radius: 14px;
  background: var(--nf-surface);
  box-shadow: var(--nf-shadow-lg);
}
.frame .bar {
  display: flex;
  gap: 7px;
  padding: 11px 14px;
  border-bottom: 1px solid var(--nf-border);
  background: var(--nf-surface-2);
}
.frame .bar i {
  width: 11px;
  height: 11px;
  border-radius: 50%;
  background: var(--nf-border);
}
.frame img {
  display: block;
  width: 100%;
  height: auto;
  aspect-ratio: 16 / 10;
  object-fit: cover;
  object-position: top left;
  background: var(--nf-bg);
}

/* numbers */
.stats {
  padding: 40px 0 8px;
}
.grid4 {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 16px;
}
@media (max-width: 860px) {
  .grid4 {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
@media (max-width: 480px) {
  .grid4 {
    grid-template-columns: 1fr;
  }
}
.stat {
  padding: 4px 0 4px 16px;
  border-left: 3px solid var(--nf-bright);
}
.stat strong {
  display: block;
  font-size: 28px;
  font-weight: 700;
  letter-spacing: -0.02em;
  line-height: 1.2;
}
.stat span {
  display: block;
  margin-top: 4px;
  color: var(--nf-text-2);
  font-size: 14px;
  line-height: 1.5;
}

/* sections */
.block {
  padding: 80px 0;
}
.block h2 {
  margin: 0;
  font-size: clamp(26px, 3.6vw, 36px);
  font-weight: 700;
  letter-spacing: -0.02em;
  line-height: 1.2;
  text-align: center;
}
.block .sub {
  max-width: 640px;
  margin: 14px auto 0;
  color: var(--nf-text-2);
  font-size: 16.5px;
  line-height: 1.6;
  text-align: center;
}
h3 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  line-height: 1.35;
}

/* feature cards */
.cards {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px;
  margin-top: 44px;
}
@media (max-width: 960px) {
  .cards {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
@media (max-width: 600px) {
  .cards {
    grid-template-columns: 1fr;
  }
}
.card {
  display: block;
  padding: 22px;
  border: 1px solid var(--nf-border);
  border-radius: 14px;
  background: var(--nf-surface);
  color: inherit;
  text-decoration: none;
  transition: border-color 0.15s, transform 0.15s;
}
.card:hover {
  border-color: var(--nf-accent);
  transform: translateY(-2px);
}
.card .ico,
.secitem .ico {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 38px;
  height: 38px;
  margin-bottom: 14px;
  border-radius: 10px;
  background: var(--nf-accent-soft);
  color: var(--nf-accent);
}
.card p,
.secitem p,
.freeitem p {
  margin: 6px 0 0;
  color: var(--nf-text-2);
  font-size: 14.5px;
  line-height: 1.6;
}

/* security, on the dark ink surface in both themes */
.ink {
  background: var(--nf-ink);
  color: var(--nf-ink-text);
}
.dark .ink {
  border-top: 1px solid var(--nf-border);
  border-bottom: 1px solid var(--nf-border);
  background: #0d1420;
}
.ink h2 {
  color: #fff;
}
.ink .sub,
.ink .secitem p {
  color: var(--nf-ink-muted);
}
.ink h3 {
  color: #eef2f7;
}
.secgrid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 28px 32px;
  margin-top: 48px;
}
@media (max-width: 960px) {
  .secgrid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
@media (max-width: 600px) {
  .secgrid {
    grid-template-columns: 1fr;
  }
}
.secitem {
  display: flex;
  gap: 16px;
}
.secitem .ico {
  flex: none;
  margin: 0;
  background: rgb(45 212 191 / 0.12);
  color: var(--nf-bright);
}

/* free */
.free {
  margin-top: 40px;
}
.freeitem {
  padding: 22px;
  border-radius: 14px;
  background: var(--nf-accent-soft);
  color: var(--nf-accent);
}
.freeitem h3 {
  margin-top: 10px;
  color: var(--nf-text);
}

/* call to action */
.cta {
  padding-top: 24px;
  padding-bottom: 0;
  text-align: center;
}
.actions.center {
  margin-top: 26px;
}
.br {
  display: none;
}
@media (min-width: 720px) {
  .br {
    display: inline;
  }
}
</style>
