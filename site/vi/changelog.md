---
title: Nhật ký thay đổi
description: Mỗi bản phát hành của Nefo thay đổi những gì, từ mới nhất đến cũ nhất, và những điều cần biết trước khi cập nhật.
---

# Nhật ký thay đổi

Mọi bản phát hành của Nefo, bản mới nhất ở trên cùng, kèm những gì mới, những gì thay đổi, những gì đã sửa, và những điều cần biết trước khi cập nhật. Mỗi bản có liên kết tới trang của nó trên GitHub, nơi có chương trình cho amd64 và arm64, trình cài đặt, danh sách checksum và chữ ký. Cách cập nhật xem ở trang [Cập nhật](./docs/update).

::: warning Quay về bản cũ sau khi cập nhật
v0.21.0, v0.22.0, v0.23.0 và v0.24.0 đều đổi định dạng cơ sở dữ liệu của panel ở lần chạy đầu tiên. Sau đó, bản cũ hơn sẽ không chịu chạy với dữ liệu này nữa. Nếu muốn giữ đường lui, hãy chép thư mục `/var/lib/nefo` trước khi cập nhật, như hướng dẫn ở trang [Cập nhật](./docs/update#if-an-update-fails).
:::

## v0.24.1

Ngày 28/11/2026 · [Bản phát hành trên GitHub](https://github.com/nefocp/nefo/releases/tag/v0.24.1)

Bản sao lưu website khôi phục được cả website ứng dụng và website reverse proxy, kiểm tra bảo mật nhanh hơn, và các bản sửa cho bảy ngôn ngữ.

**Thay đổi**

- **Bản sao lưu website ghi lại loại của website,** nên khi khôi phục trên máy chủ chưa có website đó, website ứng dụng (Node.js, Python, Java) được tạo lại kèm môi trường chạy, lệnh khởi động, cổng và giới hạn bộ nhớ, còn website reverse proxy kèm địa chỉ đích và các tùy chọn. Hãy cài môi trường chạy của ứng dụng ở trang **Phần mềm** trước. Bản sao lưu tạo bằng các bản cũ hơn vẫn được khôi phục thành website tĩnh. Bản sao lưu của website ứng dụng Docker sẽ bị từ chối, kèm các bước cần làm: cài ứng dụng từ Kho ứng dụng trước, rồi khôi phục bản sao lưu của ứng dụng. Xem [Khôi phục trên máy chủ mới](./docs/restore-new-server).
- **Kiểm tra bảo mật chạy các mục song song,** mỗi chương trình được hỏi tối đa 5 giây và cả lượt kiểm tra tối đa 25 giây. Mục nào không xong kịp được đánh dấu **kết quả chưa đầy đủ** thay vì giữ chân các mục khác. Xem [Kiểm tra bảo mật](./docs/security-check).

**Đã sửa**

- Trên máy chủ đang bận, kiểm tra bảo mật có thể chạy rất lâu rồi kết thúc bằng lỗi máy chủ.
- Trong container có giới hạn bộ nhớ, trang chủ tính cả page cache và phần bộ nhớ mà kernel tự giải phóng khi chương trình cần là đang dùng, nên bộ nhớ trông gần đầy sau một lần sao lưu hay một lần tải tệp lớn. Phần cài đặt swap giờ cũng hiện bộ nhớ của container.
- Trên máy chủ có kho PHP, phiên bản PHP mới nhất của kho (8.6 trên Ubuntu 24.04) bị nhận nhầm là PHP của chính bản phân phối: trang **Phần mềm** ghi lẫn nguồn của hai phiên bản, và web server cài sau đó nhận phiên bản này thay vì phiên bản của bản phân phối.
- Ứng dụng do danh mục Kho ứng dụng mới hơn thêm vào không có logo ở ngoài trang Kho ứng dụng.
- Với các ngôn ngữ có chữ dài hơn, menu tài khoản bị cắt mất danh sách ngôn ngữ (giờ chia hai cột), các thẻ trong bảng của website bị cắt mất tên (giờ xuống hai dòng), và thanh trên cùng có thể bị tràn xuống dòng thứ hai trên màn hình hẹp.
- Đã sửa một số câu chữ tiếng Trung, tiếng Nga và tiếng Indonesia trong giao diện.

## v0.24.0

Ngày 27/11/2026 · [Bản phát hành trên GitHub](https://github.com/nefocp/nefo/releases/tag/v0.24.0)

Giao diện bảy ngôn ngữ, sao lưu lên Google Drive, khôi phục trên máy chủ mới, và kiểm tra bảo mật.

**Mới**

- **Giao diện bằng bảy ngôn ngữ:** tiếng Trung, tiếng Tây Ban Nha, tiếng Bồ Đào Nha, tiếng Nga và tiếng Indonesia, bên cạnh tiếng Anh và tiếng Việt, kể cả nội dung của Kho ứng dụng. Mỗi ngôn ngữ là một tệp riêng mà trình duyệt chỉ tải về khi dùng tới, và các tệp của giao diện giờ được gửi đi ở dạng nén gzip: script chính giảm từ 478 KB xuống 163 KB.
- **Sao lưu lên Google Drive,** dùng OAuth client của riêng bạn, đăng nhập bằng mã thiết bị (device code), chỉ được truy cập các tệp do panel tạo ra, và việc tải lên tự chạy tiếp sau khi bị ngắt giữa chừng. Xem [Google Drive](./docs/backups#google-drive).
- **Bản sao từ xa được mã hóa:** mỗi kho lưu trữ (S3, SFTP, Google Drive) có thể mã hóa bản sao lưu mà nó nhận bằng một passphrase: scrypt và AES-256-GCM theo từng khối có đánh số, nên bản sao bị cắt cụt hay bị sửa đổi sẽ bị từ chối. Lệnh `nefo decrypt-backup` giải mã một bản sao bằng tay. Xem [Mã hóa bản sao](./docs/backups#encrypt-the-copies).
- **Khôi phục trên máy chủ mới:** **Tìm bản sao lưu** liệt kê các bản sao lưu trên một kho lưu trữ mà panel này chưa biết, và khi khôi phục, panel tạo website chưa có trên máy chủ, cùng tên miền, tên miền phụ (alias), phiên bản PHP, tệp tin và cơ sở dữ liệu kèm mật khẩu của chúng. Khôi phục bản sao lưu của một website đã xóa cũng tạo lại website đó theo cách này. Xem [Khôi phục trên máy chủ mới](./docs/restore-new-server).
- **Website:** nhân bản website ứng dụng Node.js, Python và Docker, và website có cơ sở dữ liệu PostgreSQL. Khi xóa website, bạn có thể giữ lại cơ sở dữ liệu của nó, và mặc định panel sao lưu website trước khi xóa. Một loại website mới, reverse proxy, chuyển tiếp tới bất kỳ địa chỉ nào, trên Nginx, Apache và OpenLiteSpeed. Xem [Website](./docs/websites#reverse-proxy-websites).
- **Kiểm tra bảo mật:** 22 mục kiểm tra nhanh máy chủ (SSH, tường lửa, bản cập nhật, cơ sở dữ liệu hay Redis mở ra Internet, cổng Docker đi vòng qua ufw, tệp truy cập được qua HTTP, chứng chỉ, cài đặt PHP…), mỗi mục có liên kết tới chỗ khắc phục, chạy khi bạn yêu cầu hoặc hằng ngày. Xem [Kiểm tra bảo mật](./docs/security-check).
- **Cảnh báo đăng nhập** cho các lần đăng nhập SSH (mọi lần, hoặc chỉ khi từ một địa chỉ mới), các đợt đăng nhập SSH sai dồn dập, và các lần đăng nhập panel. Xem [Cảnh báo đăng nhập](./docs/mail-alerts#sign-in-alerts).
- **Chặn quốc gia** cho từng website, dùng dữ liệu quốc gia miễn phí của DB-IP, chỉ tải về khi có website dùng tới. Tính năng này dùng được cả trên website reverse proxy. Xem [Chặn quốc gia và khóa file](./docs/website-protection).
- **Khóa file** cho từng website: mã nguồn của website chuyển sang chỉ đọc với tài khoản PHP và SFTP của nó, trừ các thư mục nó cần ghi, kèm cảnh báo hằng ngày (tùy chọn) khi tệp PHP thay đổi. Lệnh `nefo unlock-files` trả các tệp về như cũ trước khi bạn gỡ Nefo.
- **Cơ sở dữ liệu:** MariaDB 11.4, 11.8 và 12.3 LTS từ kho của MariaDB, và nâng cấp tại chỗ, có dump trước và có đường lui nếu thất bại; chuyển qua lại giữa MariaDB và MySQL, mang theo các database; xoay vòng (rotate) log truy vấn chậm. Xem [Cơ sở dữ liệu](./docs/databases#mariadb-versions).
- **PostgreSQL:** các phiên bản 15 đến 19 từ dự án PostgreSQL, truy cập từ xa cho từng database (TLS, scram-sha-256, mỗi địa chỉ một quy tắc tường lửa), và nâng cấp phiên bản chính (major), có dump trước và có đường lui. Các bản dump mà quá trình nâng cấp giữ lại được liệt kê, kèm nút xóa. Xem [PostgreSQL](./docs/postgresql).
- **Bản ghi email trên trang DNS:** MX, một bản ghi SPF gộp chung và DMARC cho Google Workspace, Microsoft 365, Zoho Mail, Cloudflare Email Routing hoặc nhà cung cấp khác, qua Cloudflare. Xem [DNS (Cloudflare)](./docs/dns#set-up-email-records).
- **Phần mềm:** Java (OpenJDK 17, 21 và 25) cho website ứng dụng, ionCube Loader cho từng phiên bản PHP, và PHP 8.6, tự xuất hiện để cài khi kho PHP có bản này. Xem [Phần mềm](./docs/software).
- **Kho ứng dụng:** thêm 15 ứng dụng, tổng cộng 42: Grafana, ntfy, Gotify, Memos, linkding, Navidrome, Vikunja, Actual Budget, changedetection.io, Healthchecks, listmonk, Kanboard, FreshRSS, Jellyfin và Mealie. Xem [Kho ứng dụng](./docs/apps).

**Thay đổi**

- Số liệu bộ nhớ theo đúng giới hạn bộ nhớ riêng của container (LXC), trên trang chủ, trong cảnh báo, ở trang thiết lập và trong các bước kiểm tra của Kho ứng dụng.
- Số liệu dung lượng đĩa trên trang thiết lập đã được sửa cho đúng, và tính cả danh sách gói của apt trên máy chủ chưa từng tải chúng về.
- Khi danh mục Kho ứng dụng mới hơn có ứng dụng mà panel cũ hơn không chạy được, panel chỉ bỏ qua đúng những ứng dụng đó. Ứng dụng do danh mục mới hơn thêm vào hiện đúng tên ở mọi nơi.

**Đã sửa**

- Không cài được PHP 8.5, vì OPcache không còn gói riêng.
- Trên Ubuntu 26.04, việc cài PostgreSQL (hoặc dịch vụ khác) có thể thất bại khi systemd tự khởi chạy lại chính nó (re-exec) sau apt. Giờ panel thử lại `systemctl` trong lúc chưa kết nối được tới systemd.
- Tường lửa web từng được đề xuất cả ở nơi bản phân phối không có module ModSecurity cho web server (Ubuntu 22.04 với Nginx). Giờ panel ghi rõ là tường lửa web không có cho trường hợp đó, kèm lý do.
- Dòng chữ trên trang WordPress khi còn trống bị dừng giữa chừng một câu.

## v0.23.0

Ngày 26/11/2026 · [Bản phát hành trên GitHub](https://github.com/nefocp/nefo/releases/tag/v0.23.0)

Khóa API, nhập website, trang thiết lập ban đầu, thêm máy chủ cơ sở dữ liệu và phiên bản PHP.

**Mới**

- **Khóa API** cho script của bạn, ở **Cài đặt** → **Khóa API**: chỉ đọc hoặc toàn quyền, danh sách địa chỉ IP được phép và ngày hết hạn. Khóa được gửi trong header `Authorization: Bearer` và panel chỉ lưu mã băm (hash) của nó; dùng sai khóa bị tính như một lần đăng nhập sai. Không khóa nào mở được terminal, Adminer, phpMyAdmin, tài khoản hay chính phần quản lý khóa. Khóa chỉ đọc không đọc được mật khẩu, tệp tin, bản xuất dữ liệu hay tải bản sao lưu về. Xem [Khóa API](./docs/api).
- **Nhập website** từ bản sao lưu đầy đủ của cPanel (website, cơ sở dữ liệu cùng user và mật khẩu, chứng chỉ và cron job), hoặc từ file nén website kèm bản dump SQL của aaPanel hay máy chủ khác. File nén độc hại bị từ chối, bản dump của MySQL 8 được chỉnh cho hợp với MariaDB, và nếu nhập thất bại thì mọi thứ đã tạo ra đều được gỡ bỏ. Xem [Nhập website](./docs/import).
- **Trang thiết lập** ở lần đăng nhập đầu tiên: chọn web server, phiên bản PHP, máy chủ cơ sở dữ liệu và thành phần thêm trên một trang, xem chúng cần khoảng bao nhiêu RAM và dung lượng đĩa, rồi cài tất cả trong một tác vụ. Lệnh `nefo setup` làm tương tự trên dòng lệnh, và trình cài đặt nhận cùng các tùy chọn đó. Không có tùy chọn thì trình cài đặt chạy như trước. Xem [Những việc đầu tiên](./docs/first-steps#set-up-the-server).
- **MySQL 8.4 LTS** thay cho MariaDB (mỗi máy chủ dùng một trong hai), lấy từ kho của Oracle với khóa ký được ghim cố định. Chỉ có trên máy chủ amd64.
- **phpMyAdmin**, chỉ vào được sau khi đăng nhập panel.
- **PostgreSQL** trên trang **Cơ sở dữ liệu**: database, chủ sở hữu, nhập và xuất, và sao lưu.
- **Memcached**, chỉ nghe trên chính máy chủ, không dùng UDP.
- **PHP 5.6 đến 7.3**, cho website chuyển từ máy chủ cũ sang. Các bản này chỉ cài sau khi bạn xác nhận, và không bao giờ được là phiên bản PHP duy nhất trên máy chủ.
- **Đánh dấu PHP hết vòng đời:** phiên bản nào đã qua hạn hỗ trợ bảo mật của PHP sẽ được đánh dấu trên trang **Phần mềm** và ở các website dùng nó. Phiên bản PHP đi kèm bản phân phối không bao giờ bị đánh dấu, vì bản phân phối vẫn tiếp tục vá lỗi cho nó. Xem [Phần mềm](./docs/software#end-of-life).
- **Composer** và **WP-CLI** trên trang **Phần mềm**.

**Thay đổi**

- Trên Ubuntu, các phiên bản PHP bổ sung giờ lấy từ `packages.sury.org`, vì `ppa:ondrej/php` không còn được cập nhật. Máy chủ nào đang dùng PPA này sẽ chuyển một lần khi panel khởi động: các tệp nguồn của PPA được dời sang `/var/lib/nefo/apt-backup`, không cài hay gỡ gói nào, và các gói PHP đã cài sẽ cập nhật từ nguồn mới sau lần `apt-get update` kế tiếp.

**Đã sửa**

- Website được tạo ở trạng thái tạm ngưng nay được lưu đúng là đang tạm ngưng.
- Thư mục mà panel tạo cho chương trình khác không còn bị gán quyền chặt của riêng panel (umask 077).
- Panel nhận ra phiên bản PHP của bản phân phối ngay trên máy chủ mới cài, trước khi apt tải danh sách gói về.
- Ký tự `@` đứng một mình (chính tên miền) không còn làm hỏng một thông báo trên trang DNS.
- Tác vụ bị lỗi đột ngột (panic) nay ghi stack trace vào log của panel.

## v0.22.0

Ngày 26/11/2026 · [Bản phát hành trên GitHub](https://github.com/nefocp/nefo/releases/tag/v0.22.0)

Theo dõi website, thêm kênh cảnh báo và DNS Cloudflare.

**Mới**

- **Theo dõi website:** mỗi website được kiểm tra 1 đến 10 phút một lần (mã trạng thái, một từ khóa nếu bạn đặt, và hạn của chứng chỉ). Website bị coi là ngừng hoạt động sau hai lần kiểm tra thất bại, và bạn nhận cảnh báo khi nó ngừng cũng như khi nó chạy lại. Panel hiển thị thời gian hoạt động trong 24 giờ và 7 ngày, thời gian phản hồi và các lần lỗi gần đây. Các yêu cầu kiểm tra không bị tính vào thống kê truy cập. Xem [Theo dõi](./docs/websites#monitoring).
- **Cảnh báo qua Discord, Slack và webhook.** Webhook được ký bằng HMAC-SHA256. URL đã lưu được che bớt, và khi gửi cảnh báo, panel từ chối địa chỉ trỏ về chính máy chủ, địa chỉ link-local và địa chỉ metadata của nhà cung cấp cloud. Xem [Thư và cảnh báo](./docs/mail-alerts).
- **Trang DNS cho các zone Cloudflare:** xem, thêm, sửa và xóa bản ghi, bật hoặc tắt proxy, xóa cache, và trỏ tên miền của website về máy chủ này, có xem trước thay đổi. Xem [DNS (Cloudflare)](./docs/dns).

## v0.21.0

Ngày 26/11/2026 · [Bản phát hành trên GitHub](https://github.com/nefocp/nefo/releases/tag/v0.21.0)

Công cụ hệ thống, khóa SSH và thêm tùy chọn cho website.

**Mới**

- **Trang Hệ thống:** cập nhật hệ điều hành (đánh dấu bản vá bảo mật, cài tất cả hoặc chỉ bản vá bảo mật, tự động cài bản vá bảo mật, báo khi cần khởi động lại), swap file và swappiness, múi giờ, đồng bộ đồng hồ, tên máy (hostname), TCP BBR và dọn dẹp ổ đĩa. Việc kiểm tra cập nhật chạy nền đợi 30 phút sau khi panel khởi động, và dùng lại danh sách gói đã làm mới trong 6 giờ trước đó. Xem [Hệ thống](./docs/os).
- **Khóa SSH** trên trang **Tường lửa & SSH**: xem, thêm và gỡ khóa. Khóa riêng tư (private key), khóa yếu và khóa trùng bị từ chối, và không gỡ được đường vào cuối cùng.
- **Website:** tắt HTTPS và xóa chứng chỉ, trên cả ba web server; HSTS, có thể kèm `includeSubDomains`; page cache cho từng website PHP trên Nginx, có nút xóa cache; nhân bản (clone) website PHP hoặc website tĩnh cùng tệp tin, cài đặt và cơ sở dữ liệu; đẩy bản staging WordPress lên website thật, sau khi sao lưu.

**Đã sửa**

- Các lần chạy apt của panel không bao giờ chồng lên nhau, và panel chờ khi chương trình khác đang giữ khóa apt thay vì báo lỗi.
- Trong container có `/etc/hosts` được bind-mount, tên máy được ghi thẳng vào tệp đó. Tên máy bị hệ thống từ chối sẽ có thông báo lỗi rõ ràng.

## v0.20.2

Ngày 26/11/2026 · [Bản phát hành trên GitHub](https://github.com/nefocp/nefo/releases/tag/v0.20.2)

**Thay đổi**

- Nginx nay nén CSS, JavaScript, SVG và phông chữ. Debian và Ubuntu mặc định chỉ nén HTML, nên trước đây các tệp này được gửi đi không nén. Thiết lập nào `nginx.conf` đã có thì được giữ nguyên. Máy chủ đang chạy sẽ nhận thay đổi này khi panel đã cập nhật khởi động.

**Đã sửa**

- Mỗi bản phát hành nay kèm danh mục kho ứng dụng đã ký. Trước đây, mỗi ngày panel tìm danh mục này một lần và ghi lỗi HTTP 404 vào log.

## v0.20.1

Ngày 26/11/2026 · [Bản phát hành trên GitHub](https://github.com/nefocp/nefo/releases/tag/v0.20.1)

**Đã sửa**

- Trình cài đặt bị lỗi trên mọi hệ điều hành được hỗ trợ: nó đọc nhầm số phiên bản của hệ điều hành trong `/etc/os-release` thay cho số phiên bản của chính nó, rồi cố tải một bản phát hành không tồn tại. Sau khi sửa, trình cài đặt đã được thử trên máy chủ Ubuntu 24.04 và Debian 12 mới cài.

## v0.20.0

Ngày 26/11/2026 · [Tag trên GitHub](https://github.com/nefocp/nefo/releases/tag/v0.20.0)

Bản phát hành công khai đầu tiên. Trình cài đặt của bản này bị lỗi (đã sửa ở v0.20.1), và trên GitHub bản này không có tệp phát hành nào, chỉ có tag. Hãy cài bản mới hơn.

Bản này gồm: website trên Nginx, Apache hoặc OpenLiteSpeed với PHP-FPM, ứng dụng Node.js và Python, chứng chỉ Let's Encrypt, triển khai từ Git và công cụ WordPress; kho ứng dụng với ứng dụng PHP và Docker; MariaDB, Redis, trình quản lý tệp, terminal trên web và sao lưu lên kho tương thích S3 và SFTP; tường lửa (ufw), fail2ban và cài đặt SSH có cơ chế chống tự khóa mình, tường lửa ứng dụng web và trình quét mã độc; bản phát hành có chữ ký và lệnh `nefo update`; giao diện tiếng Anh và tiếng Việt.
