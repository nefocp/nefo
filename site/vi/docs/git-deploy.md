---
title: Triển khai từ Git
description: Triển khai website từ GitHub, GitLab, Gitea hoặc bất kỳ máy chủ Git nào, bằng tay hoặc mỗi lần push, với khóa triển khai, webhook có chữ ký và lệnh sau khi triển khai.
---

# Triển khai từ Git

Nefo triển khai được website từ một kho Git trên GitHub, GitLab, Gitea, Forgejo hay bất kỳ máy chủ Git nào, qua HTTPS hoặc SSH. Nefo triển khai khi bạn bấm nút, hoặc mỗi lần bạn push nhờ một webhook có chữ ký, và có thể chạy thêm lệnh sau khi triển khai. Mọi thứ đều chạy bằng user Linux riêng của website.

Tính năng này nằm ở tab **Triển khai Git** của từng website, trừ website của ứng dụng Docker.

## Trước khi bắt đầu

- **Địa chỉ kho.** Nefo nhận ba dạng:
  - `https://github.com/owner/repo.git` cho kho công khai;
  - `git@github.com:owner/repo.git` (dạng GitHub hiển thị) cho kho riêng tư;
  - `ssh://git@git.example.com:2222/owner/repo.git` cho máy chủ SSH dùng cổng khác.

  Nefo từ chối `http://`, và cả địa chỉ có chứa mật khẩu hay token, vì chúng sẽ hiện trong nhật ký. Với kho riêng tư, hãy dùng địa chỉ SSH và khóa triển khai của website.
- **Thư mục.** Chọn nơi đặt mã nguồn:
  - **Thư mục web** (`/var/www/example.com/public`): cho website mà chính kho là website, với `index.html` hoặc `index.php` ở gốc kho.
  - **Thư mục website** (`/var/www/example.com`): cho dự án có thư mục `public` riêng, như Laravel hay Symfony. Website khi đó hiển thị thư mục `public` của kho.

  Website ứng dụng Node.js hoặc Python không có lựa chọn: mã nguồn được đặt vào **Thư mục ứng dụng** (`/var/www/example.com/app`), nơi chương trình của website chạy. Hãy đặt `package.json` hoặc `requirements.txt` ở gốc kho.
- **Thư mục trống cho lần đầu.** Trang chờ của panel không tính, các thư mục `logs`, `tmp` và `.ssh` trong thư mục website, cũng như thư mục `.venv` của ứng dụng Python trong thư mục ứng dụng, cũng không tính. Hãy chuyển các file khác đi trước, hoặc triển khai vào thư mục còn lại.
- **Git** không cần cài sẵn: nếu chưa có, Nefo tự cài ở lần triển khai đầu.

## Thiết lập kho

1. Mở **Website**, rồi tab **Triển khai Git** của website.
2. Nhập **Địa chỉ kho (repository)**.
3. Nhập **Nhánh**, ví dụ `main`.
4. Trong mục **Đặt mã nguồn vào**, chọn **Thư mục web** hoặc **Thư mục website**. Website ứng dụng thì hiện **Thư mục ứng dụng**, không có gì để chọn.
5. Không bắt buộc: viết **Lệnh sau khi triển khai (tùy chọn)** (xem [Lệnh sau khi triển khai](#lenh-sau-trien-khai)).
6. Bấm **Lưu**.

Lần đầu lưu, Nefo tạo khóa triển khai và secret webhook cho website, rồi báo "Đã lưu cài đặt. Hãy thêm khóa triển khai và webhook bên dưới vào kho, rồi triển khai."

## Thêm khóa triển khai (kho riêng tư)

Kho HTTPS công khai không cần khóa. Với kho riêng tư, hãy thêm **Khóa công khai** trong phần **Khóa triển khai (deploy key)** vào kho dưới dạng deploy key chỉ đọc:

- **GitHub:** kho → Settings → Deploy keys → Add deploy key. Để tắt "Allow write access".
- **GitLab:** dự án → Settings → Repository → Deploy keys, không cấp quyền ghi.
- **Gitea và Forgejo:** kho → Settings → Deploy keys.

Mỗi website có một khóa ed25519 riêng, được tạo bằng user của website tại `/var/www/example.com/.ssh/panel_deploy`. **Tạo khóa mới** sẽ thay khóa này: khóa cũ ngừng hoạt động ngay, nên hãy thêm khóa mới vào kho và xóa khóa cũ ở đó.

Lần kết nối SSH đầu tiên, Nefo chấp nhận host key của máy chủ Git và ghi nhớ nó trong `/var/www/example.com/.ssh/known_hosts`. Nếu về sau host key đổi, việc triển khai báo lỗi "The Git server's SSH host key is not the one seen before." Nếu máy chủ Git thật sự đã đổi khóa, hãy xóa file đó rồi triển khai lại.

## Triển khai

Trong phần **Triển khai**, bấm **Triển khai ngay**. Nếu vừa đổi cài đặt, hãy lưu trước. Nhật ký hiện từng bước:

1. Nefo tải nhánh từ kho về.
2. Các file được kho theo dõi lấy phiên bản của nhánh.
3. Lệnh sau khi triển khai chạy, nếu bạn có viết.

File không được kho theo dõi vẫn giữ nguyên, nên file tải lên và file mà kho bỏ qua (như `.env`) được giữ lại sau mỗi lần triển khai. Sau lần triển khai đầu, Nefo xóa trang chờ của panel khi kho đã đặt website vào `public`.

::: warning
Những thay đổi bạn sửa trực tiếp trên máy chủ ở các file mà kho theo dõi sẽ bị ghi đè ở lần triển khai sau. Hãy commit chúng vào kho.
:::

Sau mỗi lần triển khai, tab hiện trạng thái (**Đã triển khai** hoặc **Thất bại**), thời điểm, 12 ký tự đầu của commit và tiêu đề commit. Việc tải mã nguồn tối đa 10 phút, lệnh sau khi triển khai tối đa 15 phút, toàn bộ lần triển khai tối đa 30 phút.

Khi tải mã nguồn lỗi, nhật ký giải thích các nguyên nhân thường gặp (bằng tiếng Anh):

| Thông báo | Cách xử lý |
|---|---|
| The repository refused the deploy key | Thêm khóa hiện trong panel vào kho. |
| The repository is private or does not exist | Kiểm tra địa chỉ. Với kho riêng tư, dùng địa chỉ SSH và thêm khóa triển khai. |
| The repository has no branch … | Kiểm tra **Nhánh**. |
| The Git server's SSH host key is not the one seen before | Xem phần khóa triển khai ở trên. |
| The repository's host name was not found | Kiểm tra địa chỉ. |
| It took longer than 10 minutes | Thử lại; kiểm tra dung lượng kho và mạng. |

## Lệnh sau khi triển khai {#lenh-sau-trien-khai}

Các lệnh chạy sau mỗi lần triển khai, trong thư mục triển khai, bằng user của website, tối đa 15 phút. Tối đa 2000 ký tự. Ví dụ cho website Laravel:

```sh
composer install --no-dev --optimize-autoloader
php artisan migrate --force
```

Với website ứng dụng, lệnh chạy trong thư mục `app`, ví dụ `npm ci` rồi `npm run build`. Lệnh `node` và `npm` là phiên bản Node.js đã chọn cho website, còn trên website Python thì `python` và `pip` là của môi trường ảo của nó (`app/.venv`).

- Lệnh chạy với `sh -e`: dừng ngay ở lệnh đầu tiên bị lỗi, và lần triển khai bị đánh dấu **Thất bại**. Lúc đó các file mới đã được đặt vào.
- Lệnh `php` là phiên bản PHP của website, Composer cũng dùng phiên bản này.
- `DEPLOY_COMMIT` chứa commit vừa triển khai, `DEPLOY_BRANCH` chứa tên nhánh.
- `HOME` là thư mục `tmp` của website.
- Lệnh không dùng được `sudo` và không khởi động lại được service hệ thống. Muốn khởi động lại chương trình của website ứng dụng sau khi triển khai, hãy bấm **Khởi động lại** ở tab **Tổng quan** của website.

## Tự triển khai mỗi lần push

Trong phần **Tự triển khai khi push**, Nefo hiện **URL webhook**, ví dụ `https://example.com/.panel/deploy/12`, và **Secret** của nó. Webhook nằm trên chính tên miền của website, nên máy chủ Git không cần truy cập cổng của panel. Web server chuyển nó tới một bộ nhận nhỏ của panel ở `127.0.0.1:7809`.

::: warning
Khi chưa có HTTPS, URL webhook dùng `http://`, và GitLab sẽ gửi secret mà không mã hóa. Hãy bật HTTPS trước: xem [Chứng chỉ SSL](./ssl).
:::

Mỗi lần push lên nhánh đã chọn sẽ triển khai website. Push lên nhánh khác, push tag, các sự kiện khác và việc xóa nhánh đều được bỏ qua.

### GitHub

1. Mở kho → Settings → Webhooks → Add webhook.
2. Dán **URL webhook** vào ô Payload URL và **Secret** vào ô Secret.
3. Chọn Content type là `application/json`.
4. Chọn "Just the push event" rồi thêm webhook.

GitHub gửi ngay một lần thử (ping), và panel trả lời, nên lần gửi đầu hiện là thành công.

### GitLab

1. Mở dự án → Settings → Webhooks.
2. Dán **URL webhook** vào ô URL và **Secret** vào ô Secret token.
3. Chọn Push events rồi thêm webhook.

### Gitea và Forgejo

1. Mở kho → Settings → Webhooks → Add webhook, chọn Gitea (hoặc Forgejo).
2. Dán **URL webhook** vào ô Target URL và **Secret** vào ô Secret.
3. Chọn content type là `application/json`, kích hoạt khi có push events, rồi thêm webhook.

### Panel kiểm tra webhook thế nào

- GitHub, Gitea, Forgejo và Gogs ký mỗi webhook bằng secret (HMAC-SHA256). GitLab gửi chính secret đó. Webhook có chữ ký sai hoặc thiếu chữ ký nhận HTTP 401 và được ghi lại cùng địa chỉ IP của bên gửi.
- Mật khẩu bảo vệ và tường lửa web của website không áp dụng cho đường dẫn webhook, nên bạn không cần tạo ngoại lệ cho nó.
- Nefo chạy mỗi lần một tác vụ. Nếu có tác vụ khác đang chạy khi push tới, việc triển khai sẽ chờ và được thử lại mỗi 15 giây, trong tối đa một giờ. Nhiều lần push trong lúc chờ chỉ cần một lần triển khai: nó lấy commit mới nhất.
- **Tạo secret mới** sẽ thay secret. Các lần push bị từ chối cho đến khi bạn dán secret mới vào webhook của kho.

Máy chủ Git hiện câu trả lời của panel trong danh sách các lần gửi gần đây: `deploying`, `queued: another task is running`, `ignored: not branch main`, hoặc một lỗi như `bad_signature`.

## Nhật ký triển khai

- Tab **Triển khai Git** hiện nhật ký của lần triển khai đang chạy hoặc lần gần nhất, kể cả lần do push khởi chạy.
- **Tài khoản → Hoạt động gần đây** ghi mọi lần triển khai là "Triển khai từ Git" hoặc "Triển khai Git thất bại", kèm website, commit và cách khởi chạy (`manual` hoặc push từ máy chủ Git).

## Ngừng triển khai từ Git

Trong phần **Ngừng triển khai từ Git**, bấm **Ngừng triển khai từ Git** rồi xác nhận. Nefo xóa các cài đặt và khóa triển khai. Các file đã triển khai vẫn nằm trong thư mục, cùng thư mục `.git` của kho.
