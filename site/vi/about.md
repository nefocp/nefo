---
title: Giới thiệu Nefo
description: Nefo là gì, dành cho ai, được làm theo những nguyên tắc nào, tình trạng và giấy phép, cùng cách nhờ trợ giúp hoặc báo lỗi.
---

# Giới thiệu Nefo

Nefo là panel quản trị miễn phí, mã nguồn mở, dành cho một máy chủ Linux. Bạn cài Nefo lên VPS của mình bằng một lệnh, rồi quản lý website, chứng chỉ, cơ sở dữ liệu, ứng dụng, tệp tin, sao lưu và bảo mật ngay trên trình duyệt, bằng một trong bảy ngôn ngữ. Nefo đang ở giai đoạn beta, theo giấy phép GNU AGPLv3, và được làm cho VPS 1 GB RAM.

Nefo là một chương trình duy nhất, `/usr/local/bin/nefo`, có sẵn giao diện web bên trong, và chạy dưới dạng dịch vụ systemd tên `nefo`. Nefo chạy trên Ubuntu 22.04, 24.04, 26.04 và Debian 12, 13, cả x86-64 lẫn ARM64. Kho ứng dụng của Nefo có 42 ứng dụng, và giao diện có tiếng Anh, tiếng Việt, tiếng Trung, tiếng Tây Ban Nha, tiếng Bồ Đào Nha, tiếng Nga và tiếng Indonesia. Những gì Nefo làm được có ở trang [Nefo là gì?](./docs/).

## Nefo dành cho ai {#who-it-is-for}

Nefo dành cho người tự vận hành VPS: người mới dựng máy chủ đầu tiên cho blog, lập trình viên triển khai ứng dụng, hay freelancer và agency chăm sóc website cho khách hàng.

Mỗi máy chủ có một tài khoản panel duy nhất là `admin`. Nefo không làm ra để kinh doanh hosting: không có tài khoản đại lý hay tài khoản khách hàng. Trang [Nefo và các panel khác](./compare) cho biết khi nào một panel khác hợp với bạn hơn.

## Nguyên tắc {#principles}

- **Nhẹ.** Nefo được làm cho VPS 1 GB RAM, và bản thân panel dùng khoảng {{ $release.ram }} bộ nhớ (đo trên một máy chủ thử nghiệm chạy v{{ $release.version }}). Nefo không cài gì cho tới khi bạn chọn, thành phần nào cũng gỡ ra được, và những tính năng buộc dịch vụ lớn phải chạy thường trực, như máy chủ mail, được để ra ngoài.
- **Mọi tính năng đều miễn phí.** Không có bản trả phí, không khóa tính năng nào. Bạn không phải tạo tài khoản, và không có khóa bản quyền.
- **Không gửi gì về.** Không thu thập dữ liệu sử dụng, không gửi thống kê, không kiểm tra bản quyền. Giao diện không tải gì từ trang khác. Tự nó, panel chỉ kết nối tới Let's Encrypt để lấy chứng chỉ và tới trang phát hành của dự án trên GitHub để tải danh mục kho ứng dụng đã ký, và chỉ kiểm tra cập nhật khi bạn yêu cầu. Xem [Quyền riêng tư](./docs/#privacy).
- **Nói rõ giới hạn của mình.** Tài liệu ghi rõ Nefo không làm gì, từ chối làm gì và vì sao. Trang [lộ trình](./roadmap) ghi những gì không nằm trong kế hoạch rõ ràng như những gì sắp làm.

## Tình trạng {#status}

Nefo đang ở giai đoạn beta. Bản phát hành công khai đầu tiên, v0.20.0, ra ngày 26/11/2026, và bản hiện tại là v{{ $release.version }} ({{ $release.date }}). Xem [nhật ký thay đổi](./changelog).

- Số phiên bản bắt đầu bằng `0.`. Cho tới bản 1.0, chỉ bản phát hành mới nhất được vá lỗi bảo mật.
- Nefo chưa được bên ngoài kiểm toán bảo mật. Xem [chính sách bảo mật](./security-policy).
- Trình cài đặt đã được thử trên máy chủ Ubuntu 22.04, 24.04, 26.04 và Debian 12, 13 mới cài, chip x86-64. Bản ARM64 chưa được thử trên máy ARM thật.

Hãy thử Nefo trên một máy chủ có thể cài lại từ đầu trước khi dùng cho website quan trọng, và luôn giữ bản sao lưu ở ngoài máy chủ.

## Giấy phép {#licence}

Nefo là phần mềm tự do theo giấy phép [GNU AGPLv3](https://github.com/nefocp/nefo/blob/main/LICENSE). Bạn được dùng Nefo cho mọi mục đích, trên bao nhiêu máy chủ tùy ý, và được đọc, sửa, chia sẻ mã nguồn. Nếu bạn cung cấp một bản Nefo đã sửa đổi cho người khác dùng qua mạng, AGPL yêu cầu bạn công bố phần sửa đổi đó.

Mã nguồn nằm trên [GitHub](https://github.com/nefocp/nefo).

## Trợ giúp và báo lỗi {#get-help-and-report-bugs}

- **Đọc tài liệu trước.** Trang [Xử lý sự cố](./docs/troubleshooting) gom các lỗi hay gặp, còn [Câu hỏi thường gặp](./docs/faq) gom các thắc mắc hay gặp.
- **Báo lỗi** ở [GitHub Issues](https://github.com/nefocp/nefo/issues). Hãy ghi phiên bản (`nefo version`), hệ điều hành, các bước dẫn tới lỗi, kết quả bạn mong đợi và điều đã xảy ra. Nếu chính panel bị lỗi, gửi kèm phần cuối log của nó: `journalctl -u nefo -n 50 --no-pager`.
- **Đừng để lộ thông tin bí mật** trong những gì bạn đăng: đường dẫn bí mật của panel, mật khẩu, khóa API và tên miền riêng tư.
- **Lỗi bảo mật thì báo riêng,** đừng mở issue công khai. Xem [chính sách bảo mật](./security-policy#report-a-vulnerability).

## Đóng góp {#contribute}

- **Báo lỗi** như hướng dẫn ở trên. Trước bản 1.0, báo lỗi từ máy chủ thật là thứ Nefo cần nhất.
- **Dịch giao diện panel.** Chép `web/src/locales/en.json` thành tệp mang mã ngôn ngữ của bạn, dịch phần giá trị (không dịch khóa), rồi khai báo ngôn ngữ đó trong `web/src/i18n.js`.
- **Sửa lỗi và gửi pull request.** Hãy đọc [CONTRIBUTING.md](https://github.com/nefocp/nefo/blob/main/CONTRIBUTING.md) trước. Nefo chạy với quyền root trên máy chủ của người dùng, nên mọi thay đổi đều được xem xét kỹ về bảo mật, và các quy tắc bảo mật cho mã trong tệp đó là bắt buộc. Với thay đổi lớn, hãy mở issue trước để thống nhất cách làm.
- **Ký CLA.** Người đóng góp ký [Thỏa thuận cấp phép cho người đóng góp](https://github.com/nefocp/nefo/blob/main/CLA.md) (CLA) một lần, khi mở pull request đầu tiên. Bạn vẫn giữ bản quyền đối với phần đóng góp của mình.
