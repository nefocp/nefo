---
title: Cơ sở dữ liệu
description: Cài MariaDB, tạo database, nhập và xuất SQL, mở truy cập từ xa, xem log truy vấn chậm, dùng Adminer và trình duyệt key Redis.
---

# Cơ sở dữ liệu

Trang **Cơ sở dữ liệu** quản lý MariaDB, máy chủ cơ sở dữ liệu cho WordPress và hầu hết ứng dụng PHP. Trang này hướng dẫn:

- cài MariaDB và cách Nefo tinh chỉnh nó cho máy chủ nhỏ;
- tạo database, xem thông tin đăng nhập và đổi mật khẩu;
- nhập và xuất SQL;
- mở truy cập từ xa cho từng database;
- log truy vấn chậm;
- Adminer, giao diện database ngay trên trình duyệt;
- trình duyệt key Redis, ở thẻ **Redis** của cùng trang.

Nefo chỉ quản lý MariaDB. PostgreSQL và MongoDB có trong [Kho ứng dụng](./apps) dưới dạng ứng dụng Docker, chỉ lắng nghe trên 127.0.0.1. Chúng không hiện trên trang **Cơ sở dữ liệu**.

## Cài MariaDB

1. Vào **Cơ sở dữ liệu**.
2. Chọn **Cài ngay**. Bạn cũng có thể cài ở **Phần mềm** → **MariaDB** → **Cài**.
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

Mỗi database có một user là `tên@localhost`, có toàn quyền trên database đó và không có quyền gì trên database khác. Khi bật truy cập từ xa, user này có thêm một bản sao cho mỗi địa chỉ được phép (xem [Mở truy cập từ xa](#mo-truy-cap-tu-xa)). Ngoài user tạm thời dùng để nhập (xem [Nhập](#nhap)), panel không tạo thêm user và không đổi quyền. Nếu cần, hãy dùng lệnh `mariadb` trên máy chủ với quyền root.

Danh sách chỉ gồm các database do panel tạo. Database bị xóa từ bên ngoài panel được đánh dấu **Không tìm thấy trong MariaDB**.

## Xem thông tin đăng nhập của database

Chọn **Xem đăng nhập** trên dòng của database. Thẻ thông tin kết nối hiện tên, user, mật khẩu và máy chủ, mỗi mục có nút sao chép. Nếu đã cài Adminer, thẻ có thêm nút **Mở bằng Adminer**.

## Đổi mật khẩu database

1. Mở menu của database và chọn **Đổi mật khẩu**.
2. Giữ **Tạo mật khẩu mạnh ngẫu nhiên**, hoặc chọn **Tự đặt mật khẩu** rồi gõ mật khẩu. Mật khẩu tự đặt theo cùng quy tắc như khi [tạo database](#tao-database).
3. Chọn **Đổi mật khẩu**.

Panel đặt mật khẩu mới và hiện nó trong thẻ thông tin kết nối. Nếu database gắn với một website WordPress do panel tạo, panel cũng ghi mật khẩu mới vào `wp-config.php` của site đó. Các ứng dụng khác sẽ không kết nối được cho đến khi bạn cập nhật cấu hình của chúng.

Nếu database đang mở truy cập từ xa, các user từ xa cũng nhận mật khẩu mới.

## Xuất và nhập

### Xuất

Mở menu của database và chọn **Xuất (.sql)**. Trình duyệt tải về một tệp SQL thường, có tên dạng `shop_20261126-143000.sql`.

Bản dump được tạo bằng `mariadb-dump --single-transaction`, nên website vẫn chạy bình thường trong lúc xuất. Bản dump có cả stored routine và trigger.

### Nhập

1. Mở menu của database và chọn **Nhập**.
2. Chọn tệp ở ô **File SQL (.sql hoặc .sql.gz)**. Tệp `.sql.gz` được giải nén ngay trong lúc nhập.
3. Không bắt buộc: đánh dấu **Xóa trống database trước (xóa toàn bộ bảng)**. Khi đó database bị xóa và tạo lại trước khi nhập, user của nó vẫn giữ quyền truy cập.
4. Chọn **Nhập**.

Mỗi lần nhập tối đa 4 GB.

Việc nhập chạy bằng một user MariaDB tạm thời, tên là `nefo-import-` kèm tên database, có toàn quyền trên database này và không có quyền gì trên database khác. User này cũng được đặt definer cho trigger, view và stored routine, nên các bản dump có chúng, như bản xuất và bản sao lưu của chính panel, vẫn nhập được như trước. Panel xóa user này khi việc nhập kết thúc, kể cả khi thất bại. Chỉ khi tệp tạo trigger, view, routine hoặc event không có mệnh đề `DEFINER`, vốn sau đó chạy bằng user này, panel mới giữ nó lại: bị khóa để không ai đăng nhập được, và chỉ có quyền trên database này. Xóa database thì user này cũng bị xóa.

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

MariaDB phải đang chạy thì mới đổi được các thiết lập này. Thay đổi có hiệu lực ngay với các kết nối mở sau đó, và vẫn giữ sau khi khởi động lại: panel lưu nó trong `/etc/mysql/mariadb.conf.d/99-panel-slowlog.cnf`. Log được ghi vào `/var/log/mysql/mariadb-slow.log`, thư mục mà cơ chế xoay vòng log của gói MariaDB đã quản lý.

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

Panel tải Adminer 6.3.1 (bản dành cho MySQL) và đối chiếu với mã SHA-256 đã ghim sẵn trước khi cài.

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
