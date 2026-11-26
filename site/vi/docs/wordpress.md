---
title: Công cụ WordPress
description: Kiểm tra và cập nhật các website WordPress sau khi sao lưu database, dùng chế độ bảo trì, chế độ debug, tự động cập nhật, đặt lại mật khẩu và tạo bản staging.
---

# Công cụ WordPress

Công cụ WordPress cho biết phiên bản và các bản cập nhật đang chờ của từng website WordPress, cập nhật chỉ với một lần bấm sau khi sao lưu database, và bật tắt chế độ bảo trì, chế độ debug, tự động cập nhật. Công cụ này còn đặt lại mật khẩu quản trị, tạo bản sao staging và đẩy bản staging lên website chính.

Công cụ áp dụng cho các website WordPress do Nefo cài: từ [Kho ứng dụng](./apps), bằng nút **Cài WordPress** trên một website PHP trống, hoặc là một bản staging. WordPress bạn tự tải lên sẽ không xuất hiện ở đây. Mục **WordPress** ở menu bên trái chỉ hiện khi máy chủ đã có website WordPress.

Công cụ dùng wp-cli 2.12.0, được Nefo tải về ở lần dùng đầu và kiểm tra với checksum đã công bố. wp-cli chạy bằng user Linux riêng của từng website với phiên bản PHP của website đó, và không nạp plugin hay giao diện, nên một plugin lỗi không gây cản trở.

## Cài WordPress

Có hai cách:

- **Kho ứng dụng**: cài WordPress hoặc WooCommerce vào một website PHP trống. Xem [Kho ứng dụng](./apps).
- **Từ một website PHP trống**: mở tab **Tổng quan** của website rồi bấm **Cài WordPress**. Điền **Tên website**, **Tên đăng nhập quản trị** (đừng dùng `admin`: đó là tên kẻ tấn công thử đầu tiên), **Email quản trị** và **Mật khẩu quản trị**, hoặc để trống mật khẩu để panel tạo mật khẩu mạnh. Mật khẩu tự nhập cần 12 đến 128 ký tự, giống như trong kho ứng dụng. Bấm **Cài WordPress**.

WordPress cần MariaDB: hãy cài trước ở trang **Cơ sở dữ liệu** (xem [Cơ sở dữ liệu](./databases)). Nên bật HTTPS trước khi cài để WordPress dùng địa chỉ `https://` ngay từ đầu; nếu bật sau, Nefo sẽ tự chuyển WordPress sang. Hãy lưu thông tin đăng nhập khi nó hiện ra: mật khẩu sẽ không hiện lại.

## Danh sách website

Mở **WordPress** ở menu bên trái. Với mỗi website, bảng hiện:

| Cột | Nội dung |
|---|---|
| **Website** | Tên miền và phiên bản PHP. Bấm vào để mở tab **WordPress** của website. |
| **Phiên bản** | Phiên bản WordPress, kèm "Đã có WordPress …" khi có bản mới, hoặc "Đã mới nhất". |
| **Plugin và giao diện** | Số plugin, giao diện và số cần cập nhật. **Xem … bản cập nhật** liệt kê chúng. |
| **Chế độ** | **Bảo trì**, **Debug** và **Tự cập nhật** khi đang bật. |

Nefo kiểm tra từng website khi bạn mở trang. Nút **Kiểm tra lại** ở trên cùng kiểm tra lại mọi website; mục cùng tên trong menu của một dòng chỉ kiểm tra website đó. Nếu không kiểm tra được, ví dụ vì database không chạy, dòng đó ghi "Không kiểm tra được website:" kèm lý do.

Menu ở cuối mỗi dòng chứa các thao tác bên dưới. Tab **WordPress** của từng website cũng có đủ các thao tác và công tắc này, cùng nút **WP Admin**.

## Cập nhật WordPress, plugin và giao diện

1. Bấm **Cập nhật tất cả** trên dòng của website, hoặc ở tab **WordPress** của website.
2. Xác nhận: "Sao lưu database của example.com, rồi cập nhật WordPress, mọi plugin và mọi giao diện?"

Việc cập nhật chạy nền kèm nhật ký:

1. Nefo xuất database ra `/var/www/example.com/tmp/panel-wp-backup-YYYYMMDD-HHMMSS.sql`. Chỉ user của website đọc được file này, và không ai mở được nó từ web. Nefo giữ ba file mới nhất. Nếu sao lưu lỗi, không có gì được cập nhật.
2. Nefo cập nhật WordPress và database của nó, rồi mọi plugin và mọi giao diện.
3. Website đang ở chế độ bảo trì thì vẫn giữ chế độ bảo trì.

Nếu có plugin hay giao diện không cập nhật được, nhật ký ghi lý do và tên file sao lưu database. Muốn quay lại, hãy tải file sao lưu về từ **Tệp tin** rồi nhập vào ở trang **Cơ sở dữ liệu**: xem [Cơ sở dữ liệu](./databases). Muốn có cả bản sao file, hãy dùng [Sao lưu](./backups).

## Chế độ bảo trì

Chọn **Bật chế độ bảo trì**, hoặc bật công tắc **Chế độ bảo trì** ở tab **WordPress** của website. Khách truy cập sẽ thấy trang bảo trì của WordPress (HTTP 503). Chế độ này giữ nguyên cho tới khi bạn tắt trong panel bằng **Tắt chế độ bảo trì**.

Chế độ bảo trì là trang của chính WordPress. Muốn đóng cả website ở tầng web server, hãy tạm ngưng website: xem [Website](./websites).

## Chế độ debug

Chọn **Bật chế độ debug**, hoặc bật công tắc **Chế độ gỡ lỗi**. Nefo đặt `WP_DEBUG` và `WP_DEBUG_LOG` trong `wp-config.php` và giữ `WP_DEBUG_DISPLAY` tắt, nên lỗi được ghi vào nhật ký và không bao giờ hiện cho khách:

- lỗi của các request web nằm trong `/var/www/example.com/logs/php-error.log`: mở tab **Nhật ký** của website và chọn **Lỗi PHP**;
- lỗi khi chạy wp-cli nằm trong `/var/www/example.com/logs/wp-debug.log`: mở file này trong **Tệp tin**.

Xong việc thì tắt bằng **Tắt chế độ debug**.

## Tự động cập nhật

**Bật tự động cập nhật** cho WordPress tự cập nhật mọi thứ: WordPress kể cả phiên bản lớn, mọi plugin và mọi giao diện.

**Tắt tự động cập nhật** đưa WordPress về mặc định: WordPress vẫn tự cài các bản vá bảo mật và bản nhỏ của chính nó, còn plugin và giao diện không tự cập nhật.

Cài đặt này áp dụng cho các plugin và giao diện đang có lúc bạn bật. Với plugin và giao diện thêm sau, hãy tắt rồi bật lại.

## Đặt lại mật khẩu quản trị

1. Chọn **Đặt lại mật khẩu quản trị**.
2. Chọn tài khoản ở ô **Quản trị viên**. Danh sách gồm các user mang vai trò quản trị.
3. Chọn **Tạo mật khẩu mạnh ngẫu nhiên**, hoặc **Tự nhập mật khẩu** dài 12 đến 128 ký tự.
4. Bấm **Đặt mật khẩu**.

Nefo hiện mật khẩu mới một lần, kèm nút **Đăng nhập**. Tài khoản bị đăng xuất trên mọi thiết bị.

## Tạo bản staging

Bản staging là một website thứ hai chứa bản sao của website, để bạn thử thay đổi mà không đụng tới website thật. Website thật vẫn chạy trong lúc sao chép.

1. Chọn **Nhân bản sang staging**.
2. Kiểm tra **Tên miền của website staging**. Mặc định là `staging.example.com` nếu bạn không đổi.
3. Bấm **Tạo bản staging**.
4. Trỏ DNS của tên miền staging về máy chủ này, rồi bật HTTPS ở tab **SSL / HTTPS** của nó.

Bản staging có:

- một website PHP mới cùng phiên bản PHP, với user Linux riêng;
- một cơ sở dữ liệu mới chứa bản sao nội dung;
- bản sao các file trong `public`, với `wp-config.php` trỏ tới cơ sở dữ liệu mới. Các drop-in cache (`object-cache.php`, `advanced-cache.php`) và liên kết tượng trưng (symlink) không được chép;
- địa chỉ website được thay bằng địa chỉ mới ở mọi nơi trong cơ sở dữ liệu;
- yêu cầu công cụ tìm kiếm không lập chỉ mục;
- cùng quy tắc rewrite với website thật.

Website staging bắt đầu bằng `http://`, chưa có chứng chỉ. Nếu một bước bị lỗi, Nefo xóa mọi thứ đã tạo cho bản staging.

Nefo từ chối tạo bản staging khi:

- MariaDB không chạy;
- tên miền staging đã có website dùng, hoặc thư mục của nó đã có trong `/var/www`;
- `wp-config.php` không nằm trong thư mục `public` của website;
- `DB_NAME` trong `wp-config.php` không phải chuỗi đơn giản trong dấu nháy, hoặc tên cơ sở dữ liệu có ký tự ngoài chữ thường, chữ số và `_`;
- cơ sở dữ liệu của website nằm trên máy chủ khác;
- website là mạng multisite.

::: tip
Ai cũng mở được website staging khi DNS của nó đã trỏ về máy chủ. Muốn giữ kín, hãy khóa bằng mật khẩu ở tab **Chặn truy cập** của nó: xem [Website](./websites).
:::

## Đẩy bản staging lên website chính {#push-the-staging-copy-to-the-live-site}

Khi những thay đổi đã thử trên bản staging đã ổn, hãy chép chúng đè lên website chính. Nefo nhớ mỗi bản staging được tạo từ website nào. Các bản staging tạo trước khi Nefo có nút này, và các [bản nhân bản](./websites#clone-a-website) của một website, không gắn với website chính nào nên không đẩy lên được.

1. Ở hàng của website staging, chọn **Đẩy lên bản chính**, hoặc bấm **Đẩy lên bản chính** ở tab **WordPress** của nó. Danh sách website ghi "Bản staging của example.com" dưới website staging.
2. Chọn nội dung cần đẩy:
   - **File và database**;
   - **Chỉ file (giao diện, plugin, tệp tải lên)**: website chính giữ nguyên bài viết, trang, đơn hàng và cài đặt;
   - **Chỉ database (bài viết, trang, cài đặt)**: website chính giữ nguyên các file.
3. Gõ tên miền của website chính để xác nhận, rồi bấm **Đẩy lên bản chính**.

Việc đẩy chạy như một tác vụ có nhật ký riêng:

1. Nefo sao lưu file và cơ sở dữ liệu của website chính. Bản sao lưu nằm ở trang [Sao lưu](./backups). Nếu sao lưu lỗi, không có gì được đẩy lên.
2. Website chính hiện trang bảo trì. Nếu trước đó website đã ở chế độ bảo trì, nó vẫn ở chế độ bảo trì sau khi đẩy.
3. **File**: các file trong thư mục `public` của website staging thay cho file của website chính, và thuộc user Linux của website chính. File nào bản staging không có sẽ bị xóa khỏi website chính; file cùng kích thước và ngày giờ được giữ nguyên. Website chính giữ `wp-config.php` của riêng nó, các drop-in cache (`object-cache.php`, `advanced-cache.php`) và các liên kết tượng trưng (symlink). Trong `.user.ini` và `wordfence-waf.php`, thư mục staging được thay bằng thư mục của website chính.
4. **Cơ sở dữ liệu**: cơ sở dữ liệu chính được làm trống rồi nhận bản sao cơ sở dữ liệu staging. Sau đó Nefo làm ngược lại những gì bản staging đã làm: địa chỉ staging (cả `http://` lẫn `https://`, cả dạng JSON) và thư mục staging được thay bằng địa chỉ và thư mục chính ở mọi nơi, website chính lấy lại địa chỉ và cài đặt cho công cụ tìm kiếm của nó. Bộ nhớ đệm đối tượng được xóa, và WordPress tạo lại quy tắc rewrite ở lượt truy cập kế tiếp.
5. Chế độ bảo trì được tắt, kể cả khi một bước bị lỗi.

::: warning
Việc đẩy thay thế website chính. Những gì thêm vào website chính kể từ lúc tạo bản staging sẽ mất: đơn hàng, bình luận và người dùng mới khi đẩy cơ sở dữ liệu, tệp tải lên khi đẩy file. Nếu website chính không chạy sau khi đẩy, hãy khôi phục bản sao lưu ở bước 1 trên trang **Sao lưu**.
:::

Nefo từ chối đẩy khi:

- website không phải bản staging, hoặc website chính của nó đã bị xóa;
- bạn đẩy cơ sở dữ liệu mà MariaDB không chạy, cơ sở dữ liệu chính không có trên trang **Cơ sở dữ liệu**, hai website dùng chung một cơ sở dữ liệu, tiền tố bảng (`$table_prefix`) của chúng khác nhau, hoặc một lý do trong danh sách ở trên áp dụng cho `wp-config.php` của một trong hai website.

Cài đặt trong `wp-config.php` không bao giờ được đẩy lên: hãy tự sửa trên website chính.

## Xóa bản staging {#delete-a-staging-copy}

Muốn xóa bản staging, hãy xóa website của nó và giữ dấu chọn **Xóa luôn các database gắn với website này**: xem [Website](./websites).
