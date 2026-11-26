---
title: Những việc đầu tiên
description: Những việc nên làm sau khi cài Nefo, từ bảo vệ tài khoản tới website đầu tiên và sao lưu.
---

# Những việc đầu tiên

Sau khi [cài Nefo](./install), hãy làm lần lượt các bước dưới đây. Mỗi bước đều ngắn và có liên kết tới trang hướng dẫn chi tiết.

## Đăng nhập

1. Mở URL mà trình cài đặt đã in ra, ví dụ `https://203.0.113.10:28443/k3x9p2qa7m1z/`.
2. Chấp nhận cảnh báo chứng chỉ của trình duyệt. Cảnh báo sẽ hết khi panel có chứng chỉ riêng (xem [bên dưới](#give-the-panel-its-own-domain)).
3. Nhập **Tên đăng nhập** `admin` và mật khẩu từ trình cài đặt, rồi bấm **Đăng nhập**.

Nếu bạn làm mất URL hoặc mật khẩu, hãy chạy `nefo info` hoặc `nefo reset-password` trên máy chủ (xem [Dòng lệnh](./cli)).

## Thiết lập máy chủ {#set-up-the-server}

Trên máy chủ mới, ngay sau khi bạn đăng nhập, panel mở trang **Thiết lập máy chủ**. Trên một trang duy nhất, bạn chọn những gì máy chủ cần, và panel cài tất cả trong một lượt, lần lượt từng phần.

1. Chọn **Web server**: **Nginx** (khuyên dùng), **Apache** cho website cần tệp `.htaccess`, **OpenLiteSpeed**, hoặc **Không cài** cho máy chủ không chạy website, ví dụ chỉ chạy ứng dụng Docker.
2. Đánh dấu một hoặc nhiều **phiên bản PHP**. Web server luôn đi kèm phiên bản PHP của hệ điều hành. Các phiên bản khác được lấy từ kho `packages.sury.org/php`, trên cả Ubuntu và Debian, và panel sẽ thêm kho đó vào máy chủ. Các phiên bản cũ đã hết hỗ trợ không có ở đây: nếu một website cũ cần, hãy cài chúng từ trang **Phần mềm**.
3. Chọn **Máy chủ cơ sở dữ liệu**: **MariaDB** (khuyên dùng), **MySQL** nếu trang Phần mềm có, hoặc **Không cài**.
4. Đánh dấu các **Thành phần thêm** bạn cần: Redis, Node.js, Python, Docker, Adminer và tường lửa web. ClamAV chỉ hiện trên máy chủ có từ 2 GB RAM, còn PostgreSQL và Memcached hiện khi trang Phần mềm có chúng.
5. Xem phần tóm tắt ở cuối trang. Phần này cho biết số CPU, RAM và dung lượng ổ đĩa còn trống của máy chủ, cùng lượng RAM và ổ đĩa ước tính mà lựa chọn của bạn tốn thêm. Nếu có thể không đủ, trang sẽ cảnh báo, nhưng bạn vẫn cài được.
6. Bấm **Cài đặt**.

Trang chọn sẵn một bộ hợp với RAM của máy chủ: Nginx, phiên bản PHP mới nhất và MariaDB, thêm Redis nếu máy có từ khoảng 1,5 GB RAM. Docker và các thành phần thêm khác không bao giờ được chọn sẵn.

Nhật ký hiện từng phần khi đang cài, và toàn bộ việc thiết lập là một tác vụ. Nếu một phần bị lỗi, quá trình dừng ở đó và cho biết những gì đã cài xong. Hãy khắc phục lỗi, rồi bấm **Cài đặt** lần nữa: các phần đã cài sẽ được bỏ qua.

Nếu muốn tự thiết lập máy chủ, bấm **Để sau**. Bạn có thể chạy lại bước thiết lập bất cứ lúc nào từ **Phần mềm** → **Chạy lại thiết lập máy chủ**, hoặc bằng lệnh [`nefo setup`](./cli#nefo-setup) trên máy chủ. Bạn cũng có thể [thiết lập máy chủ ngay khi cài Nefo](./install#set-the-server-up-in-the-same-run).

Panel thôi mở trang thiết lập sau khi đăng nhập khi việc thiết lập đã xong hoặc đã được bỏ qua, và trên máy chủ đã có web server hoặc website.

## Đổi mật khẩu

Hãy thay mật khẩu do trình cài đặt tạo bằng một mật khẩu bạn lưu trong trình quản lý mật khẩu.

1. Ở menu bên trái, mở **Tài khoản**. Bạn cũng có thể mở từ menu tài khoản ở góc trên bên phải: **Mật khẩu và 2FA**.
2. Trong phần **Đổi mật khẩu**, điền **Mật khẩu hiện tại**, **Mật khẩu mới** và **Nhập lại mật khẩu mới**. Mật khẩu mới phải dài từ 12 đến 72 ký tự.
3. Bấm **Đổi mật khẩu**.

Panel sẽ đăng xuất bạn trên mọi thiết bị. Hãy đăng nhập lại bằng mật khẩu mới.

## Bật xác thực hai lớp

Khi bật xác thực hai lớp, mỗi lần đăng nhập cần mật khẩu và một mã 6 số từ ứng dụng xác thực như Google Authenticator, Authy hoặc 2FAS. Khi chưa bật, trang chủ và menu thông báo sẽ hiện "Chưa bật xác thực hai lớp".

1. Mở **Tài khoản**.
2. Trong phần **Xác thực hai lớp**, bấm **Thiết lập**.
3. Quét mã QR bằng ứng dụng xác thực, hoặc nhập **Khóa thiết lập** bằng tay.
4. Nhập mã 6 số mà ứng dụng hiển thị, rồi bấm **Bật**.

Từ giờ, sau khi nhập mật khẩu, trang đăng nhập sẽ hỏi thêm **Mã xác thực hai lớp**.

::: warning Luôn giữ một đường lui
Nếu mất điện thoại, bạn chỉ tắt được xác thực hai lớp từ máy chủ, bằng `nefo reset-password --disable-2fa` (xem [Xử lý sự cố](./troubleshooting#lost-your-2fa-phone)). Hãy giữ quyền truy cập SSH, hoặc cất khóa thiết lập ở nơi an toàn.
:::

## Cài web server và PHP

Website cần có web server. Nefo chạy một web server cho mỗi máy chủ. Nếu bước [thiết lập máy chủ](#set-up-the-server) đã cài web server, hãy bỏ qua phần này.

1. Mở **Website**. Trên máy chủ mới, trang này hiện **Cài web server**.
2. Chọn một web server:
   - **Nginx**: nhanh và nhẹ, là lựa chọn tốt nhất nếu website của bạn không cần tệp `.htaccess`.
   - **Apache**: đọc được tệp `.htaccess`, thứ mà nhiều plugin WordPress và website chuyển từ shared hosting cần đến. Tốn RAM hơn một chút.
   - **OpenLiteSpeed**: đọc được quy tắc rewrite trong tệp `.htaccess` và có sẵn LiteSpeed Cache. Được cài từ kho phần mềm riêng của LiteSpeed.
3. Bấm nút cài, ví dụ **Cài Nginx**.

Panel sẽ cài web server cùng phiên bản PHP của hệ điều hành, và mở cổng 80, 443 nếu tường lửa (ufw) đang bật. Việc này mất khoảng một đến ba phút, tiến trình hiện ngay trên trang.

Muốn đổi sang web server khác về sau, bạn phải xóa hết website trước. Các phiên bản PHP khác, MariaDB và những thành phần còn lại được cài ở trang **Phần mềm**. Xem [Phần mềm](./software).

## Đặt tên miền riêng cho panel {#give-the-panel-its-own-domain}

Khi có tên miền riêng, panel được cấp chứng chỉ Let's Encrypt miễn phí và trình duyệt hết cảnh báo. Bạn cần cài web server trước, vì web server trả lời Let's Encrypt ở cổng 80.

1. Ở nhà cung cấp DNS, tạo bản ghi A cho một tên như `panel.example.com`, trỏ về địa chỉ IP của máy chủ. Bản ghi phải trỏ thẳng về máy chủ, không qua proxy hay CDN (với Cloudflare, chọn "DNS only").
2. Đảm bảo cổng 80 đang mở, kể cả trên tường lửa cloud của nhà cung cấp.
3. Trong panel, mở **Cài đặt → Chứng chỉ**.
4. Ở ô **Tên miền của panel**, nhập `panel.example.com` rồi bấm **Xin chứng chỉ**.

Panel kiểm tra tên miền có trỏ về máy chủ này không, rồi mới xin chứng chỉ từ Let's Encrypt. Trong vòng một phút, panel dùng chứng chỉ mới và hiện địa chỉ mới, ví dụ `https://panel.example.com:28443/k3x9p2qa7m1z/`. Từ giờ hãy dùng địa chỉ đó: cổng và đường dẫn bí mật vẫn giữ nguyên. Nếu vào bằng địa chỉ IP, trình duyệt sẽ lại cảnh báo, vì chứng chỉ được cấp cho tên miền.

Chứng chỉ tự động gia hạn. Muốn quay lại dùng chứng chỉ tự ký, bấm **Xóa** trên cùng trang đó.

Nếu không xin được chứng chỉ, xem [Xử lý sự cố](./troubleshooting#a-certificate-request-fails).

## Tạo website đầu tiên

1. Trỏ bản ghi A của tên miền, ví dụ `example.com`, về địa chỉ IP của máy chủ.
2. Mở **Website** rồi bấm **Thêm website**.
3. Nhập **Tên miền**, không gõ `http://` hay `www.`. Giữ dấu chọn ở **Nhận cả www.example.com** nếu bạn muốn dùng cả hai tên.
4. Chọn **Loại**: một phiên bản PHP, **Tĩnh (chỉ HTML)**, hoặc ứng dụng (Node.js hay Python).
5. Bấm **Tạo website**.
6. Bấm **Bật HTTPS** bên cạnh website để nhận chứng chỉ miễn phí.

Tải tệp của bạn lên `/var/www/example.com/public`, bằng trang **Tệp tin** hoặc qua SFTP. Muốn cài WordPress hay ứng dụng khác, hãy dùng **Kho ứng dụng**. Xem [Website](./websites), [Chứng chỉ SSL](./ssl) và [Kho ứng dụng](./apps).

## Thiết lập sao lưu

1. Mở **Sao lưu**.
2. Trong phần **Sao lưu tự động**, bấm **Thêm lịch**.
3. Chọn thứ cần sao lưu (website, database hoặc thư mục), tần suất, và **Số bản sao lưu giữ lại**.
4. Bấm **Lưu lịch**.

Bản sao lưu website gồm toàn bộ tệp và các database gắn với website đó. Bản sao lưu được giữ trên máy chủ, trong `/var/backups/panel`. Hãy thêm **Kho lưu trữ từ xa** (tương thích S3 hoặc SFTP) để vẫn còn một bản nếu máy chủ gặp sự cố. Xem [Sao lưu](./backups).

## Bật tường lửa

Nếu trang chủ báo "Tường lửa đang tắt", mọi cổng do chương trình mở ra đều truy cập được từ Internet.

1. Mở **Tường lửa & SSH**.
2. Bấm **Bật tường lửa**.

Panel vẫn giữ mở SSH và cổng của chính panel. Nếu trang báo máy chủ chưa cài ufw, hãy cài trước bằng `apt install ufw`. Xem [Bảo mật](./security).
