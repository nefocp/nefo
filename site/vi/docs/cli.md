---
title: Dòng lệnh
description: Mọi lệnh nefo và tùy chọn của chúng (info, reset-password, update, setup, unlock-files, decrypt-backup), tệp cấu hình, và cách quản lý dịch vụ nefo bằng systemctl và journalctl.
---

# Dòng lệnh

Chương trình `nefo` ở `/usr/local/bin/nefo` vừa chạy panel, vừa có các lệnh dành cho root trên máy chủ: `nefo info` hiện địa chỉ của panel, `nefo reset-password` đặt mật khẩu mới (và có thể tắt 2FA), `nefo update` cài bản phát hành mới nhất, và `nefo setup` cài phần mềm. Hãy chạy mọi lệnh bằng root.

Trang này liệt kê các lệnh, rồi hướng dẫn quản lý dịch vụ `nefo` và giải thích tệp cấu hình.

## Các lệnh {#commands}

| Lệnh | Tác dụng |
|---|---|
| `nefo info` | Hiện URL của panel |
| `nefo reset-password` | Đặt mật khẩu ngẫu nhiên mới và đăng xuất mọi phiên |
| `nefo update` | Cài bản phát hành mới nhất và khởi động lại panel |
| `nefo setup` | Cài web server, các phiên bản PHP, máy chủ cơ sở dữ liệu và các thành phần thêm |
| `nefo unlock-files` | Trả lại tệp cho các website đang bị khóa (trước khi bạn gỡ panel) |
| `nefo decrypt-backup` | Giải mã một bản sao lưu đã được kho lưu trữ từ xa mã hóa |
| `nefo version` | Hiện phiên bản |
| `nefo init` | Tạo tệp cấu hình và tài khoản quản trị đầu tiên |
| `nefo serve` | Chạy panel (systemd làm việc này cho bạn) |
| `nefo help` | Hiện danh sách lệnh |

Chạy `nefo <command> -h` để xem các tùy chọn của một lệnh. Tùy chọn viết với một hay hai dấu gạch đều được: `-port` và `--port` là như nhau.

Mọi lệnh, trừ `version`, `help` và `decrypt-backup`, đều đọc tệp cấu hình `/etc/nefo/config.json`. Muốn dùng tệp khác, thêm `--config /path/to/config.json`, hoặc đặt biến môi trường `NEFO_CONFIG`.

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
| `--user` | Tên tài khoản (mặc định `admin`, tài khoản duy nhất của panel) |
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

## nefo setup

Thiết lập máy chủ giống trang thiết lập của panel (xem [Những việc đầu tiên](./first-steps#set-up-the-server)): lệnh lần lượt cài một web server, các phiên bản PHP, một máy chủ cơ sở dữ liệu và các thành phần thêm. Những phần đã cài sẽ được bỏ qua, nên bạn có thể chạy lại sau khi bị lỗi. Khi lệnh chạy xong, panel không mở trang thiết lập sau khi đăng nhập nữa.

```sh
nefo setup --web nginx --php 8.3,8.4 --db mariadb --redis
```

```
This server: 1 CPU, 960 MB of RAM (250 MB in use), 19.5 GB of free disk.

What the setup installs, one after the other:
  1. Nginx and PHP 8.3  about 40 MB of RAM, 55 MB of disk
  2. PHP 8.4            about 25 MB of RAM, 50 MB of disk, from packages.sury.org
  3. MariaDB            about 228 MB of RAM, 300 MB of disk
  4. Redis              up to 64 MB of RAM, 10 MB of disk
In all: about 357 MB more RAM and 415 MB of disk.

Install? [y/N]
```

Sau khi bạn trả lời `y`, lệnh hiện nhật ký của từng phần đang cài. Nếu một phần bị lỗi, lệnh dừng ở đó, cho biết những gì đã cài xong và kết thúc với mã lỗi. Nếu không có `--web`, `--php`, `--db` hay thành phần thêm nào, lệnh đề xuất bộ lựa chọn hợp với RAM của máy chủ, giống trang thiết lập.

Khi panel đang chạy, `nefo setup` nhờ panel thực hiện việc cài đặt như một tác vụ của nó: việc cài chờ các tác vụ khác xong, hiện trong panel, và vẫn tiếp tục kể cả khi bạn đóng phiên SSH. Lệnh nói chuyện với panel qua một socket trong thư mục dữ liệu, chỉ root mở được. Khi panel không chạy, `nefo setup` tự cài.

| Tùy chọn | Tác dụng |
|---|---|
| `--web` | `nginx`, `apache`, `openlitespeed` hoặc `none`. Web server đi kèm phiên bản PHP của hệ điều hành |
| `--php` | Các phiên bản PHP, cách nhau bằng dấu phẩy, ví dụ `8.3,8.4`, hoặc `none` |
| `--db` | `mariadb`, `mysql` hoặc `none`; `mariadb-11.4`, `mariadb-11.8` hoặc `mariadb-12.3` để cài một dòng LTS từ kho của MariaDB, khi máy chủ hỗ trợ (xem [Phiên bản MariaDB](./databases#mariadb-versions)). `--mariadb` giống `--db mariadb` |
| `--redis`, `--node`, `--python`, `--docker` | Cài thêm Redis, Node.js (bản LTS mới nhất), Python cho website ứng dụng, hoặc Docker |
| `--adminer`, `--waf` | Cài thêm Adminer hoặc tường lửa web. Cả hai đều cần web server |
| `--clamav` | Cài thêm ClamAV, trên máy chủ có từ 2 GB RAM |
| `--postgresql`, `--memcached` | Cài thêm PostgreSQL hoặc Memcached, khi trang Phần mềm có chúng |
| `--yes`, `-y` | Cài mà không hỏi. Bắt buộc khi không có terminal, ví dụ trong một script |
| `--dry-run` | Chỉ hiện những gì sẽ được cài |
| `--skip` | Không cài gì: panel không mở trang thiết lập sau khi đăng nhập nữa |
| `--wait` | Thời gian chờ panel khởi động, ví dụ `60s`. Trình cài đặt dùng tùy chọn này |
| `--check` | Chỉ kiểm tra các tùy chọn. Trình cài đặt dùng nó trước khi cấu hình bất cứ thứ gì |
| `--config` | Tệp cấu hình cần đọc |

Trình cài đặt nhận đúng các tùy chọn này để thiết lập máy chủ ngay trong lần cài (xem [Cài đặt](./install#set-the-server-up-in-the-same-run)).

## nefo unlock-files

Trả lại tệp cho mọi website đang bị khóa tệp (**Website** → (website) → **Khóa file**, xem [Chặn quốc gia và khóa file](./website-protection#lock-a-website-s-files)), với đúng chủ sở hữu, nhóm và quyền mà từng tệp có trước khi khóa, rồi xóa các nhóm khóa của website. Hãy chạy lệnh bằng root, khi panel đã dừng, trước khi gỡ Nefo (xem [Gỡ cài đặt](./uninstall#unlock-the-websites-files)).

```sh
systemctl stop nefo
nefo unlock-files
```

```
Unlocked the files of example.com
Unlocked the files of blog.example.com
```

Khi không có website nào bị khóa, lệnh in `No website's files are locked.`

| Tùy chọn | Tác dụng |
|---|---|
| `--config` | Tệp cấu hình cần đọc |

## nefo decrypt-backup

Giải mã một bản sao lưu đã được kho lưu trữ từ xa mã hóa (tệp `.tar.gz.enc`), ví dụ một bản bạn tải về từ Google Drive. Chỉ cần passphrase: lệnh chạy được trên bất kỳ máy chủ nào, kể cả máy chưa thiết lập panel, và không đọc tệp cấu hình. Xem [Khôi phục trên máy chủ mới](./restore-new-server#decrypt-a-copy-by-hand).

```sh
nefo decrypt-backup example.com_20261126-030000.tar.gz.enc
```

```
Passphrase:
Decrypted to example.com_20261126-030000.tar.gz
SHA-256: 3f1c…
```

Passphrase không bao giờ được đưa trên dòng lệnh, nơi người dùng khác có thể nhìn thấy. Trong terminal, lệnh hỏi passphrase mà không hiện ký tự nào; nếu không có terminal, passphrase là dòng đầu tiên của đầu vào chuẩn (stdin) hoặc của `-passphrase-file`.

| Tùy chọn | Tác dụng |
|---|---|
| `-o` | Tệp đầu ra. Mặc định là tên tệp đầu vào bỏ đi `.enc`. `-o -` ghi ra đầu ra chuẩn (stdout), ví dụ `nefo decrypt-backup -o - FILE.enc \| tar xz` |
| `-passphrase-file` | Đọc passphrase từ dòng đầu tiên của tệp này |
| `-force` | Thay tệp đầu ra nếu tệp đã tồn tại |

Kết quả được ghi vào một tệp tạm và chỉ được đổi tên khi toàn bộ bản sao đã giải mã xong, nên passphrase sai hay tệp hỏng sẽ không để lại gì (trừ khi dùng `-o -`, khi đó phần cuối tệp được kiểm tra sau cùng). Tệp không được mã hóa thì được chép nguyên như cũ, kèm một ghi chú. Mã SHA-256 lệnh in ra là mã mà panel đã ghi lại cho bản sao lưu đó.

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

## Quản lý dịch vụ nefo {#manage-the-nefo-service}

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

Khởi động lại panel không làm dừng website, database hay các dịch vụ khác. Bạn cũng có thể khởi động lại từ **Cài đặt** → **Panel** → **Nguồn** → **Khởi động lại panel**.

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
