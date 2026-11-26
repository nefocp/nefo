---
title: Cơ sở dữ liệu
description: Cài MariaDB hoặc MySQL, tạo database, nhập và xuất SQL, mở truy cập từ xa, xem log truy vấn chậm, dùng Adminer, phpMyAdmin và trình duyệt key Redis, và quản lý database PostgreSQL.
---

# Cơ sở dữ liệu

Trang **Cơ sở dữ liệu** quản lý máy chủ cơ sở dữ liệu cho WordPress và hầu hết ứng dụng PHP: MariaDB, hoặc MySQL 8.4 nếu bạn chọn nó. Trang này hướng dẫn:

- chọn MariaDB hay MySQL, cách cài và cách Nefo tinh chỉnh nó cho máy chủ nhỏ;
- tạo database, xem thông tin đăng nhập và đổi mật khẩu;
- nhập và xuất SQL;
- mở truy cập từ xa cho từng database;
- log truy vấn chậm;
- Adminer và phpMyAdmin, giao diện database ngay trên trình duyệt;
- trình duyệt key Redis, ở thẻ **Redis** của cùng trang;
- PostgreSQL, ở thẻ **PostgreSQL** riêng, và trạng thái của Memcached.

PostgreSQL chạy được bên cạnh MariaDB, cài từ kho phần mềm của hệ điều hành (xem [PostgreSQL](#postgresql)). MongoDB, và PostgreSQL dạng container, có trong [Kho ứng dụng](./apps) dưới dạng ứng dụng Docker, chỉ lắng nghe trên 127.0.0.1. Các ứng dụng Docker đó không hiện trên trang **Cơ sở dữ liệu**.

## Chọn MariaDB hay MySQL {#choose-mariadb-or-mysql}

Mỗi máy chủ chạy một máy chủ cơ sở dữ liệu tương thích MySQL: MariaDB hoặc MySQL 8.4 LTS. Bạn chọn lúc cài, ở trang **Cơ sở dữ liệu** hoặc trên thẻ **Máy chủ cơ sở dữ liệu** của trang **Phần mềm**. Khi đã cài một cái, panel từ chối cài cái còn lại.

| | MariaDB (khuyên dùng) | MySQL 8.4 LTS |
|---|---|---|
| Nguồn | Kho phần mềm của hệ điều hành | Kho của Oracle, `repo.mysql.com` (nhánh `mysql-8.4-lts`) |
| RAM, ước chừng | Dung lượng buffer pool cộng khoảng 100 MB | Dung lượng buffer pool cộng khoảng 150 MB; cộng khoảng 300 MB từ khoảng 4 GB RAM, khi performance schema được bật |
| Hỗ trợ | Theo thời gian hỗ trợ của hệ điều hành | Oracle hỗ trợ 8.4 LTS đến năm 2032 |
| Hệ điều hành | Mọi hệ điều hành Nefo hỗ trợ | Ubuntu 22.04, 24.04, 26.04 và Debian 12, 13, chỉ trên amd64 (x86-64) |
| Hợp với | WordPress và hầu hết ứng dụng PHP | Ứng dụng cần đúng MySQL, hoặc đội ngũ đã quen dùng MySQL |

Cả hai đều dùng được mọi chức năng trên trang này, cùng sao lưu, nhân bản website, staging WordPress và các ứng dụng PHP của kho ứng dụng. Adminer và phpMyAdmin mở được cả hai. Trên máy chủ arm64, hoặc hệ điều hành mà Oracle không có gói, trang **Phần mềm** chỉ đưa ra MariaDB và nói rõ lý do.

::: warning Đổi về sau
Panel không tự chuyển một máy chủ từ cái này sang cái kia. Muốn chuyển, hãy xuất mọi database (hoặc tạo bản sao lưu), xóa các database, gỡ máy chủ cơ sở dữ liệu đang dùng, cài cái còn lại, rồi tạo lại các database và nhập bản dump. Hai bên không đọc được thư mục dữ liệu của nhau: panel từ chối cài cái này lên trên dữ liệu của cái kia còn giữ trong `/var/lib/mysql`, nên hãy chuyển thư mục đó đi hoặc xóa trước. Bản dump chuyển qua lại được miễn là nó không dùng thứ mà bên kia không có; chẳng hạn collation `utf8mb4_uca1400_ai_ci` của MariaDB 11 không có trong MySQL.
:::

## Cài MariaDB

1. Vào **Cơ sở dữ liệu**.
2. Chọn **Cài MariaDB (khuyên dùng)**. Bạn cũng có thể cài ở trang **Phần mềm**: giữ lựa chọn **MariaDB** trên thẻ **Máy chủ cơ sở dữ liệu** rồi chọn **Cài MariaDB**.
3. Chờ khoảng một đến hai phút. Log tiến trình hiện từng bước.

Nefo cài gói `mariadb-server` và `mariadb-client` từ kho phần mềm của hệ điều hành. Sau đó panel ghi cấu hình, khởi động MariaDB, rồi xóa các user ẩn danh và database `test` mà một số phiên bản tạo sẵn.

### Cách MariaDB được tinh chỉnh

Nefo ghi cấu hình vào `/etc/mysql/mariadb.conf.d/90-panel.cnf`. Thiết lập tốn RAM nhất là InnoDB buffer pool, phụ thuộc vào RAM của máy chủ:

| RAM máy chủ | `innodb_buffer_pool_size` |
|---|---|
| Dưới 1,5 GB | 128M |
| Từ 1,5 GB | 256M |
| Từ 3 GB | 512M |
| Từ 7 GB | 1G |

Các thiết lập còn lại giống nhau trên mọi máy chủ:

| Thiết lập | Giá trị | Lý do |
|---|---|---|
| `bind-address` | `127.0.0.1` | MariaDB chỉ nhận kết nối từ chính máy chủ này |
| `skip-name-resolve` | bật | Không tra DNS cho mỗi kết nối |
| `performance_schema` | `OFF` | Tiết kiệm RAM |
| `max_connections` | `100` | |
| `character-set-server` | `utf8mb4` | Unicode đầy đủ, kể cả emoji |
| `collation-server` | `utf8mb4_unicode_ci` | |

Khi đó MariaDB dùng khoảng bằng dung lượng buffer pool cộng thêm 100 MB. Trang **Phần mềm** hiện con số ước tính này.

::: warning
Đừng sửa `90-panel.cnf`. Panel sẽ ghi đè tệp này.
:::

## Cài MySQL {#install-mysql}

1. Vào **Cơ sở dữ liệu** và chọn **Cài MySQL 8.4 thay thế**. Ở trang **Phần mềm**, chọn **MySQL 8.4 LTS** trên thẻ **Máy chủ cơ sở dữ liệu** rồi chọn **Cài MySQL 8.4 LTS**.
2. Chờ khoảng hai đến bốn phút. Log tiến trình hiện từng bước.

Nefo tự thêm kho của Oracle, thay vì cài gói `mysql-apt-config` của Oracle:

- Panel tải khóa ký từ `https://repo.mysql.com/RPM-GPG-KEY-mysql-2025` và chỉ chấp nhận nếu dấu vân tay đúng là `BCA4 3417 C3B4 85DD 128E C6D4 B7B3 B788 A8D3 785C`, khóa MySQL Release Engineering mà tài liệu của MySQL công bố. Khóa được lưu ở `/etc/apt/keyrings/panel-mysql.gpg`.
- `/etc/apt/sources.list.d/panel-mysql.list` chỉ tin khóa đó cho riêng kho này, và chỉ có nhánh `mysql-8.4-lts`.
- Oracle thỉnh thoảng gia hạn khóa; bản hiện tại hết hạn vào ngày 23 tháng 10 năm 2027. Mỗi tuần panel tìm bản gia hạn của cùng khóa đó và thay bản cũ, nhờ vậy `apt-get update` vẫn chạy bình thường.

Sau đó panel cài `mysql-community-server` và `mysql-community-client` mà không hỏi gì. Tài khoản `root` của MySQL không có mật khẩu: nó chỉ đăng nhập qua socket nội bộ (`auth_socket`), với quyền root trên máy chủ, và panel không lưu mật khẩu root nào. Cuối cùng panel xóa user ẩn danh và database `test` nếu có.

### Cách MySQL được tinh chỉnh {#how-mysql-is-tuned}

Nefo ghi `/etc/mysql/mysql.conf.d/90-panel.cnf` trước khi gói khởi động MySQL lần đầu, nên MySQL không bao giờ chạy với cấu hình mặc định của Oracle trên máy chủ của bạn. Các thiết lập bộ nhớ phụ thuộc vào RAM của máy chủ:

| RAM máy chủ | `innodb_buffer_pool_size` | `max_connections` | `table_open_cache` | `temptable_max_ram` |
|---|---|---|---|---|
| Dưới 1,5 GB | 128M | 75 | 400 | 32M |
| Từ 1,5 GB | 256M | 100 | 600 | 64M |
| Từ 3 GB | 512M | 150 | 1000 | 128M |
| Từ 7 GB | 1024M | 200 | 2000 | 256M |

Các thiết lập còn lại:

| Thiết lập | Giá trị | Lý do |
|---|---|---|
| `bind-address` | `127.0.0.1` | MySQL chỉ nhận kết nối từ chính máy chủ này, cho đến khi một database mở truy cập từ xa |
| `mysqlx` | `OFF` | Không có cổng X Protocol 33060: PHP và panel dùng giao thức thường |
| `disable_log_bin` | bật | Không có binary log làm đầy ổ đĩa: máy chủ không có bản sao (replica) |
| `performance_schema` | `OFF` dưới khoảng 3,5 GB RAM, `ON` từ mức đó trở lên | Riêng nó đã tốn 100 đến 200 MB |
| `skip-name-resolve`, `host_cache_size` | bật, `0` | Không tra DNS cho mỗi kết nối |
| `innodb_log_buffer_size` | `16M` | Mặc định của MySQL 8.4 là 64M |
| `character-set-server`, `collation-server` | `utf8mb4`, `utf8mb4_unicode_ci` | Giống MariaDB, để bản dump chuyển qua lại được |

Khi đó MySQL dùng khoảng bằng dung lượng buffer pool cộng thêm 150 MB, hoặc cộng 300 MB khi bật performance schema. Trang **Phần mềm** hiện con số ước tính này; trang **Dịch vụ** cho biết mức dùng thực tế.

User đăng nhập bằng `caching_sha2_password`, mặc định của MySQL 8.4, mà `mysqli` và PDO của PHP đều hỗ trợ. `mysql_native_password` vẫn tắt như MySQL 8.4 phát hành: nó băm bằng SHA-1 không có salt, và MySQL 9 đã bỏ hẳn nó. Vì vậy mã băm mật khẩu cũ từ máy chủ khác, như cPanel lưu, không dùng lại được trên MySQL: database chuyển sang cần có mật khẩu của nó, hoặc một mật khẩu mới.

Trên MySQL, panel chạy SQL bằng lệnh `mysql` thay cho `mariadb`, xuất bằng `mysqldump`, và dịch vụ tên là `mysql`.

::: warning
Đừng sửa `90-panel.cnf`. Panel sẽ ghi đè tệp này.
:::

## Tạo database

1. Vào **Cơ sở dữ liệu** và chọn **Thêm database**.
2. Nhập tên ở ô **Database**. Panel tạo kèm một user cùng tên, user này chỉ dùng được database này.
3. Để trống ô **Mật khẩu** để panel tạo mật khẩu mạnh ngẫu nhiên, hoặc tự gõ mật khẩu.
4. Không bắt buộc: chọn một site ở ô **Website**. Khi đó bản sao lưu của website sẽ có cả database này.
5. Chọn **Tạo database**.

Sau đó một thẻ có tiêu đề **Thông tin kết nối của** kèm tên database hiện ra, gồm **Database**, **User**, **Mật khẩu** và **Máy chủ**. Máy chủ luôn là `localhost`.

Quy tắc đặt tên và mật khẩu:

- Tên bắt đầu bằng chữ cái, chỉ dùng `a-z`, `0-9` và `_`, tối đa 32 ký tự. Các tên sau bị giữ lại, không dùng được: `mysql`, `information_schema`, `performance_schema`, `sys`, `test`, `root`, `mariadb`, `phpmyadmin`, `debian_sys_maint`. Panel cũng từ chối tên đã được một database hoặc một user database trong MariaDB dùng.
- Mật khẩu dài 12 đến 64 ký tự, gồm chữ cái, chữ số và `! # % * + , . : = ? @ ^ _ ~ -`. Mật khẩu do panel tạo dài 24 ký tự.

Database mới dùng `utf8mb4` và `utf8mb4_unicode_ci`.

### User và quyền

Mỗi database có một user là `tên@localhost`, có toàn quyền trên database đó và không có quyền gì trên database khác. Khi bật truy cập từ xa, user này có thêm một bản sao cho mỗi địa chỉ được phép (xem [Mở truy cập từ xa](#mo-truy-cap-tu-xa)). Ngoài user tạm thời dùng để nhập (xem [Nhập](#nhap)), panel không tạo thêm user và không đổi quyền. Nếu cần, hãy dùng lệnh `mariadb` (`mysql` trên MySQL) trên máy chủ với quyền root.

Danh sách chỉ gồm các database do panel tạo. Database bị xóa từ bên ngoài panel được đánh dấu **Không tìm thấy trong MariaDB** (hoặc **Không tìm thấy trong MySQL**).

## Xem thông tin đăng nhập của database

Chọn **Xem đăng nhập** trên dòng của database. Thẻ thông tin kết nối hiện tên, user, mật khẩu và máy chủ, mỗi mục có nút sao chép. Nếu đã cài Adminer hoặc phpMyAdmin, thẻ có thêm nút **Mở bằng Adminer** hoặc **Mở bằng phpMyAdmin**.

## Đổi mật khẩu database

1. Mở menu của database và chọn **Đổi mật khẩu**.
2. Giữ **Tạo mật khẩu mạnh ngẫu nhiên**, hoặc chọn **Tự đặt mật khẩu** rồi gõ mật khẩu. Mật khẩu tự đặt theo cùng quy tắc như khi [tạo database](#tao-database).
3. Chọn **Đổi mật khẩu**.

Panel đặt mật khẩu mới và hiện nó trong thẻ thông tin kết nối. Nếu database gắn với một website WordPress do panel tạo, panel cũng ghi mật khẩu mới vào `wp-config.php` của site đó. Các ứng dụng khác sẽ không kết nối được cho đến khi bạn cập nhật cấu hình của chúng.

Nếu database đang mở truy cập từ xa, các user từ xa cũng nhận mật khẩu mới.

## Xuất và nhập

### Xuất

Mở menu của database và chọn **Xuất (.sql)**. Trình duyệt tải về một tệp SQL thường, có tên dạng `shop_20261126-143000.sql`.

Bản dump được tạo bằng `mariadb-dump --single-transaction` (`mysqldump` trên MySQL, kèm `--set-gtid-purged=OFF`), nên website vẫn chạy bình thường trong lúc xuất. Bản dump có cả stored routine và trigger.

### Nhập

1. Mở menu của database và chọn **Nhập**.
2. Chọn tệp ở ô **File SQL (.sql hoặc .sql.gz)**. Tệp `.sql.gz` được giải nén ngay trong lúc nhập.
3. Không bắt buộc: đánh dấu **Xóa trống database trước (xóa toàn bộ bảng)**. Khi đó database bị xóa và tạo lại trước khi nhập, user của nó vẫn giữ quyền truy cập.
4. Chọn **Nhập**.

Mỗi lần nhập tối đa 4 GB.

Việc nhập chạy bằng một user MariaDB tạm thời, tên là `nefo-import-` kèm tên database, có toàn quyền trên database này và không có quyền gì trên database khác. User này cũng được đặt definer cho trigger, view và stored routine, nên các bản dump có chúng, như bản xuất và bản sao lưu của chính panel, vẫn nhập được như trước. Panel xóa user này khi việc nhập kết thúc, kể cả khi thất bại. Chỉ khi tệp tạo trigger, view, routine hoặc event không có mệnh đề `DEFINER`, vốn sau đó chạy bằng user này, panel mới giữ nó lại: bị khóa để không ai đăng nhập được, và chỉ có quyền trên database này. Xóa database thì user này cũng bị xóa.

Trên MySQL, tên user dài tối đa 32 ký tự, nên với database có tên dài, tên user được rút ngắn và thêm một đoạn mã băm ngắn. Ở đó definer được cho phép bằng `SET_ANY_DEFINER` và `ALLOW_NONEXISTENT_DEFINER` thay cho `SET USER` của MariaDB; không có `SYSTEM_USER`, user này vẫn không thể khiến một đối tượng chạy bằng quyền root. Dòng đầu tiên trong bản dump của các phiên bản MariaDB mới, `/*M!999999\- enable the sandbox mode */`, mà client của MySQL từ chối, được bỏ qua khi nhập vào MySQL.

::: warning
Nếu tệp dùng database khác, ví dụ `USE other_db`, `CREATE DATABASE other_db` hoặc `INSERT INTO other_db.posts`, MariaDB từ chối câu lệnh đó và việc nhập dừng lại với thông báo "File SQL dùng một database khác là other_db." Những gì tệp đã làm trước dòng đó vẫn còn trong database. Hãy xóa các dòng `CREATE DATABASE` và `USE` khỏi tệp, hoặc xuất lại database mà không dùng `--databases`, rồi nhập lại với ô **Xóa trống database trước (xóa toàn bộ bảng)** được đánh dấu. Các dòng `CREATE DATABASE` và `USE` ghi đúng tên database này thì không sao.
:::

## Mở truy cập từ xa

Mặc định MariaDB chỉ nhận kết nối từ chính máy chủ. Bạn có thể cho phép một số địa chỉ kết nối tới một database, ví dụ công cụ báo cáo trên máy tính ở văn phòng.

Trước tiên, hãy cài và bật tường lửa ufw ở trang **Tường lửa & SSH**. Panel từ chối mở truy cập từ xa khi chưa cài ufw.

1. Mở menu của database và chọn **Truy cập từ xa**.
2. Ở mục **Địa chỉ được phép**, nhập một địa chỉ. Chọn **Thêm địa chỉ** để nhập thêm.
3. Chọn **Lưu**.

Kết nối bằng user và mật khẩu của database tới địa chỉ IP công khai của máy chủ, cổng 3306.

### Địa chỉ được phép

- Từng địa chỉ IPv4 hoặc IPv6, ví dụ `203.0.113.10` hoặc `2001:db8::1`.
- Dải IPv4 dạng CIDR, ví dụ `198.51.100.0/24`, không rộng hơn `/16`.
- Không dùng được dải IPv6, vì MariaDB không khớp được chúng. Hãy nhập từng địa chỉ IPv6.
- Mỗi database tối đa 10 địa chỉ.
- Địa chỉ loopback, `0.0.0.0` và địa chỉ multicast bị từ chối.

### Panel thay đổi những gì

| Ở đâu | Thay đổi |
|---|---|
| User MariaDB | Với mỗi địa chỉ, một user `tên@địa-chỉ` có cùng mật khẩu và cùng quyền như user cục bộ |
| Tường lửa (ufw) | Với mỗi địa chỉ, một luật chỉ cho phép cổng 3306/tcp từ địa chỉ đó, kèm ghi chú `MariaDB tên` |
| Cấu hình MariaDB | `/etc/mysql/mariadb.conf.d/99-panel-remote.cnf` cho MariaDB lắng nghe trên mọi giao diện mạng (`0.0.0.0`, hoặc `::` khi có địa chỉ IPv6 được phép). Tệp này chỉ tồn tại khi còn ít nhất một database mở truy cập từ xa |

MySQL cũng làm như vậy: tệp là `/etc/mysql/mysql.conf.d/99-panel-remote.cnf`, ghi chú của các luật là `MySQL tên`, và mỗi khi thay đổi, user từ xa bị xóa rồi tạo lại, vì MySQL không có `CREATE OR REPLACE USER`. Các thông báo bên dưới khi đó nhắc tới MySQL.

MariaDB khởi động lại ở lần đầu bật truy cập từ xa, và khi tắt nó ở database cuối cùng. Website mất kết nối database trong vài giây. Nếu lúc đó đang có tác vụ khác chạy, hộp thoại báo **MariaDB cần khởi động lại trước khi nhận kết nối từ xa.** Hãy chọn **Khởi động lại MariaDB** khi tác vụ kia đã xong.

Nếu MariaDB không khởi động được với tệp truy cập từ xa, panel xóa tệp đó và khởi động MariaDB không có nó, để website vẫn chạy.

::: warning
- Nếu tường lửa đang tắt, cổng 3306 mở cho tất cả. MariaDB vẫn chỉ nhận các địa chỉ đã liệt kê.
- Kết nối không được mã hóa trừ khi máy khách yêu cầu TLS. Với dữ liệu nhạy cảm, hãy dùng SSH tunnel hoặc VPN.
- Chỉ liệt kê những địa chỉ bạn kiểm soát và không thay đổi.
:::

Để tắt truy cập từ xa, mở lại **Truy cập từ xa** và chọn **Tắt**. Khi xóa database, các user từ xa và luật tường lửa của nó cũng bị xóa.

## Tìm truy vấn chậm

MariaDB có thể ghi lại mọi truy vấn chạy lâu hơn một ngưỡng. Việc này giúp bạn tìm ra nguyên nhân làm website chậm.

1. Vào **Cơ sở dữ liệu** → **Truy vấn chậm**. Thẻ này hiện ra khi đã cài MariaDB.
2. Đặt **Chậm hơn** bằng một số giây, từ 0,1 đến 60.
3. Bật công tắc **Ghi lại truy vấn chậm** và chọn **Áp dụng**.

MariaDB phải đang chạy thì mới đổi được các thiết lập này. Thay đổi có hiệu lực ngay với các kết nối mở sau đó, và vẫn giữ sau khi khởi động lại: panel lưu nó trong `/etc/mysql/mariadb.conf.d/99-panel-slowlog.cnf`. Log được ghi vào `/var/log/mysql/mariadb-slow.log`, thư mục mà cơ chế xoay vòng log của gói MariaDB đã quản lý. Trên MySQL, thiết lập nằm ở `/etc/mysql/mysql.conf.d/99-panel-slowlog.cnf` và log là `/var/log/mysql/mysql-slow.log`; hãy dọn nó bằng **Xóa log** khi nó lớn dần.

Bảng hiện 200 mục mới nhất, mới nhất ở trên, đọc từ 4 MB cuối của log:

| Cột | Ý nghĩa |
|---|---|
| **Thời điểm** | Lúc truy vấn chạy |
| **User và máy** | Ai chạy, ví dụ `shop@localhost` |
| **Database** | Database được dùng |
| **Thời gian chạy** | Truy vấn mất bao lâu, tính bằng giây |
| **Số dòng đã quét** | MariaDB đã đọc bao nhiêu dòng |
| **Truy vấn** | Câu truy vấn. Chọn nó để xem toàn bộ, tối đa 2.000 ký tự. Truy vấn dài hơn được đánh dấu **Đã rút gọn** |

Chọn **Làm mới** để đọc lại log, và **Xóa log** để làm trống log.

::: tip
Ghi log làm máy chủ tốn thêm chút công sức. Khi xong, hãy tắt công tắc và chọn **Áp dụng**.
:::

## Adminer

Adminer giúp bạn xem và sửa database ngay trên trình duyệt, giống phpMyAdmin.

### Cài Adminer

1. Cài web server trước, ở trang **Phần mềm**. Adminer chạy trên web server đó cùng PHP.
2. Vào **Phần mềm** → **Adminer (giao diện database)** và chọn **Cài**.

Panel tải Adminer 6.3.1 (bản có đủ mọi driver của Adminer) và đối chiếu với mã SHA-256 đã ghim sẵn trước khi cài. Adminer mở được database MariaDB hoặc MySQL, và cả database PostgreSQL khi đã cài PostgreSQL (xem [PostgreSQL trong Adminer](#postgresql-in-adminer)). Driver SQLite của nó từ chối mọi lần đăng nhập, vì SQLite không có mật khẩu.

### Mở Adminer

1. Chọn **Mở bằng Adminer** ở đầu trang **Cơ sở dữ liệu**, hoặc trong thẻ thông tin kết nối của một database. **Phần mềm** → **Mở Adminer** cũng làm việc tương tự.
2. Adminer mở trong thẻ mới, tại địa chỉ panel của bạn cộng thêm `adminer/`, ví dụ `https://203.0.113.10:28443/k3x9p2qa7m1z/adminer/`.
3. Đăng nhập Adminer bằng user và mật khẩu của database, lấy từ **Xem đăng nhập**.

### Adminer được bảo vệ thế nào

Chỉ truy cập được Adminer qua bước đăng nhập của panel:

- Adminer chạy trên một site nội bộ của web server, chỉ lắng nghe trên 127.0.0.1, ở một cổng ngẫu nhiên từ 40000 đến 59999.
- Site nội bộ đó chỉ trả lời các yêu cầu mang một token bí mật. Yêu cầu khác nhận `404`.
- Panel chỉ gắn token vào yêu cầu từ một phiên đăng nhập panel hợp lệ. Không có phiên panel, `adminer/` trả về `401` và yêu cầu không tới được Adminer.
- Cookie phiên của panel không bao giờ được chuyển sang Adminer.

Vì vậy không thể truy cập Adminer từ Internet, và các chương trình khác trên máy chủ cũng không dùng được nó nếu không có token.

Adminer nhập được tệp tối đa 256 MB, và mỗi yêu cầu chạy tối đa 600 giây. Với bản dump lớn hơn, hãy dùng **Nhập** trên trang **Cơ sở dữ liệu**.

Gỡ Adminer sẽ xóa các tệp và site nội bộ của nó. Database của bạn không bị ảnh hưởng. Gỡ web server cũng gỡ luôn Adminer.

## phpMyAdmin {#phpmyadmin}

phpMyAdmin là giao diện database còn lại, cho MariaDB và MySQL. Hãy cài cái bạn thích, hoặc cả hai: chúng không dùng chung tệp nào.

### Cài phpMyAdmin {#install-phpmyadmin}

1. Cài web server trước, ở trang **Phần mềm**. phpMyAdmin chạy trên web server đó bằng PHP.
2. Vào **Phần mềm** → **phpMyAdmin (giao diện database)** và chọn **Cài**.

Panel tải phpMyAdmin 5.2.3 (bản đủ mọi ngôn ngữ) từ `files.phpmyadmin.net` và đối chiếu với mã SHA-256 đã ghim sẵn trước khi giải nén vào `/usr/share/nefo-phpmyadmin`. Panel bỏ đi script setup, các ví dụ và tài liệu cục bộ (các liên kết trợ giúp mở docs.phpmyadmin.net). phpMyAdmin để tệp tạm ở `/var/lib/nefo-phpmyadmin/tmp`.

Panel ghi tệp `config.inc.php` của phpMyAdmin, với một khóa mã hóa cookie ngẫu nhiên:

- Bạn đăng nhập bằng user và mật khẩu của database, và chỉ vào máy chủ cơ sở dữ liệu của máy này. Không chọn được máy chủ khác, không đăng nhập được khi để trống mật khẩu, và không đăng nhập được bằng `root`.
- phpMyAdmin không kiểm tra phiên bản mới và không gửi báo cáo lỗi.
- Biểu mẫu đổi mật khẩu của nó bị ẩn. Hãy đổi mật khẩu database ở trang **Cơ sở dữ liệu**, nơi panel cập nhật luôn `wp-config.php` của website WordPress.

### Mở phpMyAdmin {#open-phpmyadmin}

1. Chọn **Mở bằng phpMyAdmin** ở đầu trang **Cơ sở dữ liệu**, trong menu của một database, hoặc trong thẻ thông tin kết nối. **Phần mềm** → **Mở phpMyAdmin** cũng làm việc tương tự.
2. phpMyAdmin mở ở thẻ mới, tại địa chỉ panel kèm `phpmyadmin/`, ví dụ `https://203.0.113.10:28443/k3x9p2qa7m1z/phpmyadmin/`.
3. Đăng nhập bằng user và mật khẩu của database lấy từ **Xem đăng nhập**.

phpMyAdmin được bảo vệ giống Adminer (xem [Adminer được bảo vệ thế nào](#adminer-đuoc-bao-ve-the-nao)), trên site nội bộ riêng với mã bí mật riêng. Không có phiên đăng nhập panel, `phpmyadmin/` trả về `401`, và khóa API cũng không vào được. Chỉ ba điểm vào của phpMyAdmin chạy bằng PHP và chỉ các tệp script, style và hình ảnh của nó được trả về như tệp; mọi thứ khác trả về `404`, kể cả script setup. phpMyAdmin gửi Content-Security-Policy riêng và tự kiểm tra mã của biểu mẫu.

Giống Adminer, phpMyAdmin nhập được tệp tới 256 MB, và một yêu cầu chạy được tới 600 giây. Gỡ phpMyAdmin sẽ xóa các tệp, site nội bộ và thiết lập của nó. Database của bạn không bị ảnh hưởng. Gỡ web server cũng gỡ luôn phpMyAdmin.

## Trình duyệt key Redis

Khi đã cài Redis, trang **Cơ sở dữ liệu** có thêm thẻ **Redis**. Để cài Redis, vào **Phần mềm** → **Redis** → **Cài**. Xem cách Redis được thiết lập ở trang [Phần mềm](./software#redis).

Thẻ này hiện phiên bản Redis, thời gian đã chạy, **Bộ nhớ đã dùng** (kèm giới hạn bộ nhớ) và **Số key**. Nếu thẻ báo **Redis đã được cài nhưng đang không chạy.**, hãy khởi động Redis ở trang **Dịch vụ**.

### Tìm key

1. Chọn một database từ 0 đến 15. Danh sách cho biết mỗi database có bao nhiêu key.
2. Nhập mẫu tên key. `*` khớp với mọi thứ, `?` khớp một ký tự, ví dụ `user:*`.
3. Chọn **Tìm**.

Việc tìm đi qua các key theo từng đợt khoảng 100 key. Nếu còn nữa, chọn **Tìm tiếp**.

### Xem một key

Chọn một key để xem **Kiểu**, **Hết hạn sau**, **Kích thước** và giá trị của nó:

| Kiểu | Panel hiện gì |
|---|---|
| string | 64 KB đầu tiên của giá trị |
| list, hash, set, sorted set | 200 phần tử đầu tiên. Mỗi phần tử bị cắt ở 4 KB |
| stream và các kiểu khác | Chỉ kiểu và thời gian hết hạn |

Với giá trị nhị phân, các ký tự không hiển thị được sẽ bị thay thế.

### Xóa key

- Để xóa một key, chọn key đó rồi chọn **Xóa key**.
- Để xóa mọi key của database đang chọn, chọn **Xóa trống database 0** (nút hiện số của database), gõ số đó để xác nhận, rồi chọn **Xóa mọi key**.

Website dùng Redis làm bộ nhớ đệm sẽ tự tạo lại dữ liệu. Dữ liệu chỉ lưu trong Redis sẽ mất.

Trình duyệt key không sửa được giá trị hay thời gian hết hạn. Hãy dùng `redis-cli` trên máy chủ cho việc đó.

Nếu Redis có mật khẩu (`requirepass` trong `/etc/redis/redis.conf`), panel đọc mật khẩu từ tệp đó và dùng nó để đăng nhập.

## Xóa database

1. Mở menu của database và chọn **Xóa**.
2. Gõ tên database để xác nhận và chọn **Xóa**.

::: danger
Thao tác này xóa database, toàn bộ bảng và user của nó. Không thể hoàn tác. Các user từ xa, luật tường lửa và lịch sao lưu tự động của database cũng bị xóa theo. Các bản sao lưu đã tạo vẫn được giữ ở trang **Sao lưu**.
:::

## Gỡ MariaDB

Gỡ MariaDB ở trang **Phần mềm**. Panel từ chối khi MariaDB còn bất kỳ database nào ngoài các database hệ thống của chính nó, kể cả database tạo bên ngoài panel. Xem trang [Phần mềm](./software) để biết việc gỡ cài đặt giữ lại những gì.

MySQL được gỡ theo cách tương tự, trên thẻ của nó, với cùng bước kiểm tra. Panel cũng xóa kho của Oracle và khóa của kho. Thư mục dữ liệu `/var/lib/mysql` được giữ lại, trừ khi bạn đánh dấu ô và gõ `mysql`.

## PostgreSQL {#postgresql}

Nefo cũng chạy được PostgreSQL, máy chủ cơ sở dữ liệu mà nhiều ứng dụng Laravel, Django, Node.js và Odoo ưa dùng. PostgreSQL được cài từ kho phần mềm của hệ điều hành và quản lý ở thẻ **PostgreSQL** của trang **Cơ sở dữ liệu**. MariaDB và PostgreSQL chạy song song được.

### Cài PostgreSQL {#install-postgresql}

1. Vào **Phần mềm** → **PostgreSQL** và chọn **Cài**.
2. Chờ một đến hai phút. Log tiến trình hiện từng bước.

Nefo cài gói `postgresql` của hệ điều hành, nên bạn có phiên bản mà hệ thống đi kèm: 14 trên Ubuntu 22.04, 16 trên Ubuntu 24.04, 15 trên Debian 12 và 17 trên Debian 13. Các phiên bản mới hơn từ kho riêng của dự án PostgreSQL (PGDG) chưa được hỗ trợ.

apt chạy với locale C, nên cluster mà gói tạo ra lưu văn bản dạng `SQL_ASCII`. Khi đó Nefo tạo lại cluster mới, còn trống này với UTF-8 (`C.UTF-8`). Cluster đã có từ trước khi cài thì không bao giờ bị tạo lại.

### Cách PostgreSQL được tinh chỉnh {#how-postgresql-is-tuned}

Nefo ghi thiết lập vào `/etc/postgresql/<phiên bản>/main/conf.d/90-nefo.conf`. Các thiết lập bộ nhớ tùy theo RAM của máy chủ:

| RAM máy chủ | `shared_buffers` | `effective_cache_size` | `work_mem` | `maintenance_work_mem` | `max_connections` |
|---|---|---|---|---|---|
| Dưới 768 MB | 64MB | 256MB | 2MB | 16MB | 30 |
| Từ 768 MB | 128MB | 512MB | 4MB | 32MB | 40 |
| Từ 1,5 GB | 256MB | 1024MB | 4MB | 64MB | 50 |
| Từ 3 GB | 512MB | 2048MB | 8MB | 128MB | 75 |
| Từ 7 GB | 1024MB | 4096MB | 16MB | 256MB | 100 |

`effective_cache_size` bằng khoảng một nửa RAM, vì cùng máy chủ còn chạy web server, PHP và có thể cả MariaDB. Trên mọi máy chủ, Nefo còn đặt `listen_addresses = 'localhost'`, `password_encryption = 'scram-sha-256'`, `random_page_cost = 1.1` (cho ổ SSD) và `jit = off`.

Khi rảnh và chưa có dữ liệu, PostgreSQL cần khoảng 30 đến 40 MB cho vài tiến trình nền. Khi bộ đệm đầy dần, nó tăng tới khoảng `shared_buffers` cộng 40 MB, và mỗi kết nối đang mở tốn thêm vài MB. Trang **Phần mềm** hiển thị `shared_buffers` cộng 40 MB.

::: warning
Đừng sửa `90-nefo.conf` hay `pg_hba.conf`. Cài lại PostgreSQL sẽ ghi đè chúng.
:::

### Ai được kết nối {#who-can-connect}

Nefo ghi `/etc/postgresql/<phiên bản>/main/pg_hba.conf`:

```
local   all  postgres                peer
local   all  all                     scram-sha-256
host    all  all       127.0.0.1/32  scram-sha-256
host    all  all       ::1/128       scram-sha-256
```

- Chỉ user hệ thống `postgres` đăng nhập không cần mật khẩu, qua socket nội bộ. Panel quản lý máy chủ theo cách đó.
- Mọi người khác đều cần mật khẩu, qua socket hoặc TCP từ chính máy chủ này. Khác với mặc định của hệ điều hành, các user Linux khác không được đăng nhập không mật khẩu ("peer"), nên user Linux của một website không mở được role database trùng tên nếu không có mật khẩu.
- Máy khác không kết nối được: PostgreSQL chỉ lắng nghe trên localhost. Truy cập từ xa cho từng database như MariaDB hiện chưa có cho PostgreSQL.

### Tạo database PostgreSQL {#create-a-postgresql-database}

1. Vào **Cơ sở dữ liệu** → **PostgreSQL** và chọn **Thêm database**.
2. Nhập tên vào ô **Database**. Một role cùng tên sẽ sở hữu database này.
3. Để trống **Mật khẩu** để panel tạo mật khẩu mạnh ngẫu nhiên, hoặc tự nhập.
4. Tùy chọn: chọn một website ở ô **Website**. Bản sao lưu của website đó sẽ có cả database này, và xóa website kèm database cũng xóa nó.
5. Chọn **Tạo database**.

Thẻ thông tin kết nối hiện **Database**, **User**, **Mật khẩu**, **Host** (`localhost`) và **Cổng** (`5432`). Ví dụ với ứng dụng Laravel, đặt `DB_CONNECTION=pgsql`, `DB_HOST=127.0.0.1`, `DB_PORT=5432` cùng database, user và mật khẩu lấy từ thẻ.

Quy tắc:

- Tên và mật khẩu theo [quy tắc của MariaDB](#tao-database). Các tên riêng của PostgreSQL cũng bị từ chối: `postgres`, `template0`, `template1`, `public`, `all` và mọi tên bắt đầu bằng `pg_`.
- Một tên không dùng được cho cả hai máy chủ: nếu đã có database hoặc user MariaDB tên `shop`, thì database PostgreSQL không đặt tên đó được, và ngược lại. Panel cũng từ chối tên mà PostgreSQL đã dùng cho một database hoặc role.
- Role đăng nhập được, không phải superuser, không tạo được database hay role. Không role nào khác kết nối được vào database, vì panel thu hồi quyền `CONNECT` của mọi người khác.
- Database mới dùng UTF-8 với locale `C.UTF-8`.

Mật khẩu không bao giờ tới PostgreSQL ở dạng rõ: panel chỉ gửi verifier SCRAM-SHA-256 của nó, nên mật khẩu cũng không thể xuất hiện trong log của PostgreSQL.

**Xem đăng nhập**, **Đổi mật khẩu** và **Xóa** hoạt động như với MariaDB. Xóa cũng đóng các kết nối đang mở tới database. Database bị xóa ngoài panel được đánh dấu **Không tìm thấy trong PostgreSQL**.

### Xuất và nhập database PostgreSQL {#export-and-import-a-postgresql-database}

**Xuất (.sql.gz)** tải về bản dump SQL thuần đã nén gzip, tạo bằng `pg_dump` dưới role riêng của database, với `--no-owner --no-privileges`, nên nhập được vào bất kỳ database nào.

**Nhập** nhận file `.sql` hoặc `.sql.gz` tới 4 GB:

- `psql` chạy bằng role riêng của database, không bao giờ bằng `postgres`, với mật khẩu nằm trong một file tạm chỉ root đọc được.
- File chạy trong một transaction duy nhất với `ON_ERROR_STOP`: khi một câu lệnh lỗi thì không có gì được nhập, và thông báo lỗi ghi rõ dòng.
- Các lệnh riêng của psql, tức các dòng bắt đầu bằng dấu gạch chéo ngược (như `\connect`, `\!` hay `\copy`), bị từ chối: psql chạy bằng root, và các lệnh đó có thể chạy chương trình hoặc ghi file trên máy chủ. Panel khởi động psql ở chế độ hạn chế (`\restrict` với một khóa ngẫu nhiên) và bỏ các dòng `\restrict` và `\unrestrict` mà các bản `pg_dump` mới ghi vào. psql không có chế độ hạn chế (cũ hơn 17.6, 16.10, 15.14 hoặc 14.19) sẽ từ chối mọi lần nhập: hãy cài các bản cập nhật hệ thống trước.
- Câu lệnh tạo hay thay đổi role, giao đối tượng cho role khác hoặc cần quyền superuser sẽ lỗi. Hãy xuất lại bản dump đó bằng `pg_dump --no-owner --no-privileges`.
- `SET transaction_timeout`, do `pg_dump` 17 trở lên ghi vào, được bỏ qua trên máy chủ cũ hơn.
- Sau đó panel đặt lại mật khẩu của role, phòng khi file đã đổi nó.

Khi đánh dấu **Xóa trống database trước (xóa mọi thứ trong đó)**, database bị xóa và tạo lại trước khi nhập, vẫn do cùng role sở hữu.

### PostgreSQL trong Adminer {#postgresql-in-adminer}

Adminer cần extension `pgsql` của PHP để mở PostgreSQL. Panel cài extension này cho phiên bản PHP chạy Adminer, và chỉ khi có PostgreSQL: khi bạn cài PostgreSQL lúc đã có Adminer, hoặc cài Adminer lúc đã có PostgreSQL. Adminer do panel phiên bản cũ cài (bản chỉ dành cho MySQL) cũng được thay luôn lúc đó.

**Mở bằng Adminer** trên dòng của một database mở trang đăng nhập PostgreSQL của Adminer, đã điền sẵn server `localhost`, user và database. Nhập mật khẩu lấy từ **Xem đăng nhập**.

### PostgreSQL trong bản sao lưu {#postgresql-in-backups}

Bản sao lưu website có cả các database PostgreSQL của website, và bản sao lưu database có thể chứa một database PostgreSQL, chạy ngay hoặc theo lịch. Bản dump là đầu ra SQL thuần của `pg_dump`, tạo bằng role của database, lưu thành `postgres/<tên>.sql` trong file nén, cạnh các bản dump MariaDB.

Khi khôi phục, mỗi database được làm trống (xóa và tạo lại, vẫn do cùng role sở hữu) rồi bản dump được nhập bằng role đó. Database đã bị xóa kể từ lúc sao lưu sẽ được tạo lại với mật khẩu mới.

### Gỡ PostgreSQL {#uninstall-postgresql}

Gỡ PostgreSQL ở trang **Phần mềm**. Panel từ chối khi PostgreSQL còn bất kỳ database nào ngoài `postgres`, `template0` và `template1`, kể cả database tạo ngoài panel. Panel cũng từ chối khi PostgreSQL đang dừng, vì lúc đó không kiểm tra được. Dữ liệu trong `/var/lib/postgresql` và thiết lập trong `/etc/postgresql` được giữ lại, trừ khi bạn đánh dấu ô và gõ `postgresql`. Xem [Phần mềm](./software#postgresql).

## Memcached {#memcached}

Khi đã cài Memcached (**Phần mềm** → **Memcached**), trang **Cơ sở dữ liệu** có thẻ **Memcached**. Thẻ này hiện phiên bản, thời gian đã chạy, **Bộ nhớ đã dùng** cùng giới hạn, **Số mục**, tỷ lệ trúng và số kết nối đang mở. Nó cũng hiện những gì chính Memcached báo: chỉ lắng nghe trên 127.0.0.1 và UDP đang tắt.

**Khởi động lại** sẽ làm trống bộ nhớ đệm. Xem [Phần mềm](./software#memcached) để biết cách nó được thiết lập.
