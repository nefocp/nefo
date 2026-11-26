---
title: Xử lý sự cố
description: Cách khắc phục các sự cố thường gặp với Nefo, từ không vào được panel tới lỗi chứng chỉ và website báo 502 hoặc 503.
---

# Xử lý sự cố

Trang này gom các sự cố hay gặp nhất, kèm cách khắc phục cho từng trường hợp. Phần lớn cách khắc phục cần đăng nhập SSH vào máy chủ bằng root. Nếu SSH cũng không vào được, hãy dùng web console trong trang quản lý của nhà cung cấp.

## Không vào được panel

### Quên địa chỉ panel

Chạy lệnh sau trên máy chủ:

```sh
nefo info
```

Kết quả hiện URL đầy đủ, gồm cổng và đường dẫn bí mật: với tên miền của panel nếu bạn đã đặt, và với địa chỉ IP đầu tiên của máy chủ. Nếu đó là địa chỉ nội bộ, hãy thêm `--host` và IP công khai, ví dụ `nefo info --host 203.0.113.10`.

### Trang báo 404

Panel chỉ trả lời dưới đường dẫn bí mật của nó, ví dụ `https://203.0.113.10:28443/k3x9p2qa7m1z/`. Mọi địa chỉ khác đều trả về `404`, kể cả địa chỉ không có đường dẫn. Hãy xem lại đường dẫn bằng `nefo info`.

Cũng hãy chắc chắn địa chỉ bắt đầu bằng `https://`. Với `http://`, panel trả về lỗi thay vì trang đăng nhập.

### Trang không tải được

Khi trình duyệt không kết nối được, hoặc chờ mãi rồi bỏ cuộc, nghĩa là panel không chạy hoặc có thứ đang chặn kết nối.

1. **Kiểm tra panel có chạy không:**

   ```sh
   systemctl status nefo
   ```

   Nếu không thấy `active (running)`, xem [Panel không khởi động](#the-panel-doesn-t-start).

2. **Kiểm tra tường lửa cloud của nhà cung cấp.** Nó phải cho phép cổng TCP của panel. Xem số cổng bằng `nefo info --port`.

3. **Kiểm tra IP của bạn còn được cho phép không.** Nếu bạn đã giới hạn cổng panel chỉ cho IP của mình, trên ufw hoặc trên tường lửa cloud, mà IP của bạn đã đổi (hay gặp với mạng gia đình và mạng di động), hãy cho phép IP mới. Với ufw, dùng IP mới của bạn và cổng của panel:

   ```sh
   ufw allow from 198.51.100.7 to any port 28443 proto tcp
   ```

4. **Kiểm tra fail2ban có chặn bạn không.** Nếu bạn đã cài fail2ban từ panel, nó chặn một địa chỉ trong một giờ sau 10 lần đăng nhập panel sai trong 10 phút (5 lần với SSH). Bỏ chặn địa chỉ của bạn:

   ```sh
   fail2ban-client set panel unbanip 198.51.100.7
   ```

   Với SSH, dùng jail `sshd` thay cho `panel`.

### "Bạn đã nhập sai quá nhiều lần"

Sau 5 lần đăng nhập sai trong 15 phút, panel từ chối đăng nhập từ địa chỉ của bạn trong 15 phút và báo "Bạn đã nhập sai quá nhiều lần. Hãy thử lại sau … giây." Nhập sai mã xác thực hai lớp cũng bị tính. Hãy chờ hết thời gian. Khóa này chỉ nằm trong bộ nhớ, nên `systemctl restart nefo` cũng xóa được nó.

### Panel không khởi động {#the-panel-doesn-t-start}

Đọc lý do trong log:

```sh
journalctl -u nefo -n 50 --no-pager
```

| Log ghi | Cách xử lý |
|---|---|
| `address already in use` | Một chương trình khác đang dùng cổng của panel. Hãy dừng chương trình đó, hoặc đổi cổng trong `/etc/nefo/config.json` (xem [Dòng lệnh](./cli#the-configuration-file)). |
| Một lỗi liên quan tới `/etc/nefo/config.json` | Tệp cấu hình có lỗi, chẳng hạn sau khi bạn sửa tay. Sửa dòng mà thông báo nhắc tới. |
| `database schema version … is newer than this panel supports` | Một phiên bản cũ đang chạy với dữ liệu của phiên bản mới hơn. Hãy cài lại bản phát hành mới nhất (xem [Cập nhật](./update#if-an-update-fails)). |

Sửa xong, chạy `systemctl restart nefo`.

## Quên mật khẩu

Đặt mật khẩu ngẫu nhiên mới trên máy chủ:

```sh
nefo reset-password
```

Lệnh in ra mật khẩu mới và đăng xuất tài khoản ở mọi nơi. Hãy đăng nhập bằng mật khẩu đó, rồi đặt mật khẩu của riêng bạn trong **Tài khoản**.

## Mất điện thoại có mã 2FA {#lost-your-2fa-phone}

Nếu bạn không còn lấy được mã từ ứng dụng xác thực, hãy tắt xác thực hai lớp từ máy chủ. Lệnh này cũng đặt mật khẩu mới:

```sh
nefo reset-password --disable-2fa
```

Đăng nhập bằng mật khẩu mới, rồi thiết lập lại xác thực hai lớp trong **Tài khoản** với điện thoại mới.

### Mã lúc nào cũng "không đúng"

Mã phụ thuộc vào thời gian. Panel chấp nhận mã hiện tại cùng mã ngay trước và ngay sau nó, và mỗi mã chỉ dùng được một lần. Nếu mã nào cũng bị từ chối, hãy kiểm tra đồng hồ của máy chủ:

```sh
timedatectl
```

Dòng `System clock synchronized` phải là `yes`. Cũng hãy kiểm tra giờ trên điện thoại.

## Trình duyệt cảnh báo chứng chỉ

Khi panel chưa có chứng chỉ riêng, nó dùng chứng chỉ tự ký, và trình duyệt nào cũng cảnh báo. Điều này là bình thường. Hãy đặt cho panel một tên miền và chứng chỉ miễn phí để hết cảnh báo (xem [Những việc đầu tiên](./first-steps#give-the-panel-its-own-domain)).

Nếu panel đã có chứng chỉ mà trình duyệt vẫn cảnh báo:

- Bạn đang vào panel bằng địa chỉ IP. Chứng chỉ được cấp cho tên miền: hãy dùng `https://panel.example.com:28443/k3x9p2qa7m1z/`.
- Chứng chỉ đã hết hạn vì không gia hạn được. Hãy xem **Cài đặt → Chứng chỉ** và bấm **Xin chứng chỉ mới**. Nguyên nhân giống như ở mục tiếp theo.

## Không xin được chứng chỉ {#a-certificate-request-fails}

Trước khi hỏi Let's Encrypt, panel kiểm tra từng tên miền có trỏ về máy chủ này không: nó đặt một tệp thử trên máy chủ rồi tải tệp đó qua tên miền bằng HTTP, ở cổng 80. Nếu không được, panel không xin chứng chỉ và liệt kê từng vấn đề dưới dòng "these names do not reach this server yet, so no certificate was requested". Các thông báo này bằng tiếng Anh:

| Thông báo | Nguyên nhân và cách xử lý |
|---|---|
| `no DNS record. Add an A record that points to this server's IP address.` | Tên miền chưa có địa chỉ. Hãy thêm bản ghi A ở nhà cung cấp DNS. Bản ghi mới có thể cần một lúc mới có hiệu lực khắp nơi. |
| `no answer over HTTP on port 80 (…). Check the DNS record and the firewall.` | Cổng 80 đang đóng, hoặc tên miền trỏ sang máy chủ khác. Hãy mở cổng 80 trên ufw và trên tường lửa cloud của nhà cung cấp, và kiểm tra web server đang chạy. |
| `answered HTTP …, but not from this server. Check that the DNS record points here.` | Một máy khác đã trả lời cho tên miền: bản ghi trỏ sang máy chủ khác, hoặc có proxy hay CDN đứng trước, như proxy của Cloudflare (đám mây màu cam). |

Mỗi dòng bắt đầu bằng tên miền và các địa chỉ mà nó trỏ tới. Nếu đó không phải địa chỉ của máy chủ bạn, hãy sửa bản ghi DNS trước. Nếu danh sách có một địa chỉ IPv6 không phải của máy chủ bạn, hãy sửa hoặc xóa bản ghi AAAA.

**Khi đứng sau proxy của Cloudflare:**

- Với website: hoặc chuyển bản ghi sang "DNS only" trong lúc xin chứng chỉ, hoặc đánh dấu **Xác thực qua DNS (Cloudflare)** ở tab **SSL / HTTPS** của website. Xác thực DNS cần API token của Cloudflare, bạn thêm token ở **Cài đặt → Cloudflare**. Tên wildcard (`*.example.com`) luôn cần xác thực DNS.
- Với tên miền riêng của panel: bản ghi phải trỏ thẳng về máy chủ, không qua proxy.

**Với tên miền riêng của panel,** tab **Chứng chỉ** sẽ báo "Cần cài web server trước" cho tới khi bạn cài web server, vì web server trả lời Let's Encrypt ở cổng 80. Hãy cài ở trang **Website** trước.

Nếu bước kiểm tra đã qua mà Let's Encrypt vẫn từ chối, log tác vụ sẽ ghi lý do. Let's Encrypt giới hạn số lần xin lỗi, nên hãy sửa nguyên nhân trước khi thử lại.

Khi gia hạn tự động bị lỗi, panel báo "Gia hạn chứng chỉ example.com bị lỗi" và thử lại sau mỗi 12 giờ. Chứng chỉ vẫn dùng được cho tới khi hết hạn.

## Website báo lỗi

| Khách truy cập thấy | Nguyên nhân | Cách xử lý |
|---|---|---|
| Dòng chữ "This website is temporarily unavailable." (HTTP 503) | Website đang tạm ngưng, do bạn tạm ngưng hoặc do đã qua ngày hết hạn. | Mở tab **Hạn dùng & tạm ngưng** của website. Bỏ dấu ở **Tạm ngừng website này**, và xóa hoặc đổi **Ngày hết hạn**. Nếu ngày đó vẫn nằm trong quá khứ, panel sẽ lại tạm ngưng website trong vòng một giờ. |
| Trang "502 Bad Gateway" hoặc "503 Service Unavailable" của web server | Chương trình phía sau website không trả lời: PHP-FPM với website PHP, hoặc ứng dụng hay container với website ứng dụng. Nginx báo 502, còn Apache và OpenLiteSpeed có thể báo 503. | Với PHP, mở **Dịch vụ** rồi bật hoặc khởi động lại dịch vụ PHP-FPM ứng với phiên bản PHP của website, ví dụ `php8.3-fpm`. Với ứng dụng, website hiện "Ứng dụng dừng": hãy đọc **Log ứng dụng** ở tab **Nhật ký** của website. |
| Mọi website đều không phản hồi | Web server đã dừng. Trang **Website** báo web server "đang không chạy". | Bật web server ở trang **Dịch vụ**. |
| HTTP 429 | Website đang bật giới hạn tốc độ (chỉ có trên Nginx), và khách gửi quá nhiều request. | Tăng giới hạn, hoặc đặt về 0 để tắt, ở tab **Chặn truy cập** của website. |
| HTTP 403 | Địa chỉ của khách nằm trong danh sách bị chặn của website, hoặc tường lửa ứng dụng web đã chặn request. | Kiểm tra **Địa chỉ IP bị chặn** ở tab **Chặn truy cập**, và các sự kiện tường lửa gần đây ở tab **Tường lửa WAF**. |

Với các lỗi khác, hãy đọc log lỗi và lỗi PHP của website ở tab **Nhật ký**.

## Log nằm ở đâu

| Log | Trên máy chủ | Trong panel |
|---|---|---|
| Panel | `journalctl -u nefo` | — |
| Log truy cập và log lỗi của website | `/var/log/nginx/`, `/var/log/apache2/` hoặc `/var/log/openlitespeed/`, tên `example.com.access.log` và `example.com.error.log` | Website → (website) → **Nhật ký** |
| Lỗi PHP của website | `/var/www/example.com/logs/php-error.log` | Website → (website) → **Nhật ký** → **Lỗi PHP** |
| Website ứng dụng | `journalctl -u panel-app-example_com` | Website → (website) → **Nhật ký** → **Log ứng dụng** |
| Cron job | `/var/log/panel-cron/` | **Cron job** |
| Các lần đăng nhập và thay đổi trong panel | — | **Tài khoản** → **Hoạt động gần đây** |

Khi báo lỗi, hãy kèm kết quả của `nefo version` và các dòng log liên quan. Nhớ xóa mật khẩu và đường dẫn bí mật của panel trước.
