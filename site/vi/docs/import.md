---
title: Nhập website
description: Chuyển website từ cPanel, aaPanel, HestiaCP, CyberPanel hoặc server khác sang, kèm file, database, chứng chỉ và cron job.
---

# Nhập website {#import-websites}

Nefo nhập website từ server khác theo hai cách:

- **Bản sao lưu đầy đủ của cPanel**: mọi website của một tài khoản cPanel, kèm database và user của chúng, chứng chỉ và cron job.
- **File nén website và bản dump SQL**: file của một website dạng `.tar.gz`, `.tgz` hoặc `.zip`, kèm các bản dump `.sql` hoặc `.sql.gz` nếu có. Dùng cách này cho aaPanel, HestiaCP, CyberPanel hay bất kỳ server nào khác.

Việc nhập không đụng tới server cũ. Nefo đọc file nén trước và cho bạn xem những gì nó tìm thấy; server này chưa thay đổi gì cho tới khi bạn xác nhận.

## Tạo bản sao lưu {#make-the-backup}

### cPanel {#cpanel}

Trong cPanel, mở **Files → Backup** rồi bấm **Download a Full Account Backup**. Giữ **Home Directory** làm nơi lưu và bấm **Generate Backup**. cPanel ghi file `backup-M.D.YYYY_HH-MM-SS_USER.tar.gz` vào thư mục home và gửi email cho bạn khi xong.

Nếu có quyền root trên WHM, chạy `/scripts/pkgacct USER`: lệnh này ghi file `/home/cpmove-USER.tar.gz`. Hai file có cùng cấu trúc và Nefo đọc được cả hai, kể cả từ các phiên bản cPanel cũ.

Bản sao lưu từng phần (tải riêng "Home Directory" hoặc "MySQL Databases") không phải bản sao lưu đầy đủ. Với các file đó, hãy dùng **File nén website và bản dump SQL**.

### aaPanel {#aapanel}

1. Mở **Website**, bấm **Backup** ở dòng của website rồi **Backup now**. aaPanel ghi file `/www/backup/site/DOMAIN/web_DOMAIN_YYYYMMDD_HHMMSS_xxxxxx.tar.gz` (bản cũ: `/www/backup/site/`).
2. Mở **Database**, bấm **Backup** ở dòng của database rồi **Backup now**. aaPanel ghi file `db_NAME_YYYYMMDD_HHMMSS_mysql_data.sql.gz` trong `/www/backup/database/`.

File nén website có thư mục của website ở cấp trên cùng; Nefo nhập những gì nằm bên trong thư mục đó. Nefo cũng đọc thư mục cũ (`/www/wwwroot/DOMAIN`) từ file `.user.ini` của website và thay nó trong database của WordPress.

### HestiaCP, CyberPanel và server khác {#hestiacp-cyberpanel-and-other-servers}

Bản sao lưu riêng của các panel này gói mỗi website trong file nén riêng (HestiaCP nén bằng zstd), nên hãy tạo một file nén thường qua SSH trên server cũ:

```bash
# File của website: thư mục chứa index.php hoặc index.html
tar czf /root/site.tar.gz -C /home/USER/web/example.com public_html
# Từng database
mysqldump --single-transaction --routines --triggers DBNAME | gzip > /root/DBNAME.sql.gz
```

Trên HestiaCP thư mục là `/home/USER/web/DOMAIN/public_html`, trên CyberPanel là `/home/DOMAIN/public_html`, trên server thường hay là `/var/www/html`. File nén có thư mục `public_html` ở cấp trên cùng cũng được: Nefo nhập những gì nằm bên trong.

## Đưa file nén lên server này {#get-the-archive-onto-this-server}

Một trong các cách sau:

- **SFTP hoặc scp** vào một thư mục của root, ví dụ `scp backup.tar.gz root@SERVER-MOI:/root/`.
- **Tệp tin**: tải file lên, hoặc dùng **Tải từ URL** (xem [Tệp tin](./files)).
- **Một địa chỉ web**: nhập địa chỉ `http` hoặc `https` khi nhập website. Nefo tải file về `/var/tmp/nefo-import` trên ổ đĩa của server này và xóa nó sau khi nhập xong. Địa chỉ thuộc mạng nội bộ của server này bị từ chối.

Nefo đọc file nén ở bất kỳ đâu, trừ thư mục hệ thống (`/etc`, `/proc`, `/sys`, `/dev`, `/run`, `/boot`) và dữ liệu của chính nó. Ổ đĩa cần đủ chỗ cho file của các website và khoảng gấp đôi dung lượng các bản dump; kế hoạch nhập cho biết dung lượng cần và dung lượng còn trống.

## Các bước nhập {#import-step-by-step}

1. Mở **Website** và bấm **Nhập** cạnh **Thêm website**.
2. Chọn **Bản sao lưu đầy đủ của cPanel** hoặc **File nén website và bản dump SQL**.
3. Nhập đường dẫn của file (**Chọn file** liệt kê các thư mục và file nén trên server này) hoặc địa chỉ web của nó. Với file nén website, thêm các bản dump SQL.
4. Bấm **Phân tích**. Nefo đọc toàn bộ file nén một lần mà không giải nén; bản sao lưu lớn mất vài phút. **Dừng** để dừng lại.
5. Xem kế hoạch và chọn những gì cần chuyển sang (xem bên dưới). Bấm **Nhập**.
6. Theo dõi nhật ký. Các website mới tạm đóng (lỗi 503) cho tới khi nhập xong. **Dừng nhập** sẽ xóa những gì đã tạo tới lúc đó.
7. Khi xong, trang liệt kê các website mới với **Mở website** và **Quản lý**, các database, và những điều nên kiểm tra.

Nếu bạn rời trang, việc nhập vẫn tiếp tục; mở lại **Website → Nhập** để xem. File nén đã phân tích được ghi nhớ trong sáu giờ.

## Những gì được nhập từ cPanel {#what-is-imported-from-cpanel}

| Trong bản sao lưu | Trên Nefo |
|---|---|
| Tên miền chính, từng addon domain và từng subdomain | Mỗi cái một website. Subdomain nội bộ mà cPanel tạo cho addon domain (`addon.main.com`) bị bỏ qua. |
| Tên miền parked (alias) | Tên miền phụ của website tên miền chính, kèm `www.` nếu server cũ có. |
| Từng thư mục gốc (`public_html`, `public_html/blog`, `addon.com`…) | Thư mục `public` của website. Thư mục của tên miền khác được nhập nằm bên trong chỉ đi vào website của tên miền đó. |
| Phiên bản PHP (`ea-php81`) | Cùng phiên bản nếu nó được cài ở đây, nếu không thì phiên bản mặc định của server này. Website không có file PHP thành website tĩnh. |
| `mysql/DB.sql` | Database cùng tên nếu tên đó còn trống ở đây, nếu không thì `TEN_2`, gắn với website dùng nó. |
| User database và mật khẩu (`mysql.sql`) | Mỗi database có một user: user mà `wp-config.php` của WordPress ghi, nếu không thì một user có quyền với nó. User giữ mật khẩu cũ: MariaDB nhận nguyên mã băm mật khẩu cũ, hoặc Nefo dùng mật khẩu trong `wp-config.php`. Nếu không được, user có mật khẩu mới. |
| WordPress | Nhận ra qua `wp-config.php`. Nefo đánh dấu website là WordPress, áp dụng quy tắc rewrite của WordPress, trỏ `wp-config.php` tới tên database, user hoặc mật khẩu mới nếu có thay đổi, và thay thư mục cũ (`/home/USER/public_html`) bằng thư mục mới trong database. |
| Chứng chỉ (`apache_tls/`, `sslcerts/`, `ssl/`) | Được cài làm chứng chỉ riêng khi bạn chọn: chỉ khi khóa của nó có trong bản sao lưu, nó còn hạn hôm nay và bao gồm mọi tên của website. Nếu không, hãy lấy chứng chỉ Let's Encrypt sau khi nhập. |
| Cron job (`cron/USER`) | Được hiển thị, và chỉ được cài khi bạn chọn: chạy bằng user riêng của website (không bao giờ là root), với thư mục gốc cũ và chương trình PHP của cPanel được thay bằng của server này. Lệnh vẫn ghi thư mục home cũ được đánh dấu. |
| `.htaccess`, `.user.ini` | Thư mục cũ được thay bằng thư mục mới, và các dòng PHP handler của cPanel được bỏ khỏi `.htaccess`. |

Kế hoạch nhập cho biết vì sao một tên miền không nhập được, ví dụ khi một website ở đây đã dùng tên miền đó.

## Những gì không được nhập {#what-is-not-imported}

- **Email**: hộp thư, chuyển tiếp, trả lời tự động và bộ lọc. Nefo không có mail server. Hãy chuyển email sang một dịch vụ email trước khi tắt server cũ. Với email do website gửi đi, xem [Thư và cảnh báo](./mail-alerts).
- **Vùng DNS**: Nefo không chạy DNS server. Hãy trỏ tên miền về server này ở nơi quản lý DNS của nó, hoặc dùng [DNS (Cloudflare)](./dns).
- **Tài khoản FTP**: website ở đây dùng SFTP, mỗi website một tài khoản (tab **SFTP** của website).
- Mailing list, database PostgreSQL, các file trong thư mục home nằm ngoài thư mục gốc của website, và cài đặt riêng của cPanel.

## File nén website {#website-archives}

- Một thư mục duy nhất ở cấp trên cùng của file nén (như bản sao lưu của aaPanel) không được giữ lại: những gì bên trong nó thành website. Thư mục home có `public_html` bên trong cũng được.
- Website có thể là website mới (bạn nhập tên miền, `www.` và phiên bản PHP) hoặc một website trống có sẵn: website PHP hoặc website tĩnh mà thư mục `public` chỉ có trang giữ chỗ.
- Mỗi bản dump SQL thành một database mới với user cùng tên và mật khẩu mới. `wp-config.php` của WordPress được trỏ tới bản dump mà nó ghi (hoặc bản dump duy nhất). Với ứng dụng khác, hãy sửa cấu hình của nó theo database, user và mật khẩu mới (**Xem đăng nhập** ở trang **Cơ sở dữ liệu**).

## Bản dump từ MySQL 8 {#dumps-from-mysql-8}

Bản dump do MySQL 8 tạo ra, như aaPanel và nhiều server cPanel đang dùng, có các collation mà MariaDB không biết, như `utf8mb4_0900_ai_ci`, và các dòng mà MariaDB từ chối. Nefo điều chỉnh chúng trong lúc đọc vào, không thay đổi dữ liệu của bạn:

- Collation của MySQL 8 thành collation gần nhất của MariaDB (`utf8mb4_unicode_520_ci`, `utf8mb4_bin` với loại phân biệt hoa thường, hoặc collation riêng của ngôn ngữ, như `utf8mb4_vietnamese_ci`).
- Các thiết lập chỉ MySQL 8 hiểu, như `DEFAULT ENCRYPTION='N'` và `SRID`, thành chú thích thường.
- `SET @@GLOBAL.GTID_PURGED` và các dòng `SQL_LOG_BIN` quanh nó bị bỏ qua.
- `CREATE DATABASE` và `USE` cho chính database của bản dump bị bỏ qua, để dữ liệu đi vào database mới.

Văn bản bên trong dữ liệu của bạn không bao giờ bị đổi. Nhật ký cho biết bao nhiêu dòng đã được điều chỉnh.

## Cách Nefo xử lý file nén {#how-nefo-treats-archives}

File nén từ server khác có thể chứa bất cứ thứ gì, nên Nefo kiểm tra từng mục:

- Tên có `..` hoặc đường dẫn tuyệt đối, liên kết tượng trưng, liên kết cứng, file thiết bị và FIFO bị bỏ qua, và nhật ký cho biết bao nhiêu.
- File mất setuid và setgid, và quyền ghi của nhóm và người khác. Mọi thứ thuộc user Linux riêng của website.
- File chỉ được ghi vào trong thư mục của website: không gì có thể nằm ra ngoài.
- File lớn hơn 8 GB bị bỏ qua; file nén giải ra lớn hơn rất nhiều so với chính nó (bom nén) bị từ chối.
- SQL chạy bằng một tài khoản database tạm chỉ được dùng database mới, không bao giờ bằng root.

Nếu việc nhập lỗi hoặc bị dừng, Nefo xóa các website và database nó đã tạo. Website nào không xóa được vẫn bị tạm đóng, kèm ghi chú rằng việc nhập chưa xong.

## Sau khi nhập {#after-the-import}

1. **Thử trước khi đổi DNS**: trỏ tên miền về server này trong file `hosts` trên máy của bạn rồi mở các website.
2. **DNS**: trỏ bản ghi A (và AAAA) của từng tên miền về server này.
3. **HTTPS**: khi DNS đã trỏ về đây, lấy chứng chỉ Let's Encrypt ở tab **SSL / HTTPS** của từng website. Nefo không gia hạn chứng chỉ đã nhập. Xem [Chứng chỉ SSL](./ssl).
4. **PHP**: kiểm tra phiên bản và cài đặt PHP của từng website ở tab **PHP**.
5. **Cron job**: thêm các cron job chưa nhập ở trang **Cron job**.
6. **Email**: đảm bảo email đã chạy ở nơi khác trước khi tắt server cũ.

## Khi có sự cố {#if-something-goes-wrong}

- **"không phải bản sao lưu đầy đủ của cPanel"**: file nén không có `userdata/main`. Đó là bản sao lưu từng phần hoặc file nén thường: hãy chọn **File nén website và bản dump SQL**.
- **"dùng một database khác"**: bản dump ghi vào database thứ hai. Hãy xuất từng database riêng, không dùng `--databases` hay `--all-databases`.
- **"không đủ dung lượng ổ đĩa"**: dọn bớt dung lượng, hoặc mỗi lần nhập ít website hơn.
- **Website WordPress báo "Error establishing a database connection"**: mở `wp-config.php` trong **Tệp tin** và so `DB_NAME`, `DB_USER` và `DB_PASSWORD` với **Xem đăng nhập** ở trang **Cơ sở dữ liệu**.
