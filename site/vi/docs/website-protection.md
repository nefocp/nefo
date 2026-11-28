---
title: Chặn quốc gia và khóa file
description: Chặn hoặc chỉ cho phép khách truy cập theo quốc gia trên một website bằng dữ liệu quốc gia miễn phí của DB-IP, và khóa code của website ở chế độ chỉ đọc đối với PHP của nó, kèm cảnh báo khi file PHP thay đổi.
---

# Chặn quốc gia và khóa file

Từ v0.24.0, Nefo có hai lớp bảo vệ cho từng website. **Chặn quốc gia** từ chối khách từ các quốc gia bạn liệt kê, hoặc chỉ cho khách từ các quốc gia đó vào, bằng dữ liệu quốc gia miễn phí của DB-IP, trên Nginx, Apache và OpenLiteSpeed. Khóa file khiến code của website thành chỉ đọc đối với PHP và tài khoản SFTP của chính website, trừ những thư mục mà website phải ghi vào, như thư mục upload.

Cả hai nằm trong cài đặt của website, ở nhóm **Bảo mật**: **Website** → (website) → **Chặn quốc gia** và **Khóa file**. Khi đang tắt, cả hai không tốn gì: không tải gì về, không thêm tiến trình nào, không nạp gì vào web server. Về tường lửa ứng dụng web, giới hạn request và trình quét mã độc, xem [Bảo mật](./security#website-security).

## Làm sao để chặn quốc gia trên một website? {#block-or-allow-countries}

1. Mở **Website**, bấm vào website, rồi mở tab **Chặn quốc gia**.
2. Chọn **Chặn các quốc gia bên dưới** hoặc **Chỉ cho phép các quốc gia bên dưới**.
3. Gõ tên quốc gia (theo ngôn ngữ của panel) hoặc mã hai chữ cái của nó vào ô tìm kiếm, rồi bấm vào quốc gia để thêm. Muốn bỏ một quốc gia, bấm dấu × trên thẻ của nó.
4. Bấm **Lưu**.

Lần đầu tiên có website trên máy chủ dùng chặn quốc gia, Nefo tải cơ sở dữ liệu quốc gia từ db-ip.com (khoảng 5 MB) trước khi lưu, việc này mất vài giây. Muốn tắt lại, chọn **Tắt: mọi quốc gia đều vào được** rồi bấm **Lưu**.

Chặn quốc gia dùng được với mọi loại website, kể cả website reverse proxy. Mỗi website có cài đặt riêng.

Khách bị chặn nhận lỗi **403 Forbidden**. Muốn hiện trang riêng, hãy đặt file `403.html` trong thư mục `public` của website và trỏ trang lỗi 403 của web server tới nó ở **Website** → (website) → **Cấu hình Nginx** (Nginx: `error_page 403 /403.html;`; Apache: `ErrorDocument 403 /403.html`, cách này cũng dùng được trong `.htaccess`; OpenLiteSpeed: một khối `errorpage 403`). Khách bị chặn vẫn tải được chính `/403.html`.

### Những gì không bao giờ bị chặn {#what-is-never-blocked}

- Các lần kiểm tra của Let's Encrypt (`/.well-known/acme-challenge/`), để chứng chỉ vẫn được gia hạn.
- Webhook triển khai Git (`/.panel/deploy/…`). Panel kiểm tra chữ ký của webhook.
- `/403.html`.
- Địa chỉ nội bộ và các địa chỉ dành riêng khác (`10.0.0.0/8`, `192.168.0.0/16`, loopback, v.v.).
- Request của chính máy chủ, như khi WordPress tự gọi chính nó để chạy các tác vụ theo lịch.

Bản thân panel không phải là website: nó có cổng riêng và không bao giờ bị chặn theo quốc gia. Muốn giới hạn ai vào được panel, hãy dùng tường lửa (xem [Bảo mật](./security#firewall-ufw)).

### Khi đứng sau Cloudflare {#behind-cloudflare}

Khi bản ghi DNS của website đi qua proxy của Cloudflare, mọi request đều đến từ địa chỉ của Cloudflare.

- **Nginx và Apache**: với request đến từ địa chỉ của Cloudflare, Nefo tra địa chỉ của khách mà Cloudflare gửi trong header `CF-Connecting-IP`. Request từ nơi khác được tra theo chính địa chỉ của nó, nên máy khách không giả được header này. Trên Apache, log truy cập và PHP của website khi đó cũng thấy địa chỉ của khách với các request đi qua Cloudflare.
- **OpenLiteSpeed**: với request từ địa chỉ của Cloudflare, Nefo dùng quốc gia mà Cloudflare gửi trong header `CF-IPCountry`. Hãy bật **IP Geolocation** trong Cloudflare (mục cài đặt **Network** của zone). Nếu không bật, khách đi qua Cloudflare sẽ không bị chặn, và với **Chỉ cho phép các quốc gia bên dưới** thì họ được cho vào.

Nefo có sẵn danh sách địa chỉ của Cloudflare và cập nhật danh sách này từ Cloudflare mỗi tháng, cùng lúc với cơ sở dữ liệu quốc gia.

### Giới hạn theo web server {#limits-per-web-server}

| Web server | Cách hoạt động | Giới hạn |
|---|---|---|
| Nginx | Module `geo` có sẵn của Nginx, chỉ được nạp khi có website dùng chặn quốc gia, và chỉ chứa các quốc gia mà các website liệt kê | Không có. Bộ nhớ: khoảng 2 đến 5 MB cho một quốc gia, khoảng 40 MB cho danh sách lớn (Hoa Kỳ, Đức và Nhật Bản cộng lại) |
| Apache | Apache kiểm tra lần lượt từng dải địa chỉ của các quốc gia trong danh sách ở mỗi request | Tối đa 50.000 dải địa chỉ cho mỗi website, nên các quốc gia lớn bị từ chối (riêng Hoa Kỳ có khoảng 270.000 dải). Trung Quốc (15.000 dải) chiếm khoảng 2,5 MB |
| OpenLiteSpeed | Tính năng tra GeoIP riêng của OpenLiteSpeed trong file cơ sở dữ liệu của DB-IP | Không có. Dưới 1 MB |

Một danh sách chứa được tối đa 100 quốc gia. Trên Apache, tab cho biết website đang dùng bao nhiêu dải địa chỉ so với giới hạn. Nếu danh sách quá lớn, Apache từ chối và thông báo nêu tên các quốc gia không vừa, quốc gia lớn nhất trước, kèm số dải địa chỉ của chúng. Hãy bỏ các quốc gia đó ra, hoặc dùng Nginx hay OpenLiteSpeed, hai web server xử lý được mọi quốc gia.

### Dữ liệu quốc gia {#the-country-data}

Dữ liệu là cơ sở dữ liệu miễn phí **IP to Country Lite** của DB-IP, IP Geolocation by [DB-IP](https://db-ip.com), theo giấy phép Creative Commons Attribution 4.0. Dữ liệu bao gồm cả IPv4 và IPv6.

- Nefo tải dữ liệu qua HTTPS vào lần đầu có website dùng chặn quốc gia.
- Trước khi thay bản cũ, Nefo kiểm tra file là một cơ sở dữ liệu đầy đủ: định dạng, kích thước, và có liệt kê hơn 200 quốc gia.
- Nefo cập nhật dữ liệu mỗi tháng một lần (DB-IP phát hành bản mới vào đầu mỗi tháng). Tab cho biết bản đang dùng.
- Khi không có website nào dùng chặn quốc gia, không có gì được tải về và web server không nạp gì cả.

Cơ sở dữ liệu nằm trong `/var/lib/panel-geoip/`. Mọi user đều đọc được, vì web server cần đọc nó: đây là dữ liệu công khai.

## Làm sao để file của website thành chỉ đọc? {#lock-a-website-s-files}

Tab **Khóa file** khiến code của website thành chỉ đọc đối với PHP, ứng dụng và tài khoản SFTP của website. Khi đó một plugin có lỗ hổng bảo mật không còn sửa được file của website hay thêm file mới, trừ trong các thư mục bạn cho phép, như thư mục upload và cache. Các panel khác gọi tính năng này là chống sửa đổi (tamper protection).

1. Mở **Website**, bấm vào website, rồi mở tab **Khóa file**.
2. Kiểm tra danh sách trong **Thư mục được ghi**. Với website WordPress, Joomla, Laravel, Drupal, Nextcloud, OpenCart, Moodle, Symfony và CodeIgniter, panel tự điền các thư mục của ứng dụng; **Dùng các thư mục của WordPress** (hoặc của ứng dụng mà website dùng) điền lại các thư mục đó. Ghi mỗi dòng một thư mục, tính từ thư mục của website, ví dụ `public/wp-content/uploads`. `*` thay cho một tên thư mục: `public/sites/*/files`. `tmp` và `logs` luôn ghi được.
3. Muốn được báo khi file PHP vẫn bị thay đổi, hãy chọn **Báo cho tôi khi file PHP thay đổi** (xem [bên dưới](#alerts-when-php-files-change)).
4. Bấm **Khóa file**.

Tab khi đó hiện **Đã khóa từ** kèm ngày. Về sau, muốn đổi thư mục được ghi hay cảnh báo, hãy sửa rồi bấm **Lưu**: file được khóa lại theo danh sách mới. **Mở khóa file** trả mọi thứ lại cho user của website, với đúng chủ sở hữu, group và quyền mà từng file có trước đó.

Tab **Khóa file** không hiện với ứng dụng Docker, vì file của chúng thuộc về các container, và với website reverse proxy, vì loại này không phục vụ file nào.

Các thư mục có sẵn:

| Ứng dụng | Thư mục được ghi |
|---|---|
| WordPress | `public/wp-content/uploads`, `public/wp-content/cache`, `public/wp-content/upgrade` |
| Joomla | `public/images`, `public/media`, `public/cache`, `public/administrator/cache`, `public/administrator/logs`, `public/tmp` |
| Laravel | `storage`, `bootstrap/cache` |
| Drupal | `public/sites/*/files` |
| Nextcloud | `data`, `public/config` |
| OpenCart | `storage`, `public/image/cache`, `public/image/catalog` |
| Moodle | `moodledata` |
| Symfony | `var` |
| CodeIgniter | `writable` |

Thư mục được ghi nào chưa có sẽ được tạo, thuộc user của website, khi bạn khóa file, nếu thư mục cha của nó đã có. Tab liệt kê các thư mục không tìm thấy.

### Những gì thay đổi khi file bị khóa {#what-changes-while-the-files-are-locked}

- PHP, ứng dụng và tài khoản SFTP của website không sửa, thêm, đổi tên hay xóa được gì ngoài các thư mục được ghi, cũng không đổi được quyền ở đó. Upload và cache vẫn hoạt động.
- WordPress không tự cập nhật, không cài hay cập nhật plugin và giao diện từ trang quản trị của nó, và không ghi lại được `wp-config.php` hay `.htaccess`. Hãy cập nhật WordPress từ tab **WordPress** (xem bên dưới).
- Website vẫn chạy: web server và PHP vẫn đọc được mọi file mà chúng đọc được trước đó. File mà chỉ user của website đọc được, như `wp-config.php` (quyền `0600`), vẫn không đọc được với web server.

Cách hoạt động: các file và thư mục mà user của website sở hữu giờ thuộc về root, không ai khác có quyền ghi. File mà group của website đọc được vẫn giữ group đó. File mà chỉ chủ sở hữu đọc được thì nhận group khóa riêng của website (`panel-lock-<uid>`, thành viên duy nhất là user của website). Không dùng module kernel, không dùng `chattr`. Những gì việc khóa đã thay đổi được ghi lại trong `/var/lib/nefo/filelock/`, nên khi mở khóa mọi thứ được trả lại đúng như cũ, kể cả sau khi máy bị sập.

### Thao tác của panel trên website đã khóa {#panel-operations-on-a-locked-website}

| Thao tác | Khi đang khóa |
|---|---|
| Tab **WordPress**: **Cập nhật tất cả**, chế độ bảo trì, chế độ debug, tự động cập nhật | Panel mở khóa file, thực hiện thay đổi rồi khóa lại |
| Đẩy bản staging lên website này | Mở khóa để đẩy lên, rồi khóa lại |
| **Triển khai Git** (bằng tay hoặc qua webhook) | Mở khóa để triển khai, rồi khóa lại |
| Khôi phục bản sao lưu của website này (hoặc của một thư mục trong đó) | Mở khóa để khôi phục, rồi khóa lại |
| Quét mã độc: cách ly hoặc khôi phục một file | Mở khóa trong lúc đó, rồi khóa lại |
| Cài phụ thuộc cho ứng dụng (Node.js, Python) | Mở khóa trong lúc đó, rồi khóa lại |
| **Tệp tin** (trình quản lý file): sửa, tải lên, tạo, đổi tên, xóa, giải nén, đổi quyền | Được phép trong các thư mục được ghi và `tmp`, `logs`; bị từ chối ở nơi khác, kèm thông báo rằng file của website đang bị khóa |
| Cài ứng dụng (WordPress hay ứng dụng khác) vào website | Bị từ chối: hãy mở khóa file trước |
| Nhập một website vào website (trống) này | Bị từ chối: hãy mở khóa file trước |
| Nhân bản website, tạo bản staging của nó, sao lưu | Vẫn chạy: các thao tác này chỉ đọc file, kể cả file mà chỉ website đọc được (như `wp-config.php`). Bản sao thuộc về website riêng của nó và không bị khóa |

Sau khi đẩy lên, triển khai hay khôi phục, file mới cũng bị khóa như các file khác; file được đưa vào thư mục được ghi vẫn thuộc về website. Mỗi thao tác mở khóa file đều ghi "Unlocking the files of example.com for this change" và "Locking the files of example.com again" vào nhật ký tác vụ của nó. Nếu khóa lại bị lỗi, tác vụ sẽ báo, và panel thử lại mỗi giờ. Trong lúc một thao tác đang chạy, tab cho biết file đang được mở khóa cho thao tác đó.

::: warning
Khóa file chặn user của chính website, không chặn root. Mọi thứ chạy bằng root, như phiên SSH bằng root hay terminal của panel, vẫn sửa được file. Trước khi gỡ panel, hãy chạy `nefo unlock-files` (xem [Gỡ cài đặt](./uninstall#unlock-the-websites-files)), nếu không file của các website đã khóa sẽ vẫn thuộc về root.
:::

### Cảnh báo khi file PHP thay đổi {#alerts-when-php-files-change}

Với **Báo cho tôi khi file PHP thay đổi**, panel chụp một bản ghi nhận (checksum SHA-256 của mọi file PHP của website, kể cả trong các thư mục được ghi) khi khóa file, và mỗi ngày một lần so các file PHP với bản chụp đó. Khi có file bị sửa, được thêm (file PHP trong `uploads` là dấu hiệu điển hình của việc bị xâm nhập) hoặc bị xóa, panel gửi cảnh báo qua các kênh đã đặt ở **Cài đặt** → **Cảnh báo**, và tab liệt kê các file đó. Panel chỉ cảnh báo lại khi danh sách khác biệt thay đổi.

Nếu chính bạn đã thay đổi các file, hãy bấm **Chụp lại bản mới**. Các thao tác mở khóa rồi khóa lại file, như cập nhật WordPress, tự chụp bản mới.

Mỗi lần kiểm tra hằng ngày đọc tối đa 200.000 file PHP hoặc 2 GB cho mỗi website. Tab sẽ báo khi lần kiểm tra dừng ở giới hạn đó.
