---
title: Cài đặt
description: Cài Nefo lên máy chủ Ubuntu hoặc Debian chỉ bằng một lệnh, rồi mở panel lần đầu.
---

# Cài đặt

Trang này hướng dẫn cách cài Nefo, giải thích trình cài đặt làm những gì, và cách mở panel lần đầu. Hãy xem [yêu cầu hệ thống](./requirements) trước.

## Chạy trình cài đặt

1. Kết nối vào máy chủ qua SSH.
2. Chạy lệnh:

   ```sh
   curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/install.sh | sudo bash
   ```

   Nếu bạn đăng nhập bằng root và máy chưa có `sudo` (thường gặp trên Debian), hãy bỏ `sudo`:

   ```sh
   curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/install.sh | bash
   ```

3. Chờ trình cài đặt in ra địa chỉ, tên đăng nhập và mật khẩu của panel. Việc này thường chỉ mất vài giây.

::: tip Các lệnh trong tài liệu này
Mọi lệnh khác trong tài liệu này đều chạy bằng root trên máy chủ, nên được ghi không có `sudo`.
:::

## Trình cài đặt in ra những gì

Phần cuối của kết quả trông như sau:

```
==> Downloading nefo-linux-amd64 (latest)
==> Verifying the release signature
==> Verifying checksum
==> Creating the configuration and the admin account
==> Installing the systemd service
==> Opening port 28443/tcp in ufw

Nefo is ready.

  URL:      https://203.0.113.10:28443/k3x9p2qa7m1z/
  Username: admin
  Password: ••••••••••••••••••••

Save this password now: it is stored only as a hash and cannot be shown again.
After signing in, change it and turn on two-factor authentication.
The certificate is self-signed, so your browser will warn you the first time.

If your provider has a cloud firewall (security group), allow TCP port 28443 there too.
Manage the service with: systemctl status|restart nefo   Logs: journalctl -u nefo
```

Cổng, đường dẫn và mật khẩu của bạn sẽ khác: chúng được chọn ngẫu nhiên cho từng máy chủ.

::: warning Hãy lưu mật khẩu ngay
Mật khẩu chỉ hiện một lần. Panel chỉ lưu bản băm (hash) của mật khẩu. Nếu bạn làm mất, hãy đặt mật khẩu mới bằng `nefo reset-password` (xem [Dòng lệnh](./cli#nefo-reset-password)).
:::

Địa chỉ trong URL là địa chỉ IP đầu tiên của máy chủ. Nếu nhà cung cấp cấp cho máy chủ một IP nội bộ và một IP công khai riêng, hãy thay địa chỉ trong URL bằng IP công khai.

## Trình cài đặt làm những gì

1. **Kiểm tra máy chủ.** Trình cài đặt dừng lại nếu không chạy bằng root, nếu thiếu systemd, `curl` hoặc `sha256sum`, nếu hệ điều hành hay CPU không được hỗ trợ, hoặc nếu Nefo đã được cài (đã có `/etc/nefo/config.json`).
2. **Tải về** chương trình cho CPU của bạn (`nefo-linux-amd64` hoặc `nefo-linux-arm64`) và danh sách checksum từ trang phát hành của dự án trên GitHub.
3. **Kiểm tra chữ ký** của danh sách checksum bằng khóa Ed25519 của dự án, được gắn sẵn trong trình cài đặt. Bản phát hành không có chữ ký hợp lệ sẽ bị từ chối với thông báo "Do not install this file." Trình cài đặt của bản build phát triển không có khóa, và sẽ báo là chữ ký không được kiểm tra.
4. **Kiểm tra checksum SHA-256** của tệp tải về. Nếu không khớp, quá trình cài dừng lại.
5. **Cài** chương trình vào `/usr/local/bin/nefo`, rồi tạo `/etc/nefo` và `/var/lib/nefo`, chỉ root đọc được.
6. **Tạo cấu hình** trong `/etc/nefo/config.json`: một cổng ngẫu nhiên từ 20000 đến 49999, một đường dẫn bí mật ngẫu nhiên dài 12 ký tự, và bật HTTPS. Trình cài đặt cũng tạo tài khoản `admin` với mật khẩu ngẫu nhiên dài 20 ký tự.
7. **Cài dịch vụ systemd** `nefo`, khởi động nó và cho nó tự chạy khi máy khởi động.
8. **Mở cổng của panel** trên ufw nếu ufw đang bật, hoặc trên firewalld nếu máy đang chạy firewalld. Nếu cả hai đều không bật, trình cài đặt không đổi tường lửa nào.

Trình cài đặt không cài gì thêm: không web server, không PHP, không database. Bạn sẽ thêm chúng sau, từ panel.

## Mở panel lần đầu

1. Mở URL mà trình cài đặt in ra trên trình duyệt, đúng y như vậy. URL bắt đầu bằng `https://` và kết thúc bằng đường dẫn bí mật cùng dấu gạch chéo.
2. Trình duyệt sẽ cảnh báo kết nối không riêng tư hoặc chứng chỉ không đáng tin. Điều này là bình thường: panel tự tạo chứng chỉ tự ký ở lần chạy đầu. Hãy chọn tiếp tục vào trang. Trên hầu hết trình duyệt, lựa chọn này nằm sau một nút kiểu "Nâng cao" (Advanced).
3. Đăng nhập bằng tên `admin` và mật khẩu từ trình cài đặt.

Sau đó hãy làm theo [những việc đầu tiên](./first-steps): đổi mật khẩu, bật xác thực hai lớp, và đặt cho panel một tên miền riêng kèm chứng chỉ miễn phí để hết cảnh báo của trình duyệt.

### Đường dẫn bí mật

Panel chỉ trả lời dưới đường dẫn bí mật của nó, ví dụ `/k3x9p2qa7m1z/`. Mọi địa chỉ khác trên cổng đó đều trả về `404`, kể cả địa chỉ trần `https://203.0.113.10:28443/`. Các công cụ dò quét thử những địa chỉ panel quen thuộc sẽ không tìm thấy gì.

Hãy giữ kín đường dẫn này như mật khẩu. Nếu quên, chạy `nefo info` trên máy chủ (xem [Dòng lệnh](./cli#nefo-info)).

Nếu bạn gõ `http://` thay vì `https://`, panel sẽ trả về lỗi thay vì trang đăng nhập.

## Mở cổng trên tường lửa cloud

Nếu nhà cung cấp có tường lửa nằm ngoài máy chủ (security group hoặc cloud firewall), hãy cho phép cổng TCP của panel ở đó. Trình cài đặt không làm việc này thay bạn được. Về sau, khi chạy website, hãy cho phép thêm cổng TCP 80 và 443. Xem [Yêu cầu hệ thống](./requirements#firewalls).

## Biến môi trường {#environment-variables}

Bạn có thể thay đổi cách trình cài đặt hoạt động bằng các biến sau:

| Biến | Mặc định | Tác dụng |
|---|---|---|
| `NEFO_VERSION` | `latest` | Cài một bản phát hành cụ thể, ví dụ `v0.1.0`, thay vì bản mới nhất. |
| `NEFO_REPO` | `nefocp/nefo` | Tải từ một kho GitHub khác (`chủ-sở-hữu/tên`), ví dụ một bản fork. |
| `NEFO_FORCE` | chưa đặt | Đặt bằng `1` để thử cài trên hệ điều hành chưa được hỗ trợ. Phần kiểm tra CPU vẫn áp dụng. |

Đặt biến ngay trước `bash`:

```sh
curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/install.sh | NEFO_VERSION=v0.1.0 bash
```

Nếu dùng `sudo`, hãy viết `| sudo NEFO_VERSION=v0.1.0 bash`.

`NEFO_REPO` chỉ đổi nơi trình cài đặt tải về. Các bản cập nhật sau này lấy từ kho được gắn sẵn trong chương trình, hoặc từ `update_url` trong tệp cấu hình (xem [Cập nhật](./update#where-updates-come-from)).

## Cài lại

Trình cài đặt không chạy khi đã có `/etc/nefo/config.json`, nên nó không thể ghi đè lên một panel đang chạy. Muốn cài lại từ đầu, hãy gỡ Nefo trước (xem [Gỡ cài đặt](./uninstall)), rồi chạy lại trình cài đặt.
