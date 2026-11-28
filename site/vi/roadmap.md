---
title: Lộ trình
description: Những việc đang làm trước Nefo 1.0, những gì đang được cân nhắc, và những gì không nằm trong kế hoạch cùng lý do.
---

# Lộ trình

Lộ trình của Nefo không ghi ngày: việc nào chạy ổn thì phát hành. Trước bản 1.0, công sức dồn vào việc làm cho những gì đã có chạy thật chắc chắn: thử trên phần cứng ARM64 thật và với các dịch vụ thật, một máy chủ thử nghiệm chạy thật lâu, báo lỗi từ người dùng bên ngoài, và một đợt rà soát bảo mật độc lập. Một số ý tưởng đang được cân nhắc, và có vài thứ được cố ý để ngoài kế hoạch.

Trang này ghi những việc đang làm, những gì đang được cân nhắc, và những gì Nefo sẽ không làm. Những gì đã phát hành có trong [nhật ký thay đổi](./changelog).

## Hiện tại: hướng tới bản 1.0 {#now-toward-1-0}

Nefo đang ở giai đoạn beta. Trước bản 1.0, công sức chủ yếu dồn vào việc làm cho những gì đã có chạy thật chắc chắn, hơn là thêm tính năng mới:

- **Thử trình cài đặt trên ARM64.** Trình cài đặt, có và không có bước cài phần mềm, đã được thử trên máy chủ Ubuntu 22.04, 24.04, 26.04 và Debian 12, 13 mới cài, chip x86-64. Bản ARM64 đã được phát hành nhưng chưa được thử trên máy ARM thật.
- **Thử với dịch vụ thật.** Sao lưu lên kho tương thích S3 (như AWS S3 và Cloudflare R2) và Google Drive, cảnh báo qua Telegram, Discord, Slack và webhook, gửi mail qua nhà cung cấp SMTP, và API của Cloudflare. Một số trong đó đến nay mới chỉ được thử với dịch vụ giả lập: Google Drive chẳng hạn, mới chỉ được thử với một dịch vụ Google giả lập.
- **Chạy một máy chủ thử nghiệm thật lâu,** với website thật, để những lỗi phải vài tuần mới lộ ra (log phình to, ổ đĩa đầy dần, bộ nhớ tăng) được phát hiện trước khi bạn gặp phải.
- **Người dùng bên ngoài và các báo lỗi của họ.** Mỗi báo lỗi có kèm các bước tái hiện đều có ích. Xem [Báo lỗi](./about#get-help-and-report-bugs).
- **Một đợt rà soát bảo mật độc lập.** Nefo chưa được bên ngoài kiểm toán bảo mật. Cho tới khi có, hãy coi Nefo là phần mềm beta chạy với quyền root, và đọc [chính sách bảo mật](./security-policy).

Cho tới bản 1.0, chỉ bản phát hành mới nhất được vá lỗi bảo mật.

## Đang cân nhắc {#being-considered}

Đây là ý tưởng, không phải lời hứa. Có cái sẽ thay đổi, có cái có thể không bao giờ ra mắt.

- **Sao lưu lên Google Drive mà không cần OAuth client của riêng bạn.** Hiện mỗi máy chủ cần một OAuth client mà bạn tạo một lần trong một dự án Google Cloud (xem [Google Drive](./docs/backups#google-drive)).
- **Địa chỉ thật của khách truy cập khi đứng sau proxy của Cloudflare,** trong log của website và trong PHP trên Nginx, dưới dạng một cài đặt cho từng website. Hiện một website Nginx đứng sau proxy thấy địa chỉ của Cloudflare ở những chỗ đó; tính năng chặn quốc gia thì đã tra đúng địa chỉ của khách truy cập.
- **Giao diện panel nhẹ nhàng, ít rối mắt hơn.**

Panel hiện có bảy ngôn ngữ. Bản dịch sang các ngôn ngữ khác luôn được hoan nghênh: xem [CONTRIBUTING.md](https://github.com/nefocp/nefo/blob/main/CONTRIBUTING.md).

## Không nằm trong kế hoạch, và lý do {#not-planned-and-why}

Nefo cần đủ nhẹ để chạy trên VPS 1 GB RAM, và đủ gọn để mã nguồn có thể được rà soát về bảo mật. Mỗi mục dưới đây sẽ thêm dịch vụ chạy thường trực, mở thêm cổng ra Internet, hoặc thêm rất nhiều mã chạy với quyền root. Vì vậy chúng được cố ý bỏ ra ngoài, và bảng dưới cho biết Nefo có gì để thay thế.

| Không làm | Lý do | Nefo có gì thay thế |
|---|---|---|
| Máy chủ mail | Mail cần nhiều dịch vụ chạy thường trực, bộ lọc thư rác và virus tốn nhiều bộ nhớ, và phải liên tục giữ uy tín cho địa chỉ IP của máy chủ. Nhiều nhà cung cấp VPS còn chặn cổng 25 chiều ra. | SMTP relay qua nhà cung cấp mail của bạn, cho website và cảnh báo. Hộp thư thì hãy dùng nhà cung cấp mail. Xem [Thư và cảnh báo](./docs/mail-alerts). |
| Máy chủ DNS | Name server phải luôn trả lời truy vấn từ Internet, và nên chạy trên nhiều hơn một máy. | Quản lý bản ghi DNS của các zone Cloudflare ngay trong panel, và chứng chỉ wildcard qua DNS của Cloudflare. Xem [DNS (Cloudflare)](./docs/dns). |
| Máy chủ FTP | Thêm một dịch vụ với cổng mở và hệ thống tài khoản riêng. | Mỗi website một tài khoản SFTP, chỉ vào được thư mục của website đó, đăng nhập bằng mật khẩu hoặc khóa SSH. Xem [Truy cập SFTP](./docs/websites#sftp-access). |
| Quản lý nhiều máy chủ từ một panel | Panel điều khiển máy chủ khác cần cài agent và để các máy tin nhau. Một máy bị xâm nhập là kéo theo tất cả. | Mỗi máy chủ một Nefo. Khóa API giúp script của bạn làm việc với từng máy chủ. Xem [Khóa API](./docs/api). |
| Tài khoản đại lý và khách hàng hosting | Tài khoản có giới hạn, gói và hạn mức (quota) riêng là một sản phẩm khác, dành cho người bán hosting. Nefo dành cho người tự vận hành máy chủ. | Một tài khoản quản trị. Muốn cho khách tải tệp lên, hãy tạo tài khoản SFTP cho website của họ. |

Nếu bạn cần một trong những thứ trên, có thể một panel khác sẽ hợp hơn: xem [Nefo và các panel khác](./compare#when-another-panel-fits-you-better).
