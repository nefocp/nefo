---
title: PostgreSQL
description: Cài PostgreSQL từ hệ điều hành hoặc các phiên bản 15 đến 19 từ dự án PostgreSQL, tạo database và chủ sở hữu, mở truy cập từ xa qua TLS, nhập và xuất, sao lưu, và nâng cấp lên phiên bản chính mới hơn.
---

# PostgreSQL

Nefo chạy PostgreSQL song song với MariaDB hoặc MySQL: phiên bản của hệ điều hành, hoặc một phiên bản chính từ 15 đến 19 lấy từ kho riêng của dự án PostgreSQL (apt.postgresql.org). Thẻ **Cơ sở dữ liệu** → **PostgreSQL** tạo database cùng chủ sở hữu của chúng, nhập và xuất database, mở một database cho các địa chỉ bạn chọn qua TLS, và nâng cấp máy chủ lên phiên bản chính mới hơn sau khi tạo bản dump trước.

PostgreSQL là máy chủ cơ sở dữ liệu mà nhiều ứng dụng Laravel, Django, Node.js và Odoo ưa dùng. MariaDB (hoặc MySQL) và PostgreSQL chạy song song được. Với MariaDB và MySQL, xem [Cơ sở dữ liệu](./databases).

## Cài PostgreSQL {#install-postgresql}

1. Vào **Phần mềm** → **PostgreSQL** và chọn **Cài**.
2. Ở mục **Phiên bản**, chọn PostgreSQL muốn cài:
   - phiên bản của hệ điều hành, ví dụ "PostgreSQL 16, phiên bản của hệ điều hành", có nhãn **Khuyên dùng**. Phiên bản này được cập nhật cùng hệ thống;
   - một phiên bản từ apt.postgresql.org, ví dụ "PostgreSQL 18 từ apt.postgresql.org", và các phiên bản chính khác mà kho của dự án PostgreSQL có. Hãy chọn một bản như vậy khi ứng dụng cần phiên bản mới hơn (hoặc cũ hơn) bản mà hệ thống đi kèm.
3. Chọn **Cài** và chờ một đến hai phút. Log tiến trình hiện từng bước.

| Hệ điều hành | Phiên bản của hệ điều hành | Từ apt.postgresql.org |
|---|---|---|
| Ubuntu 22.04 | 14 | 15, 16, 17, 18, 19 |
| Ubuntu 24.04 | 16 | 15, 17, 18, 19 |
| Ubuntu 26.04 | 18 | 15, 16, 17, 19 |
| Debian 12 | 15 | 16, 17, 18, 19 |
| Debian 13 | 17 | 15, 16, 18, 19 |

Panel đưa ra các phiên bản chính mà dự án PostgreSQL vẫn còn hỗ trợ. Trên hệ điều hành khác, hoặc trên bộ xử lý mà kho không có gói, chỉ có phiên bản của hệ điều hành, và hộp thoại nói rõ lý do.

Khi bạn chọn một phiên bản từ apt.postgresql.org, panel:

- tải khóa ký của kho từ `https://www.postgresql.org/media/keys/ACCC4CF8.asc` và từ chối nếu dấu vân tay không đúng là `B97B 0AFC AA1A 47F0 44F2 44A0 7FCC 7D46 ACCC 4CF8` (khóa "PostgreSQL Debian Repository");
- ghi khóa vào `/etc/apt/keyrings/panel-pgdg.gpg` và thêm `/etc/apt/sources.list.d/panel-pgdg.list`, tệp chỉ tin khóa đó cho riêng kho này;
- cài gói server theo đúng tên, ví dụ `postgresql-18`. Panel không bao giờ cài gói `postgresql` từ kho đó, vì gói này luôn kéo theo phiên bản chính mới nhất, và khi đó một lần cập nhật hệ thống sẽ cài thêm máy chủ thứ hai.

Bản cập nhật bảo mật cho cả hai loại phiên bản đều đến cùng các bản cập nhật hệ thống (trang **Hệ thống**, hoặc `apt upgrade`).

Sau đó thẻ PostgreSQL trên trang **Phần mềm** hiện **Từ hệ điều hành** hoặc **Từ apt.postgresql.org**, và đầu thẻ **Cơ sở dữ liệu** → **PostgreSQL** cũng vậy.

Mỗi lúc chỉ chạy một phiên bản PostgreSQL. Panel từ chối:

- cài phiên bản khác khi đã có một phiên bản. Hãy [nâng cấp](#upgrade-postgresql) phiên bản đó, hoặc gỡ nó trước;
- cài phiên bản khác với phiên bản có dữ liệu còn lại trên máy chủ từ lần cài trước (trong `/var/lib/postgresql/<version>`). Hãy cài lại đúng phiên bản đó để dùng dữ liệu, rồi nâng cấp nếu muốn, hoặc xóa dữ liệu trước.

apt chạy với locale C, nên cluster mà gói tạo ra lưu văn bản dạng `SQL_ASCII`. Khi đó Nefo tạo lại cluster mới, còn trống này với UTF-8 (`C.UTF-8`). Cluster đã có từ trước khi cài thì không bao giờ bị tạo lại.

## Cách PostgreSQL được tinh chỉnh {#how-postgresql-is-tuned}

Nefo ghi thiết lập vào `/etc/postgresql/<version>/main/conf.d/90-nefo.conf`. Các thiết lập bộ nhớ tùy theo RAM của máy chủ:

| RAM máy chủ | `shared_buffers` | `effective_cache_size` | `work_mem` | `maintenance_work_mem` | `max_connections` |
|---|---|---|---|---|---|
| Dưới 768 MB | 64MB | 256MB | 2MB | 16MB | 30 |
| Từ 768 MB | 128MB | 512MB | 4MB | 32MB | 40 |
| Từ 1,5 GB | 256MB | 1024MB | 4MB | 64MB | 50 |
| Từ 3 GB | 512MB | 2048MB | 8MB | 128MB | 75 |
| Từ 7 GB | 1024MB | 4096MB | 16MB | 256MB | 100 |

`effective_cache_size` bằng khoảng một nửa RAM, vì cùng máy chủ còn chạy web server, PHP và có thể cả MariaDB. Trên mọi máy chủ, Nefo còn đặt `listen_addresses = 'localhost'`, `password_encryption = 'scram-sha-256'`, `random_page_cost = 1.1` (cho ổ SSD) và `jit = off`. Trên VPS dạng container (LXC), RAM là giới hạn bộ nhớ của container (xem [VPS là container](./first-steps#a-vps-that-is-a-container)).

Khi rảnh và chưa có dữ liệu, PostgreSQL cần khoảng 30 đến 40 MB cho vài tiến trình nền. Khi bộ đệm đầy dần, nó tăng tới khoảng `shared_buffers` cộng 40 MB, và mỗi kết nối đang mở tốn thêm vài MB. Trang **Phần mềm** hiển thị `shared_buffers` cộng 40 MB.

::: warning
Đừng sửa `90-nefo.conf` hay `pg_hba.conf`. Cài lại PostgreSQL sẽ ghi đè chúng. Các dòng bạn tự thêm vào `pg_hba.conf` vẫn được giữ khi bạn thay đổi truy cập từ xa: panel chỉ đụng tới các dòng của chính nó (xem [Truy cập từ xa](#postgresql-remote-access)).
:::

## Ai được kết nối {#who-can-connect}

Nefo ghi `/etc/postgresql/<version>/main/pg_hba.conf`:

```
local   all  postgres                peer
local   all  all                     scram-sha-256
host    all  all       127.0.0.1/32  scram-sha-256
host    all  all       ::1/128       scram-sha-256
```

- Chỉ user hệ thống `postgres` đăng nhập không cần mật khẩu, qua socket nội bộ. Panel quản lý máy chủ theo cách đó.
- Mọi người khác đều cần mật khẩu, qua socket hoặc TCP từ chính máy chủ này. Khác với mặc định của hệ điều hành, các user Linux khác không được đăng nhập không mật khẩu ("peer"), nên user Linux của một website không mở được role database trùng tên nếu không có mật khẩu.
- Máy khác không kết nối được, trừ khi bạn cho phép địa chỉ của chúng vào một database (xem [Truy cập từ xa](#postgresql-remote-access)). Khi chưa có database nào mở truy cập từ xa, PostgreSQL chỉ lắng nghe trên localhost.

## Tạo database PostgreSQL {#create-a-postgresql-database}

1. Vào **Cơ sở dữ liệu** → **PostgreSQL** và chọn **Thêm database**.
2. Nhập tên vào ô **Database**. Một role cùng tên sẽ sở hữu database này.
3. Để trống **Mật khẩu** để panel tạo mật khẩu mạnh ngẫu nhiên, hoặc tự nhập.
4. Không bắt buộc: chọn một site ở ô **Website**. Bản sao lưu của website đó sẽ có cả database này, và xóa website kèm các database của nó cũng xóa database này.
5. Chọn **Tạo database**.

Thẻ thông tin kết nối hiện **Database**, **User**, **Mật khẩu**, **Máy chủ** (`localhost`) và **Cổng** (`5432`). Ví dụ với ứng dụng Laravel, đặt `DB_CONNECTION=pgsql`, `DB_HOST=127.0.0.1`, `DB_PORT=5432` cùng database, user và mật khẩu lấy từ thẻ.

Quy tắc:

- Tên và mật khẩu theo [quy tắc của MariaDB](./databases#create-a-database). Các tên riêng của PostgreSQL cũng bị từ chối: `postgres`, `template0`, `template1`, `public`, `all` và mọi tên bắt đầu bằng `pg_`.
- Một tên không dùng được cho cả hai máy chủ: nếu đã có database hoặc user MariaDB tên `shop`, thì database PostgreSQL không đặt tên đó được, và ngược lại. Panel cũng từ chối tên mà PostgreSQL đã dùng cho một database hoặc role.
- Role đăng nhập được, không phải superuser, không tạo được database hay role. Không role nào khác kết nối được vào database, vì panel thu hồi quyền `CONNECT` của mọi người khác.
- Database mới dùng UTF-8 với locale `C.UTF-8`.

Mật khẩu không bao giờ tới PostgreSQL ở dạng rõ: panel chỉ gửi verifier SCRAM-SHA-256 của nó, nên mật khẩu cũng không thể xuất hiện trong log của PostgreSQL.

**Xem đăng nhập**, **Đổi mật khẩu** và **Xóa** hoạt động như với MariaDB. Xóa cũng đóng các kết nối đang mở tới database, và gỡ các dòng truy cập từ xa cùng luật tường lửa của nó. Database bị xóa ngoài panel được đánh dấu **Không tìm thấy trong PostgreSQL**.

## Máy chủ khác có kết nối được tới database PostgreSQL không? {#postgresql-remote-access}

Có, từ v0.24.0, cho từng database một. Mặc định PostgreSQL chỉ nhận kết nối từ chính máy chủ. Bạn có thể cho phép một số địa chỉ kết nối tới một database, bằng user (role) và mật khẩu riêng của database đó, qua TLS.

Trước tiên, hãy cài tường lửa ufw ở trang **Tường lửa & SSH** và bật nó lên. Panel từ chối mở truy cập từ xa khi chưa cài ufw.

1. Vào **Cơ sở dữ liệu** → **PostgreSQL**, mở menu của database và chọn **Truy cập từ xa**.
2. Ở mục **Địa chỉ được phép**, nhập một địa chỉ. Chọn **Thêm địa chỉ** để nhập thêm.
3. Chọn **Lưu**.

Sau đó dòng của database hiện "Từ xa: 1" (số địa chỉ), và đầu thẻ hiện "1 mở cho máy chủ khác".

Kết nối tới địa chỉ IP công khai của máy chủ bằng user và mật khẩu lấy từ **Xem đăng nhập**, và dùng TLS, ví dụ:

```sh
psql "host=203.0.113.10 port=5432 dbname=shop user=shop sslmode=require"
```

### Địa chỉ được phép {#allowed-addresses}

- Từng địa chỉ IPv4 hoặc IPv6, ví dụ `198.51.100.7` hoặc `2001:db8::1`.
- Dải dạng CIDR: IPv4 không rộng hơn `/16` (ví dụ `198.51.100.0/24`), IPv6 không rộng hơn `/48` (ví dụ `2001:db8:5::/48`).
- Mỗi database tối đa 10 địa chỉ.
- Địa chỉ loopback, link-local (`fe80::`), `0.0.0.0`, `::` và địa chỉ multicast bị từ chối.

### Panel thay đổi những gì {#what-the-panel-changes}

| Ở đâu | Thay đổi |
|---|---|
| `pg_hba.conf` (PostgreSQL 16 trở lên) | Một dòng ở cuối, `include_if_exists nefo-remote.conf`, kèm một chú thích. Các địa chỉ được phép nằm trong `/etc/postgresql/<version>/main/nefo-remote.conf` |
| `pg_hba.conf` (PostgreSQL 14 và 15) | Một khối ở cuối, giữa `# BEGIN Nefo remote access` và `# END Nefo remote access` |
| Các dòng đó | Mỗi địa chỉ một dòng, chỉ cho database này và role riêng của nó, đăng nhập bằng mật khẩu: `hostssl "shop" "shop" 198.51.100.7/32 scram-sha-256` |
| Thiết lập PostgreSQL | `/etc/postgresql/<version>/main/conf.d/95-nefo-remote.conf` đặt `listen_addresses = '*'`. Tệp này chỉ tồn tại khi còn ít nhất một database mở truy cập từ xa |
| Tường lửa (ufw) | Với mỗi địa chỉ, một luật chỉ cho phép cổng của cluster (mặc định 5432) qua TCP từ địa chỉ đó, kèm ghi chú `PostgreSQL name` |

Panel không bao giờ sửa các dòng khác của `pg_hba.conf`. Khi các dòng của bạn đứng trước các dòng của panel, chúng được đọc trước: một dòng `reject` của bạn cho cùng địa chỉ sẽ thắng. Trước khi PostgreSQL nạp lại `pg_hba.conf` đã thay đổi, panel yêu cầu PostgreSQL kiểm tra nó (`pg_hba_file_rules`). Nếu PostgreSQL thấy lỗi, các tệp cũ được đặt lại và không có gì thay đổi.

Nếu cluster dùng cổng khác 5432, các luật tường lửa dùng cổng đó, và thông tin kết nối cũng hiện cổng đó.

### Mã hóa {#encryption}

Hộp thoại cho biết kết nối từ máy khác có được mã hóa hay không:

- **Kết nối từ máy khác được mã hóa**: các dòng là `hostssl`, nên PostgreSQL từ chối những kết nối từ các địa chỉ đó mà không dùng TLS. Máy khách cần `sslmode=require` (hoặc chặt hơn). Debian và Ubuntu tạo cluster với chứng chỉ tự ký của hệ thống (`/etc/ssl/certs/ssl-cert-snakeoil.pem`). Nếu cluster đang tắt TLS, panel tạo chứng chỉ tự ký riêng (`nefo-ssl.crt` và `nefo-ssl.key` cạnh `pg_hba.conf`) và bật TLS trong `95-nefo-remote.conf`; TLS tắt trở lại khi truy cập từ xa cuối cùng được tắt. Cả hai chứng chỉ đều tự ký, nên máy khách không xác minh được máy chủ bằng chúng: `sslmode=require` có mã hóa, nhưng `verify-ca` và `verify-full` sẽ thất bại trừ khi bạn cài chứng chỉ của riêng mình.
- **Kết nối từ máy khác không được mã hóa**: PostgreSQL không có TLS, nên các dòng là `host`. Hãy dùng SSH tunnel hoặc VPN cho mọi dữ liệu nhạy cảm.

### Khởi động lại {#restarts}

Thay đổi `listen_addresses` cần khởi động lại; các địa chỉ được phép và TLS chỉ cần nạp lại (reload). Vì vậy PostgreSQL khởi động lại ở lần đầu bật truy cập từ xa, và khi tắt nó ở database cuối cùng. Website mất kết nối database trong vài giây, và các transaction đang mở bị hủy. Thêm hay bớt địa chỉ về sau không làm PostgreSQL khởi động lại.

Nếu lúc đó đang có tác vụ khác chạy, hộp thoại báo **PostgreSQL cần khởi động lại trước khi nhận kết nối từ máy khác.** Hãy chọn **Khởi động lại** trong hộp thoại khi tác vụ kia đã xong.

Nếu PostgreSQL không khởi động được với `95-nefo-remote.conf`, panel xóa tệp đó và khởi động PostgreSQL không có nó, để website vẫn chạy.

::: warning
- Nếu tường lửa đang tắt, cổng này mở cho tất cả. PostgreSQL vẫn chỉ cho các địa chỉ đã liệt kê đăng nhập, và chỉ khi có mật khẩu.
- Chỉ liệt kê những địa chỉ bạn kiểm soát và không thay đổi.
:::

Để tắt truy cập từ xa, mở lại **Truy cập từ xa** và chọn **Tắt**. Xóa database cũng gỡ các dòng và luật tường lửa của nó; nếu đó là database cuối cùng mở truy cập từ xa, PostgreSQL khởi động lại để chỉ lắng nghe trên localhost như trước.

## Xuất và nhập database PostgreSQL {#export-and-import-a-postgresql-database}

**Xuất (.sql.gz)** tải về bản dump SQL thuần đã nén gzip, tạo bằng `pg_dump` dưới role riêng của database, với `--no-owner --no-privileges`, nên nhập được vào bất kỳ database nào.

**Nhập** nhận file `.sql` hoặc `.sql.gz` tới 4 GB:

- `psql` chạy bằng role riêng của database, không bao giờ bằng `postgres`, với mật khẩu nằm trong một file tạm chỉ root đọc được.
- File chạy trong một transaction duy nhất với `ON_ERROR_STOP`: khi một câu lệnh lỗi thì không có gì được nhập, và thông báo lỗi ghi rõ dòng.
- Các lệnh riêng của psql, tức các dòng bắt đầu bằng dấu gạch chéo ngược (như `\connect`, `\!` hay `\copy`), bị từ chối: psql chạy bằng root, và các lệnh đó có thể chạy chương trình hoặc ghi file trên máy chủ. Panel khởi động psql ở chế độ hạn chế (`\restrict` với một khóa ngẫu nhiên) và bỏ các dòng `\restrict` và `\unrestrict` mà các bản `pg_dump` mới ghi vào. psql không có chế độ hạn chế (cũ hơn 17.6, 16.10, 15.14 hoặc 14.19) sẽ từ chối mọi lần nhập: hãy cài các bản cập nhật hệ thống trước.
- Câu lệnh tạo hay thay đổi role, giao đối tượng cho role khác hoặc cần quyền superuser sẽ lỗi. Hãy xuất lại bản dump đó bằng `pg_dump --no-owner --no-privileges`.
- `SET transaction_timeout`, do `pg_dump` 17 trở lên ghi vào, được bỏ qua trên máy chủ cũ hơn.
- Sau đó panel đặt lại mật khẩu của role, phòng khi file đã đổi nó.

Khi đánh dấu **Xóa trống database trước (xóa mọi thứ trong đó)**, database bị xóa và tạo lại trước khi nhập, vẫn do cùng role sở hữu.

## Nâng cấp PostgreSQL lên phiên bản mới hơn {#upgrade-postgresql}

Bạn có thể chuyển máy chủ lên một phiên bản chính mới hơn từ apt.postgresql.org, ví dụ từ 15 của hệ điều hành lên 17, mà vẫn giữ mọi database, user, mật khẩu và thiết lập truy cập từ xa.

1. Vào **Cơ sở dữ liệu** → **PostgreSQL** và chọn **Nâng cấp**. Nút này hiện ra khi có phiên bản mới hơn và PostgreSQL đang chạy.
2. Chọn **Phiên bản mới**.
3. Đọc **Những gì sẽ diễn ra**, rồi đánh dấu **Tôi hiểu rằng website và ứng dụng sẽ mất kết nối database trong lúc nâng cấp.**
4. Chọn **Nâng cấp lên 17** (phiên bản bạn đã chọn) và theo dõi log.

Panel làm những việc sau, theo thứ tự:

1. Kiểm tra rằng phiên bản mới thực sự mới hơn, chỉ có một cluster (`main`), không có gói extension nào được cài cho phiên bản cũ (ví dụ `postgresql-15-postgis-3`), và ổ đĩa còn chỗ cho gấp đôi dữ liệu cộng 256 MB.
2. Lưu mọi database bằng `pg_dumpall` vào một tệp nén duy nhất, `/var/backups/nefo-postgresql/postgresql-<old>-before-<new>-<date>.sql.gz`, chỉ root đọc được. Lúc này chưa có gì thay đổi: nếu việc dump thất bại, nâng cấp dừng ở đó.
3. Gỡ gói `postgresql` của hệ điều hành, gói chỉ trỏ tới một phiên bản (lấy từ apt.postgresql.org thì nó sẽ trỏ tới bản mới nhất và cài thêm máy chủ thứ hai), sau khi đánh dấu `postgresql-<old>` là gói cài thủ công. Sau đó thêm kho apt.postgresql.org nếu chưa có và cài `postgresql-<new>`. Cluster trống mà gói này tạo ra bị xóa. Nếu apt kéo theo bất kỳ máy chủ PostgreSQL nào khác, việc nâng cấp dừng lại và hoàn tác.
4. Chạy `pg_upgradecluster -v <new> -m upgrade <old> main`. Trong lúc đó PostgreSQL bị dừng: khoảng một phút với máy chủ nhỏ, lâu hơn với database lớn. `pg_upgrade` sao chép các tệp dữ liệu, nên cluster cũ vẫn nguyên như cũ.
5. Đưa cho cluster mới `pg_hba.conf` cũ, phần tinh chỉnh của panel (`conf.d/90-nefo.conf`) và thiết lập truy cập từ xa (từ phiên bản 16, các địa chỉ được phép chuyển sang `nefo-remote.conf`), khởi động lại nó, và kiểm tra rằng nó chạy trên cùng cổng với cùng các database, database nào cũng trả lời.
6. Chỉ khi đó mới xóa cluster cũ và gỡ các gói của nó (`postgresql-<old>` và `postgresql-client-<old>`).

Nếu bất kỳ bước nào từ 3 đến 5 thất bại, panel hoàn tác: xóa cluster mới, gỡ các gói của phiên bản mới, đặt lại thiết lập của cluster cũ và khởi động lại nó. Log kết thúc bằng "PostgreSQL 15 runs again with its data as before". Database của bạn vẫn như trước. Kho apt.postgresql.org được giữ lại một khi đã cài gói từ đó, vì `postgresql-common` và `libpq5` đã được cập nhật từ kho này và nếu gỡ kho thì chúng sẽ không nhận thêm bản cập nhật nào; gói `postgresql` của hệ điều hành không được cài lại.

Việc nâng cấp bị từ chối, kèm lý do trong hộp thoại, khi:

| Lý do | Cách xử lý |
|---|---|
| Có gói extension được cài cho phiên bản cũ | pg_upgrade cần cùng các extension đó được build cho phiên bản mới, thứ panel không cài. Hãy nâng cấp bằng tay với `pg_upgradecluster` sau khi cài chúng cho phiên bản mới, hoặc gỡ các extension trước |
| Có nhiều hơn một cluster | Panel chỉ quản lý `main`. Hãy nâng cấp bằng tay |
| Không đủ dung lượng đĩa | Giải phóng đúng lượng mà thông báo nêu |

Một database mà `pg_upgrade` không chuyển được, ví dụ bảng có cột `regproc`, làm bước 4 thất bại; khi đó panel hoàn tác như mô tả ở trên.

### Các bản dump được giữ lại {#the-dumps-kept}

Tệp đã lưu vẫn nằm trong `/var/backups/nefo-postgresql` dù kết quả thế nào, và thẻ **PostgreSQL** liệt kê nó ở mục **Bản dump được giữ lại**: nó được tạo cho lần nâng cấp nào (ví dụ "PostgreSQL 15 → 17"), lúc nào, dung lượng và đường dẫn. Hãy xóa nó bằng **Xóa bản dump này** khi bạn chắc rằng mọi thứ đều chạy tốt.

Để tự tay khôi phục mọi thứ từ tệp này trên một máy chủ mới:

```sh
zcat postgresql-15-before-17-20261127-101500.sql.gz | runuser -u postgres -- psql -X -d postgres
```

## PostgreSQL trong Adminer {#postgresql-in-adminer}

Adminer cần extension `pgsql` của PHP để mở PostgreSQL. Panel cài extension này cho phiên bản PHP chạy Adminer, và chỉ khi có PostgreSQL: khi bạn cài PostgreSQL lúc đã có Adminer, hoặc cài Adminer lúc đã có PostgreSQL. Adminer do panel phiên bản cũ cài (bản chỉ dành cho MySQL) cũng được thay luôn lúc đó.

**Mở bằng Adminer** trên dòng của một database mở trang đăng nhập PostgreSQL của Adminer, đã điền sẵn server `localhost`, user và database. Nhập mật khẩu lấy từ **Xem đăng nhập**.

## PostgreSQL trong bản sao lưu {#postgresql-in-backups}

Bản sao lưu website có cả các database PostgreSQL của website, và bản sao lưu database có thể chứa một database PostgreSQL, chạy ngay hoặc theo lịch. Bản dump là đầu ra SQL thuần của `pg_dump`, tạo bằng role của database, lưu thành `postgres/<name>.sql` trong file nén, cạnh các bản dump MariaDB.

Sao lưu, xuất và nhập dùng `pg_dump` và `psql` của phiên bản đã cài, từ hệ điều hành hoặc từ apt.postgresql.org, và dùng cổng của cluster, kể cả khi đó không phải 5432.

Khi khôi phục, mỗi database được làm trống (xóa và tạo lại, vẫn do cùng role sở hữu) rồi bản dump được nhập bằng role đó. Database đã bị xóa kể từ lúc sao lưu sẽ được tạo lại, với mật khẩu của role lấy từ bản sao lưu nếu bản sao lưu có nó (từ v0.24.0), nếu không thì với mật khẩu mới.

## Gỡ PostgreSQL {#uninstall-postgresql}

Gỡ PostgreSQL ở trang **Phần mềm**. Panel từ chối khi PostgreSQL còn bất kỳ database nào ngoài `postgres`, `template0` và `template1`, kể cả database tạo ngoài panel. Panel cũng từ chối khi PostgreSQL đang dừng, vì lúc đó không kiểm tra được. Dữ liệu trong `/var/lib/postgresql` và thiết lập trong `/etc/postgresql` được giữ lại, trừ khi bạn đánh dấu ô và gõ `postgresql`. Xem [Phần mềm](./software#postgresql).

Việc gỡ cũng xóa kho apt.postgresql.org và khóa của nó, nếu panel đã thêm chúng, cùng mọi luật tường lửa còn lại cho các database PostgreSQL. Khi extension `pgsql` của PHP (được thêm cho Adminer) vẫn cần `libpq5`, panel đưa `libpq5` về phiên bản của hệ điều hành, để nó lại nhận các bản cập nhật bảo mật của hệ thống.

Kho ứng dụng cũng có PostgreSQL dưới dạng dịch vụ Docker trên 127.0.0.1, cho các ứng dụng muốn có máy chủ riêng. Bản đó không hiện trên trang **Cơ sở dữ liệu**: xem [Kho ứng dụng](./apps#install-a-service).
