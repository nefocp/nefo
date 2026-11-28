---
title: Khôi phục trên máy chủ mới
description: Tìm các bản sao lưu trên kho S3, SFTP hoặc Google Drive từ một máy chủ Nefo mới, khôi phục các website chưa có trên máy cùng database của chúng, và tự giải mã một bản sao lưu bằng tay.
---

# Khôi phục trên máy chủ mới

Nếu mất một máy chủ, một máy chủ mới có Nefo có thể tìm các bản sao lưu trên kho lưu trữ từ xa của bạn (tương thích S3, SFTP hoặc Google Drive) bằng **Tìm bản sao lưu**, rồi khôi phục chúng. Website chưa có trên máy chủ mới sẽ được tạo lại từ bản sao lưu, với tên miền, alias, phiên bản PHP, file và database của nó. Tính năng này có từ v0.24.0.

Trang này hướng dẫn: tìm các bản sao lưu, những gì một lần khôi phục tạo ra và không tạo ra, các loại bản sao lưu khác, và cách tự giải mã một bản sao. Về sao lưu nói chung, xem [Sao lưu](./backups).

## Bạn cần gì {#what-you-need}

- thông tin truy cập kho: bucket và các key của nó, tài khoản SFTP, hoặc một Google OAuth client thuộc cùng project Google Cloud;
- passphrase, nếu các bản sao được mã hóa;
- một máy chủ có Nefo, web server, các phiên bản PHP mà website của bạn dùng, và MariaDB hoặc MySQL (xem [Những việc đầu tiên](./first-steps#set-up-the-server)).

## Làm sao khôi phục một website trên máy chủ mới? {#restore-on-another-server}

1. Cài Nefo lên máy chủ mới, rồi cài web server, PHP và MariaDB (xem [Những việc đầu tiên](./first-steps)).
2. Vào **Sao lưu** → **Kho lưu trữ từ xa** và cấu hình đúng kho cũ: cùng bucket và thư mục, cùng máy chủ SFTP và thư mục, hoặc cùng thư mục Google Drive. Với Google Drive, hãy dùng cùng project Google Cloud (xem [Google Drive](./backups#google-drive)). Đánh dấu **Mã hóa bản sao lưu trước khi tải lên** với passphrase cũ là không bắt buộc: bạn cũng có thể nhập passphrase ở bước 4.
3. Chọn **Tìm bản sao lưu** trên dòng của kho. Panel liệt kê các file sao lưu trên kho chưa có trong danh sách sao lưu của nó, và đọc phần đầu của từng file (`manifest.json` của nó): file sao lưu cái gì, được tạo lúc nào, kích thước, phiên bản PHP và các database. Panel không tải phần còn lại về.
4. Nếu một số bản sao lưu đã mã hóa không mở được bằng passphrase của kho, hãy nhập passphrase đã dùng lúc tải chúng lên rồi chọn **Đọc lại**.
5. Chọn **Thêm vào danh sách** cho một bản sao lưu, hoặc **Thêm tất cả vào danh sách (…)**, kèm số bản sao lưu mà panel đọc được. Trên máy chủ không có gì thay đổi: các bản sao lưu giờ đã nằm trong danh sách **Sao lưu**, và chỉ được giữ trên kho.
6. Khôi phục từng bản bằng **Khôi phục**, như ở [Khôi phục bản sao lưu](./backups#restore-a-backup). **Khôi phục** trong danh sách tìm được làm bước 5 và 6 cùng một lúc.

Bản sao lưu mà panel không đọc được vẫn nằm trong danh sách kèm lý do, ví dụ sai passphrase, file bị hỏng, hoặc manifest ghi một website khác với thư mục chứa file. **Tìm lại** đọc lại kho.

Các bản sao lưu tìm thấy trên kho không có checksum ghi trong danh sách. Lần khôi phục đầu tiên đọc toàn bộ file một lượt trước khi thay đổi bất cứ thứ gì, nên file bị hỏng sẽ bị từ chối, và lúc đó checksum được ghi lại.

## Khôi phục một website chưa có trên máy này {#restoring-a-website-that-doesn-t-exist-here}

Khi website của một bản sao lưu chưa có trên máy chủ này, hộp thoại khôi phục sẽ báo điều đó, và việc khôi phục tạo website:

- với cùng tên miền và alias. Nếu một alias đã thuộc về website khác, website được tạo mà không có alias;
- với cùng phiên bản PHP, hoặc phiên bản mới nhất đã cài trên máy này, kèm một cảnh báo trong log;
- được đánh dấu là WordPress (hoặc ứng dụng khác) cùng quy tắc rewrite và các chỉ thị web server riêng, khi bản sao lưu có chúng;
- với các file của nó, thuộc về user của website mới;
- với các database và user của chúng. Bản sao lưu tạo từ v0.24.0 trở đi giữ mật khẩu của từng user database, nên cấu hình riêng của website (`wp-config.php` và các tệp tương tự) vẫn chạy được. Với bản sao lưu cũ hơn, user nhận mật khẩu mới: WordPress tự nhận mật khẩu đó trong `wp-config.php`; với phần mềm khác, hãy chép mật khẩu từ **Cơ sở dữ liệu**.

Không có trong bản sao lưu nên không được tạo lại: chứng chỉ SSL (hãy cấp chứng chỉ mới ở tab **SSL / HTTPS** của website khi DNS đã trỏ về máy chủ mới), cron job, tài khoản SFTP, chương trình nền, chuyển hướng, bảo vệ bằng mật khẩu và các cài đặt khác của website.

Bản sao lưu tạo bằng Nefo 0.24.1 trở lên ghi lại loại của từng website: website ứng dụng (Node.js, Python, Java) được tạo lại kèm môi trường chạy, lệnh khởi động, cổng và giới hạn bộ nhớ; website reverse proxy kèm địa chỉ đích và các tùy chọn. Hãy cài môi trường chạy của ứng dụng ở trang **Phần mềm** trước; nếu thiếu, việc khôi phục sẽ dừng và báo cho bạn. Bản sao lưu tạo bằng các bản cũ hơn chỉ ghi phiên bản PHP, nên website như vậy được tạo lại thành website tĩnh: hãy tạo website trước với cùng tên miền và cùng loại (xem [Website](./websites#create-a-website)), rồi mới khôi phục, khi đó việc khôi phục chỉ thay file và database của nó.

Khôi phục không bao giờ thay một website khi chưa được xác nhận: bạn gõ tên miền của website vào hộp thoại, dù website đã có hay chưa. Khôi phục đè lên một website đang có sẽ thay file và database của nó, như mọi khi.

::: tip
Bản sao lưu website chứa mật khẩu của user các database, bên cạnh file và bản dump. Hãy coi bản sao lưu nhạy cảm như chính website. Mã hóa các bản sao từ xa giúp chúng không đọc được trên kho.
:::

## Các loại bản sao lưu khác {#other-kinds-of-backups}

| Loại | Trên máy chủ mới |
|---|---|
| **Database** | Được tạo lại nếu chưa có, với mật khẩu user lấy từ bản sao lưu khi bản sao lưu có lưu |
| **Thư mục** | Được tạo lại nếu chưa có |
| **Ứng dụng** | Cài ứng dụng từ Kho ứng dụng trước (image của nó được tải lại), rồi mới khôi phục |

## Tự giải mã một bản sao {#decrypt-a-copy-by-hand}

Bạn cũng có thể tự tải một file sao lưu từ kho về: từ S3 console, bằng một SFTP client, hoặc từ Google Drive trên trình duyệt. Một bản sao đã mã hóa (`.tar.gz.enc`) chỉ cần passphrase của nó. Hãy giải mã bằng chương trình `nefo`, trên bất kỳ máy chủ Linux nào đã cài nó:

```sh
nefo decrypt-backup example.com_20261126-030000.tar.gz.enc
```

Lệnh hỏi passphrase và ghi `example.com_20261126-030000.tar.gz` ngay bên cạnh. Xem [Dòng lệnh](./cli#nefo-decrypt-backup).

File sao lưu chứa các file của website trong `files/` và bản dump database trong `databases/`. Giải nén nó bằng `tar xzf`.

## Định dạng của bản sao đã mã hóa {#format-of-encrypted-copies}

Phần này để tham khảo, và để giải mã một bản sao mà không cần Nefo. Mọi số đều theo thứ tự big-endian.

| Vị trí (offset) | Kích thước | Nội dung |
|---|---|---|
| 0 | 7 | `NEFOENC` (ASCII) |
| 7 | 1 | Phiên bản định dạng: 1 |
| 8 | 1 | Cách sinh khóa: 1 = scrypt |
| 9 | 1 | Chi phí scrypt, log2(N): 16 |
| 10 | 1 | scrypt r: 8 |
| 11 | 1 | scrypt p: 1 |
| 12 | 1 | Thuật toán mã hóa: 1 = AES-256-GCM |
| 13 | 1 | log2(kích thước phần): 16, tức 64 KiB |
| 14 | 2 | Số 0 |
| 16 | 32 | Salt, ngẫu nhiên cho mỗi file |
| 48 | 12 | Giá trị kiểm tra khóa |
| 60 | 4 | CRC-32 (IEEE) của byte 0 đến 59 |

- scrypt(passphrase, salt, N, r, p) cho ra 44 byte. Byte 0–31 là khóa AES-256; byte 32–43 phải bằng giá trị kiểm tra khóa, nếu không thì passphrase sai.
- Sau header 64 byte là các phần (chunk). File sao lưu được cắt thành các phần có kích thước bằng kích thước phần. Phần cuối ngắn hơn, có thể rỗng, nên luôn có size ÷ chunk + 1 phần (làm tròn xuống, rồi cộng một), với size là kích thước file sao lưu và chunk là kích thước phần.
- Mỗi phần được niêm phong bằng AES-256-GCM, với header 64 byte làm dữ liệu bổ sung (additional data), và nonce gồm: số thứ tự của phần tính từ 0 (8 byte), 3 byte 0, rồi `01` cho phần cuối và `00` cho các phần khác. Mỗi phần được lưu dưới dạng ciphertext, theo sau là tag 16 byte.
- Kích thước của file mã hóa là 64 + size + 16 × (size ÷ 65536 + 1).
