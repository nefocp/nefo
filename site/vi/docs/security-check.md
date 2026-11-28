---
title: Kiểm tra bảo mật
description: Kiểm tra bảo mật của Nefo tìm các cài đặt khiến máy chủ bị hở (SSH, tường lửa, bản cập nhật, cơ sở dữ liệu mở, cổng Docker, file bị lộ, chứng chỉ, PHP) và dẫn mỗi phát hiện tới chỗ sửa, khi bạn yêu cầu hoặc mỗi ngày.
---

# Kiểm tra bảo mật

Kiểm tra bảo mật, có từ v0.24.0, chạy 22 bước kiểm tra nhanh trên máy chủ: SSH còn nhận mật khẩu, tường lửa đang tắt, bản cập nhật bảo mật đang chờ, cơ sở dữ liệu hoặc Redis mở ra Internet, cổng Docker đi vòng qua ufw, file website mà ai cũng tải được, chứng chỉ sắp hết hạn và cài đặt PHP. Mỗi phát hiện có mức độ nghiêm trọng và một nút dẫn tới trang để sửa nó.

Việc kiểm tra chạy khi bạn bắt đầu, và mặc định mỗi ngày một lần. Nó không gửi gì ra ngoài máy chủ của bạn. Mở **Kiểm tra bảo mật** trong nhóm **Bảo mật** ở menu bên trái.

## Chạy kiểm tra {#run-the-check}

1. Bấm **Chạy kiểm tra**. Việc kiểm tra mất vài giây.
2. Đọc các phát hiện ở tab **Cần sửa**. Mỗi phát hiện có một mức độ:
   - **Nghiêm trọng**: sửa ngay. Ví dụ: root đăng nhập SSH được bằng mật khẩu, hoặc một website để lộ bản dump cơ sở dữ liệu.
   - **Cảnh báo**: sửa sớm. Ví dụ: tường lửa đang tắt, hoặc có bản cập nhật bảo mật đang chờ.
   - **Lưu ý**: nên biết, thường để nguyên cũng được. Ví dụ: SSH dùng cổng 22.
3. Bấm nút của một phát hiện, như **Tới Tường lửa & SSH**, để mở chỗ sửa nó. Sau đó chạy kiểm tra lại.

Các bộ đếm ở đầu trang cho biết số vấn đề **Nghiêm trọng**, **Cảnh báo** và **Lưu ý** còn mở, và số mục kiểm tra **Đạt**. Bấm **Đạt** ở cuối trang để xem từng mục kiểm tra không tìm thấy gì cần sửa. Các mục kiểm tra không áp dụng cho máy chủ (không có Redis, không có website, không có Docker…) được liệt kê sau dòng "Không áp dụng cho máy chủ này".

## Kiểm tra bảo mật xem những gì? {#what-is-checked}

| Mục kiểm tra | Mức độ | Chỗ sửa |
|---|---|---|
| root đăng nhập SSH được bằng mật khẩu | Nghiêm trọng | **Tường lửa & SSH** → SSH |
| SSH nhận mật khẩu cho các tài khoản khác | Cảnh báo | **Tường lửa & SSH** → SSH |
| SSH dùng cổng 22 | Lưu ý | **Tường lửa & SSH** → SSH |
| Tường lửa (ufw) chưa cài, hoặc đang tắt | Cảnh báo | **Tường lửa & SSH** |
| fail2ban chưa cài, không chạy, hoặc không theo dõi SSH (không có jail `sshd`) | Cảnh báo | **Tường lửa & SSH** → fail2ban, hoặc **Dịch vụ** |
| Panel được truy cập không qua HTTPS | Nghiêm trọng | `/etc/nefo/config.json` (xem bên dưới) |
| Panel dùng chứng chỉ tự ký của nó | Lưu ý | **Cài đặt** |
| Một tài khoản panel chưa bật xác thực hai lớp | Nghiêm trọng | **Tài khoản** |
| Có bản cập nhật bảo mật đang chờ | Cảnh báo | **Hệ thống** |
| Lần kiểm tra cập nhật gần nhất đã quá một tuần, hoặc chưa từng chạy | Lưu ý | **Hệ thống** |
| Tự động cập nhật bảo mật đang tắt | Cảnh báo | **Hệ thống** |
| Cần khởi động lại để hoàn tất cập nhật | Lưu ý | **Cài đặt** |
| Một tài khoản không phải root có ID người dùng 0 | Nghiêm trọng | **Terminal** |
| Một tài khoản để trống mật khẩu | Nghiêm trọng | **Terminal** (`passwd -l <account>`) |
| MariaDB, MySQL, PostgreSQL, Redis, Valkey, Memcached hoặc MongoDB lắng nghe trên một địa chỉ khác 127.0.0.1 | Nghiêm trọng khi tường lửa cho mọi người vào cổng đó, nếu không thì Lưu ý | **Cơ sở dữ liệu** |
| Redis không có mật khẩu | Nghiêm trọng trên địa chỉ công khai mà tường lửa cho qua, Cảnh báo trên địa chỉ công khai khác, Lưu ý khi chỉ lắng nghe trên máy chủ này | **Cơ sở dữ liệu** → Redis |
| Một container Docker mở cổng trên mọi địa chỉ | Cảnh báo | **Docker** |
| Một website chưa có HTTPS | Cảnh báo | **Website** → (website) → **SSL / HTTPS** |
| Một website có chứng chỉ nhưng không chuyển hướng sang HTTPS | Lưu ý | **Website** → (website) → **SSL / HTTPS** |
| Một chứng chỉ đã hết hạn | Nghiêm trọng | **Website** → (website) → **SSL / HTTPS** |
| Một chứng chỉ còn 14 ngày hoặc ít hơn là hết hạn | Cảnh báo | **Website** → (website) → **SSL / HTTPS** |
| Một website để lộ file lẽ ra phải riêng tư | Nghiêm trọng với dữ liệu bí mật, Cảnh báo với bản sao và file nén | **Tệp tin** |
| Một website có file hoặc thư mục ai cũng ghi được | Cảnh báo | **Tệp tin** |
| PHP hiện lỗi cho khách (`display_errors`) | Cảnh báo | **Phần mềm** → PHP → **Quản lý** |
| PHP để lộ phiên bản (`expose_php`) | Lưu ý | **Phần mềm** → PHP → **Quản lý** |
| Có bản cập nhật lõi, plugin hoặc giao diện WordPress đang chờ | Cảnh báo | **Website** → (website) → **WordPress** |
| Chế độ debug của WordPress (`WP_DEBUG`) đang bật | Cảnh báo | **Website** → (website) → **WordPress** |

Một vài chi tiết:

- **Cơ sở dữ liệu trên mạng**: việc kiểm tra đọc xem mỗi chương trình lắng nghe trên những địa chỉ nào. Với địa chỉ công khai, nó xem tới tường lửa: không có tường lửa, hoặc có luật cho mọi người vào cổng đó, là nghiêm trọng. Cổng mà tường lửa chỉ mở cho một số địa chỉ (như **Truy cập từ xa** của một cơ sở dữ liệu), hoặc không mở chút nào, là lưu ý. Địa chỉ của mạng riêng (10.x, 172.16–31.x, 192.168.x) là lưu ý.
- **Docker** tự ghi luật tường lửa riêng, đứng trước luật của ufw, nên ufw không chặn cổng mà container mở ra. Khi chỉ website trên máy chủ này dùng cổng đó, hãy mở nó trên `127.0.0.1` (ví dụ `127.0.0.1:8080:80`).
- **File mà website phục vụ**: việc kiểm tra liệt kê cấp trên cùng của thư mục `public` của từng website, rồi hỏi web server các file không bao giờ nên công khai: file `.env`, thư mục `.git`, `.svn` và `.hg`, `.htpasswd`, bản dump cơ sở dữ liệu (`.sql`, `.sql.gz`, `.dump`, `.sqlite`…), bản sao của `wp-config.php`, bản sao lưu (`.bak`, `.old`, `.orig`, `.save`, `.swp`, tên kết thúc bằng `~`), `phpinfo.php` và `info.php`, cùng các file nén có tên giống bản sao của website (trong tên có "backup", "www", "site", "wordpress" hoặc tên miền). Một phát hiện nghĩa là web server đã trả về file đó. Cấu hình Nginx của Nefo ẩn các file bắt đầu bằng dấu chấm, nên trên Nginx file `.env` vẫn được tìm thấy nhưng không bị báo. Các website đang tạm ngưng, chuyển hướng đi nơi khác, yêu cầu mật khẩu hoặc là ứng dụng đứng sau proxy thì không được hỏi.
- **File ai cũng ghi được**: việc kiểm tra duyệt thư mục của website (`/var/www/<domain>`) và không đi theo liên kết tượng trưng. Thư mục có sticky bit, như một thư mục `tmp` dùng chung, thì không sao. Hãy đặt 644 cho file và 755 cho thư mục.
- **PHP**: việc kiểm tra đọc file `php.ini` và `conf.d` của từng phiên bản PHP, và đọc các pool PHP-FPM để tìm `display_errors` được bật cho riêng một website.
- **WordPress**: các bản cập nhật lấy từ lần kiểm tra gần nhất ở trang **WordPress** (hoặc tab **WordPress** của website). Kiểm tra bảo mật không bao giờ tự chạy wp-cli. Sau khi bạn cập nhật một website, lần kiểm tra tiếp theo ở đó sẽ cho biết còn lại gì.
- **Panel không có HTTPS**: Nefo bật HTTPS ngay khi cài. Nếu HTTPS đang tắt, hãy đặt `"enabled": true` trong phần `"tls"` của `/etc/nefo/config.json` rồi chạy `systemctl restart nefo`. Panel chỉ lắng nghe trên 127.0.0.1 thì không bị báo.

## Kiểm tra mất bao lâu? {#limits}

Việc kiểm tra đọc dữ liệu của chính panel và một vài file, và chỉ hỏi web server của chính máy chủ này (127.0.0.1) về các file có trong thư mục của website. Mỗi mục kiểm tra có giới hạn, nên một lần chạy vẫn ngắn trên máy chủ có nhiều website:

| Giới hạn | Giá trị |
|---|---|
| Một lần chạy do bạn bắt đầu | Tối đa 25 giây. Mục kiểm tra chưa xong lúc đó được ghi là đã dừng ở giới hạn, và kết quả ghi rõ là chưa đầy đủ |
| Mỗi chương trình mà việc kiểm tra hỏi (fail2ban, Docker, apt…) | Tối đa 5 giây; chương trình không trả lời kịp thì được bỏ qua |
| File và thư mục được duyệt trong thư mục của một website | 20.000 (100.000 cho tất cả website cộng lại) |
| Số mục được đọc trong thư mục `public` của một website | 5.000 |
| Request tới web server của máy chủ này | Tổng cộng 200, 4 request cùng lúc, mỗi request tối đa 3 giây |

## Chấp nhận một phát hiện {#accept-a-finding}

Có những phát hiện không sao trên máy chủ của bạn: SSH ở cổng 22 có fail2ban bảo vệ, hoặc một cổng cơ sở dữ liệu bạn cố ý mở cho một địa chỉ. Bấm **Chấp nhận** ở phát hiện đó. Nó chuyển sang tab **Đã chấp nhận** và không còn được tính.

Phát hiện đã chấp nhận sẽ quay lại khi tình trạng của nó thay đổi, ví dụ khi một tài khoản khác có ID người dùng 0, hoặc một cơ sở dữ liệu bắt đầu lắng nghe trên một địa chỉ khác. Nó cũng quay lại nếu đã được sửa rồi lại xuất hiện. Muốn tự đưa nó trở lại, mở tab **Đã chấp nhận** và bấm **Hiện lại**. Việc chấp nhận và hiện lại đều được ghi vào nhật ký hoạt động.

## Kiểm tra hằng ngày và cảnh báo {#daily-check-and-alerts}

Khi **Chạy mỗi ngày** đang bật (mặc định), panel chạy kiểm tra mỗi ngày một lần ở chế độ nền, trong đợt bảo trì hằng giờ của nó, nhưng không bao giờ trong 10 phút đầu sau khi panel khởi động. Nhờ vậy trang chủ luôn có kết quả gần đây.

Bật **Báo cho tôi khi lần kiểm tra hằng ngày thấy vấn đề mới** (mặc định tắt) để nhận tin nhắn qua các kênh cảnh báo (**Cài đặt** → **Cảnh báo**) khi lần kiểm tra hằng ngày thấy một vấn đề nghiêm trọng hoặc một cảnh báo mà lần kiểm tra trước chưa có. Lưu ý và phát hiện đã chấp nhận không bao giờ được gửi. Xem [Cảnh báo](./mail-alerts#alerts).

## Trên trang chủ {#on-the-dashboard}

Khi lần kiểm tra gần nhất thấy vấn đề nghiêm trọng hoặc cảnh báo, danh sách **Cần xử lý** ở trang chủ và chuông ở góc trên bên phải hiện "Kiểm tra bảo mật: 3 vấn đề cần sửa" kèm nút **Xem**. Khi không còn vấn đề nghiêm trọng nào, ô **Rủi ro bảo mật** cho biết lần kiểm tra gần nhất chạy lúc nào, và bấm vào đó sẽ mở trang kiểm tra bảo mật.

Muốn được báo khi có người đăng nhập SSH và panel, xem [Cảnh báo đăng nhập](./mail-alerts#sign-in-alerts).
