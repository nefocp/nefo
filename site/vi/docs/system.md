---
title: Công cụ hệ thống
description: Trang chủ, cron job, dịch vụ, chương trình nền, tiến trình và cổng, lịch sử đăng nhập SSH, terminal trên web, log, và khởi động lại panel hoặc máy chủ.
---

# Công cụ hệ thống

**Trang chủ** hiện trực tiếp CPU, bộ nhớ, ổ đĩa và tải của máy chủ, kèm lịch sử 7 ngày, và liệt kê những gì cần xử lý. Các công cụ máy chủ của Nefo còn quản lý cron job, dịch vụ systemd, chương trình nền cần chạy liên tục, tiến trình và cổng đang mở, lịch sử đăng nhập SSH và một terminal root ngay trong trình duyệt, và khởi động lại panel hoặc máy chủ.

Trang này giới thiệu từng công cụ đó, và nơi xem log. Phần lớn nằm trong nhóm **Máy chủ** ở menu bên trái. Cập nhật hệ điều hành, bộ nhớ swap, múi giờ và tên máy, TCP BBR và dọn dẹp ổ đĩa nằm ở trang [Hệ thống](./os).

## Trang chủ {#dashboard}

**Trang chủ** mở ra ngay sau khi bạn đăng nhập.

- Phần trên cùng của trang hiện tên máy chủ, hệ điều hành, thời gian hoạt động, số CPU và dung lượng RAM. Trên VPS là container (LXC), RAM là giới hạn của container, có ghi "giới hạn của container" (xem [VPS là một container](./first-steps#a-vps-that-is-a-container)). Các nút ở đây cho phép **Kiểm tra cập nhật**, mở **Terminal** và dùng menu **Nguồn**.
- Bốn đồng hồ hiện trực tiếp **Tải hệ thống**, CPU, **Bộ nhớ** và **Ổ đĩa /**, cập nhật mỗi 5 giây. Tải hệ thống là tải trung bình 1 phút chia cho số CPU.
- Các ô đếm số website, cơ sở dữ liệu, tài khoản SFTP, cron job, bản sao lưu và rủi ro bảo mật. Chọn một ô để mở trang tương ứng. Khi không còn vấn đề nghiêm trọng nào, ô **Rủi ro bảo mật** cho biết lần [kiểm tra bảo mật](./security-check) gần nhất chạy lúc nào.
- **Lịch sử tài nguyên** vẽ biểu đồ CPU, RAM, Ổ đĩa, Tải hoặc Mạng trong 1 giờ, 24 giờ hoặc 7 ngày. Dưới biểu đồ là các giá trị **Hiện tại**, **Trung bình** và **Cao nhất**. Với mạng, biểu đồ hiện tốc độ **Vào** và **Ra**, cùng tổng lưu lượng trong khoảng thời gian đã chọn.
- **Cần xử lý** liệt kê các vấn đề cần bạn xem, nặng nhất trước.
- **Phần mềm đã cài** hiện các thành phần chính và trạng thái của chúng.

Panel ghi lại tải mỗi phút và giữ lịch sử 7 ngày. Sau khi mới cài, biểu đồ sẽ xuất hiện sau vài phút. Cuối menu bên trái cũng hiện mức dùng CPU và RAM hiện tại.

### Mục "Cần xử lý" kiểm tra những gì {#what-needs-attention-checks}

- Chưa bật xác thực hai lớp.
- Root đăng nhập SSH được bằng mật khẩu.
- Tường lửa đang tắt.
- Chứng chỉ của một website gia hạn bị lỗi, đã hết hạn hoặc còn dưới 14 ngày.
- Một website còn dưới 7 ngày là hết hạn.
- Một lịch sao lưu bị lỗi.
- Ổ đĩa đã đầy từ 90% trở lên.
- Có bản cập nhật bảo mật của hệ điều hành đang chờ (xem [Cập nhật](./os#updates)).
- Máy chủ cần khởi động lại để hoàn tất việc cài bản cập nhật.
- Lần [kiểm tra bảo mật](./security-check) gần nhất thấy vấn đề nghiêm trọng hoặc cảnh báo.
- Một website được theo dõi không phản hồi (xem [Theo dõi](./websites#monitoring)).

Chuông ở góc trên bên phải (**Thông báo**) hiện cùng danh sách này. Cả hai chỉ thấy được khi bạn đang đăng nhập. Muốn nhận tin qua Telegram, email, Discord, Slack hoặc webhook, hãy thiết lập [cảnh báo](./mail-alerts).

## Cron job {#cron-jobs}

Cron job chạy lệnh theo lịch. Vào **Cron job** ở menu bên trái.

### Thêm cron job {#add-a-cron-job}

1. Chọn **Thêm cron job**.
2. Nhập **Tên**, tối đa 100 ký tự.
3. Chọn **Chạy bằng user**: root, hoặc user của một website (hiện kèm tên miền). Tác vụ của website nên chạy bằng user của chính website đó, chỉ dùng root cho tác vụ của máy chủ.
4. Đặt **Lịch chạy**. Chọn một lịch thường dùng (**5 phút một lần**, **Mỗi giờ**, **Mỗi ngày lúc 03:00**, **Chủ nhật lúc 04:00**, **Khi máy chủ khởi động**) hoặc tự gõ. Bên dưới ô nhập, panel diễn giải lịch bằng lời.
5. Nhập **Lệnh**: một lệnh shell trên một dòng, tối đa 2000 ký tự, chạy bằng `/bin/sh`.
6. Để **Bật** ở trạng thái bật, rồi chọn **Lưu**.

Với user của một website có PHP, liên kết **Dùng lệnh cron của WordPress** sẽ điền sẵn lệnh cron của WordPress, ví dụ `php8.3 /var/www/example.com/public/wp-cron.php`, chạy 5 phút một lần.

### Cú pháp lịch chạy {#schedule-syntax}

Lịch chạy gồm năm trường, theo giờ của máy chủ:

| Trường | Giá trị |
|---|---|
| phút | 0–59 |
| giờ | 0–23 |
| ngày trong tháng | 1–31 |
| tháng | 1–12 |
| thứ trong tuần | 0–7 (0 và 7 là Chủ nhật) |

Trong mỗi trường, dùng `*` cho mọi giá trị, một danh sách như `1,15`, một dải như `1-5`, hoặc một bước như `*/10`. Bạn cũng có thể dùng một trong các lịch có sẵn thay cho năm trường: `@hourly`, `@daily` (hoặc `@midnight`), `@weekly`, `@monthly`, `@yearly` (hoặc `@annually`) và `@reboot`. Panel từ chối số nằm ngoài khoảng cho phép, ví dụ phút 60.

| Lịch | Chạy lúc |
|---|---|
| `*/5 * * * *` | 5 phút một lần |
| `0 3 * * *` | 03:00 mỗi ngày |
| `30 2 * * 1` | 02:30 mỗi thứ Hai |
| `0 0 1 * *` | nửa đêm ngày 1 hằng tháng |

### Cron job chạy thế nào {#how-cron-jobs-run}

- Panel ghi các cron job vào `/etc/cron.d/panel`, nên cron của hệ thống vẫn chạy chúng kể cả khi panel dừng. Đừng sửa file đó: panel sẽ ghi đè.
- Kết quả và lỗi của lệnh được ghi nối vào log của cron job, `/var/log/panel-cron/<id>.log`, file này thuộc về user chạy cron job.
- Dấu `%` trong lệnh hoạt động đúng như bạn gõ: panel tự thoát ký tự cho cron.
- Cron job sẽ ngừng chạy khi user của nó không còn, ví dụ sau khi bạn xóa website.

### Chạy, kiểm tra và sửa cron job {#run-check-and-change-a-job}

- Công tắc ở cột **Bật** bật hoặc tắt một cron job. Cron job đã tắt vẫn nằm trong danh sách.
- **Chạy ngay** chạy cron job tức thì, bằng user của nó, và hiện kết quả trực tiếp. Lệnh sẽ bị dừng sau một giờ.
- **Log**, trong menu ⋯ của dòng, hiện phần cuối log của cron job. Log lớn hơn 5 MB được cắt chỉ còn 1 MB cuối.
- **Sửa** và **Xóa** nằm trong cùng menu đó. Xóa cron job thì log của nó cũng bị xóa.

Phía trên danh sách, chọn **Tất cả**, **Đang bật** hoặc **Đang tắt**, và tìm theo tên, lệnh hoặc user.

## Dịch vụ {#services}

**Dịch vụ** liệt kê các dịch vụ hệ thống mà Nefo quản lý, nếu chúng đã được cài:

| Dịch vụ | Unit systemd |
|---|---|
| Nefo | `nefo` |
| Nginx, Apache hoặc OpenLiteSpeed | `nginx`, `apache2`, `lshttpd` |
| PHP-FPM, mỗi phiên bản đã cài một dịch vụ | ví dụ `php8.3-fpm` |
| MariaDB hoặc MySQL | `mariadb` hoặc `mysql` |
| PostgreSQL | ví dụ `postgresql@16-main` |
| Redis | `redis-server` |
| Memcached | `memcached` |
| fail2ban | `fail2ban` |
| SSH | `ssh` |
| Cron | `cron` |
| Tường lửa (ufw) | `ufw` |

Các dịch vụ khác, như Docker hay Postfix, không có trong danh sách. Mỗi dòng hiện trạng thái (**Đang chạy**, **Đã dừng**, **Lỗi**…), thời gian dịch vụ đã chạy, ghi chú "không tự chạy khi khởi động" nếu có, và bộ nhớ đang dùng.

Thao tác tùy theo dịch vụ:

| Dịch vụ | Thao tác |
|---|---|
| Web server, PHP-FPM | **Bật**, **Dừng**, **Khởi động lại**, **Nạp lại** |
| Nefo | **Khởi động lại** |
| SSH | **Khởi động lại**, **Nạp lại** |
| Các dịch vụ còn lại | **Bật**, **Dừng**, **Khởi động lại** |

- **Dừng** sẽ hỏi lại, vì các website cần dịch vụ đó sẽ ngừng hoạt động.
- Trước khi khởi động lại hoặc nạp lại web server, panel kiểm tra cấu hình của nó. Nếu kiểm tra thất bại, không có gì được khởi động lại và lỗi sẽ hiện ra.
- Không thể dừng SSH từ panel, để bạn không tự khóa mình ở ngoài.
- Khởi động lại Nefo thì trang sẽ tự tải lại sau vài giây.
- **Log** hiện 150 dòng cuối trong journal của dịch vụ.

Panel không thay đổi việc một dịch vụ có tự chạy khi khởi động hay không.

## Chương trình nền {#background-programs}

Chương trình nền là các lệnh cần chạy liên tục, ví dụ queue worker. Panel chạy mỗi chương trình thành một dịch vụ systemd, bằng user Linux của một website, và tự chạy lại khi chương trình dừng. Bạn cần có ít nhất một website trước.

### Thêm chương trình {#add-a-program}

1. Vào **Chương trình nền** và chọn **Thêm chương trình**.
2. Nhập **Tên**, tối đa 60 ký tự.
3. Chọn **Website**. Chương trình chạy bằng user Linux của website này, không bao giờ chạy bằng root.
4. Có thể đặt **Thư mục làm việc**: một thư mục bên trong thư mục của website, ví dụ `app`. Để trống để dùng chính thư mục của website.
5. Nhập **Lệnh**: một chương trình kèm tham số, trên một dòng, tối đa 1000 ký tự.
6. Có thể thêm **Biến môi trường**: mỗi dòng một `NAME=value`, tối đa 20. Trên máy chủ, chỉ root đọc được các biến này.
7. Để **Chạy ngay và khi máy chủ khởi động** ở trạng thái bật, rồi chọn **Tạo**.

Với website PHP, liên kết **Dùng queue worker của Laravel** sẽ điền sẵn `php8.3 artisan queue:work --sleep=3 --tries=3 --max-time=3600`, theo phiên bản PHP của website.

### Chương trình nền chạy thế nào {#how-background-programs-run}

- Mỗi chương trình là một dịch vụ systemd tên `panel-daemon-<id>.service`.
- Khi chương trình thoát, vì bất kỳ lý do gì, systemd chạy lại nó sau 3 giây.
- Khi máy chủ khởi động, chương trình chạy sau MariaDB và Redis.
- Chương trình chạy với các lớp bảo vệ của systemd: `/usr`, `/boot` và `/etc` chỉ đọc, `/home` và `/root` bị ẩn, và chương trình có `/tmp` riêng.
- Thư mục làm việc phải tồn tại và nằm trong thư mục của website, kể cả khi đi qua liên kết tượng trưng (symlink).
- Kết quả của chương trình được ghi vào journal của hệ thống.

### Điều khiển chương trình {#control-a-program}

Mỗi dòng hiện trạng thái (đang chạy, đang chạy lại, lỗi, đang dừng hoặc đã dừng), số lần đã chạy lại và bộ nhớ đang dùng.

- Dòng hiện nút **Chạy** (khi chương trình đang dừng) hoặc **Khởi động lại** (khi chương trình đang chạy). Các thao tác khác, kể cả **Dừng**, nằm trong menu ⋯ của dòng. **Dừng** giữ chương trình ở trạng thái dừng, kể cả sau khi khởi động lại máy chủ, cho đến khi bạn chạy lại.
- **Log** hiện 200 dòng cuối chương trình đã ghi.
- **Sửa** để thay đổi chương trình. **Lưu** sẽ khởi động lại chương trình với cài đặt mới.
- **Xóa** dừng và gỡ chương trình.

Khi bạn xóa một website, các chương trình nền của nó cũng bị dừng và gỡ bỏ.

## Tiến trình và cổng {#processes-and-ports}

**Tiến trình** cho biết những gì đang chạy trên máy chủ, chia thành ba mục: **Tiến trình**, **Cổng** và **Đăng nhập SSH**.

### Tiến trình {#processes}

Danh sách hiện các tiến trình bận nhất, tối đa 300. Sắp xếp theo PID, CPU % hoặc bộ nhớ bằng tiêu đề cột, và lọc theo tên, user hoặc PID. **Tự làm mới mỗi 5 giây** giúp danh sách luôn cập nhật.

CPU % được đo trong vài giây gần nhất: 100% là một nhân CPU chạy liên tục.

Để dừng một tiến trình, chọn nút dừng ở cuối dòng của nó, rồi:

- **Dừng** yêu cầu chương trình làm xong việc rồi thoát (tín hiệu TERM). Chương trình được viết tốt sẽ lưu dữ liệu trước.
- **Buộc dừng** kết thúc chương trình ngay (tín hiệu KILL). Chỉ dùng khi **Dừng** không có tác dụng: dữ liệu chưa lưu sẽ mất.

Dịch vụ do systemd chạy có thể được systemd chạy lại. Panel không dừng được tiến trình init của hệ thống (PID 1), chính panel, tiến trình đã khởi động panel, và các luồng của kernel. Mỗi lần dừng đều được ghi vào nhật ký hoạt động.

### Cổng {#ports}

**Cổng** liệt kê các cổng TCP và UDP mà chương trình đang mở, kèm **Địa chỉ**, **Chương trình** và nơi **Truy cập được từ**: **chỉ máy chủ này** hoặc **mạng bên ngoài**.

Cổng mở ra mạng vẫn có thể bị tường lửa chặn từ bên ngoài. Xem [Tường lửa](./security#firewall-ufw).

## Đăng nhập SSH {#ssh-sign-ins}

**Tiến trình** → **Đăng nhập SSH** liệt kê các lượt đăng nhập SSH và các lần thử thất bại trong 30 ngày qua, mới nhất trước, tối đa 200. Mỗi dòng hiện thời gian, kết quả (**thành công**, **sai mật khẩu** hoặc **user không tồn tại**), user, địa chỉ IP và phương thức, ví dụ `publickey` hoặc `password`.

Chọn **Tất cả**, **Chỉ lượt thành công** hoặc **Chỉ lượt thất bại**. Danh sách được đọc từ journal của dịch vụ SSH.

Nhiều lần thất bại thường có nghĩa là ai đó đang dò mật khẩu. Hãy tắt đăng nhập bằng mật khẩu, hoặc cài fail2ban để chặn các địa chỉ đó. Xem trang [Bảo mật](./security#brute-force-protection-fail2ban).

## Terminal {#terminal}

**Terminal** mở một shell root (bash) của máy chủ ngay trong trình duyệt.

::: warning
Terminal giống một phiên SSH bằng root: mọi lệnh chạy với toàn quyền, và một lệnh sai có thể xóa dữ liệu hoặc làm dừng máy chủ. Panel không hoàn tác được những gì bạn chạy ở đây.
:::

- Phiên kết thúc khi bạn rời trang, hoặc sau 30 phút không gõ gì. Chọn **Kết nối lại** để mở phiên mới.
- Mở được tối đa 4 terminal cùng lúc, tính cả terminal của container.
- Nhật ký hoạt động ghi lại lúc mở, lúc đóng terminal và thời gian đã mở. Nó không ghi lại các lệnh.

## Log {#logs}

Nefo không có một trang log chung. Mỗi log nằm cạnh thứ mà nó thuộc về:

| Log | Ở đâu |
|---|---|
| Log truy cập, log lỗi, lỗi PHP và log ứng dụng của website | **Website** → (website) → **Nhật ký** |
| Sự kiện tường lửa của website | **Website** → (website) → **Tường lửa WAF** |
| Kết quả của cron job | **Cron job** → **Log** |
| Journal của dịch vụ | **Dịch vụ** → **Log** |
| Kết quả của chương trình nền | **Chương trình nền** → **Log** |
| Container và dự án Compose | **Docker** |
| Những gì đã làm trong panel | **Tài khoản** → **Hoạt động gần đây** |
| Log dịch vụ của chính panel | `journalctl -u nefo` trên máy chủ |

Log truy cập, log lỗi và lỗi PHP của website hiện 300 dòng cuối. Chọn **Làm mới** trong cửa sổ log để tải các dòng mới.

## Khởi động lại panel hoặc máy chủ {#restart-the-panel-or-reboot-the-server}

Vào **Cài đặt** → **Panel** → **Nguồn**. **Trang chủ** cũng có các thao tác này trong menu **Nguồn**.

- **Khởi động lại panel** chỉ khởi động lại panel. Website, cơ sở dữ liệu và các dịch vụ khác vẫn chạy, panel sẽ trở lại sau vài giây.
- **Khởi động lại máy chủ** khởi động lại toàn bộ máy chủ. Website và cơ sở dữ liệu ngừng cho tới khi máy chủ lên lại, thường mất một đến hai phút. Bạn phải gõ tên máy chủ để xác nhận.

Trong cả hai trường hợp, trang tự tải lại khi panel phản hồi trở lại. Cả hai đều bị từ chối khi đang có tác vụ chạy nền, như cài đặt hay sao lưu: hãy chờ tác vụ xong. Cả hai đều được ghi vào nhật ký hoạt động.

Trên máy chủ, bạn cũng có thể khởi động lại panel bằng lệnh:

```sh
systemctl restart nefo
```
