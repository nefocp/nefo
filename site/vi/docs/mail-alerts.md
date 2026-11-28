---
title: Thư và cảnh báo
description: Cho website gửi email qua nhà cung cấp SMTP, và nhận cảnh báo qua Telegram, email, Discord, Slack hoặc webhook khi máy chủ cần bạn chú ý hoặc khi có người đăng nhập.
---

# Thư và cảnh báo

Nefo không có mail server. Website gửi email qua Postfix, đóng vai trò relay chỉ gửi, tới nhà cung cấp SMTP của bạn (Gmail, SendGrid, Amazon SES và các nhà cung cấp khác). Cảnh báo đến với bạn qua Telegram, email, Discord, Slack hoặc webhook có chữ ký khi CPU, bộ nhớ hoặc ổ đĩa lên cao, một dịch vụ bị dừng, một lần sao lưu thất bại, một website không phản hồi, hoặc có người đăng nhập.

Trang này nói về cả hai việc: gửi email từ website, và cảnh báo. Để thiết lập bản ghi DNS của nhà cung cấp email (MX, SPF, DMARC), xem [DNS (Cloudflare)](./dns#set-up-email-records).

## Không có mail server, và đó là chủ ý {#no-mail-server-by-design}

Nefo không tạo hộp thư và không nhận email. Một mail server đầy đủ tốn bộ nhớ và cần chăm sóc liên tục, không hợp với VPS nhỏ. Hãy dùng nhà cung cấp email cho hộp thư của bạn.

Để gửi thư đi, Nefo cài Postfix làm relay chỉ gửi. Postfix chỉ nghe trên chính máy chủ này và chuyển mọi thư cho nhà cung cấp của bạn (Gmail, SendGrid, Amazon SES và các nhà cung cấp khác), đăng nhập bằng tài khoản bạn nhập vào.

## Gửi email từ website {#send-e-mail-from-websites}

Khi đã thiết lập relay, hàm `mail()` của PHP sẽ hoạt động, vì Postfix cung cấp `/usr/sbin/sendmail` mà PHP dùng để gửi thư. Form liên hệ, email quên mật khẩu và thông báo của WordPress gửi được mà không cần plugin. Nếu chưa có relay, website không gửi được email.

### Thiết lập relay {#set-up-the-relay}

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

### Các ô và quy tắc {#fields-and-rules}

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

### Gửi thư thử {#send-a-test-message}

1. Bên dưới form, nhập một địa chỉ vào ô **Gửi thư thử đến…**.
2. Chọn **Gửi thử**.

Panel theo dõi thư trong log mail và chờ nhà cung cấp trả lời tối đa một phút. Kết quả cho biết thư đã được gửi, chưa gửi được (Postfix sẽ tiếp tục thử lại) hay bị nhà cung cấp từ chối, kèm câu trả lời của nhà cung cấp. Nếu nhà cung cấp không trả lời trong một phút, thư vẫn nằm trong hàng đợi và Postfix tiếp tục thử gửi.

### Kiểm tra relay {#check-the-relay}

Ở đầu khung cài đặt:

- **Đang chuyển tiếp qua …** nghĩa là Postfix đang chạy và dùng nhà cung cấp của bạn. **Postfix không chạy** nghĩa là Postfix đang dừng.
- **… thư đang chờ gửi** là số thư còn trong hàng đợi. Postfix thử gửi lại mỗi thư trong tối đa 2 ngày.
- **Lỗi gửi gần nhất:** hiện lý do gần nhất khiến một thư không gửi được, đúng như nhà cung cấp trả về.

### Gỡ relay {#remove-the-relay}

Chọn **Gỡ** và xác nhận. Panel gỡ Postfix và xóa cài đặt. Website sẽ không gửi được email nữa.

## Cảnh báo {#alerts}

Cảnh báo gửi tin cho bạn qua Telegram, email, Discord, Slack hoặc webhook của riêng bạn, khi máy chủ cần bạn chú ý. Vào **Cài đặt** → **Cảnh báo**. Kênh nào cũng là tùy chọn: bạn cấu hình bao nhiêu kênh cũng được, và mỗi cảnh báo được gửi tới tất cả các kênh cùng lúc. Một kênh bị lỗi không làm các kênh khác dừng lại.

Panel kiểm tra máy chủ mỗi phút. Mỗi sự cố được báo một lần, báo lại mỗi 6 giờ nếu vẫn còn, và báo thêm một lần khi đã được khắc phục. Mỗi tin bắt đầu bằng tên máy chủ trong ngoặc vuông. Email cảnh báo có tiêu đề "Server alert:" kèm tên máy chủ.

### Khi nào có cảnh báo {#what-sends-an-alert}

| Cảnh báo | Gửi khi | Mặc định |
|---|---|---|
| **CPU trên (%)** | mức dùng CPU trung bình trong 5 phút chạm ngưỡng | 90 |
| **Bộ nhớ trên (%)** | mức dùng bộ nhớ chạm ngưỡng | 90 |
| **Ổ đĩa trên (%)** | ổ đĩa (`/`) đầy tới mức đó | 90 |
| **Một dịch vụ bị dừng (Nginx, PHP, MariaDB…)** | một dịch vụ có tự chạy khi khởi động nhưng đang không chạy | bật |
| **Sao lưu, khôi phục hoặc gia hạn chứng chỉ thất bại** | một lần sao lưu, khôi phục, gia hạn chứng chỉ website hoặc gia hạn chứng chỉ của panel bị lỗi | bật |
| Theo dõi website (tab **Theo dõi** của từng website) | một website được theo dõi không phản hồi (2 lần kiểm tra lỗi liên tiếp), hoạt động trở lại, hoặc dùng chứng chỉ còn hạn dưới 14 ngày | tắt |
| **Đăng nhập SSH** | có người đăng nhập qua SSH: mọi lần, hoặc chỉ khi từ địa chỉ mới | **Không báo** |
| **Số lần đăng nhập SSH sai trong 10 phút** | có ngần ấy lần đăng nhập SSH thất bại trong vòng 10 phút | 0 (tắt) |
| **Đăng nhập panel** | có người đăng nhập panel: mọi lần, hoặc chỉ khi từ địa chỉ mới | **Không báo** |
| **Báo cho tôi khi lần kiểm tra hằng ngày thấy vấn đề mới** (ở trang **Kiểm tra bảo mật**) | lần [kiểm tra bảo mật](./security-check) hằng ngày thấy một vấn đề nghiêm trọng hoặc cảnh báo mới | tắt |
| **Báo cho tôi khi file PHP thay đổi** (tab **Khóa file** của website) | lần kiểm tra hằng ngày của một website đang khóa thấy file PHP bị sửa, thêm hoặc xóa (xem [Khóa file](./website-protection#alerts-when-php-files-change)) | tắt |

- Đặt ngưỡng là 0 để tắt. Các ngưỡng khác phải từ 10 đến 100.
- Các dịch vụ được theo dõi là web server (Nginx, Apache hoặc OpenLiteSpeed), PHP-FPM, MariaDB hoặc MySQL, PostgreSQL, Redis, Memcached và fail2ban, nếu đã cài.
- Panel chỉ bắt đầu kiểm tra khi có ít nhất một kênh nhận tin.
- Theo dõi website được bật riêng cho từng website, và vẫn chạy khi chưa có kênh nào: tab **Theo dõi** hiện kết quả. Cảnh báo website không phản hồi cũng được báo lại mỗi 6 giờ như các cảnh báo khác. Xem [Website](./websites#monitoring).

### Cảnh báo đăng nhập {#sign-in-alerts}

Nhận tin nhắn khi có người đăng nhập vào máy chủ qua SSH hoặc vào panel, để một lần đăng nhập không phải do bạn không bị lọt qua.

1. Vào **Cài đặt** → **Cảnh báo** và thiết lập ít nhất một kênh.
2. Ở mục **Đăng nhập**, chọn cho **Đăng nhập SSH** và **Đăng nhập panel**:
   - **Không báo**;
   - **Mỗi lần đăng nhập**;
   - **Chỉ khi từ địa chỉ mới**: lần đăng nhập từ một địa chỉ mà tài khoản chưa từng đăng nhập từ đó trong 180 ngày qua.
3. Tùy chọn: ở ô **Số lần đăng nhập SSH sai trong 10 phút**, nhập số lần thất bại sẽ gửi cảnh báo, từ 3 đến 100000. 0 là tắt.
4. Bấm **Lưu**.

Cảnh báo SSH ghi tên tài khoản, địa chỉ, cách tài khoản đăng nhập và thời gian, ví dụ:

```
[server1] 🔑 SSH sign-in: admin from 203.0.113.10 (new address) with a key, 2026-11-27 10:31:05 +07
```

Nhiều lần đăng nhập trong cùng một phút được gộp vào một tin. Khi có một loạt lần thử thất bại, tin nhắn ghi các địa chỉ đã thử nhiều nhất:

```
[server1] ⚠️ 57 failed SSH sign-in attempts in the last 10 minutes, from 198.51.100.7 (40), 203.0.113.5 (17).
```

Cách hoạt động:

- Không có gì theo dõi log SSH liên tục. Mỗi phút một lần, panel đọc những gì dịch vụ SSH (`ssh.service` hoặc `sshd.service`) đã ghi vào system journal kể từ lần đọc trước, và nhớ chỗ đã dừng. Khi tắt, cảnh báo SSH không tốn chút tài nguyên nào.
- Khi bạn bật cảnh báo SSH, chúng bắt đầu từ lúc đó: các lần đăng nhập trước đó không bao giờ được báo. Địa chỉ của các lần đăng nhập SSH trong 30 ngày qua được coi là đã biết.
- Với panel, địa chỉ của các lần đăng nhập có trong nhật ký hoạt động được coi là đã biết.
- Với IPv6, mọi địa chỉ trong cùng một dải /64 được tính là một địa chỉ, vì hầu hết nhà cung cấp cấp một dải /64 cho một khách hàng. Panel nhớ tối đa 500 địa chỉ cho SSH và 500 cho panel.
- Nếu panel đã dừng hơn một giờ, khi chạy lại nó chỉ đọc một giờ gần nhất của log SSH.
- Lần thử thất bại là sai mật khẩu và tên tài khoản không tồn tại. Khi đã tắt đăng nhập bằng mật khẩu, key bị từ chối không được tính. Mỗi giờ chỉ gửi tối đa một cảnh báo về loạt lần thử thất bại.
- Muốn chặn các địa chỉ cứ thử mãi, hãy cài fail2ban ở trang **Tường lửa & SSH**. Xem [Chống dò mật khẩu](./security#brute-force-protection-fail2ban).

### Nhận cảnh báo qua Telegram {#get-alerts-by-telegram}

1. Trong Telegram, mở BotFather, gửi `/newbot` và trả lời các câu hỏi. BotFather sẽ trả về token của bot: một dãy số, dấu hai chấm, rồi một chuỗi dài gồm chữ và số.
2. Mở bot vừa tạo và gửi cho nó một tin bất kỳ. Bot không nhắn cho bạn được khi bạn chưa nhắn cho nó.
3. Lấy chat ID của bạn, ví dụ từ userinfobot. Đó là một con số.
4. Ở **Cài đặt** → **Cảnh báo**, nhập **Bot token** và **Chat ID**.
5. Chọn **Lưu**, rồi chọn **Gửi thử**.

Muốn gửi cảnh báo vào một nhóm hoặc kênh, hãy thêm bot vào đó trước. Chat ID của nhóm bắt đầu bằng dấu trừ. Với kênh công khai, bạn cũng có thể nhập `@name` của kênh.

Sau khi lưu, token hiện dưới dạng các dấu chấm. Cứ để nguyên các dấu chấm đó để giữ token đã lưu. Nếu xóa **Chat ID** rồi lưu, token cũng bị xóa theo.

### Nhận cảnh báo qua email {#get-alerts-by-e-mail}

Email cảnh báo được panel gửi thẳng qua một máy chủ SMTP, với cài đặt riêng, tách khỏi relay dành cho website. Bạn có thể dùng chung tài khoản của nhà cung cấp.

1. Ở mục **Email**, nhập **Máy chủ SMTP** và **Cổng** (mặc định 587).
2. Nhập **Tên đăng nhập** và **Mật khẩu** của tài khoản, nếu máy chủ SMTP yêu cầu đăng nhập.
3. Nhập địa chỉ **Người gửi** và **Người nhận**.
4. Chọn **Lưu**, rồi chọn **Gửi thử**.

Với cổng 465, kết nối được mã hóa ngay từ đầu. Với các cổng khác, panel dùng STARTTLS khi máy chủ SMTP hỗ trợ. Nếu máy chủ không hỗ trợ mà có tên đăng nhập, panel sẽ không gửi, để mật khẩu không bao giờ đi ở dạng rõ. Mật khẩu đã lưu cũng hiện dưới dạng các dấu chấm, giống token Telegram.

### Nhận cảnh báo qua Discord {#get-alerts-in-discord}

1. Trong Discord, mở cài đặt của kênh sẽ nhận cảnh báo, rồi chọn **Tích hợp** (Integrations) → **Webhook** → **Webhook mới**.
2. Đặt tên cho webhook nếu muốn, rồi chọn **Sao chép URL webhook**.
3. Ở **Cài đặt** → **Cảnh báo**, dán URL vào ô **URL webhook** trong mục **Discord**.
4. Chọn **Lưu**, rồi chọn **Gửi thử**.

URL có dạng `https://discord.com/api/webhooks/<id>/<token>`. Địa chỉ trên `discordapp.com` và các bản `ptb.`, `canary.` cũng dùng được, và có thể thêm `?thread_id=` để gửi vào một thread. Cảnh báo dài hơn 2000 ký tự, giới hạn của Discord, sẽ bị cắt bớt. Các lượt nhắc trong cảnh báo, như `@everyone`, không gửi thông báo cho ai.

### Nhận cảnh báo qua Slack {#get-alerts-in-slack}

1. Vào [api.slack.com/apps](https://api.slack.com/apps), chọn **Create New App** → **From scratch**, rồi chọn workspace của bạn.
2. Mở **Incoming Webhooks**, bật lên, rồi chọn **Add New Webhook to Workspace** và chọn kênh.
3. Sao chép **Webhook URL**. URL bắt đầu bằng `https://hooks.slack.com/services/`.
4. Ở **Cài đặt** → **Cảnh báo**, dán URL vào ô **URL webhook** trong mục **Slack**, chọn **Lưu**, rồi chọn **Gửi thử**.

Panel chỉ nhận incoming webhook: workflow webhook của Slack cần các trường dữ liệu khác.

### Gửi cảnh báo tới webhook {#send-alerts-to-a-webhook}

Webhook gửi mỗi cảnh báo dưới dạng JSON tới địa chỉ bạn chọn: script của riêng bạn, n8n, Home Assistant, hay một ứng dụng chat không có trong danh sách này. Trong mục **Webhook**, nhập **URL webhook** và, nếu bên nhận có kiểm tra chữ ký, một **Khóa bí mật để ký (tùy chọn)** dài từ 16 đến 200 ký tự, không có khoảng trắng. Sau đó chọn **Lưu** và **Gửi thử**.

Mỗi cảnh báo là một request `POST` với `Content-Type: application/json` và nội dung như sau:

```json
{"server":"vps-1","text":"⚠️ The disk is 93% full.","time":"2026-11-26T08:15:00Z","kind":"alert"}
```

| Trường | Giá trị |
|---|---|
| `server` | Tên máy chủ (host name) |
| `text` | Nội dung cảnh báo, không kèm tên máy chủ. Sự cố đã được khắc phục bắt đầu bằng "✅ Resolved:" |
| `time` | Thời điểm gửi, theo giờ UTC, định dạng RFC 3339 |
| `kind` | `alert`, hoặc `test` khi bấm **Gửi thử** |

Mọi câu trả lời 2xx đều được tính là đã gửi. Panel chờ tối đa 10 giây, không đi theo chuyển hướng, và không hiện nội dung bên nhận trả về, chỉ hiện mã trạng thái.

#### Kiểm tra chữ ký {#verify-the-signature}

Khi có khóa bí mật, mỗi request có header `X-Nefo-Signature: sha256=` theo sau là HMAC-SHA256 của nội dung, ở dạng hex, với khóa bí mật làm key. Hãy tính trên đúng nội dung nhận được, trước khi đọc JSON, và so sánh theo thời gian không đổi (constant time). Với Python:

```python
import hashlib, hmac

def verify(secret: bytes, body: bytes, signature: str) -> bool:
    expected = "sha256=" + hmac.new(secret, body, hashlib.sha256).hexdigest()
    return hmac.compare_digest(expected, signature)
```

Với Node.js, so sánh `'sha256=' + crypto.createHmac('sha256', secret).update(rawBody).digest('hex')` với header bằng `crypto.timingSafeEqual`, sau khi kiểm tra hai chuỗi dài bằng nhau. Nội dung được ký có cả `time`: hãy từ chối request cũ hơn vài phút để chống gửi lại (replay).

#### Địa chỉ webhook không được dùng {#webhook-addresses}

- URL bắt đầu bằng `https://`. Địa chỉ `http://` thường chỉ dùng được sau khi bạn đánh dấu **Cho phép địa chỉ http:// (không mã hóa, cho máy nhận trong mạng riêng của bạn)**: khi không mã hóa, cảnh báo và chữ ký có thể bị đọc trên đường đi.
- URL không chứa tên đăng nhập hay mật khẩu, không có `#`, và dài tối đa 2048 ký tự.
- Để webhook không chạm được tới các dịch vụ của chính máy chủ, panel không bao giờ kết nối tới địa chỉ loopback (`127.0.0.0/8`, `::1`, `localhost`), địa chỉ link-local (`169.254.0.0/16`, `fe80::/10`), dịch vụ metadata của cloud (`169.254.169.254`, `100.100.100.200`, `168.63.129.16`, `fd00:ec2::254`), `0.0.0.0/8` hay địa chỉ multicast. Tên máy chủ được kiểm tra lại mỗi lần gửi cảnh báo, trên các địa chỉ mà tên đó phân giải ra, nên một tên trỏ về `127.0.0.1` cũng bị từ chối.
- Mạng riêng (`10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`, `fd00::/8`) được phép, cho máy nhận ở nhà hoặc trong một container.

### URL webhook đã lưu {#saved-webhook-urls}

URL của Discord, Slack và webhook chứa token của chúng, nên sau khi lưu chúng hiện dưới dạng các dấu chấm, giống token Telegram, và khóa bí mật cũng vậy. Cứ để nguyên các dấu chấm để giữ giá trị đã lưu. Xóa một URL rồi lưu để ngừng dùng kênh đó. Xóa URL của webhook cũng xóa luôn khóa bí mật và quyền dùng http://.

### Gửi cảnh báo thử {#send-a-test-alert}

**Gửi thử** gửi một tin thử qua mọi kênh đã lưu. Nút này dùng cài đặt đã lưu, nên hãy chọn **Lưu** trước. Kết quả hiện mỗi kênh một dòng, **Telegram**, **Email**, **Discord**, **Slack** hoặc **Webhook**, kèm **đã gửi** hoặc nội dung lỗi. Nếu chưa có kênh nào, panel báo "Hãy cấu hình một kênh nhận tin rồi lưu trước."

Với Discord, Slack và webhook, lỗi sẽ là một trong các trường hợp sau:

- **Không kết nối được tới webhook** kèm lý do, ví dụ `no such host` hoặc `connection refused`.
- **Webhook từ chối tin nhắn (HTTP 404…). Hãy kiểm tra URL.**: bên nhận trả về mã trạng thái không thuộc nhóm 2xx. Discord và Slack có kèm lý do, như `Unknown Webhook` hoặc `invalid_token`. Chuyển hướng (3xx) cũng bị tính là từ chối.
- **Webhook không trả lời trong 10 giây.**
- **… là địa chỉ loopback, link-local hoặc dịch vụ metadata của cloud**: xem [Địa chỉ webhook không được dùng](#webhook-addresses).

Khi một cảnh báo thật không gửi được, log của panel sẽ ghi lại kèm tên kênh, không bao giờ kèm URL của kênh.

### Cảnh báo và trang chủ {#alerts-and-the-dashboard}

Danh sách **Cần xử lý** ở **Trang chủ** và chuông ở góc trên bên phải chỉ hiện vấn đề khi bạn đang đăng nhập, và chúng kiểm tra những thứ khác, như xác thực hai lớp và tường lửa. Cảnh báo thì đến được với bạn cả khi bạn không mở panel. Xem trang [Công cụ hệ thống](./system#dashboard).
