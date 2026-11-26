---
title: Dòng lệnh
description: Mọi lệnh nefo và tùy chọn của chúng, tệp cấu hình, và cách quản lý dịch vụ nefo bằng systemctl và journalctl.
---

# Dòng lệnh

Chương trình `nefo` ở `/usr/local/bin/nefo` vừa chạy panel, vừa có vài lệnh dành cho root trên máy chủ. Trang này liệt kê các lệnh đó, rồi hướng dẫn quản lý dịch vụ `nefo` và giải thích tệp cấu hình. Hãy chạy mọi lệnh bằng root.

## Các lệnh

| Lệnh | Tác dụng |
|---|---|
| `nefo info` | Hiện URL của panel |
| `nefo reset-password` | Đặt mật khẩu ngẫu nhiên mới và đăng xuất mọi phiên |
| `nefo update` | Cài bản phát hành mới nhất và khởi động lại panel |
| `nefo version` | Hiện phiên bản |
| `nefo init` | Tạo tệp cấu hình và tài khoản quản trị đầu tiên |
| `nefo serve` | Chạy panel (systemd làm việc này cho bạn) |
| `nefo help` | Hiện danh sách lệnh |

Chạy `nefo <lệnh> -h` để xem các tùy chọn của một lệnh. Tùy chọn viết với một hay hai dấu gạch đều được: `-port` và `--port` là như nhau.

Mọi lệnh, trừ `version` và `help`, đều đọc tệp cấu hình `/etc/nefo/config.json`. Muốn dùng tệp khác, thêm `--config /đường/dẫn/config.json`, hoặc đặt biến môi trường `NEFO_CONFIG`.

## nefo info

Hiện URL của panel và vị trí các tệp của nó.

```sh
nefo info
```

```
URL:    https://panel.example.com:28443/k3x9p2qa7m1z/
URL:    https://203.0.113.10:28443/k3x9p2qa7m1z/
Config: /etc/nefo/config.json
Data:   /var/lib/nefo
```

Dòng đầu chỉ có khi panel đã có tên miền riêng (**Cài đặt** → **Chứng chỉ**). Dòng thứ hai dùng địa chỉ IP đầu tiên của máy chủ. Nếu nhà cung cấp cho máy chủ một địa chỉ nội bộ và một địa chỉ công khai riêng, hãy truyền địa chỉ công khai vào:

```sh
nefo info --host 203.0.113.10
```

| Tùy chọn | Tác dụng |
|---|---|
| `--host` | Tên máy hoặc địa chỉ IP hiện trong URL |
| `--port` | Chỉ in số cổng, tiện cho script |
| `--config` | Tệp cấu hình cần đọc |

## nefo reset-password

Đặt mật khẩu ngẫu nhiên mới cho tài khoản, đăng xuất tài khoản trên mọi thiết bị, rồi in mật khẩu ra. Dùng lệnh này khi bạn quên mật khẩu.

```sh
nefo reset-password
```

```
New password for admin: ••••••••••••••••••••
```

Nếu bạn còn mất cả điện thoại có ứng dụng xác thực, hãy thêm `--disable-2fa`. Tùy chọn này tắt xác thực hai lớp của tài khoản:

```sh
nefo reset-password --disable-2fa
```

```
New password for admin: ••••••••••••••••••••
Two-factor authentication is off. Turn it on again after signing in.
```

| Tùy chọn | Tác dụng |
|---|---|
| `--disable-2fa` | Tắt luôn xác thực hai lớp |
| `--user` | Tên tài khoản (mặc định `admin`) |
| `--config` | Tệp cấu hình cần đọc |

Panel vẫn chạy bình thường, bạn không cần khởi động lại. Việc đặt lại được ghi vào nhật ký hoạt động.

## nefo update

Kiểm tra trang phát hành của dự án trên GitHub. Nếu có phiên bản mới hơn, lệnh tải về, kiểm tra chữ ký và checksum, thay chương trình rồi khởi động lại panel.

```sh
nefo update
```

Chỉ để xem có bản cập nhật hay không:

```sh
nefo update --check
```

```
Installed: v0.1.0
Latest:    v0.2.0
```

| Tùy chọn | Tác dụng |
|---|---|
| `--check` | Chỉ hiện phiên bản đang cài và phiên bản mới nhất |
| `--config` | Tệp cấu hình cần đọc |

Xem [Cập nhật](./update) để biết bản cập nhật được kiểm tra thế nào và nên làm gì khi cập nhật lỗi.

## nefo version

Hiện phiên bản và commit dùng để build. `nefo --version` và `nefo -v` cho kết quả giống nhau.

```sh
nefo version
```

```
Nefo v0.1.0 (commit 1a2b3c4)
```

## nefo init

Tạo tệp cấu hình và tài khoản quản trị đầu tiên. Trình cài đặt đã chạy lệnh này cho bạn, nên thường bạn không cần dùng. Lệnh từ chối ghi đè lên tệp cấu hình đã có.

Lệnh chọn một cổng ngẫu nhiên từ 20000 đến 49999 và một đường dẫn bí mật ngẫu nhiên, bật HTTPS, rồi tạo tài khoản `admin` với mật khẩu ngẫu nhiên. Nếu thư mục dữ liệu đã có tài khoản, lệnh giữ nguyên chúng, chỉ ghi tệp cấu hình mới và không in mật khẩu.

| Tùy chọn | Tác dụng |
|---|---|
| `--config` | Nơi ghi tệp cấu hình (mặc định `/etc/nefo/config.json`) |
| `--data` | Thư mục dữ liệu (mặc định `/var/lib/nefo`) |
| `--listen` | Địa chỉ lắng nghe, ví dụ `:28443`, thay cho cổng ngẫu nhiên |
| `--host` | Tên máy hoặc địa chỉ IP hiện trong URL được in ra |
| `--dev` | Chế độ phát triển: HTTP thường trên `127.0.0.1:8080` với đường dẫn `dev`. Không bao giờ dùng trên máy chủ thật. |

## nefo serve

Chạy panel. Dịch vụ `nefo` khởi động nó bằng `nefo serve --config /etc/nefo/config.json`, nên bạn không cần tự chạy. Chạy thêm một bản nữa sẽ lỗi, vì dịch vụ đang dùng cổng đó.

## Quản lý dịch vụ nefo

| Lệnh | Tác dụng |
|---|---|
| `systemctl status nefo` | Xem panel có đang chạy không, kèm các dòng log mới nhất |
| `systemctl restart nefo` | Khởi động lại panel |
| `systemctl stop nefo` | Dừng panel |
| `systemctl start nefo` | Bật panel |
| `journalctl -u nefo` | Xem log của panel |
| `journalctl -u nefo -f` | Theo dõi log khi có dòng mới |
| `journalctl -u nefo -n 100 --no-pager` | Xem 100 dòng cuối |
| `journalctl -u nefo --since "1 hour ago"` | Xem log trong một giờ qua |

Khởi động lại panel không làm dừng website, database hay các dịch vụ khác. Bạn cũng có thể khởi động lại từ **Cài đặt → Panel → Khởi động lại panel**.

Khi dừng, panel để tác vụ đang chạy (như cài phần mềm) chạy xong trước, tối đa 75 giây. Nếu panel dừng vì lỗi, systemd tự bật lại sau 3 giây.

Tệp dịch vụ nằm ở `/etc/systemd/system/nefo.service`:

```ini
[Unit]
Description=Nefo server control panel
Documentation=https://nefo.io
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/nefo serve --config /etc/nefo/config.json
Restart=on-failure
RestartSec=3
UMask=0077
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

Panel chạy bằng root, vì nó quản lý toàn bộ máy chủ.

## Tệp cấu hình {#the-configuration-file}

Chỉ root đọc được `/etc/nefo/config.json`. Trình cài đặt ghi tệp này, ví dụ:

```json
{
  "listen": ":28443",
  "entrance": "k3x9p2qa7m1z",
  "data_dir": "/var/lib/nefo",
  "tls": {
    "enabled": true
  }
}
```

| Trường | Ý nghĩa |
|---|---|
| `listen` | Địa chỉ và cổng panel lắng nghe. `:28443` nghĩa là cổng 28443 trên mọi địa chỉ. |
| `entrance` | Đường dẫn bí mật, gồm 8 đến 32 chữ thường hoặc chữ số. Mọi đường dẫn khác trả về `404`. |
| `data_dir` | Thư mục dữ liệu: database `nefo.db` của panel, chứng chỉ và khóa. |
| `tls.enabled` | HTTPS cho panel. Hãy luôn bật. |
| `tls.cert`, `tls.key` | Đường dẫn tới chứng chỉ và khóa riêng của bạn cho panel (không bắt buộc). Chúng được ưu tiên hơn chứng chỉ Let's Encrypt của panel và chứng chỉ tự ký. |
| `acme_directory` | Nhà cấp chứng chỉ (không bắt buộc). Để trống nghĩa là Let's Encrypt. Khi thử nghiệm, đặt `https://acme-staging-v02.api.letsencrypt.org/directory`: chứng chỉ staging không được trình duyệt tin cậy, nhưng các lần xin lỗi không bị tính vào giới hạn của Let's Encrypt. |
| `update_url` | Địa chỉ tải bản cập nhật (không bắt buộc). Để trống nghĩa là trang phát hành của dự án trên GitHub. Phải bắt đầu bằng `https://`. |
| `dev` | Chế độ phát triển. Không bao giờ bật trên máy chủ thật. |

Sau khi sửa tệp, khởi động lại panel bằng `systemctl restart nefo`. Nếu tệp có lỗi, panel không khởi động được: `journalctl -u nefo` cho biết lý do.

::: warning Đổi cổng
Nếu bạn đổi cổng trong `listen`, hãy mở cổng mới trước, cả trên ufw (ví dụ `ufw allow 31234/tcp`) lẫn trên tường lửa cloud của nhà cung cấp. Nếu không, bạn sẽ không vào được panel sau khi khởi động lại.
:::
