---
title: Phần mềm
description: Cài và gỡ web server, PHP 5.6 đến 8.6, ionCube Loader, MariaDB hoặc MySQL, PostgreSQL, Redis, Memcached, Adminer, phpMyAdmin, Node.js, Python, Java, Docker, tường lửa web và ClamAV, cùng quản lý extension PHP và thiết lập php.ini.
---

# Phần mềm

Nefo không cài gì cho đến khi bạn chọn. Trang **Phần mềm** cài và gỡ từng thành phần tùy chọn, kèm mức RAM ước chừng mà nó dùng: web server, PHP 5.6 đến 8.6 và ionCube Loader, MariaDB hoặc MySQL, PostgreSQL, Redis, Memcached, Node.js, Python, Java, Docker, tường lửa web và ClamAV. Trang đó từ chối gỡ những gì vẫn còn được dùng.

Trang này hướng dẫn: những gì cài được, khi nào việc gỡ bị từ chối, gỡ cài đặt giữ lại gì, cùng các thiết lập của PHP, Node.js, Python, Java và Redis. Máy chủ nhỏ sẽ luôn nhẹ nếu chỉ chạy những gì website thực sự dùng.

Việc cài và gỡ chạy dưới dạng tác vụ có log trực tiếp. Panel chỉ chạy một tác vụ tại một thời điểm. Phần mềm được lấy từ kho phần mềm của hệ điều hành, trừ khi trang này ghi khác.

## Những gì cài được {#what-you-can-install}

Mỗi thẻ cho biết thành phần đó tốn khoảng bao nhiêu RAM khi chạy. Đầu trang cộng tổng các mục đã cài.

| Mục | Thành phần | RAM, ước tính |
|---|---|---|
| **Nginx, Apache hoặc OpenLiteSpeed** | Nginx | khoảng 15 MB |
| | Apache | khoảng 40 MB |
| | OpenLiteSpeed | khoảng 26 MB |
| **Các phiên bản PHP** | PHP 7.4, 8.0, 8.1, 8.2, 8.3, 8.4, 8.5 và 8.6 (và các bản mới hơn khi kho PHP có chúng), và trong **Phiên bản cũ** là 5.6, 7.0, 7.1, 7.2 và 7.3 | khoảng 25 MB mỗi bản |
| | ionCube Loader | không tốn RAM riêng: nó được nạp vào các worker PHP |
| **Cơ sở dữ liệu và bộ nhớ đệm** | MariaDB (bản của hệ điều hành, hoặc 11.4, 11.8 hay 12.3 LTS từ kho của MariaDB), hoặc thay vào đó MySQL 8.4 LTS | MariaDB: dung lượng buffer pool cộng khoảng 100 MB (xem [Cơ sở dữ liệu](./databases#how-mariadb-is-tuned)). MySQL: dung lượng buffer pool cộng khoảng 150 MB, hoặc cộng khoảng 300 MB từ khoảng 3,5 GB RAM (xem [Cơ sở dữ liệu](./databases#how-mysql-is-tuned)) |
| | PostgreSQL | khoảng 30 đến 40 MB khi rảnh, tăng dần tới `shared_buffers` cộng khoảng 40 MB (xem [PostgreSQL](./postgresql#how-postgresql-is-tuned)) |
| | Redis | tối đa 64, 128 hoặc 256 MB, tùy RAM của máy chủ |
| | Memcached | tối đa 64, 128 hoặc 256 MB, tùy RAM của máy chủ; vài MB khi còn trống |
| | Adminer (giao diện database) | không tốn RAM khi không dùng |
| | phpMyAdmin (giao diện database) | không tốn RAM khi không dùng |
| **Môi trường chạy** | Node.js | không tốn RAM khi không dùng |
| | Ứng dụng Python | không tốn RAM khi không dùng |
| | Java (OpenJDK) | không tốn RAM khi không dùng; chương trình Java của một website ứng dụng dùng mức RAM mà heap của nó cần |
| **Container** | Docker | khoảng 90 MB, cộng với các container của bạn |
| **Bảo mật** | Tường lửa web (ModSecurity) | khoảng 40 MB |
| | Phần mềm diệt virus ClamAV | khoảng 15 MB, và cần khoảng 1,4 GB RAM trống khi quét |
| **Công cụ** | Composer | không tốn RAM, chỉ chạy khi được gọi |
| | WP-CLI | không tốn RAM, chỉ chạy khi được gọi |

## Cài một thành phần {#install-an-item}

1. Vào **Phần mềm**.
2. Chọn **Cài** trên thẻ của thành phần đó.
3. Theo dõi log. Khi tác vụ xong, thẻ hiện **Đã cài** kèm phiên bản.

Một số thẻ sau đó có thêm liên kết: **Mở Cơ sở dữ liệu**, **Mở PostgreSQL**, **Mở Redis**, **Mở Memcached**, **Mở Adminer**, **Mở phpMyAdmin** hoặc **Mở Docker**.

## Gỡ một thành phần {#uninstall-an-item}

1. Chọn **Gỡ cài đặt** trên thẻ của thành phần. Với một phiên bản PHP, nút này nằm trong menu của phiên bản đó.
2. Đọc phần giải thích trong hộp thoại về những gì việc gỡ sẽ làm.
3. Với MariaDB, MySQL, PostgreSQL và Docker, bạn có thể đánh dấu ô để xóa luôn dữ liệu. Khi đó bạn phải gõ tên thành phần để xác nhận.
4. Chọn **Gỡ cài đặt**.

### Khi việc gỡ bị từ chối {#when-an-uninstall-is-refused}

Panel không bao giờ gỡ phần mềm đang được dùng. Khi một thành phần chưa gỡ được, thẻ của nó hiện lý do thay vì một nút bấm được.

| Thành phần | Bị từ chối khi |
|---|---|
| Web server | Còn bất kỳ website nào, trong panel hoặc dưới dạng tệp cấu hình trong thư mục site của web server |
| Một phiên bản PHP | Có website đang dùng nó, hoặc đó là phiên bản PHP duy nhất (web server và Adminer cần ít nhất một bản). Một phiên bản còn được hỗ trợ (7.4 trở lên) cũng được giữ lại khi nó là bản duy nhất bên cạnh các [phiên bản cũ](#older-php-versions) |
| Composer | Có script triển khai Git chạy `composer` |
| WP-CLI | Còn website WordPress: bộ công cụ WordPress chạy nó cho các website này |
| MariaDB hoặc MySQL | Nó còn bất kỳ database nào ngoài các database hệ thống của nó. Panel hỏi chính máy chủ cơ sở dữ liệu, nên database tạo bằng tay hoặc bằng công cụ khác cũng được tính. Nếu nó đang dừng và thư mục dữ liệu đã bị chuyển chỗ, panel không kiểm tra được nên từ chối |
| PostgreSQL | PostgreSQL còn bất kỳ database nào ngoài `postgres`, `template0` và `template1`, theo lời chính PostgreSQL, hoặc nó đang dừng nên panel không kiểm tra được |
| Node.js | Có website ứng dụng dùng Node.js mặc định của máy chủ, hoặc `node` trong `/usr/local/bin` không do panel cài |
| Một dòng ở **Các phiên bản Node.js** | Đó là bản mặc định của máy chủ, hoặc có website ứng dụng đã chọn nó |
| Ứng dụng Python | Có website ứng dụng chạy Python |
| Java (OpenJDK) | Có website ứng dụng chạy Java. Gỡ riêng một phiên bản Java bị từ chối khi có website ứng dụng dùng phiên bản đó |
| Docker | Có website chạy ứng dụng Docker, còn dự án Compose, hoặc Docker còn container, đang chạy hay đã dừng |

Redis, Memcached, Adminer, phpMyAdmin, ionCube Loader, tường lửa web và ClamAV luôn gỡ được.

### Gỡ cài đặt giữ lại gì {#what-an-uninstall-keeps}

| Thành phần | Được giữ | Bị gỡ |
|---|---|---|
| Web server | Thư mục website trong `/var/www`, chứng chỉ, PHP | Web server, cấu hình panel tạo cho nó, Adminer, và luật tường lửa cho cổng 80 và 443 |
| Phiên bản PHP | | Phiên bản đó, các extension và thư viện chỉ chúng dùng |
| MariaDB | Thư mục dữ liệu `/var/lib/mysql`, trừ khi bạn đánh dấu **Xóa luôn thư mục dữ liệu (/var/lib/mysql)** và gõ `mariadb`. Cài lại MariaDB sẽ dùng lại thư mục này | Gói server và client, các thiết lập panel tạo cho MariaDB, cùng kho của MariaDB và khóa của kho nếu MariaDB được lấy từ đó |
| MySQL | Thư mục dữ liệu `/var/lib/mysql`, trừ khi bạn đánh dấu **Xóa luôn thư mục dữ liệu (/var/lib/mysql)** và gõ `mysql`. Cài lại MySQL sẽ dùng lại thư mục này | Gói server và client, các thiết lập panel tạo cho MySQL, cùng kho của Oracle và khóa của kho |
| PostgreSQL | Dữ liệu trong `/var/lib/postgresql` và thiết lập cluster trong `/etc/postgresql`, trừ khi bạn đánh dấu **Xóa luôn dữ liệu và thiết lập (/var/lib/postgresql và /etc/postgresql)** và gõ `postgresql`. Các gói được gỡ mà không purge, nên cài lại PostgreSQL sẽ chạy lại đúng cluster đó | Các gói PostgreSQL, cùng kho apt.postgresql.org và khóa của kho (`/etc/apt/sources.list.d/panel-pgdg.list` và `/etc/apt/keyrings/panel-pgdg.gpg`) nếu panel đã thêm chúng |
| Redis | | Redis, extension PHP của nó, và mọi dữ liệu trong Redis. PHP-FPM khởi động lại |
| Memcached | | Memcached, extension PHP của nó, và mọi dữ liệu trong bộ nhớ đệm. PHP-FPM khởi động lại |
| Adminer | Database của bạn | Các tệp của Adminer và site nội bộ của nó |
| phpMyAdmin | Database của bạn | Các tệp của phpMyAdmin, tệp tạm, site nội bộ và thiết lập của nó |
| Node.js | Các phiên bản Node.js khác. Cả thư mục của bản mặc định, nếu có website ứng dụng đã chọn phiên bản đó | `node`, `npm` và `npx` trong `/usr/local/bin`, cùng bản Node.js mà chúng trỏ tới |
| Ứng dụng Python | Bản thân `python3`, vì hệ thống cần nó, và môi trường ảo trong thư mục của các ứng dụng cũ | Gói venv và pip |
| Java (OpenJDK) | | Mọi phiên bản Java đã cài, cùng các gói chỉ chúng cần |
| ionCube Loader | | Loader trong mọi phiên bản PHP và `/usr/local/lib/ioncube`. PHP-FPM nạp lại, và các tệp PHP đã mã hóa ngừng chạy |
| Docker | Image, volume và build cache trong `/var/lib/docker`, trừ khi bạn đánh dấu **Xóa luôn mọi image, volume và build cache (/var/lib/docker)** và gõ `docker`. Cài lại Docker sẽ dùng lại chúng | Docker và plugin Compose |
| Tường lửa web | Nhật ký của tường lửa | ModSecurity và bộ luật OWASP. Web server khởi động lại |
| ClamAV | | ClamAV và cơ sở dữ liệu virus của nó |
| Composer | Website Laravel và Drupal, vốn chỉ cần nó lúc cài | `/usr/local/bin/composer` và gói `composer` của hệ điều hành |
| WP-CLI | | `/usr/local/lib/panel/wp-cli.phar`. Bộ công cụ WordPress sẽ tự cài lại khi cần |

::: danger
Xóa dữ liệu của MariaDB, MySQL, PostgreSQL hoặc Docker là không thể hoàn tác. Hãy chắc chắn rằng bạn đã sao lưu những gì còn cần.
:::

## Menu chỉ hiện những gì đã cài {#the-menu-shows-what-is-installed}

Menu bên trái ẩn các trang của phần mềm chưa cài:

- **Docker** hiện ra khi đã cài Docker.
- **WordPress** hiện ra khi đã có một website WordPress.
- Trên trang **Cơ sở dữ liệu**, các thẻ **PostgreSQL**, **Redis** và **Memcached** hiện ra khi đã cài phần mềm tương ứng, còn **Truy vấn chậm** hiện ra khi đã cài MariaDB hoặc MySQL.

**Website**, **Cơ sở dữ liệu** và **Phần mềm** luôn hiện, vì đó là nơi bạn cài các thành phần. Trang bạn đang mở vẫn được hiện.

## Web server {#web-server}

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

## Các phiên bản PHP {#php-versions}

Bạn có thể cài nhiều phiên bản PHP song song và chọn phiên bản cho từng website ở **Website** → (site) → **PHP**.

- Phiên bản đi kèm hệ điều hành được lấy từ kho phần mềm của hệ điều hành. Các phiên bản khác được lấy từ kho `packages.sury.org/php` của Ondřej Surý, trên cả Ubuntu và Debian. Panel hỏi trước khi thêm kho đó vào máy chủ, và chỉ chấp nhận khóa ký của kho có fingerprint `15058500 A0235D97 F5D10063 B188E2B6 95BD4743`.
- Mỗi phiên bản được cài kèm các extension: fpm, cli, mysql, curl, gd, mbstring, xml, zip, intl, bcmath và opcache. Từ PHP 8.5, OPcache là một phần của chính PHP, nên 8.5 trở lên không có gói opcache riêng. Nếu đã cài Redis hoặc Memcached, extension redis hoặc memcached cũng được thêm.
- Trước PHP 8.0, json là một gói riêng và cũng được thêm. PHP 5.6 còn được cài kèm mcrypt, nếu kho có gói này.
- Panel chuyển pool PHP-FPM mặc định của phiên bản sang chế độ khởi tạo worker theo nhu cầu (ondemand), nên một phiên bản không website nào dùng gần như không tốn RAM.

Danh sách phiên bản đi theo kho PHP. Khi `packages.sury.org/php` đã có trên máy chủ (kho được thêm cùng phiên bản PHP đầu tiên không phải của hệ điều hành), trang đưa ra các phiên bản mà kho có cho hệ điều hành của bạn, đọc từ danh sách gói của apt. Một phiên bản PHP mới, ví dụ PHP 8.7, tự xuất hiện khi kho phát hành nó và danh sách gói được làm mới (mỗi lần cài đều chạy `apt-get update`, và tác vụ apt hằng ngày của hệ thống cũng vậy). Phiên bản mà kho không có sẽ không được đưa ra. Trước khi thêm kho, trang đưa ra các phiên bản mà Nefo biết, từ 7.4 đến 8.6. Các phiên bản đã cài, phiên bản của hệ điều hành và mục **Phiên bản cũ** luôn được liệt kê.

Chọn **Quản lý** trên một phiên bản đã cài để mở phần **Extension** và **Cài đặt** của nó.

### Phiên bản không còn cập nhật bảo mật {#end-of-life}

Mỗi phiên bản PHP chỉ nhận cập nhật bảo mật từ PHP đến một ngày nhất định (xem [php.net/supported-versions.php](https://www.php.net/supported-versions.php)): 8.1 đến 31/12/2025, 8.2 đến 31/12/2026, 8.3 đến 31/12/2027, 8.4 đến 31/12/2028, 8.5 đến 31/12/2029 và 8.6 đến 31/12/2030. 7.4 và 8.0 đã hết. Sau ngày đó, phiên bản lấy từ `packages.sury.org` hiện nhãn **Hết vòng đời — không còn cập nhật bảo mật** kèm một dòng ghi ngày, trên trang Phần mềm, trong cửa sổ **Quản lý** và trong menu chọn phiên bản PHP của website. Panel so ngày đó với hôm nay, nên nhãn tự xuất hiện.

Phiên bản PHP đi kèm hệ điều hành (8.1 trên Ubuntu 22.04, 8.3 trên 24.04, 8.5 trên 26.04, 8.2 trên Debian 12, 8.4 trên Debian 13) không bao giờ có nhãn này: đội bảo mật của hệ điều hành vẫn vá nó trong suốt thời gian bản phát hành còn được hỗ trợ.

Hãy chuyển website khỏi phiên bản có nhãn khi có thể. Bạn vẫn cài được phiên bản đó mà không cần xác nhận thêm; chỉ các [phiên bản cũ](#older-php-versions) mới hỏi.

### Các phiên bản PHP cũ {#older-php-versions}

PHP 5.6, 7.0, 7.1, 7.2 và 7.3 nằm trong mục **Phiên bản cũ**, được thu gọn bên dưới các phiên bản khác. Chúng dành cho website chuyển từ máy chủ khác (cPanel, aaPanel…) mà chưa chạy được trên PHP mới hơn.

::: danger Hết vòng đời: không còn cập nhật bảo mật
PHP đã ngừng cập nhật bảo mật cho các phiên bản này từ nhiều năm trước: 5.6 từ năm 2018, 7.3 từ năm 2021 (xem [php.net/eol.php](https://www.php.net/eol.php)). Các lỗ hổng đã biết của chúng sẽ không bao giờ được vá, nên website chạy trên chúng dễ bị tấn công hơn. Chỉ dùng chúng để giữ một website cũ hoạt động trong lúc bạn cập nhật nó, rồi chuyển nó sang phiên bản còn được hỗ trợ. Đừng dùng cho website mới.
:::

- Mỗi phiên bản hiện nhãn **Hết vòng đời — không còn cập nhật bảo mật**, trên trang Phần mềm và trong menu chọn phiên bản PHP của các website.
- Khi cài một phiên bản, panel yêu cầu bạn xác nhận rủi ro đó trước.
- Phải cài một phiên bản còn được hỗ trợ (7.4 trở lên) trước, và phiên bản được hỗ trợ cuối cùng không gỡ được khi còn phiên bản cũ. Vì vậy một phiên bản cũ không bao giờ là bản PHP duy nhất của máy chủ: web server, Adminer, Composer và các công cụ của panel luôn có một bản còn được hỗ trợ.
- Kho ứng dụng giữ nguyên yêu cầu phiên bản tối thiểu của nó: WordPress cần PHP 7.4, các ứng dụng khác cần bản mới hơn. Bộ công cụ WordPress vẫn quản lý được website WordPress chạy phiên bản cũ, nhưng không cập nhật chính WordPress ở đó, vì WordPress hiện tại cần PHP 7.4. Nó vẫn cập nhật plugin và giao diện; WP-CLI bỏ qua các bản cập nhật cần PHP mới hơn.
- Pool PHP-FPM của website và các thiết lập php.ini giống như ở phiên bản mới hơn: mọi thiết lập panel ghi đều có trong PHP 5.6.

`packages.sury.org/php` có PHP 5.6 đến 7.3 cho Ubuntu 22.04, 24.04, 26.04 và Debian 12, 13, trên amd64 và arm64. Khi cài, panel cũng hỏi apt, và từ chối kèm lý do nếu kho không có phiên bản đó.

### Máy chủ từng dùng PPA ondrej/php {#ondrej-php-ppa}

Các phiên bản panel trước đây lấy các phiên bản PHP bổ sung trên Ubuntu từ `ppa:ondrej/php`. PPA đó đã bị đóng băng và không còn nhận cập nhật: người duy trì nay phát hành các gói cho Ubuntu tại `packages.sury.org/php` (xem thông báo trên [launchpad.net/~ondrej/+archive/ubuntu/php](https://launchpad.net/~ondrej/+archive/ubuntu/php)).

Khi panel khởi động trên một máy chủ Ubuntu vẫn còn PPA này, nó chuyển máy chủ sang kho mới một lần, đúng như thông báo của PPA hướng dẫn:

1. Thêm `packages.sury.org/php` với khóa đã ghim (`/usr/share/keyrings/sury-php.gpg` và `/etc/apt/sources.list.d/sury-php.list`), trừ khi đã có nguồn cho kho này.
2. Chuyển file nguồn của PPA trong `/etc/apt/sources.list.d`, cùng khóa của PPA `/etc/apt/trusted.gpg.d/ondrej-ubuntu-php.gpg` nếu có, vào thư mục `apt-backup` trong thư mục dữ liệu của panel (`/var/lib/nefo/apt-backup`). Khóa đó được tin cậy cho mọi kho, và nay không còn gì cần đến nó. Các PPA khác, kể cả của cùng người duy trì, được giữ nguyên.

Không gói nào được cài, gỡ hay hạ phiên bản, và panel không chạy `apt-get update` lúc khởi động. Lần cập nhật kế tiếp (lần kiểm tra cập nhật của panel, một lần cài, hoặc lệnh `apt update` của bạn) sẽ đọc nguồn mới, và các gói PHP đã cài nhận bản cập nhật tiếp theo từ đó. Log của panel ghi lại những gì đã thay đổi. Nếu không tải được khóa, PPA được giữ nguyên và panel thử lại ở lần khởi động sau. Dòng PPA nằm trong chính `/etc/apt/sources.list`, hoặc trong một file có cả các kho khác, sẽ không bị sửa: log sẽ nhắc bạn tự thay nó.

### Extension {#extensions}

Phần **Extension** liệt kê các extension tùy chọn có gói cho phiên bản này. Chọn **Cài** hoặc **Gỡ**, và PHP-FPM sẽ được nạp lại.

Các extension tùy chọn gồm: apcu, gmp, igbinary, imagick, imap, ldap, memcached, mongodb, msgpack, pgsql, redis, soap, sqlite3, ssh2, tidy, xdebug, xsl và yaml.

::: warning
xdebug làm chậm mọi site dùng phiên bản PHP đó. Đừng để nó trên máy chủ đang chạy thật.
:::

### Thiết lập php.ini {#php-ini-settings}

Phần **Cài đặt** đặt giá trị php.ini cho mọi website dùng phiên bản PHP này. Để trống một ô để dùng mặc định của PHP, rồi chọn **Lưu**.

| Thiết lập | Mặc định | Giá trị cho phép |
|---|---|---|
| `date.timezone` | UTC | Tên múi giờ, ví dụ `Europe/Berlin` |
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

Các giá trị được ghi vào `/etc/php/<version>/fpm/conf.d/99-panel.ini` và được kiểm tra bằng `php-fpm -t`. Nếu PHP-FPM từ chối, tệp cũ được đặt lại và không có gì thay đổi. Giới hạn RAM, dung lượng upload và thời gian chạy được đặt riêng cho từng website, trong phần cài đặt của website đó.

### ionCube Loader {#ioncube}

Thẻ **ionCube Loader**, bên dưới các phiên bản PHP, giúp chạy các tệp PHP được mã hóa bằng ionCube Encoder. Một số script PHP thương mại cần nó, như WHMCS và các phần mềm tính cước hay quản lý giấy phép khác, và website chuyển từ cPanel sang cũng thường cần.

1. Cài trước các phiên bản PHP mà website của bạn dùng.
2. Trên thẻ **ionCube Loader**, chọn **Cài**.

Panel tải các loader từ ionCube (`downloads.ioncube.com`) qua HTTPS. Panel cài một bản phát hành đã ghim, 15.5.1, và đối chiếu bản tải về với mã SHA-256 mà panel lưu cho bản đó, vì ionCube không công bố checksum hay chữ ký nào. Bản tải về không khớp sẽ bị từ chối và không có gì thay đổi. Loader chạy được trên máy chủ amd64 và arm64.

Các loader được đặt trong `/usr/local/lib/ioncube`, kèm giấy phép của ionCube (`LICENSE.txt`). Với mỗi phiên bản PHP đã cài mà loader hỗ trợ, panel ghi `00-ioncube.ini` vào `/etc/php/<version>/fpm/conf.d` và `/etc/php/<version>/cli/conf.d`, để loader được nạp trước OPcache, như ionCube yêu cầu. Panel kiểm tra rằng `php<version> -v` khởi động được cùng loader, rồi nạp lại PHP-FPM. Nếu PHP không khởi động được với loader, panel gỡ tệp đó ra, để phiên bản đó vẫn chạy mà không có loader, và tác vụ báo phiên bản nào bị lỗi.

Thẻ liệt kê mọi phiên bản PHP đã cài cùng trạng thái của loader với từng phiên bản:

| Trạng thái | Ý nghĩa |
|---|---|
| **đã nạp** | Loader đã được nạp cho PHP-FPM và dòng lệnh |
| **chưa nạp** | Có loader cho phiên bản này, nhưng chưa được nạp. Chọn **Nạp vào mọi phiên bản** trên thẻ |
| **ionCube không có loader cho phiên bản này** | PHP 8.0: ionCube chưa bao giờ phát hành loader cho phiên bản này |
| "chưa có loader cho PHP 8.6" | Phiên bản PHP mới hơn các loader mà panel cài (15.5.1 hỗ trợ tới PHP 8.5) |

Bản phát hành đã ghim có loader cho PHP 5.6, 7.0 đến 7.4 và 8.1 đến 8.5. Phiên bản PHP bạn cài sau này cũng nhận loader, nếu có loader cho nó. Gỡ một phiên bản PHP sẽ xóa `00-ioncube.ini` của phiên bản đó.

**Gỡ cài đặt** gỡ loader khỏi mọi phiên bản PHP, nạp lại PHP-FPM, và xóa `/usr/local/lib/ioncube`. Khi đó các tệp PHP đã mã hóa ngừng chạy.

Để kiểm tra một website, đặt một tệp như sau vào thư mục của nó và mở tệp đó, rồi xóa tệp:

```php
<?php var_dump(extension_loaded('ionCube Loader'));
```

::: tip
ionCube Loader được dùng miễn phí nhưng không phải mã nguồn mở. Giấy phép của ionCube ([ioncube.com/legal/loaders/eula](https://www.ioncube.com/legal/loaders/eula)) cho phép dùng nó miễn phí. Panel tải nó từ ionCube khi bạn cài và không phân phối kèm nó.
:::

## Node.js {#node-js}

Thẻ **Node.js** cài bản LTS mới nhất từ nodejs.org, có kiểm tra mã SHA-256 được công bố. Node.js được đặt trong `/opt/node-v<version>-linux-<arch>`, còn `node`, `npm` và `npx` được liên kết vào `/usr/local/bin`. Đây là Node.js mặc định của máy chủ, dùng cho các website ứng dụng không chọn phiên bản riêng.

### Nhiều phiên bản Node.js {#several-node-js-versions}

Khi đã cài Node.js, thẻ **Các phiên bản Node.js** liệt kê ba dòng LTS mới nhất và mọi dòng đã cài. Chọn **Cài** để thêm một dòng song song với các dòng khác. Việc này không đổi bản mặc định.

Chọn dòng Node.js cho một website ứng dụng ở **Website** → (site) → **Runtime**. Dòng đang được website ứng dụng dùng, và bản mặc định, không gỡ được từ thẻ này. Hãy gỡ bản mặc định ở thẻ **Node.js**.

Danh sách được lấy từ nodejs.org, nên máy chủ cần truy cập Internet để hiện nó.

::: warning
Gỡ Node.js mặc định cũng làm hỏng các cron job và chương trình nền có gọi `node`.
:::

## Python {#python}

Thẻ **Ứng dụng Python** cài `python3-venv` và `python3-pip`, để website ứng dụng chạy được Python (Django, Flask, FastAPI…). Mỗi website ứng dụng Python có môi trường ảo riêng trong `app/.venv`. Panel không cài công cụ build như `build-essential` và `python3-dev`, nên gói nào không có sẵn wheel và phải biên dịch sẽ cài không được cho đến khi bạn tự cài các công cụ đó bằng `apt`.

Gỡ cài đặt sẽ gỡ venv và pip. Bản thân `python3` vẫn được giữ, vì hệ thống cần nó.

## Java {#java}

Thẻ **Java (OpenJDK)** cài OpenJDK từ hệ điều hành, cho website ứng dụng viết bằng Java, chẳng hạn ứng dụng Spring Boot chạy bằng `java -jar`. Đó là bản JDK headless (`openjdk-<version>-jdk-headless`): `java`, cùng `javac`, `jar` và các công cụ khác mà bước build trong script [triển khai Git](./git-deploy) cần, không kèm các phần dành cho giao diện desktop. Nó được đặt trong `/usr/lib/jvm/java-<version>-openjdk-<arch>` và nhận bản cập nhật bảo mật cùng phần còn lại của hệ thống.

**Cài** trên thẻ cài phiên bản hỗ trợ dài hạn (LTS) mới nhất mà hệ điều hành có. Các phiên bản có sẵn tùy theo hệ điều hành:

| Hệ điều hành | Phiên bản Java |
|---|---|
| Ubuntu 22.04 | 17, 21, 25 |
| Ubuntu 24.04 | 17, 21, 25 |
| Ubuntu 26.04 | 17, 21, 25 |
| Debian 12 | 17 |
| Debian 13 | 21, 25 |

### Nhiều phiên bản Java {#several-java-versions}

Khi đã cài Java, thẻ **Các phiên bản Java** liệt kê các phiên bản LTS mà hệ điều hành có và mọi phiên bản đã cài. Chọn **Cài** để thêm một phiên bản song song với các phiên bản khác. Phiên bản LTS mới hơn tự xuất hiện khi hệ điều hành có nó.

Chọn phiên bản cho một website ứng dụng ở **Website** → (site) → **Runtime** (xem [Website ứng dụng Java](./websites#java-app-websites)). Phiên bản đang được website ứng dụng dùng thì không gỡ được, và thẻ cho biết có ứng dụng đang dùng nó. **Gỡ cài đặt** trên thẻ **Java (OpenJDK)** gỡ mọi phiên bản đã cài, và bị từ chối khi còn website ứng dụng nào chạy Java.

Mỗi phiên bản tốn khoảng 390 đến 480 MB dung lượng đĩa, kể cả các gói đã tải về. Không có gì chạy khi chưa có website nào dùng nó.

## Redis {#redis}

Redis là bộ nhớ đệm trong RAM. WordPress có thể dùng nó với một plugin Redis object cache.

Khi bạn cài Redis, panel:

- cài `redis-server` và extension redis cho mọi phiên bản PHP đã cài;
- giới hạn RAM của Redis cho phù hợp với máy chủ: 64 MB khi RAM dưới 1,5 GB, 128 MB từ 1,5 GB, 256 MB từ 3 GB;
- đặt chính sách `allkeys-lru`, để Redis bỏ các key ít được dùng gần đây nhất khi đầy, như một bộ nhớ đệm;
- để Redis lắng nghe trên `127.0.0.1:6379`, chỉ truy cập được từ chính máy chủ này.

Để xem bên trong Redis, vào **Cơ sở dữ liệu** → **Redis**. Xem trang [Cơ sở dữ liệu](./databases#redis-key-browser).

Trước khi gỡ Redis, hãy tắt plugin bộ nhớ đệm trên các website. Chúng vẫn chạy được khi không có bộ nhớ đệm, chỉ chậm hơn một chút.

## PostgreSQL {#postgresql}

Thẻ **PostgreSQL** cài phiên bản của hệ điều hành, hoặc một phiên bản chính khác từ kho của dự án PostgreSQL (apt.postgresql.org), tùy bạn chọn trong hộp thoại cài. Panel tinh chỉnh nó theo RAM của máy chủ bằng một file trong `conf.d`, chỉ cho nó lắng nghe trên localhost, và ghi `pg_hba.conf` yêu cầu mật khẩu với mọi người trừ user hệ thống `postgres` qua socket nội bộ. Database của nó nằm ở **Cơ sở dữ liệu** → **PostgreSQL**. Xem [PostgreSQL](./postgresql) để biết về các phiên bản, việc nâng cấp, truy cập từ xa, quy tắc nhập và sao lưu.

Khi PostgreSQL lấy từ apt.postgresql.org, thẻ ghi **Từ apt.postgresql.org**; nếu không thì ghi **Từ hệ điều hành**. Gỡ PostgreSQL cũng xóa kho đó, nếu panel đã thêm nó.

Khi đã cài Adminer, panel thêm extension `pgsql` của PHP cho phiên bản PHP chạy Adminer, để Adminer mở được cả database PostgreSQL. Extension này chỉ được thêm cùng với PostgreSQL.

## Memcached {#memcached}

Memcached là bộ nhớ đệm nhỏ trong RAM, dùng cho session PHP và cache của ứng dụng (ví dụ cache driver `memcached` của Laravel).

Khi bạn cài Memcached, panel:

- cài `memcached` và extension memcached cho mọi phiên bản PHP đã cài, và thêm extension này cho các phiên bản PHP cài sau;
- ghi `/etc/memcached.conf`: Memcached chỉ lắng nghe trên `127.0.0.1:11211`, chỉ qua TCP, tắt UDP (`-U 0`), nên không thể bị lợi dụng để khuếch đại tấn công;
- giới hạn bộ nhớ theo máy chủ: 64 MB khi dưới 1,5 GB RAM, 128 MB từ 1,5 GB, 256 MB từ 3 GB, và cho phép 512 kết nối khi dưới 1,5 GB RAM, 1024 kết nối từ mức đó trở lên.

Khi bộ nhớ đầy, Memcached bỏ các mục lâu không dùng nhất. Để xem các con số của nó, vào **Cơ sở dữ liệu** → **Memcached**.

Trước khi gỡ Memcached, hãy chuyển session và cache của website sang nơi khác: ứng dụng có cache driver là Memcached sẽ lỗi khi không có nó.

## Máy chủ cơ sở dữ liệu, Adminer và phpMyAdmin {#mariadb-and-adminer}

Thẻ **Máy chủ cơ sở dữ liệu** cài MariaDB hoặc MySQL 8.4 LTS, được tinh chỉnh theo RAM của máy chủ và chỉ truy cập được từ chính máy chủ. Mỗi máy chủ chỉ có một trong hai.

Khi chưa cài cái nào, thẻ cho bạn chọn, mặc định là **MariaDB**: nó nhẹ hơn và là nền tảng của WordPress cùng hầu hết ứng dụng PHP.

- Ở mục **MariaDB**, bạn chọn phiên bản khi kho riêng của MariaDB có dòng LTS mới hơn bản của hệ điều hành: bản của hệ điều hành (mặc định, ví dụ "10.11, bản của hệ điều hành"), hoặc **11.4 LTS từ MariaDB**, **11.8 LTS từ MariaDB** hay **12.3 LTS từ MariaDB** (trên Ubuntu 26.04 và Debian 13 chỉ có 12.3). Mỗi lựa chọn ghi rõ MariaDB hỗ trợ nó đến khi nào. Xem [Các phiên bản MariaDB](./databases#mariadb-versions).
- **MySQL 8.4 LTS** lấy từ kho của Oracle và chỉ có trên Ubuntu 22.04, 24.04, 26.04 và Debian 12, 13, với máy chủ amd64. Ở những nơi khác, thẻ nói rõ vì sao không có.

Khi đã cài một cái, thẻ cho biết đó là cái nào, phiên bản và nguồn gốc (**từ gói của bản phân phối**, **từ kho của MariaDB** hoặc **từ kho của Oracle**), RAM ước tính, số database, cùng **Mở Cơ sở dữ liệu** và **Gỡ cài đặt**. Thẻ còn có:

- **Nâng cấp**, với MariaDB, khi có dòng LTS mới hơn: xem [Nâng cấp MariaDB](./databases#upgrade-mariadb).
- Máy chủ còn lại làm phương án thay thế, **Dùng MySQL 8.4 LTS thay thế** hoặc **Dùng MariaDB thay thế**, kèm nút chuyển: panel chuyển các database, user và mật khẩu của chúng sang, và quay lại nếu có bước nào lỗi. Xem [Chuyển giữa MariaDB và MySQL](./databases#switch-between-mariadb-and-mysql).
- **Các bản dump đang giữ**: các bản xuất mà một lần nâng cấp hoặc chuyển đổi đã tạo, kèm thư mục, dung lượng và nút xóa. Xem [Bản dump từ các lần nâng cấp và chuyển đổi](./databases#dumps-from-upgrades-and-switches).

Adminer và phpMyAdmin giúp xem database trên trình duyệt và chỉ truy cập được qua bước đăng nhập của panel. Cả hai cần web server và PHP, và đều dùng được với MariaDB lẫn MySQL. Xem trang [Cơ sở dữ liệu](./databases#phpmyadmin).

## Docker {#docker}

Thẻ **Docker** cài `docker.io` từ kho phần mềm của hệ điều hành, kèm plugin Compose nếu có. Sau đó mục **Docker** hiện ra trên menu.

Cổng do container Docker mở sẽ bỏ qua tường lửa ufw. Vì vậy panel chỉ mở cổng container trên 127.0.0.1, và bạn truy cập chúng qua một website. Xem trang [Docker](./docker).

## Tường lửa web và ClamAV {#web-firewall-and-clamav}

- **Tường lửa web (ModSecurity)** cài ModSecurity cùng bộ luật OWASP Core Rule Set. Nó cần web server. Sau khi cài, bật cho từng website ở **Website** → (site) → **Tường lửa WAF**. Khi hệ điều hành không có module ModSecurity cho web server đang dùng, thẻ nói rõ lý do và không cho cài: Ubuntu 22.04 không có module cho Nginx. Apache và OpenLiteSpeed vẫn dùng được ở đó, còn Nginx dùng được từ Ubuntu 24.04 trở đi.
- **Phần mềm diệt virus ClamAV** là bộ quét bổ sung, tùy chọn, cho việc quét mã độc. Cơ sở dữ liệu virus của nó được tải ngay sau khi cài, nên vài phút sau việc quét mới dùng được nó. ClamAV hợp với máy chủ từ 2 GB RAM trở lên.

Xem trang [Bảo mật](./security) cho cả hai.

## Công cụ: Composer và WP-CLI {#tools}

Nhóm **Công cụ** hiện hai công cụ dòng lệnh mà panel tự cài khi có thứ cần đến chúng. Mỗi thẻ cho biết công cụ đã cài chưa, phiên bản, nguồn gốc và thứ đang dùng nó. Nút **Cập nhật** cài bản phát hành chính thức mới nhất. Panel tải nó qua HTTPS cùng checksum mà dự án công bố bên cạnh, và từ chối file tải về không khớp: công cụ đang cài vẫn giữ nguyên. File mới thay file cũ trong một bước duy nhất.

### Composer {#composer}

Composer là trình quản lý gói của PHP. Cài Laravel hoặc Drupal từ Kho ứng dụng sẽ cài nó, và script triển khai Git cùng terminal cũng có thể chạy nó.

- **Cài** cài gói `composer` của hệ điều hành, được cập nhật bảo mật cùng phần còn lại của hệ thống.
- **Cập nhật** cài bản ổn định mới nhất từ getcomposer.org vào `/usr/local/bin/composer`, có kiểm tra checksum SHA-256 công bố. Bản này đứng trước trong `PATH`, nên `composer` trong terminal, script triển khai Git và việc panel cài Laravel, Drupal đều dùng nó. Gói của hệ điều hành vẫn được giữ.
- **Gỡ cài đặt** gỡ cả hai. Việc gỡ bị từ chối khi có script triển khai Git chạy `composer`. Website Laravel và Drupal không chặn việc gỡ: chúng chỉ cần Composer lúc cài và vẫn chạy bình thường khi không có nó. Cài lại một trong hai ứng dụng này sẽ cài lại Composer.

### WP-CLI {#wp-cli}

WP-CLI chạy WordPress từ dòng lệnh. [Bộ công cụ WordPress](./wordpress) dùng nó để cập nhật, bật chế độ bảo trì, đặt lại mật khẩu và tạo bản staging, và tự cài nó ở lần đầu cần đến.

- **Cài** và **Cập nhật** tải bản phát hành mới nhất từ GitHub về `/usr/local/lib/panel/wp-cli.phar`, có kiểm tra checksum SHA-512 mà bản phát hành công bố.
- **Gỡ cài đặt** bị từ chối khi còn website WordPress.
