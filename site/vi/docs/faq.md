---
title: Câu hỏi thường gặp
description: Trả lời ngắn cho các câu hỏi thường gặp về Nefo, giấy phép, quyền riêng tư, hệ điều hành và tính năng được hỗ trợ.
---

# Câu hỏi thường gặp

Câu trả lời ngắn cho những câu hỏi hay gặp nhất. Mỗi câu trả lời đều có liên kết tới trang chi tiết.

## Nefo có thật sự miễn phí không?

Có. Mọi tính năng đều miễn phí cho tất cả mọi người, kể cả doanh nghiệp, và không có gói trả phí. Nefo là phần mềm tự do theo giấy phép GNU AGPLv3.

Giấy phép thương mại dành cho doanh nghiệp muốn đổi thương hiệu Nefo (white-label), giữ kín phần sửa đổi của mình, hoặc nhúng Nefo vào một sản phẩm độc quyền. Nó không mở khóa thêm tính năng nào. Xem [COMMERCIAL.md](https://github.com/nefocp/nefo/blob/main/COMMERCIAL.md).

## Nefo có gửi dữ liệu đi đâu không?

Không thu thập dữ liệu sử dụng, không gửi thống kê, không kiểm tra bản quyền. Tự bản thân panel chỉ liên hệ Let's Encrypt để xin và gia hạn chứng chỉ, và ở bản phát hành chính thức, tải danh mục kho ứng dụng đã được ký mỗi ngày một lần. Mọi việc khác, như kiểm tra cập nhật, cảnh báo hay sao lưu từ xa, chỉ diễn ra khi bạn yêu cầu hoặc thiết lập. Xem [Quyền riêng tư](./#privacy).

## Nefo hỗ trợ hệ điều hành nào?

Ubuntu 22.04, 24.04 và 26.04, cùng Debian 12 và 13. Trình cài đặt từ chối các hệ điều hành khác. Xem [Yêu cầu hệ thống](./requirements).

## Nefo có chạy trên máy chủ ARM không?

Có. Nefo chạy trên ARM 64-bit (arm64) cũng như x86 64-bit (amd64). Trình cài đặt tự chọn đúng chương trình cho CPU của bạn.

## Dùng được những web server nào?

Nginx, Apache hoặc OpenLiteSpeed, mỗi máy chủ một web server. Bạn chọn khi cài web server:

- Nginx nhẹ nhất.
- Apache đọc được tệp `.htaccess` đi kèm một số ứng dụng PHP.
- OpenLiteSpeed đọc được quy tắc rewrite trong tệp `.htaccess` và có sẵn LiteSpeed Cache.

Với cả ba, PHP đều chạy bằng PHP-FPM. Xem [Website](./websites).

## Sau này có đổi sang web server khác được không?

Được, nhưng chỉ khi máy chủ không còn website nào. Panel từ chối gỡ web server khi nó vẫn đang phục vụ website, vì cấu hình của từng website được viết riêng cho web server đó. Hãy xóa các website, gỡ web server ở trang **Phần mềm**, rồi cài web server kia. Nhớ sao lưu website trước. Xem [Phần mềm](./software).

## Nefo có làm máy chủ email được không?

Không. Nefo không có máy chủ mail: nó không nhận email và không tạo hộp thư. Website và cảnh báo có thể gửi email qua SMTP relay của một nhà cung cấp như Gmail, SendGrid, Mailgun hay Amazon SES. Hãy dùng nhà cung cấp mail cho hộp thư của bạn. Xem [Thư và cảnh báo](./mail-alerts).

## Có quản lý nhiều máy chủ từ một panel được không?

Chưa. Hãy cài Nefo trên từng máy chủ và quản lý riêng từng cái. Quản lý nhiều máy chủ từ một panel nằm trong kế hoạch phát triển.

## Có chuyển website từ panel khác sang được không?

Chưa có công cụ tự động nhập từ panel khác. Hãy chuyển từng website bằng tay: tạo website trong Nefo, tải tệp lên (bằng trình quản lý tệp, tải từ URL hoặc qua SFTP), tạo database rồi nhập tệp SQL. Xem [Website](./websites), [Tệp tin](./files) và [Cơ sở dữ liệu](./databases).

## Có tạo tài khoản đăng nhập riêng cho khách hàng được không?

Không. Mỗi máy chủ chỉ có một tài khoản panel là `admin`. Nefo không có tài khoản riêng cho khách hàng hay đại lý. Muốn cho khách tải tệp lên website của họ, hãy tạo tài khoản SFTP cho website đó; tài khoản này bị giới hạn trong thư mục của website. Xem [Website](./websites).

## Panel có những ngôn ngữ nào?

Tiếng Anh và tiếng Việt. Panel theo ngôn ngữ của trình duyệt, và bạn có thể đổi ở trang đăng nhập hoặc trong menu tài khoản.

Rất hoan nghênh bản dịch mới. Hãy chép `web/src/locales/en.json` thành một tệp đặt tên theo mã ngôn ngữ của bạn, dịch phần giá trị, rồi đăng ký ngôn ngữ trong `web/src/i18n.js`. Xem [CONTRIBUTING.md](https://github.com/nefocp/nefo/blob/main/CONTRIBUTING.md).

## Báo lỗ hổng bảo mật thế nào?

Hãy báo riêng, đừng mở issue công khai. Trong [kho GitHub](https://github.com/nefocp/nefo), mở tab "Security" rồi chọn "Report a vulnerability". Ghi kèm phiên bản (`nefo version`), hệ điều hành và các bước tái hiện lỗi. Xem [SECURITY.md](https://github.com/nefocp/nefo/blob/main/SECURITY.md).

## Làm sao để ủng hộ dự án?

- Báo lỗi kèm các bước tái hiện, hệ điều hành và `nefo version`.
- Dịch panel sang ngôn ngữ của bạn, hoặc cải thiện bản dịch có sẵn.
- Sửa lỗi và gửi pull request (xem [CONTRIBUTING.md](https://github.com/nefocp/nefo/blob/main/CONTRIBUTING.md)).
- Gắn sao cho dự án trên [GitHub](https://github.com/nefocp/nefo) và giới thiệu cho người khác.
- Mua giấy phép thương mại cho công ty của bạn (xem [COMMERCIAL.md](https://github.com/nefocp/nefo/blob/main/COMMERCIAL.md)).
