---
title: Phần mềm
description: Cài và gỡ web server, các phiên bản PHP, MariaDB, Redis, Adminer, Node.js, Python, Docker, tường lửa web và ClamAV, cùng quản lý extension PHP và thiết lập php.ini.
---

# Phần mềm

Trang **Phần mềm** liệt kê mọi thành phần tùy chọn mà máy chủ có thể chạy, mỗi mục có nút **Cài** và **Gỡ cài đặt**. Máy chủ nhỏ sẽ luôn nhẹ nếu chỉ chạy những gì website thực sự dùng. Trang này hướng dẫn: những gì cài được, khi nào việc gỡ bị từ chối, gỡ cài đặt giữ lại gì, cùng các thiết lập của PHP, Node.js, Python và Redis.

Việc cài và gỡ chạy dưới dạng tác vụ có log trực tiếp. Panel chỉ chạy một tác vụ tại một thời điểm. Phần mềm được lấy từ kho phần mềm của hệ điều hành, trừ khi trang này ghi khác.

## Những gì cài được

Mỗi thẻ cho biết thành phần đó tốn khoảng bao nhiêu RAM khi chạy. Đầu trang cộng tổng các mục đã cài.

| Mục | Thành phần | RAM, ước tính |
|---|---|---|
| **Nginx, Apache hoặc OpenLiteSpeed** | Nginx | khoảng 15 MB |
| | Apache | khoảng 40 MB |
| | OpenLiteSpeed | khoảng 26 MB |
| **Các phiên bản PHP** | PHP 7.4, 8.0, 8.1, 8.2, 8.3, 8.4 và 8.5 | khoảng 25 MB mỗi bản |
| **Cơ sở dữ liệu và bộ nhớ đệm** | MariaDB | dung lượng buffer pool cộng khoảng 100 MB (xem [Cơ sở dữ liệu](./databases#cach-mariadb-đuoc-tinh-chinh)) |
| | Redis | tối đa 64, 128 hoặc 256 MB, tùy RAM của máy chủ |
| | Adminer (giao diện database) | không tốn RAM khi không dùng |
| **Môi trường chạy** | Node.js | không tốn RAM khi không dùng |
| | Ứng dụng Python | không tốn RAM khi không dùng |
| **Container** | Docker | khoảng 90 MB, cộng với các container của bạn |
| **Bảo mật** | Tường lửa web (ModSecurity) | khoảng 40 MB |
| | Phần mềm diệt virus ClamAV | khoảng 15 MB, và cần khoảng 1,4 GB RAM trống khi quét |

## Cài một thành phần

1. Vào **Phần mềm**.
2. Chọn **Cài** trên thẻ của thành phần đó.
3. Theo dõi log. Khi tác vụ xong, thẻ hiện **Đã cài** kèm phiên bản.

Một số thẻ sau đó có thêm liên kết: **Mở Cơ sở dữ liệu**, **Mở Redis**, **Mở Adminer** hoặc **Mở Docker**.

## Gỡ một thành phần

1. Chọn **Gỡ cài đặt** trên thẻ của thành phần. Với một phiên bản PHP, nút này nằm trong menu của phiên bản đó.
2. Đọc phần giải thích trong hộp thoại về những gì việc gỡ sẽ làm.
3. Với MariaDB và Docker, bạn có thể đánh dấu ô để xóa luôn dữ liệu. Khi đó bạn phải gõ tên thành phần để xác nhận.
4. Chọn **Gỡ cài đặt**.

### Khi việc gỡ bị từ chối

Panel không bao giờ gỡ phần mềm đang được dùng. Khi một thành phần chưa gỡ được, thẻ của nó hiện lý do thay vì một nút bấm được.

| Thành phần | Bị từ chối khi |
|---|---|
| Web server | Còn bất kỳ website nào, trong panel hoặc dưới dạng tệp cấu hình trong thư mục site của web server |
| Một phiên bản PHP | Có website đang dùng nó, hoặc đó là phiên bản PHP duy nhất (web server và Adminer cần ít nhất một bản) |
| MariaDB | MariaDB còn bất kỳ database nào ngoài các database hệ thống của nó. Panel hỏi chính MariaDB, nên database tạo bằng tay hoặc bằng công cụ khác cũng được tính. Nếu MariaDB đang dừng và thư mục dữ liệu đã bị chuyển chỗ, panel không kiểm tra được nên từ chối |
| Node.js | Có website ứng dụng dùng Node.js mặc định của máy chủ, hoặc `node` trong `/usr/local/bin` không do panel cài |
| Một dòng ở **Các phiên bản Node.js** | Đó là bản mặc định của máy chủ, hoặc có website ứng dụng đã chọn nó |
| Ứng dụng Python | Có website ứng dụng chạy Python |
| Docker | Có website chạy ứng dụng Docker, còn dự án Compose, hoặc Docker còn container, đang chạy hay đã dừng |

Redis, Adminer, tường lửa web và ClamAV luôn gỡ được.

### Gỡ cài đặt giữ lại gì

| Thành phần | Được giữ | Bị gỡ |
|---|---|---|
| Web server | Thư mục website trong `/var/www`, chứng chỉ, PHP | Web server, cấu hình panel tạo cho nó, Adminer, và luật tường lửa cho cổng 80 và 443 |
| Phiên bản PHP | | Phiên bản đó, các extension và thư viện chỉ chúng dùng |
| MariaDB | Thư mục dữ liệu `/var/lib/mysql`, trừ khi bạn đánh dấu **Xóa luôn thư mục dữ liệu (/var/lib/mysql)** và gõ `mariadb`. Cài lại MariaDB sẽ dùng lại thư mục này | Gói server và client, cùng các thiết lập panel tạo cho MariaDB |
| Redis | | Redis, extension PHP của nó, và mọi dữ liệu trong Redis. PHP-FPM khởi động lại |
| Adminer | Database của bạn | Các tệp của Adminer và site nội bộ của nó |
| Node.js | Các phiên bản Node.js khác. Cả thư mục của bản mặc định, nếu có website ứng dụng đã chọn phiên bản đó | `node`, `npm` và `npx` trong `/usr/local/bin`, cùng bản Node.js mà chúng trỏ tới |
| Ứng dụng Python | Bản thân `python3`, vì hệ thống cần nó, và môi trường ảo trong thư mục của các ứng dụng cũ | Gói venv và pip |
| Docker | Image, volume và build cache trong `/var/lib/docker`, trừ khi bạn đánh dấu **Xóa luôn mọi image, volume và build cache (/var/lib/docker)** và gõ `docker`. Cài lại Docker sẽ dùng lại chúng | Docker và plugin Compose |
| Tường lửa web | Nhật ký của tường lửa | ModSecurity và bộ luật OWASP. Web server khởi động lại |
| ClamAV | | ClamAV và cơ sở dữ liệu virus của nó |

::: danger
Xóa dữ liệu của MariaDB hoặc Docker là không thể hoàn tác. Hãy chắc chắn rằng bạn đã sao lưu những gì còn cần.
:::

## Menu chỉ hiện những gì đã cài

Menu bên trái ẩn các trang của phần mềm chưa cài:

- **Docker** hiện ra khi đã cài Docker.
- **WordPress** hiện ra khi đã có một website WordPress.
- Trên trang **Cơ sở dữ liệu**, thẻ **Redis** hiện ra khi đã cài Redis, còn **Truy vấn chậm** hiện ra khi đã cài MariaDB.

**Website**, **Cơ sở dữ liệu** và **Phần mềm** luôn hiện, vì đó là nơi bạn cài các thành phần. Trang bạn đang mở vẫn được hiện.

## Web server

Panel chỉ chạy một web server tại một thời điểm và cấu hình mọi website cho web server đó:

- Nginx nhẹ nhất. PHP chạy bằng PHP-FPM.
- Apache đọc được tệp `.htaccess` đi kèm một số ứng dụng PHP, và tốn RAM hơn một chút.
- OpenLiteSpeed đọc được quy tắc rewrite trong tệp `.htaccess` và có sẵn LiteSpeed Cache cho WordPress. Nó được cài từ kho phần mềm riêng của LiteSpeed.

Cài web server cũng cài phiên bản PHP của hệ điều hành cùng PHP-FPM. Nếu tường lửa ufw đang bật, panel mở cổng 80 và 443, và cả 443/udp cho HTTP/3 với OpenLiteSpeed.

Panel từ chối cài web server thứ hai khi đã có một web server khác, vì cả hai đều cần cổng 80 và 443.

Để chuyển sang web server khác:

1. Xóa hết website trước. Hãy sao lưu nếu sau này còn cần.
2. Chọn **Gỡ cài đặt** trên web server hiện tại và gõ tên của nó (`nginx`, `apache` hoặc `openlitespeed`) để xác nhận.
3. Cài web server mới.

Xem [Website](./websites) để tạo và cấu hình site.

## Các phiên bản PHP

Bạn có thể cài nhiều phiên bản PHP song song và chọn phiên bản cho từng website ở **Website** → (site) → **PHP**.

- Phiên bản đi kèm hệ điều hành được lấy từ kho phần mềm của hệ điều hành. Các phiên bản khác được lấy từ kho `ondrej/php` trên Ubuntu, hoặc `packages.sury.org` trên Debian. Panel hỏi trước khi thêm kho đó vào máy chủ.
- Mỗi phiên bản được cài kèm các extension: fpm, cli, mysql, curl, gd, mbstring, xml, zip, intl, bcmath và opcache. Nếu đã cài Redis, extension redis cũng được thêm.
- Panel chuyển pool PHP-FPM mặc định của phiên bản sang chế độ khởi tạo worker theo nhu cầu (ondemand), nên một phiên bản không website nào dùng gần như không tốn RAM.

Chọn **Quản lý** trên một phiên bản đã cài để mở phần **Extension** và **Cài đặt** của nó.

### Extension

Phần **Extension** liệt kê các extension tùy chọn có gói cho phiên bản này. Chọn **Cài** hoặc **Gỡ**, và PHP-FPM sẽ được nạp lại.

Các extension tùy chọn gồm: apcu, gmp, igbinary, imagick, imap, ldap, memcached, mongodb, msgpack, pgsql, redis, soap, sqlite3, ssh2, tidy, xdebug, xsl và yaml.

::: warning
xdebug làm chậm mọi site dùng phiên bản PHP đó. Đừng để nó trên máy chủ đang chạy thật.
:::

### Thiết lập php.ini

Phần **Cài đặt** đặt giá trị php.ini cho mọi website dùng phiên bản PHP này. Để trống một ô để dùng mặc định của PHP, rồi chọn **Lưu**.

| Thiết lập | Mặc định | Giá trị cho phép |
|---|---|---|
| `date.timezone` | UTC | Tên múi giờ, ví dụ `Asia/Ho_Chi_Minh` |
| `max_input_vars` | 1000 | 100 đến 100000 |
| `max_input_time` | 60 | -1 đến 3600 |
| `default_socket_timeout` | 60 | 1 đến 600 |
| `session.gc_maxlifetime` | 1440 | 60 đến 2592000 |
| `realpath_cache_size` | 4096K | Một số, có thể kèm K hoặc M |
| `display_errors` | Off | On hoặc Off |
| `expose_php` | Off | On hoặc Off |
| `allow_url_fopen` | On | On hoặc Off |
| `short_open_tag` | Off | On hoặc Off |
| `opcache.enable` | 1 | 1 hoặc 0 |
| `opcache.memory_consumption` | 128 | 32 đến 2048 |
| `opcache.max_accelerated_files` | 10000 | 1000 đến 1000000 |
| `opcache.validate_timestamps` | 1 | 1 hoặc 0 |
| `opcache.revalidate_freq` | 2 | 0 đến 3600 |

Các giá trị được ghi vào `/etc/php/<phiên-bản>/fpm/conf.d/99-panel.ini` và được kiểm tra bằng `php-fpm -t`. Nếu PHP-FPM từ chối, tệp cũ được đặt lại và không có gì thay đổi. Giới hạn RAM, dung lượng upload và thời gian chạy được đặt riêng cho từng website, trong phần cài đặt của website đó.

## Node.js

Thẻ **Node.js** cài bản LTS mới nhất từ nodejs.org, có kiểm tra mã SHA-256 được công bố. Node.js được đặt trong `/opt/node-v<phiên-bản>-linux-<kiến-trúc>`, còn `node`, `npm` và `npx` được liên kết vào `/usr/local/bin`. Đây là Node.js mặc định của máy chủ, dùng cho các website ứng dụng không chọn phiên bản riêng.

### Nhiều phiên bản Node.js

Khi đã cài Node.js, thẻ **Các phiên bản Node.js** liệt kê ba dòng LTS mới nhất và mọi dòng đã cài. Chọn **Cài** để thêm một dòng song song với các dòng khác. Việc này không đổi bản mặc định.

Chọn dòng Node.js cho một website ứng dụng ở **Website** → (site) → **Runtime**. Dòng đang được website ứng dụng dùng, và bản mặc định, không gỡ được từ thẻ này. Hãy gỡ bản mặc định ở thẻ **Node.js**.

Danh sách được lấy từ nodejs.org, nên máy chủ cần truy cập Internet để hiện nó.

::: warning
Gỡ Node.js mặc định cũng làm hỏng các cron job và chương trình nền có gọi `node`.
:::

## Python

Thẻ **Ứng dụng Python** cài `python3-venv` và `python3-pip`, để website ứng dụng chạy được Python (Django, Flask, FastAPI…). Mỗi website ứng dụng Python có môi trường ảo riêng trong `app/.venv`. Panel không cài công cụ build như `build-essential` và `python3-dev`, nên gói nào không có sẵn wheel và phải biên dịch sẽ cài không được cho đến khi bạn tự cài các công cụ đó bằng `apt`.

Gỡ cài đặt sẽ gỡ venv và pip. Bản thân `python3` vẫn được giữ, vì hệ thống cần nó.

## Redis

Redis là bộ nhớ đệm trong RAM. WordPress có thể dùng nó với một plugin Redis object cache.

Khi bạn cài Redis, panel:

- cài `redis-server` và extension redis cho mọi phiên bản PHP đã cài;
- giới hạn RAM của Redis cho phù hợp với máy chủ: 64 MB khi RAM dưới 1,5 GB, 128 MB từ 1,5 GB, 256 MB từ 3 GB;
- đặt chính sách `allkeys-lru`, để Redis bỏ các key ít được dùng gần đây nhất khi đầy, như một bộ nhớ đệm;
- để Redis lắng nghe trên `127.0.0.1:6379`, chỉ truy cập được từ chính máy chủ này.

Để xem bên trong Redis, vào **Cơ sở dữ liệu** → **Redis**. Xem trang [Cơ sở dữ liệu](./databases#trinh-duyet-key-redis).

Trước khi gỡ Redis, hãy tắt plugin bộ nhớ đệm trên các website. Chúng vẫn chạy được khi không có bộ nhớ đệm, chỉ chậm hơn một chút.

## MariaDB và Adminer

MariaDB là máy chủ cơ sở dữ liệu, được tinh chỉnh theo RAM của máy chủ và chỉ truy cập được từ chính máy chủ. Adminer giúp xem database trên trình duyệt và chỉ truy cập được qua bước đăng nhập của panel. Adminer cần web server và PHP. Xem trang [Cơ sở dữ liệu](./databases).

## Docker

Thẻ **Docker** cài `docker.io` từ kho phần mềm của hệ điều hành, kèm plugin Compose nếu có. Sau đó mục **Docker** hiện ra trên menu.

Cổng do container Docker mở sẽ bỏ qua tường lửa ufw. Vì vậy panel chỉ mở cổng container trên 127.0.0.1, và bạn truy cập chúng qua một website. Xem trang [Docker](./docker).

## Tường lửa web và ClamAV

- **Tường lửa web (ModSecurity)** cài ModSecurity cùng bộ luật OWASP Core Rule Set. Nó cần web server. Sau khi cài, bật cho từng website ở **Website** → (site) → **Tường lửa WAF**.
- **Phần mềm diệt virus ClamAV** là bộ quét bổ sung, tùy chọn, cho việc quét mã độc. Cơ sở dữ liệu virus của nó được tải ngay sau khi cài, nên vài phút sau việc quét mới dùng được nó. ClamAV hợp với máy chủ từ 2 GB RAM trở lên.

Xem trang [Bảo mật](./security) cho cả hai.
