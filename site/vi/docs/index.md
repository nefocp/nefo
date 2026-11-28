---
title: Nefo là gì?
description: Nefo là panel quản trị miễn phí, mã nguồn mở, cài trên máy chủ Linux của chính bạn để quản lý website, cơ sở dữ liệu, ứng dụng, sao lưu và bảo mật ngay trên trình duyệt.
---

# Nefo là gì?

Nefo là panel quản trị miễn phí, mã nguồn mở (GNU AGPLv3), cài lên máy chủ Linux của chính bạn chỉ bằng một lệnh. Trên trình duyệt, bằng một trong bảy ngôn ngữ, bạn quản lý website và chứng chỉ SSL, cơ sở dữ liệu, ứng dụng, tệp tin, sao lưu và bảo mật. Nefo là một chương trình duy nhất, thiết kế cho VPS 1 GB RAM, và không gửi dữ liệu sử dụng đi đâu cả.

Trang này giới thiệu Nefo làm được gì, dành cho ai, những gì Nefo cố ý không làm, và Nefo gửi gì ra ngoài mạng.

## Một panel gói trong một chương trình {#a-control-panel-in-one-program}

Bạn cài Nefo lên VPS chỉ bằng một lệnh. Nefo là một chương trình duy nhất, `/usr/local/bin/nefo`, có sẵn giao diện web bên trong, và chạy dưới dạng dịch vụ systemd tên `nefo`. Sau đó bạn quản lý máy chủ ngay trên trình duyệt: website, chứng chỉ, cơ sở dữ liệu, tệp tin, sao lưu, phần mềm và bảo mật.

- **Mọi tính năng đều miễn phí.** Không có gói trả phí, không tính phí theo website hay theo tài khoản.
- **Không cần tài khoản, không có máy chủ bản quyền.** Không phải đăng ký hay kích hoạt gì cả.
- **Thiết kế cho máy chủ nhỏ.** Nefo chạy được trên VPS 1 GB RAM, cả x86-64 lẫn ARM64. Bản thân panel dùng khoảng 34 MB bộ nhớ, đo trên máy chủ thử nghiệm sau khi cập nhật lên v0.24.0.
- **Giao diện bảy ngôn ngữ:** tiếng Anh, tiếng Việt, tiếng Trung, tiếng Tây Ban Nha, tiếng Bồ Đào Nha, tiếng Nga và tiếng Indonesia. Bạn chọn ngôn ngữ ở trang đăng nhập hoặc trong menu tài khoản, và trình duyệt chỉ tải ngôn ngữ bạn dùng.

Để bắt đầu, hãy xem [yêu cầu hệ thống](./requirements), rồi [cài đặt Nefo](./install) và làm theo [những việc đầu tiên](./first-steps).

## Nefo dành cho ai {#who-it-is-for}

Nefo dành cho người tự vận hành VPS: người mới dựng máy chủ đầu tiên cho blog, lập trình viên triển khai ứng dụng, hay freelancer và agency chăm sóc website cho khách hàng.

Mỗi máy chủ có một tài khoản panel duy nhất là `admin`. Nefo không có tài khoản riêng cho khách hàng hay đại lý.

## Nefo làm được gì {#what-you-can-do-with-it}

### Website {#websites}

- Chạy website PHP, website tĩnh và website ứng dụng (Node.js, Python hoặc Java) trên Nginx, Apache hoặc OpenLiteSpeed, cùng website reverse proxy chuyển tiếp tới một địa chỉ khác. Mỗi website có một user Linux riêng, và có thể có tài khoản SFTP bị giới hạn trong thư mục của nó. Nhân bản website sang tên miền mới, và xóa website kèm bản sao lưu trước. Xem [Website](./websites).
- Bật HTTPS với chứng chỉ Let's Encrypt miễn phí, tự động gia hạn, chứng chỉ wildcard qua DNS của Cloudflare, hoặc chứng chỉ của riêng bạn. Xem [Chứng chỉ SSL](./ssl).
- Chặn hoặc chỉ cho phép khách truy cập theo quốc gia, và khóa mã nguồn của website ở chế độ chỉ đọc với PHP của nó. Xem [Chặn quốc gia và khóa file](./website-protection).
- Triển khai từ GitHub, GitLab, Gitea hay bất kỳ máy chủ Git nào, khi bạn bấm hoặc mỗi lần push. Xem [Triển khai từ Git](./git-deploy).
- Giữ các website WordPress luôn được cập nhật, có sao lưu database trước mỗi lần cập nhật, chế độ bảo trì và bản staging. Xem [Công cụ WordPress](./wordpress).
- Cài 42 ứng dụng chỉ với một cú nhấp: ứng dụng PHP như WordPress, Joomla, Nextcloud, Laravel và ứng dụng Docker như Ghost, Uptime Kuma, n8n, Gitea, Grafana, Jellyfin. Xem [Kho ứng dụng](./apps).

### Dữ liệu {#data}

- Chạy MariaDB (bản của hệ điều hành, hoặc 11.4, 11.8, 12.3 LTS) hoặc MySQL 8.4 đã tinh chỉnh cho máy chủ nhỏ, với database, người dùng, nhập và xuất SQL, truy cập từ xa cho từng database, và chuyển qua lại giữa hai loại mà vẫn giữ database. Adminer và phpMyAdmin nằm sau trang đăng nhập của panel. Có trình duyệt khóa cho Redis, và cài thêm được Memcached. Xem [Cơ sở dữ liệu](./databases).
- Chạy PostgreSQL, bản của hệ điều hành hoặc 15 đến 19 từ dự án PostgreSQL, với database và chủ sở hữu riêng, truy cập từ xa qua TLS và nâng cấp lên phiên bản lớn mới hơn. Xem [PostgreSQL](./postgresql).
- Nhập website từ bản sao lưu đầy đủ của cPanel, hoặc từ file nén website và bản dump SQL của aaPanel, HestiaCP, CyberPanel hay bất kỳ máy chủ nào. Xem [Nhập website](./import).
- Quản lý tệp tin với trình soạn thảo, tải lên và tải về, nén và giải nén, cùng thùng rác giữ tệp đã xóa trong 30 ngày. Xem [Tệp tin](./files).
- Sao lưu website, database, thư mục và ứng dụng theo lịch, lưu trên máy chủ này, lên kho tương thích S3, máy chủ SFTP hoặc Google Drive, mã hóa bằng passphrase nếu bạn muốn, và khôi phục lại, kể cả trên máy chủ mới. Xem [Sao lưu](./backups) và [Khôi phục trên máy chủ mới](./restore-new-server).

### Máy chủ {#server}

- Cài và gỡ từng thành phần (web server, các phiên bản PHP và ionCube Loader, MariaDB hoặc MySQL, PostgreSQL, Redis, Node.js, Python, Java, Docker…). Nefo từ chối gỡ một thành phần khi website, database hay container vẫn đang dùng nó. Xem [Phần mềm](./software).
- Chạy container Docker và dự án Compose. Nefo kiểm tra tệp Compose để chặn các thiết lập không an toàn, như container đặc quyền (privileged) hay cổng mở ra Internet. Xem [Docker](./docker).
- Quản lý tường lửa (ufw), fail2ban, cài đặt SSH, tường lửa ứng dụng web cho từng website và trình quét mã độc. Xem [Bảo mật](./security).
- Chạy kiểm tra bảo mật: 22 mục kiểm tra nhanh trên máy chủ, mỗi mục có liên kết tới chỗ sửa, chạy khi bạn muốn hoặc hằng ngày. Xem [Kiểm tra bảo mật](./security-check).
- Dùng terminal ngay trên trình duyệt, quản lý dịch vụ, tiến trình, chương trình nền và cron job. Xem [Công cụ hệ thống](./system).
- Theo dõi để biết từng website còn trả lời, kèm thời gian hoạt động và thời gian phản hồi. Xem [Theo dõi](./websites#monitoring).
- Cho website gửi email qua SMTP relay, và nhận cảnh báo qua Telegram, email, Discord, Slack hoặc webhook, cả khi có người đăng nhập. Xem [Thư và cảnh báo](./mail-alerts).
- Quản lý bản ghi DNS của các zone Cloudflare, trỏ tên miền của website về máy chủ này, và thêm bản ghi MX, SPF, DMARC của nhà cung cấp mail. Xem [DNS (Cloudflare)](./dns).

## Bảo mật của chính panel {#security-of-the-panel-itself}

Nefo chạy với quyền root, nên nó bảo vệ kỹ trang đăng nhập của mình:

- Panel chỉ trả lời dưới một đường dẫn bí mật, ví dụ `/k3x9p2qa7m1z/`. Mọi địa chỉ khác trên cổng của panel đều trả về `404`.
- Panel dùng HTTPS ngay từ lần chạy đầu. Lúc đầu là chứng chỉ tự ký, sau đó bạn có thể thay bằng chứng chỉ miễn phí cho tên miền riêng của panel.
- Đăng nhập hỗ trợ xác thực hai lớp. Sau 5 lần đăng nhập sai trong 15 phút, địa chỉ IP đó bị khóa 15 phút.
- Các lần đăng nhập và thay đổi được ghi vào nhật ký hoạt động, giữ trong 90 ngày.

## Những gì Nefo không làm {#what-nefo-doesn-t-do}

- **Không có máy chủ mail.** Nefo không nhận email và không tạo hộp thư. Website và cảnh báo gửi email qua SMTP relay của một nhà cung cấp mail. Hãy dùng nhà cung cấp mail cho hộp thư của bạn.
- **Không có máy chủ DNS hay FTP.** Nefo quản lý bản ghi DNS của tên miền tại Cloudflare, và mỗi website có thể có tài khoản SFTP thay cho FTP.
- **Mỗi panel một máy chủ.** Mỗi máy chủ chạy Nefo riêng. Việc quản lý nhiều máy chủ từ một panel không nằm trong kế hoạch (xem [lộ trình](../roadmap#not-planned-and-why)).
- **Không có tài khoản cho khách hàng.** Mỗi máy chủ chỉ có một tài khoản quản trị.
- **Không nhập mail, DNS hay tài khoản FTP.** Khi nhập từ cPanel, Nefo mang sang website, database, chứng chỉ và cron job. Xem [Những gì không được nhập](./import#what-is-not-imported).
- **Chỉ Ubuntu và Debian.** Ubuntu 22.04, 24.04 và 26.04, cùng Debian 12 và 13. Xem [Yêu cầu hệ thống](./requirements).

## Tình trạng phát triển {#status}

Nefo còn non trẻ. Số phiên bản bắt đầu bằng `0.`, và cho tới phiên bản 1.0, chỉ bản phát hành mới nhất được vá lỗi bảo mật. Bạn xem phiên bản đang dùng ở **Cài đặt** → **Panel**, hoặc bằng lệnh `nefo version` trên máy chủ.

Trình cài đặt đã được thử trên máy chủ Ubuntu 22.04, 24.04, 26.04 và Debian 12, 13 mới cài, trên x86-64. Bản ARM64 chưa được thử trên phần cứng ARM thật. Nefo cũng chưa được bên ngoài kiểm định bảo mật.

Hãy thử Nefo trên một máy chủ có thể cài lại từ đầu trước khi dùng cho website quan trọng, và luôn giữ một bản sao lưu ở ngoài máy chủ. Báo lỗi trên [kho GitHub](https://github.com/nefocp/nefo).

[Nhật ký thay đổi](../changelog) ghi những gì thay đổi trong từng bản phát hành, [lộ trình](../roadmap) ghi những việc đang làm và những gì không nằm trong kế hoạch, còn [chính sách bảo mật](../security-policy) hướng dẫn cách báo lỗ hổng và tự kiểm tra một bản phát hành.

## Giấy phép {#licence}

Nefo là phần mềm tự do theo giấy phép GNU AGPLv3. Bạn được dùng cho mọi mục đích, kể cả thương mại. Nếu bạn cung cấp một bản Nefo đã sửa đổi cho người khác dùng qua mạng, AGPL yêu cầu bạn công bố phần sửa đổi đó.

## Quyền riêng tư {#privacy}

Nefo không thu thập dữ liệu sử dụng, không gửi thống kê và không kiểm tra bản quyền. Giao diện web không tải gì từ trang khác: phông chữ, biểu tượng và script đều nằm sẵn trong chương trình.

Panel chỉ tự kết nối ra ngoài cho những việc sau:

| Khi nào | Tới đâu | Để làm gì |
|---|---|---|
| Khi có chứng chỉ đến hạn (kiểm tra mỗi giờ) | Let's Encrypt | Xin hoặc gia hạn chứng chỉ cho website và cho panel. Tài khoản Let's Encrypt của panel không kèm địa chỉ email. Trừ khi bạn dùng xác thực DNS, panel tải trước một tệp thử qua tên miền của bạn bằng HTTP để kiểm tra tên miền có trỏ về máy chủ này không. |
| Mỗi ngày một lần, ở bản phát hành chính thức | Trang phát hành (releases) của dự án trên GitHub | Tải danh mục của kho ứng dụng đã được ký. Danh mục không có chữ ký hợp lệ của dự án sẽ bị bỏ qua. |

Mọi kết nối khác chỉ diễn ra khi bạn yêu cầu:

- **Cập nhật:** Nefo chỉ kiểm tra phiên bản mới khi bạn bấm **Kiểm tra cập nhật** hoặc chạy `nefo update`. Nefo không bao giờ tự cập nhật. Xem [Cập nhật](./update).
- **Những gì bạn thiết lập:** cảnh báo (Telegram, máy chủ SMTP của bạn, Discord, Slack hoặc webhook của bạn), theo dõi website (chính các website của bạn, qua tên miền, như một người truy cập), sao lưu từ xa (kho S3 hoặc SFTP của bạn, hoặc Google Drive), xác thực DNS và trang DNS (API của Cloudflare), chuyển tiếp mail (nhà cung cấp mail của bạn), và chặn quốc gia, tính năng tải cơ sở dữ liệu quốc gia của DB-IP và danh sách địa chỉ của Cloudflare mỗi tháng một lần khi có website dùng nó.
- **Phần mềm bạn cài:** các gói lấy từ kho của hệ điều hành qua apt, một số thành phần lấy từ chính dự án của chúng, như Node.js từ nodejs.org, các phiên bản PHP khác từ kho packages.sury.org, OpenLiteSpeed từ kho của LiteSpeed, các phiên bản MariaDB và PostgreSQL từ kho của dự án khi bạn chọn chúng, và ionCube Loader từ ionCube. Một số phần mềm đã cài tự kết nối ra ngoài, ví dụ ClamAV tự tải cơ sở dữ liệu virus.

Kiểm tra bảo mật không gửi gì ra ngoài máy chủ. Phần lớn yêu cầu từ panel tự giới thiệu là `Nefo/<version>`. Chúng không chứa thông tin về máy chủ hay website của bạn, ngoài những gì công việc cần, như tên miền trong yêu cầu xin chứng chỉ.
