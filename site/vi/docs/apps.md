---
title: Kho ứng dụng
description: Cài ứng dụng PHP vào website, ứng dụng Docker có website riêng và các dịch vụ cơ sở dữ liệu, rồi quản lý, cập nhật, sao lưu và gỡ chúng.
---

# Kho ứng dụng

Kho ứng dụng cài các ứng dụng có sẵn chỉ với một lần bấm. Nefo tạo những gì ứng dụng cần, như cơ sở dữ liệu, website, mật khẩu và container, và kiểm tra RAM, ổ đĩa của máy chủ trước. Trang này nói về các loại ứng dụng, danh mục, cách cài, và cách quản lý ứng dụng đã cài.

Mở **Kho ứng dụng** ở menu bên trái. Tab **Kho ứng dụng** liệt kê các ứng dụng, tab **Đã cài** liệt kê ứng dụng trên máy chủ này. Ở trang **Website**, nút **Cài từ App store** cũng dẫn tới đây.

## Ba loại ứng dụng

| Loại | Được cài vào đâu | Có gì |
|---|---|---|
| **Ứng dụng PHP** | Vào một website PHP trống do bạn tạo | Cơ sở dữ liệu riêng, file nằm trong website, đúng quy tắc rewrite. Chạy bằng user Linux của website. |
| **Ứng dụng Docker** | Vào một website mới Nefo tạo riêng cho nó | Container chỉ nghe trên `127.0.0.1`, có website đứng trước. |
| **Dịch vụ** | Không có website | Container chỉ nghe trên `127.0.0.1`, cho website và ứng dụng của bạn. Mỗi dịch vụ chỉ cài một lần trên mỗi máy chủ. |

Ứng dụng PHP và ứng dụng Docker cần web server đã được cài: xem [Website](./websites). Ứng dụng Docker và dịch vụ còn cần Docker. Hãy cài Docker ở trang **Phần mềm**; nút **Cài Docker** trong kho ứng dụng mở trang đó. Xem [Docker](./docker).

## Danh mục

Ở phiên bản danh mục 2026112501, kho có các ứng dụng sau. RAM tối thiểu là tổng RAM của máy chủ mà dưới mức đó Nefo từ chối cài (swap không được tính).

| Ứng dụng | Loại | Là gì | RAM tối thiểu |
|---|---|---|---|
| WordPress | Ứng dụng PHP | Tạo website và blog | 450 MB |
| WooCommerce | Ứng dụng PHP | WordPress kèm WooCommerce: cửa hàng trực tuyến | 900 MB |
| Joomla | Ứng dụng PHP | CMS cho website và cổng thông tin | 450 MB |
| Nextcloud | Ứng dụng PHP | Đám mây riêng cho tệp, lịch và danh bạ | 900 MB |
| Drupal | Ứng dụng PHP | CMS cho nội dung có cấu trúc, dựng bằng Composer | 900 MB |
| Laravel | Ứng dụng PHP | Dự án Laravel mới kèm cơ sở dữ liệu | 450 MB |
| OpenCart | Ứng dụng PHP | Giỏ hàng trực tuyến | 900 MB |
| Moodle | Ứng dụng PHP | Nền tảng học tập | 900 MB |
| Ghost | Ứng dụng Docker | Blog và bản tin | 900 MB |
| Uptime Kuma | Ứng dụng Docker | Theo dõi website và dịch vụ | 450 MB |
| n8n | Ứng dụng Docker | Tự động hóa quy trình | 900 MB |
| Gitea | Ứng dụng Docker | Máy chủ Git riêng | 450 MB |
| Open WebUI | Ứng dụng Docker | Giao diện trò chuyện cho Ollama hoặc API tương thích OpenAI | 1800 MB |
| Vaultwarden | Ứng dụng Docker | Máy chủ quản lý mật khẩu cho ứng dụng Bitwarden | 450 MB |
| RustFS | Ứng dụng Docker | Lưu trữ đối tượng tương thích S3 | 900 MB |
| FileBrowser Quantum | Ứng dụng Docker | Quản lý tệp trên trình duyệt | 450 MB |
| code-server | Ứng dụng Docker | VS Code trên trình duyệt | 900 MB |
| Stirling-PDF | Ứng dụng Docker | Công cụ xử lý PDF | 1800 MB |
| IT-Tools | Ứng dụng Docker | Công cụ cho lập trình viên trên trình duyệt | 230 MB |
| Directus | Ứng dụng Docker | Headless CMS | 900 MB |
| Wiki.js | Ứng dụng Docker | Wiki | 900 MB |
| Umami | Ứng dụng Docker | Thống kê truy cập tôn trọng quyền riêng tư | 900 MB |
| AdGuard Home | Ứng dụng Docker | Chặn quảng cáo qua địa chỉ DNS-over-HTTPS riêng | 450 MB |
| Odoo | Ứng dụng Docker | ERP (bản Community) | 1800 MB |
| PostgreSQL | Dịch vụ | Máy chủ cơ sở dữ liệu PostgreSQL | 900 MB |
| MongoDB | Dịch vụ | Cơ sở dữ liệu tài liệu MongoDB | 900 MB |
| Ollama | Dịch vụ | Chạy mô hình ngôn ngữ mở bằng CPU | 3584 MB |

Danh mục có sẵn trong panel. Mỗi ngày một lần, Nefo cũng kiểm tra danh mục mới hơn được công bố cùng các bản phát hành, và chỉ dùng nó khi chữ ký hợp lệ. **Cập nhật danh mục** kiểm tra ngay. Phần đầu trang ghi phiên bản danh mục và cho biết nó "có sẵn trong panel" hay "tải về, đã kiểm tra chữ ký".

Mỗi thẻ cho biết phiên bản, loại, RAM và dung lượng ổ đĩa ứng dụng cần. Nút **Cài** mở phần mô tả, giấy phép, tài nguyên cần có, ghi chú và form cài đặt. Thẻ của ứng dụng mà máy chủ không chạy được sẽ ghi lý do, và có nút **Chi tiết** thay cho **Cài**. Dịch vụ đã cài có nút **Quản lý**. Nhãn "Giấy phép hạn chế" đánh dấu giấy phép không được OSI công nhận là mã nguồn mở.

## Kiểm tra trước khi cài

Khi bạn mở một ứng dụng, Nefo so nhu cầu của nó với máy chủ. Nefo từ chối cài khi:

- tổng RAM của máy chủ thấp hơn mức tối thiểu của ứng dụng;
- dung lượng ổ đĩa còn trống ít hơn image và dữ liệu của ứng dụng cộng thêm 1 GB;
- kiến trúc hoặc tập lệnh của bộ xử lý không phù hợp với ứng dụng.

Nefo cảnh báo nhưng vẫn cho cài khi máy chủ ít RAM hơn mức nên có, khi RAM đang trống ít hơn mức ứng dụng có thể dùng, hoặc khi tổng giới hạn bộ nhớ của mọi ứng dụng đã cài vượt quá RAM của máy chủ.

## Cài ứng dụng PHP

1. Tạo một website PHP trống với phiên bản PHP mà ứng dụng hỗ trợ: xem [Website](./websites). Phiên bản PHP tối thiểu: WordPress 7.4, Nextcloud và Laravel 8.2, WooCommerce, Joomla, Drupal và Moodle 8.3, OpenCart 8.0. OpenCart hỗ trợ tới PHP 8.3, Moodle tới PHP 8.4.
2. Đảm bảo MariaDB đã cài và đang chạy: xem [Cơ sở dữ liệu](./databases).
3. Trong kho ứng dụng, bấm nút **Cài** của ứng dụng.
4. Chọn **Website**. Danh sách chỉ có các website PHP trống với phiên bản PHP phù hợp. Nếu chưa có, hãy dùng **Tạo website PHP**.
5. Với ứng dụng có hỏi, điền **Tên site**, **Quản trị viên** (đừng dùng `admin`: đó là tên kẻ tấn công thử đầu tiên), **Email quản trị** và **Mật khẩu quản trị**. Để trống mật khẩu để tạo mật khẩu mạnh ngẫu nhiên; mật khẩu tự nhập cần 12 đến 128 ký tự. WooCommerce còn hỏi quốc gia của cửa hàng và đơn vị tiền tệ.
6. Bấm **Cài**. Quá trình cài chạy nền và hiện trong danh sách tác vụ.

Khi cài xong, Nefo hiện **Địa chỉ đăng nhập**, **Quản trị viên** và **Mật khẩu quản trị** một lần. Hãy lưu lại ngay.

Thư mục `public` của website phải trống; trang chờ của panel thì không sao. Drupal và Laravel được dựng bằng Composer, còn Moodle cũng cần cả thư mục website: website của chúng phải mới và trống, thư mục website trở thành project và `public` là thư mục web.

Ứng dụng PHP có:

- một cơ sở dữ liệu và user cơ sở dữ liệu mới, gắn với website;
- các file của ứng dụng, được tải về và kiểm tra với checksum trong danh mục;
- quy tắc rewrite của ứng dụng, và giới hạn PHP cao hơn khi ứng dụng cần;
- một cron job chạy bằng user của website cho Moodle (mỗi phút) và WooCommerce (mỗi năm phút).

Nếu cài lỗi, Nefo dọn trống website và xóa cơ sở dữ liệu vừa tạo. Website WordPress cũng xuất hiện trong [Công cụ WordPress](./wordpress).

## Cài ứng dụng Docker

1. Trỏ DNS của tên miền dành cho ứng dụng về IP của máy chủ, ví dụ `app.example.com` về `203.0.113.10`.
2. Trong kho ứng dụng, bấm nút **Cài** của ứng dụng.
3. Nhập **Tên miền của website mới** và, nếu muốn, **Tên miền khác**.
4. Một số ứng dụng còn hỏi **Email quản trị**. Hãy dùng tên miền thật, không dùng `.test` hay `.local`.
5. Bấm **Cài**.
6. Khi cài xong, bật HTTPS ở tab **SSL / HTTPS** của website mới: xem [Chứng chỉ SSL](./ssl). Ứng dụng khởi động lại với địa chỉ `https://`. Một số ứng dụng cần HTTPS mới chạy đầy đủ, và form cài đặt có ghi rõ.

Cách bạn có tài khoản quản trị đầu tiên tùy theo ứng dụng, và form cài đặt sẽ cho biết:

| Cách | Ứng dụng | Bạn làm gì |
|---|---|---|
| Panel tự tạo | RustFS, code-server, AdGuard Home, Odoo, Umami | Mở **Thông tin đăng nhập** ở tab **Đã cài**. |
| Email của bạn và một mật khẩu ngẫu nhiên | Open WebUI, Directus, Wiki.js | Mở **Thông tin đăng nhập**. |
| Tài khoản đăng ký đầu tiên | Ghost, Uptime Kuma, n8n, Gitea | Đăng ký ngay sau khi cài: tài khoản đầu tiên sẽ là quản trị viên. |
| Lời mời | Vaultwarden | Đăng ký trên trang của ứng dụng bằng đúng email mà panel đã mời. |
| Mật khẩu dùng một lần | FileBrowser Quantum | Mở **Thông tin đăng nhập**; ứng dụng yêu cầu đặt mật khẩu mới ở lần đăng nhập đầu. |
| Mật khẩu của website | Stirling-PDF | Website hỏi user `admin` và mật khẩu trong **Thông tin đăng nhập**. |
| Không có đăng nhập | IT-Tools | Không cần làm gì. |

Ứng dụng Docker có:

- một website mới chuyển tiếp tới ứng dụng. Website tạm đóng (trả về 503) cho tới khi cài xong và các tài khoản mặc định của ứng dụng đã bị khóa;
- các container chạy dưới dạng một dự án Docker Compose, có giới hạn bộ nhớ và log được xoay vòng ở mức 10 MB;
- các file nằm trong `/var/lib/panel-apps/<tên>`, chỉ root đọc được: file Compose, các mật khẩu được tạo và dữ liệu của container;
- các cổng chỉ mở trên `127.0.0.1`, không bao giờ mở ra Internet;
- image được ghim đúng một phiên bản.

## Cài dịch vụ

PostgreSQL, MongoDB và Ollama không có website. Bấm **Cài**: không cần điền gì. Nefo tự chọn một cổng trống trên `127.0.0.1` và tạo mật khẩu. Các ứng dụng trên máy chủ này dùng được dịch vụ, còn từ Internet thì không. Mỗi dịch vụ chỉ cài được một lần trên mỗi máy chủ.

Mở **Thông tin đăng nhập** ở tab **Đã cài** để xem địa chỉ, user, mật khẩu và **Chuỗi kết nối**.

## Kiểm tra an toàn với ứng dụng Docker

Công thức (recipe) của ứng dụng Docker trong danh mục là dữ liệu, không bao giờ là script. Nefo chuyển nó thành file Compose và kiểm tra file đó bằng đúng các quy tắc áp dụng cho dự án Compose của bạn, không có ngoại lệ:

- không container đặc quyền (privileged), không cấp thêm đặc quyền (capability), không giao thiết bị của máy chủ;
- không dùng chung namespace mạng, tiến trình hay namespace khác của máy chủ;
- không nới lỏng giới hạn bảo mật, không đặt tham số nhân (sysctl);
- cổng chỉ mở trên `127.0.0.1`;
- không gắn thư mục hệ thống hay socket Docker vào container.

Xem [Docker](./docker) về các quy tắc này. Mỗi lần cài đều có mật khẩu mới được tạo, và mật khẩu ban đầu bị xóa khỏi cấu hình container sau lần khởi động đầu. Danh mục tải về chỉ được dùng khi chữ ký hợp lệ.

## Quản lý ứng dụng đã cài

Tab **Đã cài** liệt kê mọi ứng dụng đã cài. Lọc bằng **Tất cả**, **Có website** hoặc **Dịch vụ**, hoặc tìm kiếm. Các cột là **Ứng dụng**, **Phiên bản**, **Trạng thái** và **Bộ nhớ** (lượng RAM container đang dùng).

Trạng thái là một trong **Đang chạy**, **Đã dừng**, **Chạy một phần**, **Không có container**, **Đang cài** hoặc **Cài lỗi**. Ứng dụng PHP hiện **Trong website**, hoặc **Website tạm ngưng**.

Menu của mỗi dòng có các thao tác sau:

| Thao tác | Dùng cho | Làm gì |
|---|---|---|
| **Mở** | Ứng dụng có website | Mở website của ứng dụng. |
| **Thông tin đăng nhập** | Ứng dụng Docker và dịch vụ | Hiện thông tin đăng nhập. Mỗi lần mở đều được ghi vào nhật ký hoạt động. |
| **Log** | Ứng dụng Docker và dịch vụ | Những dòng mới nhất trong log của container. |
| **Bật**, **Dừng**, **Khởi động lại** | Ứng dụng Docker và dịch vụ | Chạy, dừng hoặc khởi động lại container. |
| **Cập nhật** | Ứng dụng Docker và dịch vụ, khi danh mục có bản mới | Xem bên dưới. |
| **Sao lưu ngay** | Ứng dụng Docker và dịch vụ | Sao lưu ứng dụng. Bản sao nằm ở trang **Sao lưu**: xem [Sao lưu](./backups). |
| **Cài đặt website** | Ứng dụng có website | Mở cài đặt của website. |
| **Gỡ cài đặt** | Dịch vụ | Gỡ dịch vụ: xem bên dưới. |
| **Xóa cùng website** | Ứng dụng có website | Mở website, nơi bạn xóa nó. |

### Cập nhật ứng dụng

Khi danh mục có bản mới hơn của một ứng dụng Docker hay dịch vụ, cột **Phiên bản** sẽ ghi, ví dụ "Có bản 2.6.0". Bấm **Cập nhật** rồi xác nhận. Nefo:

1. sao lưu ứng dụng trước. Nếu sao lưu lỗi, không có gì thay đổi;
2. tải image mới về trước khi dừng bất cứ thứ gì;
3. khởi động lại ứng dụng với image mới và kiểm tra ứng dụng có phản hồi không. Nếu không, bản cũ sẽ chạy lại.

Một số ứng dụng không cho cập nhật một lần bấm khi đổi phiên bản chính: "Bản cập nhật này (…) đổi phiên bản chính và cần làm thủ công. Hãy xem hướng dẫn nâng cấp của ứng dụng." Điều này áp dụng cho PostgreSQL, Wiki.js và Odoo, và cho MongoDB khi đổi phiên bản phụ.

Ứng dụng PHP không cập nhật ở đây: "Hãy cập nhật trong trang quản trị của ứng dụng." Với WordPress, hãy dùng [Công cụ WordPress](./wordpress).

Ứng dụng được cài trước khi có kho ứng dụng hiện nhãn **Bản cài cũ**. **Chuyển đổi và cập nhật** chuyển dữ liệu của nó sang thư mục của kho ứng dụng, chạy bằng Docker Compose rồi cập nhật, sau khi đã sao lưu.

## Gỡ ứng dụng

- **Dịch vụ:** bấm **Gỡ cài đặt**, gõ tên của nó rồi xác nhận. Container, dữ liệu và mật khẩu của dịch vụ bị xóa. Các bản sao lưu vẫn được giữ.
- **Ứng dụng có website:** hãy xóa website: xem [Website](./websites). Với ứng dụng Docker, việc này xóa container, thư mục của nó trong `/var/lib/panel-apps` và các image không còn ứng dụng nào dùng. Với ứng dụng PHP, giữ dấu chọn **Xóa luôn các database gắn với website này** để xóa cả cơ sở dữ liệu của nó.

::: danger
Gỡ ứng dụng sẽ xóa vĩnh viễn dữ liệu của nó. Hãy sao lưu trước bằng **Sao lưu ngay** nếu bạn có thể cần lại.
:::
