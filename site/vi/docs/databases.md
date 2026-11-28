---
title: Cơ sở dữ liệu
description: Cài MariaDB hoặc MySQL, chọn phiên bản MariaDB, nâng cấp MariaDB, chuyển giữa MariaDB và MySQL, tạo database, nhập và xuất SQL, mở truy cập từ xa, xem log truy vấn chậm, và dùng Adminer, phpMyAdmin cùng trình duyệt key Redis.
---

# Cơ sở dữ liệu

Nefo chạy một máy chủ cơ sở dữ liệu tương thích MySQL trên mỗi máy chủ: MariaDB (bản của hệ điều hành, hoặc 11.4, 11.8 hay 12.3 LTS từ MariaDB) hoặc MySQL 8.4 LTS, được tinh chỉnh cho VPS nhỏ và chỉ truy cập được từ chính máy chủ. Trang **Cơ sở dữ liệu** tạo database và user, nhập và xuất SQL, mở một database cho các địa chỉ bạn chọn, và hiện các truy vấn chậm. PostgreSQL chạy được song song với nó.

Trang này hướng dẫn:

- chọn MariaDB hay MySQL và phiên bản MariaDB, cách cài, cách Nefo tinh chỉnh nó cho máy chủ nhỏ, cách nâng cấp MariaDB, và cách chuyển từ cái này sang cái kia;
- tạo database, xem thông tin đăng nhập và đổi mật khẩu;
- nhập và xuất SQL;
- mở truy cập từ xa cho từng database;
- log truy vấn chậm;
- Adminer và phpMyAdmin, giao diện database ngay trên trình duyệt;
- trình duyệt key Redis, ở thẻ **Redis** của cùng trang, và trạng thái của Memcached.

PostgreSQL có thẻ **PostgreSQL** riêng trên trang này và trang hướng dẫn riêng: xem [PostgreSQL](./postgresql). MongoDB, và PostgreSQL dạng container, có trong [Kho ứng dụng](./apps) dưới dạng ứng dụng Docker, chỉ lắng nghe trên 127.0.0.1. Các ứng dụng Docker đó không hiện trên trang **Cơ sở dữ liệu**.

## Chọn MariaDB hay MySQL {#choose-mariadb-or-mysql}

Mỗi máy chủ chạy một máy chủ cơ sở dữ liệu tương thích MySQL: MariaDB hoặc MySQL 8.4 LTS. Bạn chọn lúc cài, ở trang **Cơ sở dữ liệu**, trên thẻ **Máy chủ cơ sở dữ liệu** của trang **Phần mềm**, hoặc trong bước thiết lập (**Thiết lập máy chủ**). Khi đã cài một cái, panel từ chối cài cái còn lại; về sau bạn có thể [chuyển](#switch-between-mariadb-and-mysql) từ cái này sang cái kia, kèm theo các database.

| | MariaDB (khuyên dùng) | MySQL 8.4 LTS |
|---|---|---|
| Nguồn | Gói của hệ điều hành, hoặc một dòng LTS từ kho riêng của MariaDB (xem [Các phiên bản MariaDB](#mariadb-versions)) | Kho của Oracle, `repo.mysql.com` (nhánh `mysql-8.4-lts`) |
| RAM, ước chừng | Dung lượng buffer pool cộng khoảng 100 MB | Dung lượng buffer pool cộng khoảng 150 MB, hoặc cộng khoảng 300 MB khi bật performance schema, mà panel bật từ khoảng 3,5 GB RAM (tức là cả máy chủ 4 GB) |
| Hỗ trợ | Theo thời gian hỗ trợ của hệ điều hành; một dòng lấy từ kho của MariaDB thì đến ngày MariaDB ngừng hỗ trợ dòng đó | Oracle hỗ trợ 8.4 LTS đến năm 2032 |
| Hệ điều hành | Mọi hệ điều hành Nefo hỗ trợ | Ubuntu 22.04, 24.04, 26.04 và Debian 12, 13, chỉ trên amd64 (x86-64) |
| Hợp với | WordPress và hầu hết ứng dụng PHP | Ứng dụng cần đúng MySQL, hoặc đội ngũ đã quen dùng MySQL |

Cả hai đều dùng được mọi chức năng trên trang này, cùng sao lưu, nhân bản website, staging WordPress và các ứng dụng PHP của kho ứng dụng. Adminer và phpMyAdmin mở được cả hai. Trên máy chủ arm64, hoặc hệ điều hành mà Oracle không có gói, trang **Phần mềm** chỉ đưa ra MariaDB và nói rõ lý do.

## Cài MariaDB {#install-mariadb}

1. Vào **Cơ sở dữ liệu**.
2. Chọn **Cài MariaDB (khuyên dùng)**. Bạn cũng có thể cài ở trang **Phần mềm**: giữ lựa chọn **MariaDB** trên thẻ **Máy chủ cơ sở dữ liệu** rồi chọn **Cài MariaDB**.
3. Chờ khoảng một đến hai phút. Log tiến trình hiện từng bước.

Nefo cài gói `mariadb-server` và `mariadb-client` từ kho phần mềm của hệ điều hành. Sau đó panel ghi cấu hình, khởi động MariaDB, rồi xóa các user ẩn danh và database `test` mà một số phiên bản tạo sẵn.

Nếu muốn cài một dòng mới hơn từ kho riêng của MariaDB thay vào đó, xem [Các phiên bản MariaDB](#mariadb-versions).

### Cách MariaDB được tinh chỉnh {#how-mariadb-is-tuned}

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

## Có thể cài những phiên bản MariaDB nào? {#mariadb-versions}

Mặc định MariaDB được lấy từ hệ điều hành. Khi đó các bản vá bảo mật của nó đến cùng các bản cập nhật hệ thống, và không có kho nào khác được thêm vào. Thay vào đó, bạn có thể cài một dòng hỗ trợ dài hạn (LTS) từ kho riêng của MariaDB, `dlm.mariadb.com`, khi cần MariaDB mới hơn bản mà hệ điều hành có:

| Dòng | MariaDB hỗ trợ đến | Có trên |
|---|---|---|
| 11.4 LTS | 29/5/2029 | Ubuntu 22.04 và 24.04, Debian 12 |
| 11.8 LTS | 4/6/2028 | Ubuntu 22.04 và 24.04, Debian 12 |
| 12.3 LTS | 12/6/2029 | Ubuntu 22.04, 24.04 và 26.04, Debian 12 và 13 |

Một dòng chỉ được đưa ra ở nơi nó mới hơn MariaDB của chính hệ điều hành:

| Hệ điều hành | MariaDB của hệ điều hành | Dòng được đưa ra |
|---|---|---|
| Ubuntu 22.04 | 10.6 | 11.4, 11.8, 12.3 |
| Ubuntu 24.04 | 10.11 | 11.4, 11.8, 12.3 |
| Debian 12 | 10.11 | 11.4, 11.8, 12.3 |
| Ubuntu 26.04 | 11.8 | 12.3 |
| Debian 13 | 11.8 | 12.3 |

MariaDB 12.3 không cài Galera (phần mềm cluster của MariaDB, thứ mà một máy chủ đơn không dùng tới); 11.4 và 11.8 cài kèm nó vì gói của chúng yêu cầu.

Để cài một dòng:

1. Vào **Phần mềm**. Trên thẻ **Máy chủ cơ sở dữ liệu**, giữ lựa chọn **MariaDB**.
2. Bên dưới, chọn phiên bản, ví dụ **11.4 LTS từ MariaDB**. Phiên bản của hệ điều hành, ví dụ "10.11, bản của hệ điều hành", vẫn là mặc định.
3. Chọn **Cài MariaDB 11.4**.

Bước thiết lập (**Thiết lập máy chủ**) cũng đưa ra các phiên bản này ở mục **MariaDB**, và lệnh `nefo setup --db mariadb-11.4` cũng vậy (xem [Dòng lệnh](./cli#nefo-setup)).

Panel làm những gì với một dòng lấy từ kho của MariaDB:

- Panel tải khóa ký của MariaDB từ `https://supplychain.mariadb.com/MariaDB-Server-GPG-KEY` (hoặc `keyserver.ubuntu.com`) và chỉ chấp nhận nếu dấu vân tay đúng là `177F 4010 FE56 CA33 3630 0305 F165 6F24 C74C D1D8`, tức "MariaDB Signing Key" mà tài liệu của MariaDB công bố. Khóa được lưu ở `/etc/apt/keyrings/panel-mariadb.gpg`, chỉ được tin cho riêng kho này.
- Panel ghi `/etc/apt/sources.list.d/panel-mariadb.list` chỉ với một mục cho đúng dòng đó, ví dụ `https://dlm.mariadb.com/repo/mariadb-server/11.4/repo/ubuntu noble main`. Khi đó máy chủ của bạn nhận các bản phát hành nhỏ (point release) của 11.4 cùng các bản cập nhật hệ thống, và không bao giờ tự lên một dòng mới hơn.
- Panel kiểm tra rằng apt sẽ thực sự cài gói MariaDB của dòng đó, rồi cài `mariadb-server` và `mariadb-client`, được tinh chỉnh giống MariaDB của hệ điều hành.

Gỡ MariaDB cũng xóa luôn tệp kho và khóa.

Panel từ chối:

- cài một dòng khi đang có một MariaDB khác: hãy dùng **Nâng cấp** (xem bên dưới);
- cài MariaDB cũ hơn bản đã tạo ra dữ liệu trong `/var/lib/mysql`, còn giữ lại từ lần cài trước, vì nó không đọc được dữ liệu đó. Hãy cài đúng phiên bản đó hoặc bản mới hơn, hoặc chuyển thư mục dữ liệu đi chỗ khác trước.

## Nâng cấp MariaDB {#upgrade-mariadb}

Bạn có thể nâng cấp MariaDB đã cài lên một dòng LTS mới hơn từ kho của MariaDB, ví dụ từ 10.11 của hệ điều hành lên 11.4, hoặc từ 11.8 của Debian 13 lên 12.3, mà các database vẫn nằm nguyên chỗ.

1. Vào **Phần mềm**. Trên thẻ **Máy chủ cơ sở dữ liệu**, chọn **Nâng cấp**.
2. Chọn dòng. Hộp thoại cho biết những gì sẽ diễn ra, và cần bao nhiêu dung lượng đĩa trống.
3. Chọn **Nâng cấp lên MariaDB 11.4** và theo dõi log.

Việc nâng cấp chạy theo thứ tự sau:

1. Mọi database được xuất ra, mỗi database một tệp SQL nén gzip, vào một thư mục mới trong `/var/backups/panel/db-server/`. Panel giữ các bản dump này cho đến khi bạn xóa chúng.
2. Các gói của dòng mới được tải về, còn gói của bản đang cài được giữ lại, để có thể quay lại mà không cần mạng.
3. MariaDB chuyển sang chế độ chỉ đọc, panel đếm số dòng của mọi bảng, rồi MariaDB dừng. Thư mục dữ liệu của nó được sao chép sang `/var/lib/mysql.before-11.4-<date>-<time>`, còn `/etc/mysql` được sao vào thư mục chứa bản dump. Từ lúc này, website không dùng được database cho đến khi nâng cấp xong, thường mất vài phút.
4. Gói mới thay gói cũ và khởi động MariaDB trên cùng dữ liệu đó, vẫn ở chế độ chỉ đọc. Sau đó `mariadb-upgrade` cập nhật các bảng hệ thống.
5. Panel kiểm tra rằng MariaDB báo đúng phiên bản mới, rằng mọi database có cùng các bảng với cùng số dòng, cùng view, routine, trigger và event, và rằng mỗi user database của panel đăng nhập được bằng mật khẩu của nó.
6. MariaDB nhận ghi trở lại, và bản sao thư mục dữ liệu bị xóa.

Nếu bất kỳ bước nào từ bước 4 trở đi thất bại, panel đưa MariaDB cũ trở lại: gỡ các gói mới, khôi phục `/etc/mysql` và thư mục dữ liệu đã sao chép, cài lại các gói cũ rồi khởi động MariaDB. Log kết thúc bằng lý do lỗi. Khi đó các database của bạn vẫn như trước khi nâng cấp.

::: warning Không thể quay về dòng cũ hơn
Sau khi nâng cấp, MariaDB đã nâng cấp các tệp dữ liệu và bảng hệ thống của nó, mà MariaDB cũ hơn không đọc được. Panel từ chối hạ cấp, ví dụ từ 11.8 về 11.4, và nói rõ lý do. Muốn quay lại, hãy xuất các database, gỡ MariaDB cùng dữ liệu, cài phiên bản cũ hơn rồi nhập lại.
:::

Panel cần dung lượng đĩa trống cho các bản dump, bản sao thư mục dữ liệu và các gói: khoảng một lần rưỡi dung lượng thư mục dữ liệu cộng 400 MB. Không đủ thì panel không bắt đầu. Panel cũng từ chối khi MariaDB đang dừng, hoặc khi cấu hình của máy chủ đặt thư mục dữ liệu ở nơi khác ngoài `/var/lib/mysql`.

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
| `performance_schema` | `OFF` dưới khoảng 3,5 GB RAM, `ON` từ mức đó trở lên (tức là cả máy chủ 4 GB) | Riêng nó đã tốn 100 đến 200 MB |
| `skip-name-resolve`, `host_cache_size` | bật, `0` | Không tra DNS cho mỗi kết nối |
| `innodb_log_buffer_size` | `16M` | Mặc định của MySQL 8.4 là 64M |
| `character-set-server`, `collation-server` | `utf8mb4`, `utf8mb4_unicode_ci` | Giống MariaDB, để bản dump chuyển qua lại được |

Khi đó MySQL dùng khoảng bằng dung lượng buffer pool cộng thêm 150 MB, hoặc cộng 300 MB khi bật performance schema. Trang **Phần mềm** hiện con số ước tính này; trang **Dịch vụ** cho biết mức dùng thực tế.

User đăng nhập bằng `caching_sha2_password`, mặc định của MySQL 8.4, mà `mysqli` và PDO của PHP đều hỗ trợ. `mysql_native_password` vẫn tắt như MySQL 8.4 phát hành: nó băm bằng SHA-1 không có salt, và MySQL 9 đã bỏ hẳn nó. Vì vậy mã băm mật khẩu cũ từ máy chủ khác, như cPanel lưu, không dùng lại được trên MySQL: database chuyển sang cần có mật khẩu của nó, hoặc một mật khẩu mới.

Trên MySQL, panel chạy SQL bằng lệnh `mysql` thay cho `mariadb`, xuất bằng `mysqldump`, và dịch vụ tên là `mysql`.

::: warning
Đừng sửa `90-panel.cnf`. Panel sẽ ghi đè tệp này.
:::

## Có thể chuyển giữa MariaDB và MySQL không? {#switch-between-mariadb-and-mysql}

Có, từ v0.24.0. Mỗi máy chủ chạy một máy chủ cơ sở dữ liệu, nhưng bạn có thể thay MariaDB bằng MySQL 8.4 LTS, hoặc MySQL bằng MariaDB, kèm theo các database, user và quyền của chúng.

1. Vào **Phần mềm**. Trên thẻ **Máy chủ cơ sở dữ liệu**, ở mục **Dùng MySQL 8.4 LTS thay thế** (hoặc **Dùng MariaDB thay thế**), chọn **Chuyển sang MySQL 8.4 LTS** (hoặc **Chuyển sang MariaDB**).
2. Đọc hộp thoại. Hộp thoại liệt kê các database và tài khoản sẽ được chuyển sang, và cách mỗi tài khoản đăng nhập sau đó.
3. Gõ `mysql` (hoặc `mariadb`) để xác nhận, rồi chọn nút chuyển.

Chuyển ngược về MariaDB sẽ cài MariaDB của hệ điều hành. Khi không có database nào, cùng tác vụ này chỉ thay máy chủ này bằng máy chủ kia.

### Việc chuyển làm những gì {#what-the-switch-does}

1. Panel tải gói của máy chủ mới, và giữ gói của máy chủ cũ để có thể quay lại. Website của bạn chưa bị ảnh hưởng gì.
2. Máy chủ cũ chuyển sang chỉ đọc: website vẫn đọc được database nhưng không ghi được. Panel đọc các tài khoản và quyền của nó, đếm số dòng của mọi bảng và xuất mọi database vào một thư mục mới trong `/var/backups/panel/db-server/`.
3. Máy chủ cũ dừng. Thư mục dữ liệu của nó được chuyển sang `/var/lib/mysql.mariadb-<date>-<time>` (hoặc `.mysql-…`), còn `/etc/mysql` được sao vào thư mục chứa bản dump. Các gói của nó bị gỡ. Từ lúc này, website không dùng được database cho đến khi việc chuyển kết thúc.
4. Máy chủ mới được cài giống như khi cài từ trang **Phần mềm**, và vẫn ở chế độ chỉ đọc. Truy cập từ xa và log truy vấn chậm được thiết lập lại như trước.
5. Các tài khoản được tạo, mọi database được nhập, và quyền được cấp lại. Bản dump từ MySQL 8 được chỉnh cho MariaDB giống cách tính năng nhập website chỉnh chúng: các collation `utf8mb4_0900_*` của nó được đổi thành collation của MariaDB (`utf8mb4_unicode_520_ci`, collation riêng của một ngôn ngữ, hoặc `utf8mb4_bin`) và các thiết lập chỉ MySQL mới có được bỏ đi. Bản dump từ MariaDB 11 sang MySQL được thay các collation `uca1400` của MariaDB bằng collation của MySQL (`utf8mb4_uca1400_ai_ci` thành `utf8mb4_0900_ai_ci`).
6. Panel kiểm tra rằng mọi database có cùng các bảng với cùng số dòng, cùng view, routine, trigger và event, và rằng mỗi user database của panel đăng nhập được bằng mật khẩu của nó. Sau đó máy chủ nhận ghi trở lại và thư mục dữ liệu cũ bị xóa.

Nếu bất kỳ bước nào từ bước 3 trở đi thất bại, panel gỡ máy chủ mới, khôi phục cấu hình và thư mục dữ liệu của máy chủ cũ, cài lại gói của nó và khởi động nó: bạn trở về đúng như lúc đầu. Bản dump vẫn được giữ trong cả hai trường hợp.

Thời gian chuyển tùy vào dung lượng các database: việc xuất, việc nhập và việc đếm dòng đều đọc toàn bộ dữ liệu một lần.

### Tài khoản và mật khẩu {#accounts-and-passwords}

MariaDB và MySQL lưu mật khẩu theo cách khác nhau, nên không phải mật khẩu nào cũng chuyển sang được:

| Tài khoản | Sau khi chuyển |
|---|---|
| User của một database trên trang **Cơ sở dữ liệu** | Giữ mật khẩu của nó, mà panel biết, trên `localhost` và trên mỗi địa chỉ được phép truy cập từ xa |
| Tài khoản bạn tự tạo với mật khẩu `mysql_native_password`, khi chuyển sang MariaDB | Giữ mật khẩu của nó, lấy từ mã băm |
| Tài khoản đăng nhập qua socket (`unix_socket` trên MariaDB, `auth_socket` trên MySQL) | Vẫn đăng nhập qua socket |
| Mọi tài khoản khác bạn tự tạo, chẳng hạn tài khoản MariaDB chuyển sang MySQL (MySQL 8.4 để `mysql_native_password` ở trạng thái tắt) hoặc tài khoản MySQL chuyển sang MariaDB (MariaDB không đọc được `caching_sha2_password`) | Được tạo với một mật khẩu ngẫu nhiên mới và bị khóa. Log liệt kê từng tài khoản. Hãy đặt mật khẩu với quyền root trên máy chủ: `ALTER USER 'name'@'localhost' IDENTIFIED BY '…' ACCOUNT UNLOCK;` |

Quyền của các tài khoản được cấp lại theo `SHOW GRANTS` của máy chủ cũ. Quyền nào máy chủ kia không có sẽ bị bỏ qua và được ghi trong log, cũng như các quyền role và proxy, vốn panel không chuyển sang.

### Những gì vẫn hoạt động {#what-keeps-working}

- Website giữ nguyên thiết lập: chúng kết nối tới `localhost`, qua cùng một socket trên cả hai máy chủ.
- phpMyAdmin và Adminer mở máy chủ mới.
- Sao lưu: các bản sao lưu mới dùng chương trình xuất của máy chủ mới, và bản sao lưu tạo trước khi chuyển vẫn khôi phục được sau đó.
- Truy cập từ xa vẫn mở cho đúng các địa chỉ cũ.

### Khi panel từ chối {#when-the-panel-refuses}

Hộp thoại liệt kê những gì cần sửa trước, dưới dòng **Hãy xử lý những điểm sau trước:**, và nút chuyển bị vô hiệu:

| Lý do | Cách xử lý |
|---|---|
| Panel không biết mật khẩu của user một database (database được chuyển sang chỉ kèm mã băm mật khẩu) | Đặt mật khẩu mới cho database đó ở trang **Cơ sở dữ liệu**, rồi ghi nó vào cấu hình của website |
| Khi chuyển sang MySQL: có bảng dùng storage engine riêng của MariaDB (Aria và các engine khác), sequence hoặc bảng system-versioned | Chuyển đổi chúng, ví dụ bằng `ALTER TABLE … ENGINE=InnoDB`, hoặc xóa chúng |
| Khi chuyển sang MySQL: có cột `INET4`, `INET6` hoặc `UUID` | Đổi kiểu cột, ví dụ sang `VARCHAR` |
| Khi chuyển sang MySQL: view, trigger hoặc routine có tên user của definer dài hơn 32 ký tự | Tạo lại chúng với definer khác |
| Tên database bắt đầu bằng dấu gạch ngang | Đổi tên database |
| Không đủ dung lượng đĩa trống: bản dump và dữ liệu đã nhập, nằm cạnh thư mục dữ liệu cũ, cần khoảng gấp đôi dung lượng các database cộng 700 MB | Giải phóng bớt dung lượng; xóa các bản dump cũ |

Việc chuyển cũng bị từ chối khi máy chủ cơ sở dữ liệu đang dừng, khi cấu hình của máy chủ đặt thư mục dữ liệu ở nơi khác ngoài `/var/lib/mysql`, và, với MySQL, ở nơi không có MySQL (arm64, hoặc hệ điều hành mà Oracle không có gói).

Một số khác biệt chỉ lộ ra lúc nhập dữ liệu, chẳng hạn index mà MariaDB đánh dấu `IGNORED`, thứ MySQL không biết. Khi đó việc nhập thất bại và panel quay lại máy chủ cũ, kèm lỗi trong log.

## Bản dump từ các lần nâng cấp và chuyển đổi {#dumps-from-upgrades-and-switches}

Thẻ **Máy chủ cơ sở dữ liệu** trên trang **Phần mềm** liệt kê các thư mục ở mục **Các bản dump đang giữ**: thời điểm, nội dung (ví dụ "MariaDB 10.11.14 → MariaDB 11.4"), kết quả **Thành công**, **Đã khôi phục bản cũ** hay **Thất bại**, số database, dung lượng và thư mục. Mỗi database là một tệp, `001.sql.gz`, `002.sql.gz` và cứ thế, kèm `manifest.json` ghi tên database của từng tệp, vì tên database có thể chứa những ký tự mà tên tệp không chứa được. Chỉ root đọc được thư mục này.

Để lấy lại một database, giải nén tệp của nó và nhập ở trang **Cơ sở dữ liệu**. Để xóa một thư mục, chọn **Xóa các bản dump này** (biểu tượng thùng rác bên cạnh). Panel từ chối khi đang chạy nâng cấp hoặc chuyển đổi.

## Tạo database {#create-a-database}

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

### User và quyền {#users-and-privileges}

Mỗi database có một user là `name@localhost`, có toàn quyền trên database đó và không có quyền gì trên database khác. Khi bật truy cập từ xa, user này có thêm một bản sao cho mỗi địa chỉ được phép (xem [Mở truy cập từ xa](#allow-remote-access)). Ngoài user tạm thời dùng để nhập (xem [Nhập](#import)), panel không tạo thêm user và không đổi quyền. Nếu cần, hãy dùng lệnh `mariadb` (`mysql` trên MySQL) trên máy chủ với quyền root.

Danh sách chỉ gồm các database do panel tạo. Database bị xóa từ bên ngoài panel được đánh dấu **Không tìm thấy trong MariaDB** (hoặc **Không tìm thấy trong MySQL**).

## Xem thông tin đăng nhập của database {#show-a-database-s-login}

Chọn **Xem đăng nhập** trên dòng của database. Thẻ thông tin kết nối hiện tên, user, mật khẩu và máy chủ, mỗi mục có nút sao chép. Nếu đã cài Adminer hoặc phpMyAdmin, thẻ có thêm nút **Mở bằng Adminer** hoặc **Mở bằng phpMyAdmin**.

## Đổi mật khẩu database {#change-a-database-password}

1. Mở menu của database và chọn **Đổi mật khẩu**.
2. Giữ **Tạo mật khẩu mạnh ngẫu nhiên**, hoặc chọn **Tự đặt mật khẩu** rồi gõ mật khẩu. Mật khẩu tự đặt theo cùng quy tắc như khi [tạo database](#create-a-database).
3. Chọn **Đổi mật khẩu**.

Panel đặt mật khẩu mới và hiện nó trong thẻ thông tin kết nối. Nếu database gắn với một website WordPress do panel tạo, panel cũng ghi mật khẩu mới vào `wp-config.php` của site đó. Các ứng dụng khác sẽ không kết nối được cho đến khi bạn cập nhật cấu hình của chúng.

Nếu database đang mở truy cập từ xa, các user từ xa cũng nhận mật khẩu mới.

## Xuất và nhập {#export-and-import}

### Xuất {#export}

Mở menu của database và chọn **Xuất (.sql)**. Trình duyệt tải về một tệp SQL thường, có tên dạng `shop_20261126-143000.sql`.

Bản dump được tạo bằng `mariadb-dump --single-transaction` (`mysqldump` trên MySQL, kèm `--set-gtid-purged=OFF`), nên website vẫn chạy bình thường trong lúc xuất. Bản dump có cả stored routine và trigger.

### Nhập {#import}

1. Mở menu của database và chọn **Nhập**.
2. Chọn tệp ở ô **File SQL (.sql hoặc .sql.gz)**. Tệp `.sql.gz` được giải nén ngay trong lúc nhập.
3. Không bắt buộc: đánh dấu **Xóa trống database trước (xóa toàn bộ bảng)**. Khi đó database bị xóa và tạo lại trước khi nhập, user của nó vẫn giữ quyền truy cập.
4. Chọn **Nhập**.

Mỗi lần nhập tối đa 4 GB.

Bản dump do máy chủ kia tạo, chẳng hạn bản sao lưu từ trước khi [chuyển](#switch-between-mariadb-and-mysql), cũng nhập được: panel viết lại tên các collation mà máy chủ hiện tại không biết ngay trong lúc tệp được đọc vào (`utf8mb4_0900_*` của MySQL 8 khi nhập vào MariaDB, các collation `uca1400` của MariaDB 11 khi nhập vào MySQL). Trên MySQL, panel còn bỏ `NO_AUTO_CREATE_USER`, thứ MariaDB ghi vào SQL mode của mỗi trigger và routine mà MySQL 8 từ chối. Ngoài ra không có gì trong tệp bị thay đổi; bản dump dùng database khác vẫn bị từ chối. Khôi phục bản sao lưu database cũng hoạt động như vậy.

Việc nhập chạy bằng một user MariaDB tạm thời, tên là `nefo-import-` kèm tên database, có toàn quyền trên database này và không có quyền gì trên database khác. User này cũng được đặt definer cho trigger, view và stored routine, nên các bản dump có chúng, như bản xuất và bản sao lưu của chính panel, vẫn nhập được như trước. Panel xóa user này khi việc nhập kết thúc, kể cả khi thất bại. Chỉ khi tệp tạo trigger, view, routine hoặc event không có mệnh đề `DEFINER`, vốn sau đó chạy bằng user này, panel mới giữ nó lại: bị khóa để không ai đăng nhập được, và chỉ có quyền trên database này. Xóa database thì user này cũng bị xóa.

Trên MySQL, tên user dài tối đa 32 ký tự, nên với database có tên dài, tên user được rút ngắn và thêm một đoạn mã băm ngắn. Ở đó definer được cho phép bằng `SET_ANY_DEFINER` và `ALLOW_NONEXISTENT_DEFINER` thay cho `SET USER` của MariaDB; không có `SYSTEM_USER`, user này vẫn không thể khiến một đối tượng chạy bằng quyền root. Dòng đầu tiên trong bản dump của các phiên bản MariaDB mới, `/*M!999999\- enable the sandbox mode */`, mà client của MySQL từ chối, được bỏ qua khi nhập vào MySQL.

::: warning
Nếu tệp dùng database khác, ví dụ `USE other_db`, `CREATE DATABASE other_db` hoặc `INSERT INTO other_db.posts`, MariaDB từ chối câu lệnh đó và việc nhập dừng lại với thông báo bắt đầu bằng "File SQL dùng một database khác là other_db." Những gì tệp đã làm trước dòng đó vẫn còn trong database. Hãy xóa các dòng `CREATE DATABASE` và `USE` khỏi tệp, hoặc xuất lại database mà không dùng `--databases`, rồi nhập lại với ô **Xóa trống database trước (xóa toàn bộ bảng)** được đánh dấu. Các dòng `CREATE DATABASE` và `USE` ghi đúng tên database này thì không sao.
:::

## Mở truy cập từ xa {#allow-remote-access}

Mặc định MariaDB chỉ nhận kết nối từ chính máy chủ. Bạn có thể cho phép một số địa chỉ kết nối tới một database, ví dụ công cụ báo cáo trên máy tính ở văn phòng.

Trước tiên, hãy cài và bật tường lửa ufw ở trang **Tường lửa & SSH**. Panel từ chối mở truy cập từ xa khi chưa cài ufw.

1. Mở menu của database và chọn **Truy cập từ xa**.
2. Ở mục **Địa chỉ được phép**, nhập một địa chỉ. Chọn **Thêm địa chỉ** để nhập thêm.
3. Chọn **Lưu**.

Kết nối bằng user và mật khẩu của database tới địa chỉ IP công khai của máy chủ, cổng 3306.

### Địa chỉ được phép {#allowed-addresses}

- Từng địa chỉ IPv4 hoặc IPv6, ví dụ `203.0.113.10` hoặc `2001:db8::1`.
- Dải IPv4 dạng CIDR, ví dụ `198.51.100.0/24`, không rộng hơn `/16`.
- Không dùng được dải IPv6, vì MariaDB không khớp được chúng. Hãy nhập từng địa chỉ IPv6.
- Mỗi database tối đa 10 địa chỉ.
- Địa chỉ loopback, `0.0.0.0` và địa chỉ multicast bị từ chối.

### Panel thay đổi những gì {#what-the-panel-changes}

| Ở đâu | Thay đổi |
|---|---|
| User MariaDB | Với mỗi địa chỉ, một user `name@address` có cùng mật khẩu và cùng quyền như user cục bộ |
| Tường lửa (ufw) | Với mỗi địa chỉ, một luật chỉ cho phép cổng 3306/tcp từ địa chỉ đó, kèm ghi chú `MariaDB name` |
| Cấu hình MariaDB | `/etc/mysql/mariadb.conf.d/99-panel-remote.cnf` cho MariaDB lắng nghe trên mọi giao diện mạng (`0.0.0.0`, hoặc `::` khi có địa chỉ IPv6 được phép). Tệp này chỉ tồn tại khi còn ít nhất một database mở truy cập từ xa |

MySQL cũng làm như vậy: tệp là `/etc/mysql/mysql.conf.d/99-panel-remote.cnf`, ghi chú của các luật là `MySQL name`, và mỗi khi thay đổi, user từ xa bị xóa rồi tạo lại, vì MySQL không có `CREATE OR REPLACE USER`. Các thông báo bên dưới khi đó nhắc tới MySQL.

MariaDB khởi động lại ở lần đầu bật truy cập từ xa, và khi tắt nó ở database cuối cùng. Website mất kết nối database trong vài giây. Nếu lúc đó đang có tác vụ khác chạy, hộp thoại báo "MariaDB cần khởi động lại trước khi nhận kết nối từ xa." Hãy chọn **Khởi động lại MariaDB** khi tác vụ kia đã xong.

Nếu MariaDB không khởi động được với tệp truy cập từ xa, panel xóa tệp đó và khởi động MariaDB không có nó, để website vẫn chạy.

::: warning
- Nếu tường lửa đang tắt, cổng 3306 mở cho tất cả. MariaDB vẫn chỉ nhận các địa chỉ đã liệt kê.
- Kết nối không được mã hóa trừ khi máy khách yêu cầu TLS. Với dữ liệu nhạy cảm, hãy dùng SSH tunnel hoặc VPN.
- Chỉ liệt kê những địa chỉ bạn kiểm soát và không thay đổi.
:::

Để tắt truy cập từ xa, mở lại **Truy cập từ xa** và chọn **Tắt**. Khi xóa database, các user từ xa và luật tường lửa của nó cũng bị xóa.

## Tìm truy vấn chậm {#find-slow-queries}

MariaDB có thể ghi lại mọi truy vấn chạy lâu hơn một ngưỡng. Việc này giúp bạn tìm ra nguyên nhân làm website chậm.

1. Vào **Cơ sở dữ liệu** → **Truy vấn chậm**. Thẻ này hiện ra khi đã cài MariaDB.
2. Đặt **Chậm hơn** bằng một số giây, từ 0,1 đến 60.
3. Bật công tắc **Ghi lại truy vấn chậm** và chọn **Áp dụng**.

MariaDB phải đang chạy thì mới đổi được các thiết lập này. Thay đổi có hiệu lực ngay với các kết nối mở sau đó, và vẫn giữ sau khi khởi động lại: panel lưu nó trong `/etc/mysql/mariadb.conf.d/99-panel-slowlog.cnf`, và log được ghi vào `/var/log/mysql/mariadb-slow.log`. Trên MySQL, thiết lập nằm ở `/etc/mysql/mysql.conf.d/99-panel-slowlog.cnf` và log là `/var/log/mysql/mysql-slow.log`.

Panel xoay vòng log để nó không làm đầy ổ đĩa, trên cả MariaDB lẫn MySQL. Mỗi giờ panel xem log một lần, và khi log lớn hơn 50 MB, hoặc mục cũ nhất trong đó đã được 7 ngày, panel đổi tên nó, yêu cầu máy chủ mở log mới (`FLUSH SLOW LOGS`, không cần khởi động lại) và nén log cũ ngay bên cạnh, thành `mariadb-slow.log-YYYYMMDD-HHMMSS.gz` (`mysql-slow.log-…` trên MySQL). Panel giữ 4 tệp mới nhất và xóa các tệp cũ hơn. Thẻ cho biết hiện panel đang giữ bao nhiêu tệp. Bảng chỉ đọc log hiện tại.

Bảng hiện 200 mục mới nhất, mới nhất ở trên, đọc từ 4 MB cuối của log:

| Cột | Ý nghĩa |
|---|---|
| **Thời điểm** | Lúc truy vấn chạy |
| **User và máy** | Ai chạy, ví dụ `shop@localhost` |
| **Database** | Database được dùng |
| **Thời gian chạy** | Truy vấn mất bao lâu, tính bằng giây |
| **Số dòng đã quét** | MariaDB đã đọc bao nhiêu dòng |
| **Truy vấn** | Câu truy vấn. Chọn nó để xem toàn bộ, tối đa 2.000 ký tự. Truy vấn dài hơn được đánh dấu **Đã rút gọn** |

Chọn **Làm mới** để đọc lại log, và **Xóa log** để làm trống log. **Xóa log** chỉ làm trống log hiện tại; các log cũ đã nén vẫn còn cho đến khi việc xoay vòng xóa chúng.

::: tip
Ghi log làm máy chủ tốn thêm chút công sức. Khi xong, hãy tắt công tắc và chọn **Áp dụng**.
:::

## Adminer {#adminer}

Adminer giúp bạn xem và sửa database ngay trên trình duyệt, giống phpMyAdmin.

### Cài Adminer {#install-adminer}

1. Cài web server trước, ở trang **Phần mềm**. Adminer chạy trên web server đó cùng PHP.
2. Vào **Phần mềm** → **Adminer (giao diện database)** và chọn **Cài**.

Panel tải Adminer 6.3.1 (bản có đủ mọi driver của Adminer) và đối chiếu với mã SHA-256 đã ghim sẵn trước khi cài. Adminer mở được database MariaDB hoặc MySQL, và cả database PostgreSQL khi đã cài PostgreSQL (xem [PostgreSQL trong Adminer](./postgresql#postgresql-in-adminer)). Driver SQLite của nó từ chối mọi lần đăng nhập, vì SQLite không có mật khẩu.

### Mở Adminer {#open-adminer}

1. Chọn **Mở bằng Adminer** ở đầu trang **Cơ sở dữ liệu**, hoặc trong thẻ thông tin kết nối của một database. **Phần mềm** → **Mở Adminer** cũng làm việc tương tự.
2. Adminer mở trong thẻ mới, tại địa chỉ panel của bạn cộng thêm `adminer/`, ví dụ `https://203.0.113.10:28443/k3x9p2qa7m1z/adminer/`.
3. Đăng nhập Adminer bằng user và mật khẩu của database, lấy từ **Xem đăng nhập**.

### Adminer được bảo vệ thế nào {#how-adminer-is-protected}

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

phpMyAdmin được bảo vệ giống Adminer (xem [Adminer được bảo vệ thế nào](#how-adminer-is-protected)), trên site nội bộ riêng với token bí mật riêng. Không có phiên đăng nhập panel, `phpmyadmin/` trả về `401`, và khóa API cũng không vào được. Chỉ ba điểm vào của phpMyAdmin chạy bằng PHP và chỉ các tệp script, style và hình ảnh của nó được trả về như tệp; mọi thứ khác trả về `404`, kể cả script setup. phpMyAdmin gửi Content-Security-Policy riêng và tự kiểm tra mã của biểu mẫu.

Giống Adminer, phpMyAdmin nhập được tệp tới 256 MB, và một yêu cầu chạy được tới 600 giây. Gỡ phpMyAdmin sẽ xóa các tệp, site nội bộ và thiết lập của nó. Database của bạn không bị ảnh hưởng. Gỡ web server cũng gỡ luôn phpMyAdmin.

## Trình duyệt key Redis {#redis-key-browser}

Khi đã cài Redis, trang **Cơ sở dữ liệu** có thêm thẻ **Redis**. Để cài Redis, vào **Phần mềm** → **Redis** → **Cài**. Xem cách Redis được thiết lập ở trang [Phần mềm](./software#redis).

Thẻ này hiện phiên bản Redis, thời gian đã chạy, **Bộ nhớ đã dùng** (kèm giới hạn bộ nhớ) và **Số key**. Nếu thẻ báo **Redis đã được cài nhưng đang không chạy.**, hãy khởi động Redis ở trang **Dịch vụ**.

### Tìm key {#find-keys}

1. Chọn một database từ 0 đến 15. Danh sách cho biết mỗi database có bao nhiêu key.
2. Nhập mẫu tên key. `*` khớp với mọi thứ, `?` khớp một ký tự, ví dụ `user:*`.
3. Chọn **Tìm**.

Việc tìm đi qua các key theo từng đợt khoảng 100 key. Nếu còn nữa, chọn **Tìm tiếp**.

### Xem một key {#view-a-key}

Chọn một key để xem **Kiểu**, **Hết hạn sau**, **Kích thước** và giá trị của nó:

| Kiểu | Panel hiện gì |
|---|---|
| string | 64 KB đầu tiên của giá trị |
| list, hash, set, sorted set | 200 phần tử đầu tiên. Mỗi phần tử bị cắt ở 4 KB |
| stream và các kiểu khác | Chỉ kiểu và thời gian hết hạn |

Với giá trị nhị phân, các ký tự không hiển thị được sẽ bị thay thế.

### Xóa key {#delete-keys}

- Để xóa một key, chọn key đó rồi chọn **Xóa key**.
- Để xóa mọi key của database đang chọn, chọn **Xóa trống database 0** (nút hiện số của database), gõ số đó để xác nhận, rồi chọn **Xóa mọi key**.

Website dùng Redis làm bộ nhớ đệm sẽ tự tạo lại dữ liệu. Dữ liệu chỉ lưu trong Redis sẽ mất.

Trình duyệt key không sửa được giá trị hay thời gian hết hạn. Hãy dùng `redis-cli` trên máy chủ cho việc đó.

Nếu Redis có mật khẩu (`requirepass` trong `/etc/redis/redis.conf`), panel đọc mật khẩu từ tệp đó và dùng nó để đăng nhập.

## Xóa database {#delete-a-database}

1. Mở menu của database và chọn **Xóa**.
2. Gõ tên database để xác nhận và chọn **Xóa**.

::: danger
Thao tác này xóa database, toàn bộ bảng và user của nó. Không thể hoàn tác. Các user từ xa, luật tường lửa và lịch sao lưu tự động của database cũng bị xóa theo. Các bản sao lưu đã tạo vẫn được giữ ở trang **Sao lưu**.
:::

Khi bạn xóa một website, các database của nó được giữ lại trừ khi bạn đánh dấu **Xóa luôn các database của website**: chúng vẫn nằm ở đây, không gắn với website nào. Khôi phục bản sao lưu của website sẽ gắn chúng lại.

## Gỡ MariaDB {#uninstall-mariadb}

Gỡ MariaDB ở trang **Phần mềm**. Panel từ chối khi MariaDB còn bất kỳ database nào ngoài các database hệ thống của chính nó, kể cả database tạo bên ngoài panel. Xem trang [Phần mềm](./software) để biết việc gỡ cài đặt giữ lại những gì.

MariaDB lấy từ kho của MariaDB được gỡ cùng tệp kho và khóa của nó.

MySQL được gỡ theo cách tương tự, trên thẻ của nó, với cùng bước kiểm tra. Panel cũng xóa kho của Oracle và khóa của kho. Thư mục dữ liệu `/var/lib/mysql` được giữ lại, trừ khi bạn đánh dấu ô và gõ `mysql`.

Để thay cái này bằng cái kia mà vẫn giữ các database, hãy [chuyển](#switch-between-mariadb-and-mysql) thay vì gỡ.

## PostgreSQL {#postgresql}

PostgreSQL chạy song song với MariaDB hoặc MySQL, ở thẻ **PostgreSQL** riêng của trang này: phiên bản của hệ điều hành hoặc từ 15 đến 19 từ dự án PostgreSQL, với database và chủ sở hữu, nhập và xuất, truy cập từ xa qua TLS, sao lưu và nâng cấp phiên bản chính. Xem [PostgreSQL](./postgresql).

## Memcached {#memcached}

Khi đã cài Memcached (**Phần mềm** → **Memcached**), trang **Cơ sở dữ liệu** có thẻ **Memcached**. Thẻ này hiện phiên bản, thời gian đã chạy, **Bộ nhớ đã dùng** cùng giới hạn, **Số mục**, tỷ lệ trúng và số kết nối đang mở. Nó cũng hiện những gì chính Memcached báo: chỉ lắng nghe trên 127.0.0.1 và UDP đang tắt.

**Khởi động lại** sẽ làm trống bộ nhớ đệm. Xem [Phần mềm](./software#memcached) để biết cách nó được thiết lập.
