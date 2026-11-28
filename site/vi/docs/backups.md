---
title: Sao lưu
description: Sao lưu website, database, thư mục và ứng dụng theo lịch có giới hạn số bản, lưu trên máy chủ này, kho tương thích S3, máy chủ SFTP hoặc Google Drive, có thể mã hóa, và khôi phục chỉ với một cú nhấp, kể cả trên máy chủ mới.
---

# Sao lưu

Nefo sao lưu website, database, thư mục và ứng dụng từ Kho ứng dụng thành các file `.tar.gz`, thủ công hoặc theo lịch hằng ngày hay hằng tuần có giới hạn số bản giữ lại. Mỗi bản sao lưu được giữ trên máy chủ, và Nefo có thể tải một bản sao lên kho tương thích S3, máy chủ SFTP hoặc Google Drive, mã hóa bằng passphrase nếu bạn muốn. Khôi phục chỉ cần chọn bản sao lưu và gõ tên để xác nhận, kể cả trên máy chủ mới.

Trang này hướng dẫn: bản sao lưu gồm những gì, sao lưu thủ công và theo lịch, kho lưu trữ từ xa và việc mã hóa, khôi phục, và cảnh báo khi sao lưu thất bại. Để khôi phục trên máy chủ khác, xem [Khôi phục trên máy chủ mới](./restore-new-server).

## Bản sao lưu gồm những gì {#what-a-backup-contains}

| Loại | Nội dung |
|---|---|
| **Website** | Các file trong thư mục của website, `/var/www/example.com`, trừ hai thư mục `tmp` và `logs` ở cấp đầu, kèm bản dump SQL của mọi database gắn với website |
| **Database** | Bản dump SQL của một database |
| **Thư mục** | Bất kỳ thư mục nào trên máy chủ, ví dụ `/etc/nginx` hoặc `/opt/app`, giữ nguyên chủ sở hữu, quyền và liên kết tượng trưng |
| **Ứng dụng** | Một ứng dụng Docker từ Kho ứng dụng: thư mục của nó trong `/var/lib/panel-apps`, các mật khẩu đã lưu, dữ liệu của container và bản dump database (xem [Sao lưu ứng dụng](#app-backups)) |

Hãy gắn database với website ngay khi tạo (xem [Cơ sở dữ liệu](./databases#create-a-database)), để bản sao lưu website có cả database đó. Bản sao lưu website bỏ qua liên kết tượng trưng và file đặc biệt.

Mỗi bản sao lưu là một file `.tar.gz` có `manifest.json`, các file nằm trong `files/` và bản dump database nằm trong `databases/`. Bản dump được tạo bằng `mariadb-dump --single-transaction`, nên website vẫn chạy bình thường.

`manifest.json` của bản sao lưu website còn ghi lại những gì cần để tạo lại website trên máy chủ khác: các alias, phiên bản PHP, quy tắc rewrite, các chỉ thị web server riêng của website, và mật khẩu của user các database.

Bản sao lưu trên máy chủ này không được mã hóa. Chỉ root đọc được chúng. Mỗi kho lưu trữ từ xa có thể mã hóa các bản sao bằng passphrase trước khi chúng rời khỏi máy chủ (xem [Mã hóa các bản sao](#encrypt-the-copies)).

## Bản sao lưu được lưu ở đâu {#where-backups-are-kept}

Mọi bản sao lưu đều được lưu trước tiên trên máy chủ này, trong `/var/backups/panel`:

```
/var/backups/panel/site/example.com/example.com_20261126-030000.tar.gz
/var/backups/panel/database/shop/shop_20261126-030000.tar.gz
/var/backups/panel/folder/etc_nginx-1a2b3c4d/etc_nginx_20261126-030000.tar.gz
```

Các thư mục có quyền `0700`, các file có quyền `0600`. Panel cũng ghi lại mã SHA-256 của từng file. Bản sao lưu cục bộ chiếm dung lượng ổ đĩa, nên hãy để ý ổ đĩa và số bản mà các lịch của bạn giữ lại.

Để chép một bản sao lưu về máy tính, mở menu của nó và chọn **Tải về**.

## Sao lưu ngay {#back-up-now}

1. Vào **Sao lưu** và chọn **Sao lưu ngay**.
2. Chọn **Website**, **Database**, **Thư mục** hoặc **Ứng dụng**. Mục **Ứng dụng** xuất hiện khi đã cài một ứng dụng từ Kho ứng dụng.
3. Chọn website, database hoặc ứng dụng ở ô **Sao lưu của**, hoặc gõ **Đường dẫn thư mục** như `/etc/nginx`. Các thư mục từng sao lưu sẽ được gợi ý.
4. Nếu đã cấu hình kho lưu trữ từ xa, bạn có thể đánh dấu **Tải lên cả kho lưu trữ từ xa**.
5. Chọn **Sao lưu ngay**.

Log tiến trình hiện từng bước. Panel chỉ chạy một tác vụ dài tại một thời điểm, nên hãy chờ tác vụ đang chạy xong trước.

Với website, bạn cũng có thể vào **Website** → (site) → **Sao lưu** và chọn **Sao lưu ngay**.

### Thư mục không sao lưu được {#folders-that-can-t-be-backed-up}

Panel từ chối các thư mục sau, mọi thư mục bên trong chúng, và mọi thư mục chứa một trong số chúng. Liên kết tượng trưng được lần theo trước khi kiểm tra.

- `/`, tức toàn bộ hệ thống;
- `/proc`, `/sys`, `/dev` và `/run`;
- dữ liệu của panel, `/var/lib/nefo`, và các bản sao lưu của nó, `/var/backups/panel`;
- `/var/lib/panel-sftp`, `/var/lib/panel-acme` và `/var/lib/panel-quarantine`.

Vì vậy không sao lưu được cả `/var` hay `/var/lib`, nhưng sao lưu được `/var/www` hoặc `/etc`.

## Sao lưu tự động {#automatic-backups}

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

### Giới hạn số bản giữ lại {#retention}

Sau mỗi lần sao lưu theo lịch, panel xóa các bản cũ nhất do lịch đó tạo vượt quá số bản cần giữ, cả trên máy chủ này lẫn trong kho từ xa. Bản sao lưu bạn tạo thủ công không bao giờ bị xóa tự động.

Bản sao lưu mà Nefo tạo trước khi xóa một website (**Sao lưu website trước**) được tính như bản tạo thủ công: nó không bao giờ bị xóa tự động. Tên của nó kết thúc bằng `_before-delete_<date>-<time>.tar.gz`.

Xóa một lịch sẽ dừng lịch đó. Các bản sao lưu đã có vẫn được giữ. Khi xóa một website, một database hoặc một ứng dụng, lịch của nó cũng bị xóa, còn các bản sao lưu vẫn nằm ở trang **Sao lưu**.

## Kho lưu trữ từ xa {#remote-storage}

Một bản sao ở nhà cung cấp khác giúp bạn an toàn nếu máy chủ này gặp sự cố. Vào **Sao lưu** → **Kho lưu trữ từ xa** và chọn **Cấu hình** (hoặc **Thêm kho lưu trữ** khi đã có một kho). Bạn có thể cấu hình cùng lúc kho tương thích S3, máy chủ SFTP và Google Drive: khi đó bản sao lưu được tải lên từng nơi.

### Kho tương thích S3 {#s3-compatible-storage}

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

### Máy chủ SFTP {#sftp-server}

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

### Ghim host key {#host-key-pinning}

**Kiểm tra và lưu** tin cậy host key mà máy chủ SFTP đưa ra lúc đó, và hiện nó ở mục **Host key được tin cậy**. Mọi lần kết nối sau phải đưa ra đúng khóa đó, nếu không việc tải bản sao lưu lên sẽ thất bại.

Trước khi dựa vào kho này, hãy so fingerprint với khóa của chính máy chủ sao lưu. Trên máy chủ sao lưu, chạy:

```sh
ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub
```

Nếu máy chủ sao lưu được cài lại hoặc bạn chủ động đổi khóa, hãy chọn **Thay đổi** rồi **Kiểm tra và lưu** lần nữa để tin cậy khóa mới. Panel sẽ cảnh báo rằng khóa đã được thay.

::: warning
Host key thay đổi cũng có thể là dấu hiệu có người đang chặn kết nối. Nếu bạn không chờ đợi thay đổi này, hãy kiểm tra máy chủ sao lưu trước khi tin cậy khóa mới.
:::

### Nefo có sao lưu lên Google Drive được không? {#google-drive}

Có, từ v0.24.0. Bản sao lưu được đưa vào một thư mục trong Google Drive của bạn. Panel chỉ xin quyền truy cập các file do chính nó tạo ra (quyền `drive.file` của Google): nó không thấy và không sửa được bất cứ thứ gì khác trong Drive của bạn. Thư mục bạn tự tạo bằng tay trong Drive thì panel không nhìn thấy, kể cả khi đặt đúng tên.

Google Drive cần một OAuth client của riêng bạn, trong một project Google Cloud. Việc này không tốn tiền và bạn chỉ tạo một lần.

Google Drive mới chỉ được thử với một dịch vụ Google giả lập, chưa được thử với tài khoản Google thật.

#### Tạo OAuth client {#create-the-oauth-client}

Các tên in đậm trong những bước này là tên của Google, không phải của panel.

1. Mở [Google Cloud console](https://console.cloud.google.com/) rồi tạo một project, hoặc chọn một project có sẵn.
2. Trong **APIs and services** → **Library**, bật **Google Drive API**.
3. Thiết lập **OAuth consent screen** với user type **External**, rồi chuyển publishing status sang **In production**.
4. Trong **Credentials**, tạo một **OAuth client ID** loại **TVs and Limited Input devices**.
5. Giữ lại client ID (kết thúc bằng `.apps.googleusercontent.com`) và client secret.

::: warning
Khi publishing status của consent screen còn là **Testing**, Google sẽ cắt quyền truy cập của panel sau 7 ngày, và từ đó việc tải lên sẽ thất bại. Hãy chuyển sang **In production**.
:::

#### Kết nối {#connect}

1. Trong **Kho lưu trữ từ xa**, chọn **Cấu hình** hoặc **Thêm kho lưu trữ**, rồi chọn **Google Drive** ở ô **Loại**. Mục **Cách tạo OAuth client** nhắc lại các bước ở trên.
2. Nhập **Client ID** và **Client secret**.
3. Không bắt buộc: đổi **Thư mục trong Google Drive**. Mặc định là `Nefo backups/<the server's host name>` (tên máy của máy chủ). Các thư mục cách nhau bằng `/`; thư mục nào chưa có sẽ được tạo.
4. Không bắt buộc: đánh dấu **Mã hóa bản sao lưu trước khi tải lên** (xem [Mã hóa các bản sao](#encrypt-the-copies)).
5. Chọn **Kết nối Google Drive**. Panel hiện một địa chỉ, `google.com/device`, và một mã như `ABCD-EFGH`.
6. Trên bất kỳ thiết bị nào (điện thoại cũng được), mở địa chỉ đó, đăng nhập bằng tài khoản Google sẽ chứa bản sao lưu, nhập mã rồi cho phép truy cập.
7. Panel chờ trong lúc bạn làm việc này, và cho biết mã còn hiệu lực bao lâu. Khi Google trả lời, panel tạo thư mục, ghi, liệt kê, đọc và xóa một file thử nhỏ ở đó, rồi lưu cấu hình.

Sau đó danh sách **Kho lưu trữ từ xa** hiện thư mục, tài khoản Google và một liên kết **Mở trong Google Drive**.

Bạn có thể rời trang trong lúc panel đang chờ: mở lại **Sao lưu** → **Kho lưu trữ từ xa** để xem mã.

#### Thay đổi hoặc kết nối lại {#change-or-reconnect}

Chọn **Thay đổi** trên dòng Google Drive:

- Để đổi thư mục hoặc chế độ mã hóa, sửa chúng rồi chọn **Kiểm tra và lưu**. Không cần đăng nhập lại. Thư mục mới sẽ được tạo nếu cần. Các bản sao lưu đã tải lên trước đó vẫn ở thư mục cũ, và panel không còn tìm thấy chúng ở đó để khôi phục.
- Để dùng tài khoản Google khác hoặc OAuth client khác, hoặc sau khi quyền truy cập bị thu hồi, chọn **Kết nối lại** và nhập mã mới. Để trống **Client secret** để giữ secret đã lưu của cùng client đó.

**Kiểm tra và lưu** cũng sửa được trường hợp thư mục đã bị xóa hoặc bị chuyển vào thùng rác của Drive: nó tạo lại thư mục.

Hãy giữ nguyên project Google Cloud. Google chỉ cho mỗi project truy cập các file do chính project đó tạo, nên với một OAuth client của project khác, panel không thấy được các bản sao lưu nó đã tải lên trước đó.

#### Cách tải lên hoạt động {#how-uploads-work}

- Mỗi bản sao lưu được tải lên theo từng phần 8 MB. Nếu kết nối bị ngắt hoặc Google đang bận, panel hỏi Google đã nhận được bao nhiêu và tiếp tục từ đó. Panel thử lại trong vài phút rồi mới bỏ cuộc.
- Trước khi tải lên, panel kiểm tra dung lượng trống của tài khoản Google và từ chối ngay khi bản sao lưu không vừa.
- Việc tải về để khôi phục cũng tiếp tục từ chỗ bị dừng.
- Thư mục trong Drive có các thư mục con giống `/var/backups/panel`: `site/example.com/…`, `database/shop/…`.

#### Ngắt kết nối {#disconnect}

**Xóa** trên dòng Google Drive sẽ quên cấu hình và báo cho Google rằng panel bỏ quyền truy cập của mình. Các bản sao lưu đã tải lên vẫn còn trong Google Drive của bạn.

#### Khi có sự cố {#when-something-goes-wrong}

| Thông báo | Cần làm gì |
|---|---|
| Quyền truy cập Google Drive đã bị thu hồi hoặc hết hạn | Chọn **Thay đổi** → **Kết nối lại**. Nếu chuyện này xảy ra khoảng 7 ngày sau khi kết nối, trước hết hãy chuyển publishing status của consent screen sang **In production**. |
| Google Drive đã đầy | Dọn bớt dung lượng trong tài khoản Google hoặc mua thêm dung lượng, rồi tải lên lại bằng **Tải lên**. |
| Thư mục Google Drive … đã bị xóa / đang nằm trong thùng rác của Drive | Khôi phục thư mục từ thùng rác của Drive (các bản sao lưu trong đó cũng trở lại cùng nó), hoặc chọn **Thay đổi** → **Kiểm tra và lưu** để tạo thư mục mới. |
| Google từ chối OAuth client này | Kiểm tra client ID và secret, và loại của client phải là **TVs and Limited Input devices**. |
| Mã đã hết hạn trước khi được nhập | Chọn lại **Kết nối Google Drive** để lấy mã mới. |

### Mã hóa các bản sao {#encrypt-the-copies}

Mỗi kho (S3, SFTP và Google Drive) đều có thể mã hóa các bản sao lưu mà nó nhận. Khi đó, bản sao trên kho không đọc được nếu không có passphrase, cả với nhà cung cấp lẫn với bất kỳ ai vào được tài khoản kho. Bản trên máy chủ này vẫn là một file `.tar.gz` bình thường.

1. Trong biểu mẫu của kho, đánh dấu **Mã hóa bản sao lưu trước khi tải lên**.
2. Nhập **Passphrase** có ít nhất 12 ký tự, và nhập lại y như vậy ở **Nhập lại passphrase**. Một câu gồm vài từ là lựa chọn tốt.
3. Chọn **Kiểm tra và lưu** (hoặc **Kết nối Google Drive**). File thử cũng được mã hóa.

::: danger
Không có passphrase thì không thể khôi phục các bản sao lưu đã mã hóa: bạn không, panel không, và không ai cả. Hãy cất nó bên ngoài máy chủ này, ví dụ trong trình quản lý mật khẩu. Nếu máy chủ này bị mất, bạn chỉ cần passphrase cùng với kho để lấy lại các bản sao lưu.
:::

Những gì thay đổi:

- Bản sao đã mã hóa mang tên của bản sao lưu cộng thêm `.enc`: `example.com_20261126-030000.tar.gz.enc`.
- Trong danh sách **Sao lưu**, nhãn của kho ở cột **Nơi lưu** hiện biểu tượng ổ khóa thay vì đám mây.
- Dòng của kho trong **Kho lưu trữ từ xa** hiện **Đã mã hóa**.
- Passphrase không bao giờ hiện lại. Để giữ nguyên passphrase khi đổi các cấu hình khác, hãy để trống **Passphrase**.

Đổi passphrase:

- Passphrase mới chỉ áp dụng cho các lần tải lên sau. Các bản sao đã tải lên vẫn giữ passphrase lúc chúng được mã hóa; chúng không được mã hóa lại.
- Máy chủ này nhớ các passphrase cũ (tối đa 10), nên việc khôi phục trên máy này vẫn chạy được.
- Trên máy chủ khác, bạn cần đúng passphrase đang dùng lúc bản sao đó được tải lên.
- Nếu bạn tắt mã hóa, các bản sao đã tải lên vẫn ở dạng mã hóa, và máy chủ này giữ passphrase của chúng để khôi phục.

Cách hoạt động: panel mã hóa file sao lưu ngay trên máy chủ này, trong lúc tải lên, bằng AES-256-GCM. Khóa được sinh từ passphrase bằng scrypt, với một salt ngẫu nhiên mới cho mỗi file, nên hai bản sao của cùng một bản sao lưu trông khác nhau. File được niêm phong theo từng phần 64 KB, mỗi phần được đánh số và phần cuối được đánh dấu, nên một bản sao bị sửa, bị cắt ngắn hay bị ghép từ các phần của bản sao khác đều bị từ chối. Không có khóa nào được lưu trên máy chủ ngoài passphrase trong cấu hình của panel. Định dạng file được mô tả ở [Khôi phục trên máy chủ mới](./restore-new-server#format-of-encrypted-copies).

Mã hóa tốn rất ít: việc sinh khóa mất khoảng một phần tư giây và 64 MB bộ nhớ trong chốc lát, một lần cho mỗi lần tải lên, và bản sao lớn hơn một phần đầu 64 byte cộng 16 byte cho mỗi 64 KB. Với kho S3, panel đọc bản sao lưu hai lần, vì S3 cần checksum của bản sao đã mã hóa trước khi bắt đầu tải lên.

### Tải lên và gỡ kho {#uploading-and-removing-storage}

- Bản sao lưu tạo mà không đánh dấu **Tải lên cả kho lưu trữ từ xa** vẫn tải lên được sau: mở menu của nó và chọn **Tải lên**. Panel chép nó vào từng kho chưa có bản đó, có mã hóa với những kho bật mã hóa.
- Cột **Nơi lưu** hiện **Máy chủ này** và một nhãn cho mỗi bản sao từ xa: S3, SFTP hoặc Google Drive, kèm biểu tượng ổ khóa với bản sao đã mã hóa.
- Xóa cấu hình kho không xóa các bản sao lưu đã tải lên đó.
- **Tìm bản sao lưu** trên dòng của một kho liệt kê các bản sao lưu trên kho đó chưa có trong danh sách, ví dụ trên máy chủ mới: xem [Khôi phục trên máy chủ mới](./restore-new-server).

## Khôi phục bản sao lưu {#restore-a-backup}

1. Vào **Sao lưu** và tìm bản sao lưu trong danh sách.
2. Chọn **Khôi phục** trên dòng của nó.
3. Gõ tên mà hộp thoại yêu cầu (tên miền, tên database, đường dẫn thư mục hoặc tên ứng dụng) rồi chọn **Khôi phục**.

Panel kiểm tra mã SHA-256 của file trước, và từ chối file bị hỏng trước khi thay đổi bất cứ thứ gì. Nếu file không còn trên máy chủ này, panel tải nó về từ kho từ xa.

Nếu bản sao được mã hóa, panel giải mã nó trong lúc tải về, bằng passphrase của kho (hoặc một passphrase cũ mà máy chủ này còn nhớ). Sau đó việc kiểm tra SHA-256 chạy trên file đã giải mã, như với mọi bản sao lưu khác. Khi một bản sao lưu chỉ còn trên kho từ xa và bản sao đó được mã hóa, hộp thoại **Khôi phục** có ô **Passphrase mã hóa (không bắt buộc)**. Để trống để dùng passphrase của kho. Hãy nhập passphrase đang dùng lúc tải bản sao lên nếu kho đã được cấu hình lại từ đó tới nay, ví dụ sau khi bạn gỡ kho rồi thêm lại với passphrase mới.

Việc khôi phục dừng lại trước khi thay đổi bất cứ thứ gì, kèm một thông báo trong log, khi:

- passphrase không mở được bản sao ("this copy is encrypted and the passphrase does not unlock it");
- bản sao đã bị sửa hoặc bị hỏng ("the encrypted copy is damaged or was changed");
- bản sao không đầy đủ ("the encrypted copy is incomplete").

Nếu có bản sao trên nhiều kho, panel thử kho tiếp theo.

::: danger
Khôi phục sẽ ghi đè nội dung hiện tại. Mọi thay đổi sau thời điểm sao lưu sẽ mất.
:::

Khôi phục thay thế những gì:

| Loại | Điều xảy ra |
|---|---|
| **Website** | Mọi file trong thư mục website bị xóa, trừ `tmp` và `logs`, rồi các file trong bản sao lưu được đặt lại, thuộc về user của website. Mỗi database trong bản sao lưu bị làm trống và nạp lại từ bản dump. Database không còn tồn tại sẽ được tạo lại, gắn với website, với mật khẩu user lấy từ bản sao lưu (bản sao lưu tạo trước v0.24.0 không có mật khẩu này: khi đó database nhận mật khẩu mới). Database không còn gắn với website nào sẽ được gắn với website vừa khôi phục. Với WordPress, mật khẩu database hiện tại được ghi vào `wp-config.php` |
| **Database** | Database bị xóa, tạo lại rỗng, rồi nạp từ bản dump |
| **Thư mục** | Bản sao lưu được giải nén cạnh nội dung hiện tại trước. Chỉ khi việc đó thành công, nội dung thư mục mới bị thay, nên một bản sao lưu hỏng sẽ giữ nguyên thư mục như cũ. File thêm vào sau thời điểm sao lưu sẽ bị xóa. Chủ sở hữu, quyền và liên kết tượng trưng được khôi phục. Thư mục chưa có sẽ được tạo |
| **Ứng dụng** | Các container của ứng dụng dừng, thư mục và dữ liệu được thay, container khởi động lại và các bản dump database được nạp. Mật khẩu lưu trong bản sao lưu cũng trở lại |

Bản dump trong bản sao lưu website và database được nạp giống như khi [nhập](./databases#import) ở trang **Cơ sở dữ liệu**: bằng một user MariaDB hoặc MySQL tạm thời chỉ dùng được database đó, nên bản dump không bao giờ ghi vào database khác.

### Khôi phục website đã bị xóa {#restore-a-website-that-was-deleted}

Nếu website của một bản sao lưu không còn trên máy chủ này, ví dụ vì bạn đã xóa nó, việc khôi phục sẽ tạo lại website từ bản sao lưu: cùng tên miền và alias, phiên bản PHP, quy tắc rewrite, file và database. Hộp thoại khôi phục sẽ báo điều này trước. Những gì bản sao lưu không chứa, như chứng chỉ, cron job và quyền truy cập SFTP, sẽ không được tạo. Xem chi tiết ở [Khôi phục trên máy chủ mới](./restore-new-server#restoring-a-website-that-doesn-t-exist-here).

Bản sao lưu tạo bằng Nefo 0.24.1 trở lên ghi lại loại của từng website, nên website không còn trên máy sẽ được tạo lại đúng loại: website ứng dụng (Node.js, Python, Java) kèm môi trường chạy, lệnh khởi động, cổng và giới hạn bộ nhớ; website reverse proxy kèm địa chỉ đích và các tùy chọn. Nếu môi trường chạy của ứng dụng chưa được cài, việc khôi phục sẽ dừng và nhắc bạn cài nó ở trang **Phần mềm** trước. Bản sao lưu tạo bằng các bản cũ hơn chỉ ghi phiên bản PHP: website như vậy sẽ trở lại thành website tĩnh, nên hãy tạo lại nó trước với cùng tên miền và cùng loại, rồi mới khôi phục. Với ứng dụng Docker, hãy cài lại ứng dụng trên cùng tên miền từ [Kho ứng dụng](./apps), rồi khôi phục bản sao lưu loại **Ứng dụng** của nó, cùng bản sao lưu website nếu thư mục website có file riêng, như nhạc của Navidrome.

## Khôi phục trên máy chủ khác {#restore-on-another-server}

Một máy chủ mới có Nefo có thể tìm các bản sao lưu trên kho lưu trữ từ xa của bạn bằng **Tìm bản sao lưu**, thêm chúng vào danh sách và khôi phục, kể cả website. Bạn cần thông tin truy cập kho, và passphrase nếu các bản sao được mã hóa. Xem [Khôi phục trên máy chủ mới](./restore-new-server).

## Sao lưu ứng dụng {#app-backups}

Các ứng dụng Docker từ [Kho ứng dụng](./apps) lưu dữ liệu bên ngoài thư mục website, trong `/var/lib/panel-apps/<name>`. Chúng có bản sao lưu loại **Ứng dụng**:

- Sao lưu một website chạy ứng dụng Docker cũng tạo thêm một bản sao lưu ứng dụng, theo cùng lịch. Khi giới hạn số bản, các bản sao lưu ứng dụng cũ bị xóa cùng với bản của website.
- Để sao lưu một ứng dụng, chọn **Ứng dụng** trong **Sao lưu ngay** hoặc **Thêm lịch** ở trang **Sao lưu**, hoặc vào **Kho ứng dụng** → **Đã cài** → **Sao lưu ngay**. Các ứng dụng dịch vụ như PostgreSQL không có website, nên hãy đặt cho chúng một lịch riêng.
- Trước khi cập nhật ứng dụng, panel sao lưu trước. Nếu bản sao lưu đó thất bại, việc cập nhật không bắt đầu.

Tùy ứng dụng, panel tạm dừng các container trong lúc chép file của chúng, hoặc dump database bằng công cụ riêng của database trước. Bản sao lưu ứng dụng được khôi phục ở trang **Sao lưu**, giống mọi bản khác. Muốn khôi phục, ứng dụng phải đang được cài. Sau khi gỡ một ứng dụng, các bản sao lưu của nó vẫn nằm trong danh sách, kèm tên và logo: hãy cài lại ứng dụng (với ứng dụng có website thì trên cùng tên miền), rồi khôi phục.

## Xóa bản sao lưu {#delete-a-backup}

Mở menu của bản sao lưu và chọn **Xóa**. Bản sao lưu bị xóa khỏi máy chủ này và khỏi kho từ xa.

## Cảnh báo khi sao lưu thất bại {#alerts-when-a-backup-fails}

Panel có thể gửi cảnh báo qua Telegram, email, Discord, Slack hoặc webhook khi một lần sao lưu hay khôi phục thất bại. Vào **Cài đặt** → **Cảnh báo** và bật **Sao lưu, khôi phục hoặc gia hạn chứng chỉ thất bại**. Xem [Thư và cảnh báo](./mail-alerts).

Thẻ **Sao lưu tự động** cũng hiện **Lỗi** cho lịch có lần chạy gần nhất bị lỗi.
