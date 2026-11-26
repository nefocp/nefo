---
title: Thư và cảnh báo
description: Cho website gửi email qua nhà cung cấp SMTP, và nhận cảnh báo qua Telegram hoặc email khi máy chủ cần bạn chú ý.
---

# Thư và cảnh báo

Trang này nói về hai việc: cách website của bạn gửi email qua một nhà cung cấp SMTP, và cách Nefo báo cho bạn qua Telegram hoặc email khi máy chủ có chuyện cần xử lý.

## Không có mail server, và đó là chủ ý

Nefo không tạo hộp thư và không nhận email. Một mail server đầy đủ tốn bộ nhớ và cần chăm sóc liên tục, không hợp với VPS nhỏ. Hãy dùng nhà cung cấp email cho hộp thư của bạn.

Để gửi thư đi, Nefo cài Postfix làm relay chỉ gửi. Postfix chỉ nghe trên chính máy chủ này và chuyển mọi thư cho nhà cung cấp của bạn (Gmail, SendGrid, Amazon SES và các nhà cung cấp khác), đăng nhập bằng tài khoản bạn nhập vào.

## Gửi email từ website

Khi đã thiết lập relay, hàm `mail()` của PHP sẽ hoạt động, vì Postfix cung cấp `/usr/sbin/sendmail` mà PHP dùng để gửi thư. Form liên hệ, email quên mật khẩu và thông báo của WordPress gửi được mà không cần plugin. Nếu chưa có relay, website không gửi được email.

### Thiết lập relay

1. Vào **Cài đặt** → **Email**.
2. Chọn **Nhà cung cấp**. Panel tự điền **Máy chủ SMTP**, **Cổng** và **Mã hóa** cho nhà cung cấp đó.
3. Nhập **Tên đăng nhập** và **Mật khẩu** của tài khoản bên nhà cung cấp. Dòng gợi ý dưới ô **Nhà cung cấp** cho biết nhà cung cấp đó cần gì.
4. Có thể điền **Gửi mọi thư với địa chỉ (tùy chọn)**, ví dụ `noreply@example.com`.
5. Lần đầu, chọn **Cài đặt và lưu**: panel cài Postfix và cấu hình nó. Những lần sau, nút sẽ là **Lưu**.

Việc thiết lập chạy nền, có log trực tiếp. Khi xong, hãy gửi thư thử (xem bên dưới).

| Nhà cung cấp | Máy chủ SMTP | Tên đăng nhập và mật khẩu |
|---|---|---|
| Gmail / Google Workspace | `smtp.gmail.com` | Địa chỉ Gmail, và mật khẩu ứng dụng (Tài khoản Google → Bảo mật → Xác minh 2 bước → Mật khẩu ứng dụng), không phải mật khẩu thường |
| Microsoft 365 / Outlook | `smtp.office365.com` | Địa chỉ Microsoft 365. Hộp thư phải được phép gửi qua SMTP |
| SendGrid | `smtp.sendgrid.net` | `apikey` (panel điền sẵn), và một API key của SendGrid có quyền Mail Send. Gửi từ địa chỉ đã xác minh |
| Mailgun | `smtp.mailgun.org` | Thông tin SMTP của tên miền trong Mailgun |
| Amazon SES | `email-smtp.us-east-1.amazonaws.com` | Thông tin SMTP của SES. Đổi máy chủ thành endpoint theo region của bạn, và gửi từ địa chỉ hoặc tên miền đã xác minh |
| Brevo | `smtp-relay.brevo.com` | SMTP login và SMTP key của Brevo |
| Máy chủ SMTP khác | Máy chủ của nhà cung cấp | Cổng 587 với STARTTLS, hoặc 465 với SSL/TLS. Cổng 25 bị chặn ở hầu hết nhà cung cấp cloud |

Mọi lựa chọn có sẵn đều dùng cổng 587 với STARTTLS.

### Các ô và quy tắc

| Ô | Nhập gì |
|---|---|
| **Máy chủ SMTP** | Tên máy chủ của nhà cung cấp, hoặc một địa chỉ IP |
| **Cổng** | Từ 1 đến 65535, thường là 587 hoặc 465 |
| **Mã hóa** | **STARTTLS** (cổng 587), **SSL/TLS** (cổng 465), hoặc **Không (chỉ trên server này)** |
| **Tên đăng nhập**, **Mật khẩu** | Nhập cả hai hoặc bỏ trống cả hai, không có khoảng trắng. Để trống **Mật khẩu** để giữ mật khẩu đã lưu |
| **Gửi mọi thư với địa chỉ (tùy chọn)** | Một địa chỉ email thay cho người gửi của mọi thư |

- **Không (chỉ trên server này)** chỉ được chấp nhận khi relay nằm trên chính máy chủ này (`localhost`, `127.0.0.1` hoặc `::1`). Nếu không mã hóa, mật khẩu sẽ đi qua internet ở dạng rõ.
- Với STARTTLS hoặc SSL/TLS, Postfix sẽ không gửi nếu không mã hóa được kết nối.
- Dùng **Gửi mọi thư với địa chỉ (tùy chọn)** khi nhà cung cấp chỉ nhận địa chỉ người gửi đã xác minh. Địa chỉ này thay người gửi của mọi thư, từ mọi website.
- Mật khẩu của nhà cung cấp được lưu trong `/etc/postfix/sasl_passwd`, chỉ root đọc được. Website không bao giờ thấy nó.

### Gửi thư thử

1. Bên dưới form, nhập một địa chỉ vào ô **Gửi thư thử đến…**.
2. Chọn **Gửi thử**.

Panel theo dõi thư trong log mail và chờ nhà cung cấp trả lời tối đa một phút. Kết quả cho biết thư đã được gửi, chưa gửi được (Postfix sẽ tiếp tục thử lại) hay bị nhà cung cấp từ chối, kèm câu trả lời của nhà cung cấp. Nếu nhà cung cấp không trả lời trong một phút, thư vẫn nằm trong hàng đợi và Postfix tiếp tục thử gửi.

### Kiểm tra relay

Ở đầu khung cài đặt:

- **Đang chuyển tiếp qua …** nghĩa là Postfix đang chạy và dùng nhà cung cấp của bạn. **Postfix không chạy** nghĩa là Postfix đang dừng.
- **… thư đang chờ gửi** là số thư còn trong hàng đợi. Postfix thử gửi lại mỗi thư trong tối đa 2 ngày.
- **Lỗi gửi gần nhất:** hiện lý do gần nhất khiến một thư không gửi được, đúng như nhà cung cấp trả về.

### Gỡ relay

Chọn **Gỡ** và xác nhận. Panel gỡ Postfix và xóa cài đặt. Website sẽ không gửi được email nữa.

## Cảnh báo

Cảnh báo gửi tin cho bạn qua Telegram, qua email, hoặc cả hai, khi máy chủ cần bạn chú ý. Vào **Cài đặt** → **Cảnh báo**.

Panel kiểm tra máy chủ mỗi phút. Mỗi sự cố được báo một lần, báo lại mỗi 6 giờ nếu vẫn còn, và báo thêm một lần khi đã được khắc phục. Mỗi tin bắt đầu bằng tên máy chủ trong ngoặc vuông. Email cảnh báo có tiêu đề "Server alert:" kèm tên máy chủ.

### Khi nào có cảnh báo

| Cảnh báo | Gửi khi | Mặc định |
|---|---|---|
| **CPU trên (%)** | mức dùng CPU trung bình trong 5 phút chạm ngưỡng | 90 |
| **Bộ nhớ trên (%)** | mức dùng bộ nhớ chạm ngưỡng | 90 |
| **Ổ đĩa trên (%)** | ổ đĩa (`/`) đầy tới mức đó | 90 |
| **Một dịch vụ bị dừng (Nginx, PHP, MariaDB…)** | một dịch vụ có tự chạy khi khởi động nhưng đang không chạy | bật |
| **Sao lưu, khôi phục hoặc gia hạn chứng chỉ thất bại** | một lần sao lưu, khôi phục, gia hạn chứng chỉ website hoặc gia hạn chứng chỉ của panel bị lỗi | bật |

- Đặt ngưỡng là 0 để tắt. Các ngưỡng khác phải từ 10 đến 100.
- Các dịch vụ được theo dõi là web server (Nginx, Apache hoặc OpenLiteSpeed), PHP-FPM, MariaDB, Redis và fail2ban, nếu đã cài.
- Panel chỉ bắt đầu kiểm tra khi có ít nhất một kênh nhận tin.

### Nhận cảnh báo qua Telegram

1. Trong Telegram, mở BotFather, gửi `/newbot` và trả lời các câu hỏi. BotFather sẽ trả về token của bot: một dãy số, dấu hai chấm, rồi một chuỗi dài gồm chữ và số.
2. Mở bot vừa tạo và gửi cho nó một tin bất kỳ. Bot không nhắn cho bạn được khi bạn chưa nhắn cho nó.
3. Lấy chat ID của bạn, ví dụ từ userinfobot. Đó là một con số.
4. Ở **Cài đặt** → **Cảnh báo**, nhập **Bot token** và **Chat ID**.
5. Chọn **Lưu**, rồi chọn **Gửi thử**.

Muốn gửi cảnh báo vào một nhóm hoặc kênh, hãy thêm bot vào đó trước. Chat ID của nhóm bắt đầu bằng dấu trừ. Với kênh công khai, bạn cũng có thể nhập `@tên` của kênh.

Sau khi lưu, token hiện dưới dạng các dấu chấm. Cứ để nguyên các dấu chấm đó để giữ token đã lưu. Nếu xóa **Chat ID** rồi lưu, token cũng bị xóa theo.

### Nhận cảnh báo qua email

Email cảnh báo được panel gửi thẳng qua một máy chủ SMTP, với cài đặt riêng, tách khỏi relay dành cho website. Bạn có thể dùng chung tài khoản của nhà cung cấp.

1. Ở mục **Email**, nhập **Máy chủ SMTP** và **Cổng** (mặc định 587).
2. Nhập **Tên đăng nhập** và **Mật khẩu** của tài khoản, nếu máy chủ SMTP yêu cầu đăng nhập.
3. Nhập địa chỉ **Người gửi** và **Người nhận**.
4. Chọn **Lưu**, rồi chọn **Gửi thử**.

Với cổng 465, kết nối được mã hóa ngay từ đầu. Với các cổng khác, panel dùng STARTTLS khi máy chủ SMTP hỗ trợ. Nếu máy chủ không hỗ trợ mà có tên đăng nhập, panel sẽ không gửi, để mật khẩu không bao giờ đi ở dạng rõ. Mật khẩu đã lưu cũng hiện dưới dạng các dấu chấm, giống token Telegram.

### Gửi cảnh báo thử

**Gửi thử** gửi một tin thử qua mọi kênh đã lưu. Nút này dùng cài đặt đã lưu, nên hãy chọn **Lưu** trước. Kết quả hiện mỗi kênh một dòng, **Telegram** hoặc **Email**, kèm **đã gửi** hoặc nội dung lỗi. Nếu chưa có kênh nào, panel báo "Hãy cấu hình Telegram hoặc email rồi lưu trước."

### Cảnh báo và trang chủ

Danh sách **Cần xử lý** ở **Trang chủ** và chuông ở góc trên bên phải chỉ hiện vấn đề khi bạn đang đăng nhập, và chúng kiểm tra những thứ khác, như xác thực hai lớp và tường lửa. Cảnh báo thì đến được với bạn cả khi bạn không mở panel. Xem trang [Công cụ hệ thống](./system#dashboard).
