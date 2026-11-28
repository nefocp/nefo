---
title: Câu hỏi thường gặp
description: Trả lời ngắn cho các câu hỏi thường gặp về Nefo, control panel miễn phí cho máy chủ Linux, về chuyện có mất phí không, giấy phép, hệ điều hành hỗ trợ, RAM, cơ sở dữ liệu, Docker, chuyển website, an toàn, quyền riêng tư, cập nhật, gỡ cài đặt và ngôn ngữ.
---

# Câu hỏi thường gặp

Câu trả lời ngắn cho những câu hỏi hay gặp nhất về Nefo, control panel miễn phí, mã nguồn mở cho một máy chủ Linux. Mỗi mục bắt đầu bằng câu trả lời ngắn, sau đó là chi tiết và liên kết tới trang hướng dẫn đầy đủ. Các thông tin ở đây đúng với Nefo v0.24.1.

## Nefo là gì? {#what-is-nefo}

Nefo là control panel miễn phí, mã nguồn mở, cài lên máy chủ Linux của chính bạn (một VPS) bằng một lệnh, để quản lý website, chứng chỉ SSL, cơ sở dữ liệu, ứng dụng, file, sao lưu và bảo mật ngay trên trình duyệt. Nefo là một chương trình duy nhất, `/usr/local/bin/nefo`, có sẵn giao diện web bên trong, và chạy dưới dạng dịch vụ systemd `nefo`. Xem [Nefo là gì?](./).

## Nefo có miễn phí không? {#is-nefo-free}

Có. Nefo là phần mềm tự do theo giấy phép GNU AGPLv3, và mọi tính năng đều miễn phí cho tất cả mọi người, kể cả doanh nghiệp. Không có bản trả phí, không có tính năng bị khóa, không có license key và không cần tạo tài khoản. Xem [Giới thiệu](../about#licence).

## Nefo chạy trên hệ điều hành nào? {#what-does-nefo-run-on}

Nefo chạy trên Ubuntu 22.04, 24.04 và 26.04 và Debian 12 và 13, trên x86 64-bit (amd64) và ARM 64-bit (arm64), cần quyền root và systemd. Trình cài đặt đã được thử trên máy chủ mới cài của từng hệ điều hành này, trên x86-64. Bản arm64 chưa được thử trên phần cứng ARM thật, nên hãy thử trước trên một máy chủ có thể cài lại từ đầu. Trên hệ điều hành khác, trình cài đặt dừng lại trừ khi bạn ép nó chạy. Xem [Yêu cầu hệ thống](./requirements).

## Nefo cần bao nhiêu RAM? {#how-much-ram-does-nefo-need}

Bản thân panel dùng khoảng 34 MB bộ nhớ, đo trên một máy chủ thử nghiệm sau khi cập nhật lên v0.24.0, và Nefo được làm cho VPS có 1 GB RAM. Những gì bạn cài sẽ cộng thêm vào đó, và trang **Phần mềm** cho biết mức ước tính: Nginx khoảng 15 MB, mỗi phiên bản PHP khoảng 25 MB, MariaDB bằng buffer pool cộng khoảng 100 MB (khoảng 228 MB trên máy chủ 1 GB). Trên máy chủ 1 GB, hãy thêm 1 đến 2 GB swap. Xem [Yêu cầu hệ thống](./requirements#memory-and-disk).

## Nefo có hỗ trợ MySQL, PostgreSQL và Docker không? {#does-nefo-support-mysql-postgresql-and-docker}

Có. Nefo chạy MariaDB (bản của hệ điều hành, hoặc 11.4, 11.8 hay 12.3 LTS) hoặc MySQL 8.4 LTS, mỗi máy chủ một loại, và có thể chuyển qua lại giữa hai loại này cùng với các database của bạn. PostgreSQL (bản của hệ điều hành, hoặc 15 đến 19 từ dự án PostgreSQL) chạy song song. Docker chạy container và dự án Compose, cùng các ứng dụng Docker của Kho ứng dụng. Xem [Cơ sở dữ liệu](./databases), [PostgreSQL](./postgresql) và [Docker](./docker).

## Dùng được những web server nào? {#which-web-servers-can-i-use}

Nginx, Apache hoặc OpenLiteSpeed, mỗi máy chủ một web server. Bạn chọn khi cài web server:

- Nginx nhẹ nhất.
- Apache đọc được tệp `.htaccess` đi kèm một số ứng dụng PHP.
- OpenLiteSpeed đọc được quy tắc rewrite trong tệp `.htaccess` và có sẵn LiteSpeed Cache.

Với cả ba, PHP đều chạy bằng PHP-FPM, mỗi website một pool. Website cũng có thể là ứng dụng Node.js, Python hoặc Java, hoặc reverse proxy tới một địa chỉ khác. Xem [Website](./websites).

## Sau này có đổi sang web server khác được không? {#can-i-switch-to-another-web-server-later}

Được, nhưng chỉ khi máy chủ không còn website nào. Panel từ chối gỡ web server khi nó vẫn đang phục vụ website, vì cấu hình của từng website được viết riêng cho web server đó. Hãy xóa các website, gỡ web server ở trang **Phần mềm**, rồi cài web server kia. Nhớ sao lưu website trước. Xem [Phần mềm](./software#web-server).

## Có chuyển website từ cPanel hay aaPanel sang được không? {#can-i-move-my-websites-from-another-panel}

Có. Nefo nhập được bản sao lưu đầy đủ của cPanel (website, database cùng user và mật khẩu, chứng chỉ, cron job), và file nén website kèm bản dump SQL từ aaPanel, HestiaCP, CyberPanel hay bất kỳ máy chủ nào khác. Mail, vùng DNS và tài khoản FTP không được nhập. Xem [Nhập website](./import).

## Nefo có an toàn không? {#is-nefo-safe}

Nefo được làm để an toàn ngay từ mặc định, nhưng đây là phần mềm beta chạy bằng root và chưa được kiểm định bảo mật độc lập. Panel chỉ trả lời dưới một đường dẫn bí mật trên một cổng ngẫu nhiên, dùng HTTPS ngay từ lần chạy đầu, hỗ trợ đăng nhập hai lớp, khóa một địa chỉ sau 5 lần đăng nhập sai trong 15 phút, và ghi lại mọi thay đổi. Các bản phát hành được ký bằng Ed25519, và [kiểm tra bảo mật](./security-check) có sẵn liệt kê những cài đặt đang để máy chủ hở. Hãy thử trước trên một máy chủ có thể cài lại từ đầu, và giữ bản sao lưu ở ngoài máy chủ. Xem [Bảo mật](./security) và [chính sách bảo mật](../security-policy).

## Nefo có gửi dữ liệu đi đâu không? {#does-nefo-send-data-anywhere}

Không thu thập dữ liệu sử dụng, không gửi thống kê, không kiểm tra bản quyền. Tự bản thân panel chỉ liên hệ Let's Encrypt để xin và gia hạn chứng chỉ, và ở bản phát hành chính thức, tải danh mục kho ứng dụng đã được ký mỗi ngày một lần. Mọi việc khác, như kiểm tra cập nhật, cảnh báo, sao lưu từ xa hay cơ sở dữ liệu quốc gia dùng để chặn theo quốc gia, chỉ diễn ra khi bạn yêu cầu hoặc thiết lập. Xem [Quyền riêng tư](./#privacy).

## Nefo có sao lưu lên Google Drive được không? {#can-nefo-back-up-to-google-drive}

Có, từ v0.24.0, bằng một OAuth client của riêng bạn, chỉ thấy được các file do panel tạo. Nefo còn sao lưu lên kho lưu trữ tương thích S3 (Amazon S3, Cloudflare R2, Backblaze B2, Wasabi, MinIO) và máy chủ SFTP, có thể mã hóa mọi bản sao từ xa bằng một passphrase, và một máy chủ mới có thể tìm thấy các bản sao lưu trên kho lưu trữ rồi khôi phục chúng. Xem [Sao lưu](./backups#google-drive) và [Khôi phục trên máy chủ mới](./restore-new-server).

## Cập nhật Nefo thế nào? {#how-do-i-update-nefo}

Mở **Cài đặt** → **Panel** và bấm **Kiểm tra cập nhật**, hoặc chạy `nefo update` bằng root. Bản cập nhật kiểm tra chữ ký và checksum của bản phát hành trước khi thay chương trình, rồi khởi động lại panel; các website vẫn chạy. Nefo không bao giờ tự cập nhật. Xem [Cập nhật](./update).

## Gỡ Nefo thế nào? {#how-do-i-uninstall-nefo}

Nefo không có lệnh gỡ cài đặt: bạn gỡ nó bằng tay. Mở khóa các website đang bị khóa bằng `nefo unlock-files`, dừng và gỡ dịch vụ `nefo` cùng `/usr/local/bin/nefo`, đóng cổng của panel rồi xóa `/etc/nefo` và `/var/lib/nefo`. Website và phần mềm của bạn vẫn chạy, trừ khi bạn gỡ chúng trước. Xem [Gỡ cài đặt](./uninstall).

## Nefo có làm máy chủ email được không? {#can-nefo-host-my-e-mail}

Không. Nefo không có máy chủ mail: nó không nhận email và không tạo hộp thư. Website và cảnh báo có thể gửi email qua SMTP relay của một nhà cung cấp như Gmail, SendGrid, Mailgun hay Amazon SES, và trang DNS có thể thêm các bản ghi MX, SPF và DMARC của nhà cung cấp mail của bạn. Hãy dùng nhà cung cấp mail cho hộp thư của bạn. Xem [Thư và cảnh báo](./mail-alerts).

## Có quản lý nhiều máy chủ từ một panel được không? {#can-i-manage-several-servers-from-one-panel}

Không. Hãy cài Nefo trên từng máy chủ và quản lý riêng từng cái. Việc quản lý nhiều máy chủ từ một panel không nằm trong kế hoạch: panel điều khiển máy chủ khác cần cài agent và để các máy tin nhau. Xem [lộ trình](../roadmap#not-planned-and-why).

## Có tạo tài khoản đăng nhập riêng cho khách hàng được không? {#can-i-give-my-clients-their-own-login}

Không. Mỗi máy chủ chỉ có một tài khoản panel là `admin`. Nefo không có tài khoản riêng cho khách hàng hay đại lý. Muốn cho khách tải tệp lên website của họ, hãy tạo tài khoản SFTP cho website đó; tài khoản này bị giới hạn trong thư mục của website. Xem [Website](./websites#sftp-access).

## Panel có những ngôn ngữ nào? {#which-languages-does-the-panel-speak}

Bảy ngôn ngữ, từ v0.24.0: tiếng Anh, tiếng Việt, tiếng Trung (giản thể), tiếng Tây Ban Nha, tiếng Bồ Đào Nha, tiếng Nga và tiếng Indonesia, kể cả nội dung của Kho ứng dụng. Panel theo ngôn ngữ của trình duyệt, và bạn có thể đổi ở trang đăng nhập hoặc trong menu tài khoản. Mỗi ngôn ngữ là một file riêng, trình duyệt chỉ tải về khi dùng tới. Tài liệu này cũng có đủ bảy ngôn ngữ đó.

Rất hoan nghênh bản dịch mới. Hãy chép `web/src/locales/en.json` thành một tệp đặt tên theo mã ngôn ngữ của bạn, dịch phần giá trị, rồi đăng ký ngôn ngữ trong `web/src/i18n.js`. Xem [CONTRIBUTING.md](https://github.com/nefocp/nefo/blob/main/CONTRIBUTING.md).

## Báo lỗ hổng bảo mật thế nào? {#how-do-i-report-a-security-problem}

Hãy báo riêng, đừng mở issue công khai. Trong [kho GitHub](https://github.com/nefocp/nefo), mở tab "Security" rồi chọn "Report a vulnerability". Ghi kèm phiên bản (`nefo version`), hệ điều hành và các bước tái hiện lỗi. Xem [chính sách bảo mật](../security-policy), được viết theo [SECURITY.md](https://github.com/nefocp/nefo/blob/main/SECURITY.md).

## Có nhật ký thay đổi và lộ trình không? {#is-there-a-changelog-and-a-roadmap}

Có. [Nhật ký thay đổi](../changelog) ghi những gì thay đổi trong từng bản phát hành, mới nhất ở trên, và những điều cần biết trước khi cập nhật. [Lộ trình](../roadmap) ghi những việc đang làm trước bản 1.0, những gì đang được cân nhắc, và những gì không nằm trong kế hoạch cùng lý do. Muốn tự kiểm tra chữ ký của một bản phát hành trước khi cài, xem [Tự kiểm tra một bản phát hành](../security-policy#check-a-release-yourself).

## Tôi có thể giúp dự án bằng cách nào? {#how-can-i-support-the-project}

- Báo lỗi kèm các bước tái hiện, hệ điều hành và `nefo version`.
- Dịch panel sang ngôn ngữ của bạn, hoặc cải thiện bản dịch có sẵn.
- Sửa lỗi và gửi pull request (xem [CONTRIBUTING.md](https://github.com/nefocp/nefo/blob/main/CONTRIBUTING.md)).
- Gắn sao cho dự án trên [GitHub](https://github.com/nefocp/nefo) và giới thiệu cho người khác.
