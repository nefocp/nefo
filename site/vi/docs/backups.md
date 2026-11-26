---
title: Sao lưu
description: Sao lưu website, database, thư mục và ứng dụng theo lịch có giới hạn số bản, lưu trên máy chủ này, kho tương thích S3 hoặc máy chủ SFTP, và khôi phục chỉ với một cú nhấp.
---

# Sao lưu

Trang **Sao lưu** tạo và khôi phục bản sao lưu của website, database, thư mục và ứng dụng từ Kho ứng dụng. Trang này hướng dẫn: bản sao lưu gồm những gì, sao lưu thủ công và theo lịch, kho lưu trữ từ xa, khôi phục, và cảnh báo khi sao lưu thất bại.

## Bản sao lưu gồm những gì

| Loại | Nội dung |
|---|---|
| **Website** | Các file trong thư mục của website, `/var/www/example.com`, trừ hai thư mục `tmp` và `logs` ở cấp đầu, kèm bản dump SQL của mọi database gắn với website |
| **Database** | Bản dump SQL của một database |
| **Thư mục** | Bất kỳ thư mục nào trên máy chủ, ví dụ `/etc/nginx` hoặc `/opt/app`, giữ nguyên chủ sở hữu, quyền và liên kết tượng trưng |
| **Ứng dụng** | Một ứng dụng Docker từ Kho ứng dụng: thư mục của nó trong `/var/lib/panel-apps`, các mật khẩu đã lưu, dữ liệu của container và bản dump database (xem [Sao lưu ứng dụng](#sao-luu-ung-dung)) |

Hãy gắn database với website ngay khi tạo (xem [Cơ sở dữ liệu](./databases#tao-database)), để bản sao lưu website có cả database đó. Bản sao lưu website bỏ qua liên kết tượng trưng và file đặc biệt.

Mỗi bản sao lưu là một file `.tar.gz` có `manifest.json`, các file nằm trong `files/` và bản dump database nằm trong `databases/`. Bản dump được tạo bằng `mariadb-dump --single-transaction`, nên website vẫn chạy bình thường.

Nefo không mã hóa bản sao lưu. Trên máy chủ này, chỉ root đọc được chúng. Hãy chọn kho lưu trữ từ xa mà bạn tin cậy, hoặc bật mã hóa bucket hay ổ đĩa ở phía nhà cung cấp.

## Bản sao lưu được lưu ở đâu

Mọi bản sao lưu đều được lưu trước tiên trên máy chủ này, trong `/var/backups/panel`:

```
/var/backups/panel/site/example.com/example.com_20261126-030000.tar.gz
/var/backups/panel/database/shop/shop_20261126-030000.tar.gz
/var/backups/panel/folder/etc_nginx-1a2b3c4d/etc_nginx_20261126-030000.tar.gz
```

Các thư mục có quyền `0700`, các file có quyền `0600`. Panel cũng ghi lại mã SHA-256 của từng file. Bản sao lưu cục bộ chiếm dung lượng ổ đĩa, nên hãy để ý ổ đĩa và số bản mà các lịch của bạn giữ lại.

Để chép một bản sao lưu về máy tính, mở menu của nó và chọn **Tải về**.

## Sao lưu ngay

1. Vào **Sao lưu** và chọn **Sao lưu ngay**.
2. Chọn **Website**, **Database**, **Thư mục** hoặc **Ứng dụng**. Mục **Ứng dụng** xuất hiện khi đã cài một ứng dụng từ Kho ứng dụng.
3. Chọn website, database hoặc ứng dụng ở ô **Sao lưu của**, hoặc gõ **Đường dẫn thư mục** như `/etc/nginx`. Các thư mục từng sao lưu sẽ được gợi ý.
4. Nếu đã cấu hình kho lưu trữ từ xa, bạn có thể đánh dấu **Tải lên cả kho lưu trữ từ xa**.
5. Chọn **Sao lưu ngay**.

Log tiến trình hiện từng bước. Panel chỉ chạy một tác vụ dài tại một thời điểm, nên hãy chờ tác vụ đang chạy xong trước.

Với website, bạn cũng có thể vào **Website** → (site) → **Sao lưu** và chọn **Sao lưu ngay**.

### Thư mục không sao lưu được

Panel từ chối các thư mục sau, mọi thư mục bên trong chúng, và mọi thư mục chứa một trong số chúng. Liên kết tượng trưng được lần theo trước khi kiểm tra.

- `/`, tức toàn bộ hệ thống;
- `/proc`, `/sys`, `/dev` và `/run`;
- dữ liệu của panel, `/var/lib/nefo`, và các bản sao lưu của nó, `/var/backups/panel`;
- `/var/lib/panel-sftp`, `/var/lib/panel-acme` và `/var/lib/panel-quarantine`.

Vì vậy không sao lưu được cả `/var` hay `/var/lib`, nhưng sao lưu được `/var/www` hoặc `/etc`.

## Sao lưu tự động

1. Vào **Sao lưu** và chọn **Thêm lịch**.
2. Chọn **Website**, **Database**, **Thư mục** hoặc **Ứng dụng**, rồi chọn đối tượng.
3. Ở ô **Thời điểm**, chọn **Hằng ngày** hoặc **Hằng tuần**. Với lịch hằng tuần, chọn **Ngày**.
4. Chọn **Giờ**. Đây là giờ của máy chủ, biểu mẫu có ghi độ lệch UTC.
5. Nhập **Số bản sao lưu giữ lại**, từ 1 đến 365.
6. Không bắt buộc: đánh dấu **Tải lên cả kho lưu trữ từ xa**.
7. Chọn **Lưu lịch**.

Với website có lối tắt: **Website** → (site) → **Sao lưu** → **Hằng ngày lúc 03:00**. Lịch này giữ 7 bản.

Thẻ **Sao lưu tự động** liệt kê các lịch cùng lần chạy gần nhất, **Xong** hoặc **Lỗi**. Mở **Lần chạy gần nhất bị lỗi** để đọc lỗi.

Cách lịch chạy:

- Panel kiểm tra mỗi phút. Nếu đang có tác vụ khác chạy, việc sao lưu chờ tới phút sau.
- Nếu máy chủ tắt đúng giờ đã hẹn, bản sao lưu sẽ chạy ngay khi panel hoạt động trở lại.
- Lịch mới sẽ chờ tới giờ hẹn đầu tiên. Muốn có bản sao lưu ngay, hãy dùng thêm **Sao lưu ngay**.

### Giới hạn số bản giữ lại

Sau mỗi lần sao lưu theo lịch, panel xóa các bản cũ nhất do lịch đó tạo vượt quá số bản cần giữ, cả trên máy chủ này lẫn trong kho từ xa. Bản sao lưu bạn tạo thủ công không bao giờ bị xóa tự động.

Xóa một lịch sẽ dừng lịch đó. Các bản sao lưu đã có vẫn được giữ. Khi xóa một website, một database hoặc một ứng dụng, lịch của nó cũng bị xóa, còn các bản sao lưu vẫn nằm ở trang **Sao lưu**.

## Kho lưu trữ từ xa

Một bản sao ở nhà cung cấp khác giúp bạn an toàn nếu máy chủ này gặp sự cố. Vào **Sao lưu** → **Kho lưu trữ từ xa** và chọn **Cấu hình**. Bạn có thể cấu hình cùng lúc kho tương thích S3 và máy chủ SFTP: khi đó bản sao lưu được tải lên cả hai nơi.

Hiện chưa hỗ trợ Google Drive.

### Kho tương thích S3

Chọn **Kho tương thích S3** và điền:

| Ô | Cần nhập |
|---|---|
| **Endpoint (https://…)** | Địa chỉ S3 của nhà cung cấp, chỉ gồm `https://` và tên máy chủ, không có đường dẫn. Ví dụ `https://s3.eu-central-1.amazonaws.com` |
| **Bucket** | Tên bucket |
| **Region** | Region của bucket. Để trống nghĩa là `us-east-1`. Cloudflare R2 dùng `auto` |
| **Thư mục trong bucket (không bắt buộc)** | Một tiền tố như `panel/` |
| **Access key ID** | |
| **Secret access key** | Khi sửa cấu hình sau này, để trống để giữ key hiện tại |

Dùng được với Amazon S3, Cloudflare R2, Backblaze B2, Wasabi và MinIO. Hãy dùng access key chỉ có quyền với bucket này.

Chọn **Kiểm tra và lưu**. Panel ghi một file thử nhỏ, đọc lại và xóa nó trước khi lưu cấu hình.

Mỗi bản sao lưu trên kho S3 tối đa 5 GB, là giới hạn của một lần tải lên.

### Máy chủ SFTP

Dùng một máy chủ khác mà bạn đăng nhập được qua SFTP (SSH), ví dụ storage box hoặc một VPS thứ hai, với tài khoản chỉ dành cho sao lưu.

Chọn **Máy chủ SFTP** và điền:

| Ô | Cần nhập |
|---|---|
| **Máy chủ** | Tên máy chủ hoặc địa chỉ IP, ví dụ `backup.example.com` |
| **Cổng** | Thường là `22` |
| **Tên đăng nhập** | Tài khoản trên máy chủ sao lưu |
| **Đăng nhập bằng** | **Mật khẩu** hoặc **Khóa riêng tư** |
| **Khóa riêng tư (OpenSSH hoặc PEM)** | Toàn bộ khóa, từ `-----BEGIN` đến `END-----`. Khóa công khai tương ứng phải nằm trong `~/.ssh/authorized_keys` trên máy chủ sao lưu |
| **Passphrase (chỉ cần cho khóa được mã hóa)** | |
| **Thư mục trên máy chủ (không bắt buộc)** | Tính từ thư mục home của tài khoản, trừ khi bắt đầu bằng `/`. Thư mục sẽ được tạo nếu chưa có |

Chọn **Kiểm tra và lưu**. Panel kết nối, ghi, liệt kê, đọc và xóa một file thử, rồi mới lưu cấu hình. Khi đã lưu khóa, nút **Thay đổi** hiện phần công khai của khóa ở mục **Khóa công khai, để thêm vào ~/.ssh/authorized_keys trên máy chủ sao lưu**, để bạn sao chép lại khi cần.

### Ghim host key

**Kiểm tra và lưu** tin cậy host key mà máy chủ SFTP đưa ra lúc đó, và hiện nó ở mục **Host key được tin cậy**. Mọi lần kết nối sau phải đưa ra đúng khóa đó, nếu không việc tải bản sao lưu lên sẽ thất bại.

Trước khi dựa vào kho này, hãy so fingerprint với khóa của chính máy chủ sao lưu. Trên máy chủ sao lưu, chạy:

```sh
ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub
```

Nếu máy chủ sao lưu được cài lại hoặc bạn chủ động đổi khóa, hãy chọn **Thay đổi** rồi **Kiểm tra và lưu** lần nữa để tin cậy khóa mới. Panel sẽ cảnh báo rằng khóa đã được thay.

::: warning
Host key thay đổi cũng có thể là dấu hiệu có người đang chặn kết nối. Nếu bạn không chờ đợi thay đổi này, hãy kiểm tra máy chủ sao lưu trước khi tin cậy khóa mới.
:::

### Tải lên và gỡ kho

- Bản sao lưu tạo mà không đánh dấu **Tải lên cả kho lưu trữ từ xa** vẫn tải lên được sau: mở menu của nó và chọn **Tải lên**. Panel chép nó vào từng kho chưa có bản đó.
- Cột **Nơi lưu** hiện **Máy chủ này** và một nhãn cho mỗi bản sao từ xa, S3 hoặc SFTP.
- Xóa cấu hình kho không xóa các bản sao lưu đã tải lên đó.

## Khôi phục bản sao lưu

1. Vào **Sao lưu** và tìm bản sao lưu trong danh sách.
2. Chọn **Khôi phục** trên dòng của nó.
3. Gõ tên mà hộp thoại yêu cầu (tên miền, tên database, đường dẫn thư mục hoặc tên ứng dụng) rồi chọn **Khôi phục**.

Panel kiểm tra mã SHA-256 của file trước, và từ chối file bị hỏng trước khi thay đổi bất cứ thứ gì. Nếu file không còn trên máy chủ này, panel tải nó về từ kho từ xa.

::: danger
Khôi phục sẽ ghi đè nội dung hiện tại. Mọi thay đổi sau thời điểm sao lưu sẽ mất.
:::

Khôi phục thay thế những gì:

| Loại | Điều xảy ra |
|---|---|
| **Website** | Mọi file trong thư mục website bị xóa, trừ `tmp` và `logs`, rồi các file trong bản sao lưu được đặt lại, thuộc về user của website. Mỗi database trong bản sao lưu bị làm trống và nạp lại từ bản dump. Database không còn tồn tại sẽ được tạo lại, gắn với website, với mật khẩu mới. Với WordPress, mật khẩu database hiện tại được ghi vào `wp-config.php` |
| **Database** | Database bị xóa, tạo lại rỗng, rồi nạp từ bản dump |
| **Thư mục** | Bản sao lưu được giải nén cạnh nội dung hiện tại trước. Chỉ khi việc đó thành công, nội dung thư mục mới bị thay, nên một bản sao lưu hỏng sẽ giữ nguyên thư mục như cũ. File thêm vào sau thời điểm sao lưu sẽ bị xóa. Chủ sở hữu, quyền và liên kết tượng trưng được khôi phục. Thư mục chưa có sẽ được tạo |
| **Ứng dụng** | Các container của ứng dụng dừng, thư mục và dữ liệu được thay, container khởi động lại và các bản dump database được nạp. Mật khẩu lưu trong bản sao lưu cũng trở lại |

Bản dump trong bản sao lưu website và database được nạp giống như khi [nhập](./databases#nhap) ở trang **Cơ sở dữ liệu**: bằng một user MariaDB tạm thời chỉ dùng được database đó, nên bản dump không bao giờ ghi vào database khác.

Chỉ khôi phục được bản sao lưu website khi website vẫn tồn tại với cùng tên miền. Nếu đã xóa website, hãy tạo lại nó với cùng tên miền rồi khôi phục.

## Sao lưu ứng dụng

Các ứng dụng Docker từ [Kho ứng dụng](./apps) lưu dữ liệu bên ngoài thư mục website, trong `/var/lib/panel-apps/<tên>`. Chúng có bản sao lưu loại **Ứng dụng**:

- Sao lưu một website chạy ứng dụng Docker cũng tạo thêm một bản sao lưu ứng dụng, theo cùng lịch. Khi giới hạn số bản, các bản sao lưu ứng dụng cũ bị xóa cùng với bản của website.
- Để sao lưu một ứng dụng, chọn **Ứng dụng** trong **Sao lưu ngay** hoặc **Thêm lịch** ở trang **Sao lưu**, hoặc vào **Kho ứng dụng** → **Đã cài** → **Sao lưu ngay**. Các ứng dụng dịch vụ như PostgreSQL không có website, nên hãy đặt cho chúng một lịch riêng.
- Trước khi cập nhật ứng dụng, panel sao lưu trước. Nếu bản sao lưu đó thất bại, việc cập nhật không bắt đầu.

Tùy ứng dụng, panel tạm dừng các container trong lúc chép file của chúng, hoặc dump database bằng công cụ riêng của database trước. Bản sao lưu ứng dụng được khôi phục ở trang **Sao lưu**, giống mọi bản khác. Muốn khôi phục, ứng dụng phải đang được cài. Sau khi gỡ một ứng dụng, các bản sao lưu của nó vẫn nằm trong danh sách, kèm tên và logo: hãy cài lại ứng dụng (với ứng dụng có website thì trên cùng tên miền), rồi khôi phục.

## Xóa bản sao lưu

Mở menu của bản sao lưu và chọn **Xóa**. Bản sao lưu bị xóa khỏi máy chủ này và khỏi kho từ xa.

## Cảnh báo khi sao lưu thất bại

Panel có thể gửi tin nhắn qua Telegram hoặc email khi một lần sao lưu hay khôi phục thất bại. Vào **Cài đặt** → **Cảnh báo** và bật **Sao lưu, khôi phục hoặc gia hạn chứng chỉ thất bại**. Xem [Thư và cảnh báo](./mail-alerts).

Thẻ **Sao lưu tự động** cũng hiện **Lỗi** cho lịch có lần chạy gần nhất bị lỗi.
