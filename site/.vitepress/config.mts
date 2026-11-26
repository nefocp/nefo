import { defineConfig, type DefaultTheme } from 'vitepress'

const GITHUB = 'https://github.com/nefocp/nefo'

// One list of pages, two languages. The file name is the same in both;
// Vietnamese pages live under /vi/.
type Page = [file: string, en: string, vi: string]
type Group = { en: string; vi: string; pages: Page[] }

const DOCS: Group[] = [
  {
    en: 'Getting started',
    vi: 'Bắt đầu',
    pages: [
      ['', 'What is Nefo?', 'Nefo là gì?'],
      ['requirements', 'Requirements', 'Yêu cầu hệ thống'],
      ['install', 'Install', 'Cài đặt'],
      ['first-steps', 'First steps', 'Những việc đầu tiên'],
      ['cli', 'Command line', 'Dòng lệnh'],
      ['update', 'Update', 'Cập nhật'],
      ['uninstall', 'Uninstall', 'Gỡ cài đặt'],
    ],
  },
  {
    en: 'Websites',
    vi: 'Website',
    pages: [
      ['websites', 'Websites', 'Website'],
      ['ssl', 'SSL certificates', 'Chứng chỉ SSL'],
      ['git-deploy', 'Deploy from Git', 'Triển khai từ Git'],
      ['wordpress', 'WordPress toolkit', 'Công cụ WordPress'],
      ['apps', 'App store', 'Kho ứng dụng'],
    ],
  },
  {
    en: 'Data',
    vi: 'Dữ liệu',
    pages: [
      ['databases', 'Databases', 'Cơ sở dữ liệu'],
      ['files', 'Files', 'Tệp tin'],
      ['backups', 'Backups', 'Sao lưu'],
    ],
  },
  {
    en: 'Server',
    vi: 'Máy chủ',
    pages: [
      ['software', 'Software', 'Phần mềm'],
      ['docker', 'Docker', 'Docker'],
      ['security', 'Security', 'Bảo mật'],
      ['os', 'System', 'Hệ thống'],
      ['system', 'System tools', 'Công cụ hệ thống'],
      ['mail-alerts', 'Mail and alerts', 'Thư và cảnh báo'],
    ],
  },
  {
    en: 'Help',
    vi: 'Trợ giúp',
    pages: [
      ['troubleshooting', 'Troubleshooting', 'Xử lý sự cố'],
      ['faq', 'FAQ', 'Câu hỏi thường gặp'],
    ],
  },
]

function sidebar(lang: 'en' | 'vi'): DefaultTheme.SidebarItem[] {
  const base = lang === 'vi' ? '/vi/docs/' : '/docs/'
  return DOCS.map((g) => ({
    text: g[lang],
    items: g.pages.map(([file, en, vi]) => ({
      text: lang === 'vi' ? vi : en,
      link: base + file,
    })),
  }))
}

// The favicon inline, so it works on every page without another request.
const FAVICON =
  "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'%3E%3Crect width='100' height='100' rx='24' fill='%232dd4bf'/%3E%3Cg transform='translate(13 13) scale(3.08333)' fill='none' stroke='%2304201c' stroke-width='3.1' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpath d='M5.6 17.7V11.7a5.2 5.2 0 0 1 10.4 0v2.6a3.4 3.4 0 0 0 3.4 3.4'/%3E%3C/g%3E%3C/svg%3E"

export default defineConfig({
  title: 'Nefo',
  titleTemplate: ':title · Nefo',
  description:
    'A free, open-source control panel for your Linux server. Websites, databases, apps, backups and security in one light binary.',
  cleanUrls: true,
  lastUpdated: false,
  sitemap: { hostname: 'https://nefo.io' },

  head: [
    ['link', { rel: 'icon', type: 'image/svg+xml', href: FAVICON }],
    ['meta', { name: 'theme-color', content: '#0b7a6e' }],
    ['meta', { property: 'og:site_name', content: 'Nefo' }],
    ['meta', { property: 'og:type', content: 'website' }],
  ],

  locales: {
    root: {
      label: 'English',
      lang: 'en',
      themeConfig: {
        nav: [
          { text: 'Docs', link: '/docs/', activeMatch: '^/docs/' },
          { text: 'Install', link: '/docs/install' },
          { text: 'Releases', link: GITHUB + '/releases' },
        ],
        sidebar: { '/docs/': sidebar('en') },
        editLink: {
          pattern: GITHUB + '/edit/main/site/:path',
          text: 'Edit this page on GitHub',
        },
        footer: {
          message: 'Free software under the GNU AGPLv3.',
          copyright: '© 2026 the Nefo contributors',
        },
      },
    },
    vi: {
      label: 'Tiếng Việt',
      lang: 'vi',
      link: '/vi/',
      description:
        'Bảng điều khiển máy chủ Linux miễn phí, mã nguồn mở. Website, cơ sở dữ liệu, ứng dụng, sao lưu và bảo mật trong một chương trình nhẹ.',
      themeConfig: {
        nav: [
          { text: 'Tài liệu', link: '/vi/docs/', activeMatch: '^/vi/docs/' },
          { text: 'Cài đặt', link: '/vi/docs/install' },
          { text: 'Phiên bản', link: GITHUB + '/releases' },
        ],
        sidebar: { '/vi/docs/': sidebar('vi') },
        editLink: {
          pattern: GITHUB + '/edit/main/site/:path',
          text: 'Sửa trang này trên GitHub',
        },
        footer: {
          message: 'Phần mềm tự do theo giấy phép GNU AGPLv3.',
          copyright: '© 2026 những người đóng góp cho Nefo',
        },
        outline: { label: 'Trong trang này' },
        docFooter: { prev: 'Trang trước', next: 'Trang sau' },
        darkModeSwitchLabel: 'Giao diện',
        lightModeSwitchTitle: 'Chuyển sang sáng',
        darkModeSwitchTitle: 'Chuyển sang tối',
        sidebarMenuLabel: 'Danh mục',
        returnToTopLabel: 'Lên đầu trang',
        langMenuLabel: 'Đổi ngôn ngữ',
        notFound: {
          title: 'KHÔNG TÌM THẤY TRANG',
          quote: 'Trang này không tồn tại hoặc đã được chuyển đi.',
          linkLabel: 'về trang chủ',
          linkText: 'Về trang chủ',
        },
      },
    },
  },

  themeConfig: {
    logo: { light: '/mark-light.svg', dark: '/mark-dark.svg', alt: '' },
    socialLinks: [{ icon: 'github', link: GITHUB }],
    search: {
      provider: 'local',
      options: {
        locales: {
          vi: {
            translations: {
              button: { buttonText: 'Tìm kiếm', buttonAriaLabel: 'Tìm kiếm' },
              modal: {
                noResultsText: 'Không có kết quả cho',
                resetButtonTitle: 'Xóa',
                displayDetails: 'Hiện chi tiết',
                footer: {
                  selectText: 'chọn',
                  navigateText: 'di chuyển',
                  closeText: 'đóng',
                },
              },
            },
          },
        },
      },
    },
  },
})
