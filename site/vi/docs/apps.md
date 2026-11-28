---
title: Kho ứng dụng
description: Cài ứng dụng PHP vào website, ứng dụng Docker có website riêng và các dịch vụ cơ sở dữ liệu, rồi quản lý, cập nhật, sao lưu và gỡ chúng.
---

# Kho ứng dụng

Kho ứng dụng của Nefo cài 42 ứng dụng chỉ với một lần bấm: ứng dụng PHP như WordPress, WooCommerce, Joomla, Nextcloud và Moodle vào một website, ứng dụng Docker như Ghost, n8n, Gitea, Grafana và Jellyfin sau một website riêng của chúng, và PostgreSQL, MongoDB và Ollama dưới dạng dịch vụ trên 127.0.0.1. Kho kiểm tra RAM và ổ đĩa của máy chủ trước khi cài.

Nefo tạo những gì mỗi ứng dụng cần, như cơ sở dữ liệu, website, mật khẩu và container. Trang này nói về các loại ứng dụng, danh mục, cách cài, và cách quản lý ứng dụng đã cài.

Mở **Kho ứng dụng** ở menu bên trái. Tab **Kho ứng dụng** liệt kê các ứng dụng, tab **Đã cài** liệt kê ứng dụng trên máy chủ này. Ở trang **Website**, nút **Cài từ App store** cũng dẫn tới đây.

## Ba loại ứng dụng {#three-kinds-of-apps}

| Loại | Được cài vào đâu | Có gì |
|---|---|---|
| **Ứng dụng PHP** | Vào một website PHP trống do bạn tạo | Cơ sở dữ liệu riêng, file nằm trong website, đúng quy tắc rewrite. Chạy bằng user Linux của website. |
| **Ứng dụng Docker** | Vào một website mới Nefo tạo riêng cho nó | Container chỉ nghe trên `127.0.0.1`, có website đứng trước. |
| **Dịch vụ** | Không có website | Container chỉ nghe trên `127.0.0.1`, cho website và ứng dụng của bạn. Mỗi dịch vụ chỉ cài một lần trên mỗi máy chủ. |

Ứng dụng PHP và ứng dụng Docker cần web server đã được cài: xem [Website](./websites). Ứng dụng Docker và dịch vụ còn cần Docker. Hãy cài Docker ở trang **Phần mềm**; nút **Cài Docker** trong kho ứng dụng mở trang đó. Xem [Docker](./docker).

## Tôi cài được những ứng dụng nào? {#the-catalog}

Ở phiên bản danh mục 2026112701, kho có 42 ứng dụng, chia theo nhóm: **Tất cả** hiện mọi nhóm cùng ứng dụng của chúng, còn nút của một nhóm chỉ hiện ứng dụng của nhóm đó. Ô tìm kiếm tìm trong tên, mô tả và tên nhóm, và bỏ qua dấu tiếng Việt (gõ "giam sat" sẽ ra các ứng dụng "Giám sát & thống kê").

RAM tối thiểu là tổng RAM của máy chủ mà dưới mức đó Nefo từ chối cài (swap không được tính).

| Ứng dụng | Loại | Nhóm | Là gì | RAM tối thiểu |
|---|---|---|---|---|
| WordPress | Ứng dụng PHP | Website & nội dung | Tạo website và blog | 450 MB |
| Joomla | Ứng dụng PHP | Website & nội dung | CMS cho website và cổng thông tin | 450 MB |
| Drupal | Ứng dụng PHP | Website & nội dung | CMS cho nội dung có cấu trúc, dựng bằng Composer | 900 MB |
| Ghost | Ứng dụng Docker | Website & nội dung | Blog và bản tin | 900 MB |
| Directus | Ứng dụng Docker | Website & nội dung | Headless CMS | 900 MB |
| Wiki.js | Ứng dụng Docker | Website & nội dung | Wiki | 900 MB |
| WooCommerce | Ứng dụng PHP | Bán hàng & doanh nghiệp | WordPress kèm WooCommerce: cửa hàng trực tuyến | 900 MB |
| OpenCart | Ứng dụng PHP | Bán hàng & doanh nghiệp | Giỏ hàng trực tuyến | 900 MB |
| Odoo | Ứng dụng Docker | Bán hàng & doanh nghiệp | ERP (bản Community) | 1800 MB |
| Stirling-PDF | Ứng dụng Docker | Công việc & tiện ích | Công cụ xử lý PDF | 1800 MB |
| Memos | Ứng dụng Docker | Công việc & tiện ích | Ghi chú nhanh bằng Markdown theo dòng thời gian | 230 MB |
| linkding | Ứng dụng Docker | Công việc & tiện ích | Quản lý bookmark | 450 MB |
| Vikunja | Ứng dụng Docker | Công việc & tiện ích | Danh sách việc cần làm, bảng Kanban và dự án | 450 MB |
| Actual Budget | Ứng dụng Docker | Công việc & tiện ích | Quản lý chi tiêu cá nhân | 450 MB |
| Kanboard | Ứng dụng Docker | Công việc & tiện ích | Bảng Kanban cho dự án | 230 MB |
| FreshRSS | Ứng dụng Docker | Công việc & tiện ích | Đọc tin từ báo và blog (RSS) | 450 MB |
| Mealie | Ứng dụng Docker | Công việc & tiện ích | Công thức nấu ăn, thực đơn và danh sách đi chợ | 900 MB |
| Nextcloud | Ứng dụng PHP | File & lưu trữ | Đám mây riêng cho tệp, lịch và danh bạ | 900 MB |
| RustFS | Ứng dụng Docker | File & lưu trữ | Lưu trữ đối tượng tương thích S3 | 900 MB |
| FileBrowser Quantum | Ứng dụng Docker | File & lưu trữ | Quản lý tệp trên trình duyệt | 450 MB |
| Navidrome | Ứng dụng Docker | Phim & nhạc | Nghe nhạc từ kho nhạc của riêng bạn | 450 MB |
| Jellyfin | Ứng dụng Docker | Phim & nhạc | Phim, phim bộ và nhạc từ máy chủ của bạn | 1800 MB |
| ntfy | Ứng dụng Docker | Thông báo & cộng đồng | Thông báo đẩy qua HTTP | 230 MB |
| Gotify | Ứng dụng Docker | Thông báo & cộng đồng | Máy chủ thông báo có ứng dụng Android | 230 MB |
| listmonk | Ứng dụng Docker | Thông báo & cộng đồng | Bản tin và danh sách email | 450 MB |
| Uptime Kuma | Ứng dụng Docker | Giám sát & thống kê | Theo dõi website và dịch vụ | 450 MB |
| Umami | Ứng dụng Docker | Giám sát & thống kê | Thống kê truy cập tôn trọng quyền riêng tư | 900 MB |
| Grafana | Ứng dụng Docker | Giám sát & thống kê | Bảng biểu đồ và cảnh báo | 900 MB |
| changedetection.io | Ứng dụng Docker | Giám sát & thống kê | Theo dõi thay đổi trên trang web | 900 MB |
| Healthchecks | Ứng dụng Docker | Giám sát & thống kê | Cảnh báo khi cron job ngừng chạy | 450 MB |
| Vaultwarden | Ứng dụng Docker | Bảo mật & mạng | Máy chủ quản lý mật khẩu cho ứng dụng Bitwarden | 450 MB |
| AdGuard Home | Ứng dụng Docker | Bảo mật & mạng | Chặn quảng cáo qua địa chỉ DNS-over-HTTPS riêng | 450 MB |
| Open WebUI | Ứng dụng Docker | AI | Giao diện trò chuyện cho Ollama hoặc API tương thích OpenAI | 1800 MB |
| Ollama | Dịch vụ | AI | Chạy mô hình ngôn ngữ mở bằng CPU | 3584 MB |
| Laravel | Ứng dụng PHP | Lập trình & tự động hóa | Dự án Laravel mới kèm cơ sở dữ liệu | 450 MB |
| code-server | Ứng dụng Docker | Lập trình & tự động hóa | VS Code trên trình duyệt | 900 MB |
| IT-Tools | Ứng dụng Docker | Lập trình & tự động hóa | Công cụ cho lập trình viên trên trình duyệt | 230 MB |
| Gitea | Ứng dụng Docker | Lập trình & tự động hóa | Máy chủ Git riêng | 450 MB |
| n8n | Ứng dụng Docker | Lập trình & tự động hóa | Tự động hóa quy trình | 900 MB |
| PostgreSQL | Dịch vụ | Cơ sở dữ liệu | Máy chủ cơ sở dữ liệu PostgreSQL | 900 MB |
| MongoDB | Dịch vụ | Cơ sở dữ liệu | Cơ sở dữ liệu tài liệu MongoDB | 900 MB |
| Moodle | Ứng dụng PHP | Giáo dục | Nền tảng học tập | 900 MB |

Danh mục có sẵn trong panel. Mỗi ngày một lần, Nefo cũng kiểm tra danh mục mới hơn được công bố cùng các bản phát hành, và chỉ dùng nó khi chữ ký hợp lệ. **Cập nhật danh mục** kiểm tra ngay. Phần đầu trang ghi phiên bản danh mục và cho biết nó "có sẵn trong panel" hay "tải về, đã kiểm tra chữ ký".

Danh mục tải về có thể chứa ứng dụng cần phiên bản Nefo mới hơn, vì công thức của chúng dùng thứ mà phiên bản này chưa có. Nefo bỏ các ứng dụng đó ra và giữ phần còn lại của danh mục. Nếu đã biết ứng dụng từ trước, Nefo giữ phiên bản nó đang biết, nên các bản đã cài vẫn chạy bình thường. Khi đó phần đầu trang ghi, ví dụ, "2 ứng dụng trong catalog mới tải cần bản panel mới hơn". Hãy cập nhật Nefo để có các ứng dụng này: xem [Cập nhật](./update). Ứng dụng mà danh mục mới hơn thêm vào hiện tên và mô tả bằng mọi ngôn ngữ mà danh mục có.

Mỗi thẻ cho biết phiên bản, loại, RAM và dung lượng ổ đĩa ứng dụng cần. Nút **Cài** mở phần mô tả, giấy phép, tài nguyên cần có, ghi chú và form cài đặt. Thẻ của ứng dụng mà máy chủ không chạy được sẽ ghi lý do, và có nút **Chi tiết** thay cho **Cài**. Dịch vụ đã cài có nút **Quản lý**. Nhãn "Giấy phép hạn chế" đánh dấu giấy phép không được OSI công nhận là mã nguồn mở.

## Kiểm tra trước khi cài {#the-checks-before-an-install}

Khi bạn mở một ứng dụng, Nefo so nhu cầu của nó với máy chủ. Nefo từ chối cài khi:

- tổng RAM của máy chủ thấp hơn mức tối thiểu của ứng dụng (trên VPS là container, giới hạn bộ nhớ của container được tính);
- dung lượng ổ đĩa còn trống ít hơn image và dữ liệu của ứng dụng cộng thêm 1 GB;
- kiến trúc hoặc tập lệnh của bộ xử lý không phù hợp với ứng dụng.

Nefo cảnh báo nhưng vẫn cho cài khi máy chủ ít RAM hơn mức nên có, khi RAM đang trống ít hơn mức ứng dụng có thể dùng, hoặc khi tổng giới hạn bộ nhớ của mọi ứng dụng đã cài vượt quá RAM của máy chủ.

## Cài ứng dụng PHP {#install-a-php-app}

1. Tạo một website PHP trống với phiên bản PHP mà ứng dụng hỗ trợ: xem [Website](./websites). Phiên bản PHP tối thiểu: WordPress 7.4, Nextcloud và Laravel 8.2, WooCommerce, Joomla, Drupal và Moodle 8.3, OpenCart 8.0. OpenCart hỗ trợ tới PHP 8.3, Moodle tới PHP 8.4.
2. Đảm bảo MariaDB (hoặc MySQL) đã cài và đang chạy: xem [Cơ sở dữ liệu](./databases).
3. Trong kho ứng dụng, bấm nút **Cài** của ứng dụng.
4. Chọn **Website**. Danh sách chỉ có các website PHP trống với phiên bản PHP phù hợp. Nếu chưa có, hãy dùng **Tạo website PHP**.
5. Với ứng dụng có hỏi, điền **Tên site**, **Quản trị viên** (đừng dùng `admin`: đó là tên kẻ tấn công thử đầu tiên), **Email quản trị** và **Mật khẩu quản trị**. Để trống mật khẩu để tạo mật khẩu mạnh ngẫu nhiên; mật khẩu tự nhập cần 12 đến 128 ký tự. WooCommerce còn hỏi quốc gia của cửa hàng và đơn vị tiền tệ.
6. Bấm **Cài**. Quá trình cài chạy nền và hiện trong danh sách tác vụ.

Khi cài xong, Nefo hiện **Địa chỉ đăng nhập**, **Quản trị viên** và **Mật khẩu quản trị** một lần. Hãy lưu lại ngay.

Thư mục `public` của website phải trống; trang chào của panel thì không sao. File của website không được [khóa](./website-protection#lock-a-website-s-files). Drupal và Laravel được dựng bằng Composer, còn Moodle cũng cần cả thư mục website: website của chúng phải mới và trống, thư mục website trở thành project và `public` là thư mục web.

Ứng dụng PHP có:

- một cơ sở dữ liệu và user cơ sở dữ liệu mới, gắn với website;
- các file của ứng dụng, được tải về và kiểm tra với checksum trong danh mục;
- quy tắc rewrite của ứng dụng, và giới hạn PHP cao hơn khi ứng dụng cần;
- một cron job chạy bằng user của website cho Moodle (mỗi phút) và WooCommerce (mỗi năm phút).

Nếu cài lỗi, Nefo dọn trống website và xóa cơ sở dữ liệu vừa tạo. Website WordPress cũng xuất hiện trong [Công cụ WordPress](./wordpress).

## Cài ứng dụng Docker {#install-a-docker-app}

1. Trỏ DNS của tên miền dành cho ứng dụng về IP của máy chủ, ví dụ `app.example.com` về `203.0.113.10`.
2. Trong kho ứng dụng, bấm nút **Cài** của ứng dụng.
3. Nhập **Tên miền của website mới** và, nếu muốn, **Tên miền khác**.
4. Một số ứng dụng còn hỏi **Email quản trị**. Hãy dùng tên miền thật, không dùng `.test` hay `.local`.
5. Bấm **Cài**.
6. Khi cài xong, bật HTTPS ở tab **SSL / HTTPS** của website mới: xem [Chứng chỉ SSL](./ssl). Ứng dụng khởi động lại với địa chỉ `https://`. Một số ứng dụng cần HTTPS mới chạy đầy đủ, và form cài đặt có ghi rõ.

Cách bạn có tài khoản quản trị đầu tiên tùy theo ứng dụng, và form cài đặt sẽ cho biết:

| Cách | Ứng dụng | Bạn làm gì |
|---|---|---|
| Panel tự tạo | RustFS, code-server, AdGuard Home, Odoo, Umami, Grafana, ntfy, Gotify, Memos, linkding, Navidrome, Vikunja, Actual Budget, listmonk, Kanboard, FreshRSS, Jellyfin, Mealie | Mở **Thông tin đăng nhập** ở tab **Đã cài**. User là `admin` (Mealie: `changeme@example.com`; Actual Budget chỉ có mật khẩu máy chủ). |
| Email của bạn và một mật khẩu ngẫu nhiên | Open WebUI, Directus, Wiki.js, Healthchecks | Mở **Thông tin đăng nhập**. |
| Tài khoản đăng ký đầu tiên | Ghost, Uptime Kuma, n8n, Gitea | Đăng ký ngay sau khi cài: tài khoản đầu tiên sẽ là quản trị viên. |
| Lời mời | Vaultwarden | Đăng ký trên trang của ứng dụng bằng đúng email mà panel đã mời. |
| Mật khẩu dùng một lần | FileBrowser Quantum | Mở **Thông tin đăng nhập**; ứng dụng yêu cầu đặt mật khẩu mới ở lần đăng nhập đầu. |
| Mật khẩu của website | Stirling-PDF, changedetection.io | Website hỏi user `admin` và mật khẩu trong **Thông tin đăng nhập**. |
| Không có đăng nhập | IT-Tools | Không cần làm gì. |

Ghi chú về một số ứng dụng:

- **Memos** và **Vikunja**: panel tạo tài khoản rồi đóng đăng ký. Bạn có thể mở lại trong cài đặt của ứng dụng.
- **ntfy**: không ai đọc hay gửi được tin nhắn nếu chưa đăng nhập. Tài khoản `admin` nằm trong cài đặt của ứng dụng, nên mật khẩu của nó là mật khẩu trong **Thông tin đăng nhập**.
- **Mealie**: đăng nhập bằng `changeme@example.com` và mật khẩu trong **Thông tin đăng nhập** (panel đã thay mật khẩu mặc định), rồi đặt địa chỉ email của bạn trong hồ sơ.
- **Actual Budget** chỉ mở qua HTTPS: hãy lấy chứng chỉ cho website của nó trước.
- **Healthchecks** và **listmonk** chỉ gửi email qua một dịch vụ SMTP bạn thiết lập trong ứng dụng; máy chủ này không tự gửi thư. Cảnh báo của Healthchecks tới ntfy, Telegram, Slack hoặc webhook vẫn chạy mà không cần SMTP.
- **Navidrome** và **Jellyfin**: xem [Thư mục nhạc và video](#music-and-video-folders).

Ứng dụng Docker có:

- một website mới chuyển tiếp tới ứng dụng. Website tạm đóng (trả về 503) cho tới khi cài xong và các tài khoản mặc định của ứng dụng đã bị khóa;
- các container chạy dưới dạng một dự án Docker Compose, có giới hạn bộ nhớ và log được xoay vòng ở mức 10 MB;
- các file nằm trong `/var/lib/panel-apps/<name>`, chỉ root đọc được: file Compose, các mật khẩu được tạo và dữ liệu của container;
- các cổng chỉ mở trên `127.0.0.1`, không bao giờ mở ra Internet;
- image được ghim đúng một phiên bản.

### Thư mục nhạc và video {#music-and-video-folders}

Navidrome và Jellyfin phát file từ một thư mục trong website của chúng, để bạn thêm file bằng [trình quản lý file](./files) hoặc SFTP:

| Ứng dụng | Thư mục | Ứng dụng thấy là |
|---|---|---|
| Navidrome | `/var/www/<domain>/music` | `/music` |
| Jellyfin | `/var/www/<domain>/media` | `/media` |

Nefo tạo thư mục này khi cài, thuộc user Linux của website, và giao cho ứng dụng ở chế độ chỉ đọc: ứng dụng phát được file nhưng không thay đổi được chúng. Navidrome quét thư mục khi khởi động và sau đó mỗi giờ một lần. Trong Jellyfin, hãy thêm một thư viện và chọn một thư mục trong `/media`.

Các file này là một phần của website, nên bản sao lưu website có cả chúng, và xóa website thì chúng cũng bị xóa. Chuyển đổi video ngay khi phát tốn nhiều CPU trên máy chủ nhỏ: nên dùng file mà thiết bị của bạn phát được nguyên dạng.

## Cài dịch vụ {#install-a-service}

PostgreSQL, MongoDB và Ollama không có website. Bấm **Cài**: không cần điền gì. Nefo tự chọn một cổng trống trên `127.0.0.1` và tạo mật khẩu. Các ứng dụng trên máy chủ này dùng được dịch vụ, còn từ Internet thì không. Mỗi dịch vụ chỉ cài được một lần trên mỗi máy chủ.

Mở **Thông tin đăng nhập** ở tab **Đã cài** để xem địa chỉ, user, mật khẩu và **Chuỗi kết nối**.

## Kiểm tra an toàn với ứng dụng Docker {#the-safety-checks-on-docker-apps}

Công thức (recipe) của ứng dụng Docker trong danh mục là dữ liệu, không bao giờ là script. Nefo chuyển nó thành file Compose và kiểm tra file đó bằng đúng các quy tắc áp dụng cho dự án Compose của bạn, không có ngoại lệ:

- không container đặc quyền (privileged), không cấp thêm đặc quyền (capability), không giao thiết bị của máy chủ;
- không dùng chung namespace mạng, tiến trình hay namespace khác của máy chủ;
- không nới lỏng giới hạn bảo mật, không đặt tham số nhân (sysctl);
- cổng chỉ mở trên `127.0.0.1`;
- không gắn thư mục hệ thống hay socket Docker vào container;
- một thư mục trong website (thư mục nhạc của Navidrome, thư mục media của Jellyfin) chỉ được gắn vào container chạy bằng chính user Linux của website, và ở chế độ chỉ đọc;
- các bước thiết lập cần mật khẩu gửi mật khẩu trong nội dung request hoặc qua đầu vào chuẩn (standard input) của chương trình, không bao giờ trên dòng lệnh.

Xem [Docker](./docker) về các quy tắc này. Mỗi lần cài đều có mật khẩu mới được tạo, và mật khẩu ban đầu bị xóa khỏi cấu hình container sau lần khởi động đầu. Danh mục tải về chỉ được dùng khi chữ ký hợp lệ.

## Quản lý ứng dụng đã cài {#manage-installed-apps}

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

### Cập nhật ứng dụng {#update-an-app}

Khi danh mục có bản mới hơn của một ứng dụng Docker hay dịch vụ, cột **Phiên bản** sẽ ghi, ví dụ "Có bản 2.6.0". Bấm **Cập nhật** rồi xác nhận. Nefo:

1. sao lưu ứng dụng trước. Nếu sao lưu lỗi, không có gì thay đổi;
2. tải image mới về trước khi dừng bất cứ thứ gì;
3. khởi động lại ứng dụng với image mới và kiểm tra ứng dụng có phản hồi không. Nếu không, bản cũ sẽ chạy lại.

Một số ứng dụng không cho cập nhật một lần bấm khi đổi phiên bản chính: "Bản cập nhật này (…) đổi phiên bản chính và cần làm thủ công. Hãy xem hướng dẫn nâng cấp của ứng dụng." Điều này áp dụng cho PostgreSQL, Wiki.js, Odoo, Grafana, Gotify, Vikunja, Healthchecks, listmonk, Jellyfin và Mealie, và cho MongoDB khi đổi phiên bản phụ.

Ứng dụng PHP không cập nhật ở đây: "Hãy cập nhật trong trang quản trị của ứng dụng." Với WordPress, hãy dùng [Công cụ WordPress](./wordpress).

Ứng dụng được cài trước khi có kho ứng dụng hiện nhãn **Bản cài cũ**. **Chuyển đổi và cập nhật** chuyển dữ liệu của nó sang thư mục của kho ứng dụng, chạy bằng Docker Compose rồi cập nhật, sau khi đã sao lưu.

## Gỡ ứng dụng {#uninstall-an-app}

- **Dịch vụ:** bấm **Gỡ cài đặt**, gõ tên của nó rồi xác nhận. Container, dữ liệu và mật khẩu của dịch vụ bị xóa. Các bản sao lưu vẫn được giữ.
- **Ứng dụng có website:** hãy xóa website: xem [Xóa website](./websites#delete-a-website). Với ứng dụng Docker, việc này xóa container, thư mục của nó trong `/var/lib/panel-apps` và các image không còn ứng dụng nào dùng. **Sao lưu website trước**, được đánh dấu sẵn trong hộp thoại xóa, sao lưu cả dữ liệu của ứng dụng, dưới dạng bản sao lưu loại **Ứng dụng**. Với ứng dụng PHP, đánh dấu **Xóa luôn các database của website** để xóa cả cơ sở dữ liệu của nó.

::: danger
Gỡ ứng dụng sẽ xóa vĩnh viễn dữ liệu của nó. Hãy sao lưu trước bằng **Sao lưu ngay** nếu bạn có thể cần lại.
:::

## Tạo bản thứ hai của một ứng dụng {#make-a-second-copy-of-an-app}

Muốn có bản thứ hai của một ứng dụng Docker kèm dữ liệu của nó, hãy nhân bản website của ứng dụng: **Website** → (website) → **Tổng quan** → **Nhân bản…**. Xem [Nhân bản website](./websites#docker-apps). n8n, Odoo và các ứng dụng cài trước khi có kho ứng dụng không nhân bản được. Ứng dụng PHP được nhân bản cùng website và cơ sở dữ liệu như mọi website PHP khác.
