---
title: Gỡ cài đặt
description: Gỡ Nefo khỏi máy chủ bằng tay, và quyết định trước chuyện gì sẽ xảy ra với website và phần mềm mà Nefo đã thiết lập.
---

# Gỡ cài đặt

Nefo không có lệnh gỡ cài đặt: bạn gỡ nó bằng tay, bằng quyền root, qua vài bước. Mở khóa các website đang bị khóa bằng `nefo unlock-files`, dừng và gỡ dịch vụ `nefo` cùng `/usr/local/bin/nefo`, đóng cổng của panel, rồi xóa `/etc/nefo` và `/var/lib/nefo`. Website, database và phần mềm của bạn vẫn chạy, trừ khi bạn gỡ chúng trước.

Trang này hướng dẫn từng bước. Trang cũng giải thích những gì còn lại trên máy chủ, vì gỡ panel không gỡ các website và phần mềm bạn đã thiết lập bằng nó.

## Những gì còn lại khi gỡ Nefo {#what-stays-when-you-remove-nefo}

Các bước trên trang này gỡ chính panel: chương trình, dịch vụ, cấu hình và dữ liệu của nó. Mọi thứ bạn đã thiết lập qua panel vẫn còn và vẫn chạy:

- web server, PHP, MariaDB, Redis, Docker và các thành phần khác ở trang **Phần mềm**;
- website của bạn trong `/var/www`, các user Linux và database của chúng;
- cron job, chương trình nền, website ứng dụng và container Docker;
- bản sao lưu giữ trên máy chủ trong `/var/backups/panel`.

Khi không còn panel, sẽ không còn gì gia hạn chứng chỉ cho website hay chạy các lịch sao lưu. Chứng chỉ của website được lưu trong thư mục dữ liệu của Nefo, `/var/lib/nefo/certs`. Nếu bạn xóa thư mục này khi website vẫn đang dùng HTTPS, web server sẽ không nạp được chứng chỉ của chúng.

Vì vậy, hãy quyết định trước:

- **Muốn gỡ sạch mọi thứ:** gỡ website và phần mềm trong panel trước (bước 2), rồi mới gỡ Nefo.
- **Muốn giữ website chạy tiếp** mà không cần panel: bỏ qua bước 2, và giữ lại `/var/lib/nefo/certs` khi xóa dữ liệu (bước 7). Chứng chỉ Let's Encrypt chỉ có hạn tối đa 90 ngày, nên hãy thiết lập cách gia hạn khác trước khi chúng hết hạn.

## 1. Giữ bản sao những gì bạn cần {#_1-keep-copies-of-what-you-need}

1. Ở trang **Sao lưu**, tạo một bản sao lưu mới cho từng website và database bạn muốn giữ, bằng **Sao lưu ngay**.
2. Chép các bản sao lưu ra ngoài máy chủ. Chúng nằm trong `/var/backups/panel`, và trên kho lưu trữ từ xa nếu bạn đã thiết lập.

## 2. Gỡ website và phần mềm trong panel {#_2-remove-websites-and-software-in-the-panel}

Hãy làm bước này khi panel còn chạy. Panel từ chối gỡ một thành phần khi vẫn còn thứ dùng đến nó, nên hãy làm theo thứ tự sau:

1. **Website:** mở từng website, ở tab **Tổng quan** bấm **Xóa**. Đánh dấu **Xóa luôn các database của website** nếu bạn không cần chúng nữa. Bỏ đánh dấu **Sao lưu website trước** nếu bạn đã giữ bản sao lưu (bước 1). Thao tác này xóa tệp của website, user Linux và chứng chỉ của nó.
2. **Cơ sở dữ liệu:** xóa các database còn lại.
3. **Docker:** xóa các container và dự án Compose bạn đã tạo.
4. **Cron job** và **Chương trình nền:** xóa những mục bạn đã tạo.
5. **Phần mềm:** bấm **Gỡ cài đặt** ở từng thành phần không còn cần, như Docker, MariaDB, Redis và web server. Với MariaDB và Docker, thư mục dữ liệu (`/var/lib/mysql` của MariaDB, `/var/lib/docker` của Docker) được giữ lại, trừ khi bạn đánh dấu ô xóa luôn dữ liệu.

Phiên bản PHP cuối cùng được giữ lại, vì web server cần ít nhất một phiên bản. Nếu không cần, hãy gỡ nó bằng apt sau khi đã gỡ Nefo.

## 3. Ghi lại cổng của panel {#_3-note-the-panel-s-port}

Bạn cần số cổng để đóng nó trên tường lửa ở bước sau:

```sh
nefo info --port
```

## 4. Mở khóa tệp của các website {#unlock-the-websites-files}

Nếu bạn đã khóa tệp của một số website (**Website** → (website) → **Khóa file**, xem [Chặn quốc gia và khóa file](./website-protection#lock-a-website-s-files)), tệp của chúng thuộc về root trong lúc bị khóa. Hãy trả tệp lại cho các website trước khi gỡ panel, nếu không chúng vẫn thuộc về root:

```sh
systemctl stop nefo
nefo unlock-files
```

Lệnh liệt kê từng website đã được mở khóa:

```
Unlocked the files of example.com
Unlocked the files of blog.example.com
```

Mọi tệp và thư mục được trả về đúng chủ sở hữu, nhóm và quyền như lúc bị khóa. Các tệp mà root đưa vào website trong thời gian đó vẫn thuộc về root. Lệnh cũng xóa các nhóm khóa của website (`panel-lock-<uid>`). Lệnh đọc các bản ghi trong `/var/lib/nefo/filelock`, nên hãy chạy nó trước khi xóa dữ liệu của panel (bước 7). Chạy lại lần nữa thì không có gì xảy ra: lệnh in `No website's files are locked.`

## 5. Dừng và gỡ dịch vụ cùng chương trình {#_5-stop-and-remove-the-service-and-the-program}

```sh
systemctl disable --now nefo
rm /etc/systemd/system/nefo.service
systemctl daemon-reload
rm /usr/local/bin/nefo
```

## 6. Đóng cổng của panel {#_6-close-the-panel-s-port}

Nếu ufw đang bật, hãy xóa các luật cho cổng của panel. Trình cài đặt đã thêm chúng với ghi chú `Nefo`.

1. Liệt kê các luật kèm số thứ tự:

   ```sh
   ufw status numbered
   ```

2. Xóa luật cho cổng của panel theo số thứ tự, ví dụ:

   ```sh
   ufw delete 3
   ```

3. ufw thường có thêm một luật cho IPv6, đánh dấu `(v6)`. Số thứ tự thay đổi sau mỗi lần xóa, nên hãy liệt kê lại rồi xóa nốt luật đó.

Nếu bạn dùng firewalld, với cổng 28443 làm ví dụ:

```sh
firewall-cmd --permanent --remove-port=28443/tcp
firewall-cmd --reload
```

Đừng quên xóa cổng đó khỏi tường lửa cloud của nhà cung cấp.

## 7. Xóa cấu hình và dữ liệu {#_7-delete-the-configuration-and-the-data}

::: danger Không thể hoàn tác
Thao tác này xóa tài khoản của panel, các cài đặt, danh sách website, lịch sao lưu, nhật ký hoạt động, chứng chỉ của panel và chứng chỉ của các website. Hãy chắc chắn bạn đã giữ lại những gì cần (bước 1).
:::

Xóa toàn bộ:

```sh
rm -rf /etc/nefo /var/lib/nefo
```

Nếu website vẫn chạy tiếp (xem ở trên), hãy giữ chứng chỉ của chúng và xóa phần còn lại:

```sh
rm -rf /etc/nefo
find /var/lib/nefo -mindepth 1 -maxdepth 1 ! -name certs -exec rm -rf {} +
```

## 8. Tùy chọn: dọn những gì Nefo để lại ở nơi khác {#_8-optional-remove-what-nefo-left-elsewhere}

Trong lúc bạn dùng, Nefo còn tạo một số tệp bên ngoài các thư mục của nó. Chỉ xóa những gì bạn không cần nữa:

| Đường dẫn | Là gì |
|---|---|
| `/var/backups/panel` | Các bản sao lưu giữ trên máy chủ |
| `/etc/cron.d/panel` và `/var/log/panel-cron` | Cron job tạo trong panel và log của chúng. Các cron job vẫn chạy cho tới khi bạn xóa tệp này. |
| `/var/lib/panel-trash` | Tệp đã xóa trong trình quản lý tệp, giữ trong 30 ngày |
| `/var/lib/panel-quarantine` | Tệp bị trình quét mã độc cách ly |
| `/var/lib/panel-geoip` | Cơ sở dữ liệu quốc gia dùng cho việc chặn quốc gia |
| `/var/backups/nefo-postgresql` | Các bản dump được tạo trước khi nâng cấp PostgreSQL |
| `/var/lib/panel-acme` | Thư mục trả lời các lần kiểm tra của Let's Encrypt. Chỉ xóa khi không còn web server nào dùng. |
| `/etc/ssh/sshd_config.d/99-panel-sftp.conf` | Luật SSH cho tài khoản của các website và SFTP |
| `/etc/fail2ban/jail.d/panel.conf` và `/etc/fail2ban/filter.d/panel.conf` | Luật fail2ban cho SSH và panel, nếu bạn cài fail2ban từ panel |
| `/etc/ssh/sshd_config.d/00-panel.conf` | Cài đặt SSH bạn đã lưu trong panel |

::: warning Cài đặt SSH
Nếu bạn đã đổi cổng SSH hoặc tắt đăng nhập bằng mật khẩu trong panel, các cài đặt đó nằm trong `/etc/ssh/sshd_config.d/00-panel.conf`. Xóa tệp này sẽ đổi cách bạn đăng nhập SSH khi SSH khởi động lại. Hãy giữ tệp, trừ khi bạn chắc chắn muốn quay về cài đặt mặc định, và hãy thử một phiên SSH mới trước khi đóng phiên hiện tại.
:::
