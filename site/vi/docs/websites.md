---
title: Website
description: Chọn web server, tạo website PHP, tĩnh, ứng dụng (Node.js, Python, Java) và reverse proxy, rồi quản lý tên miền, cài đặt PHP, tùy chọn truy cập, SFTP, thống kê, nhật ký, nhân bản và xóa website.
---

# Website

Website trong Nefo là một tên miền do web server duy nhất của máy chủ phục vụ, Nginx, Apache hoặc OpenLiteSpeed: website PHP, website tĩnh, ứng dụng (Node.js, Python hoặc Java) chạy sau proxy, hoặc reverse proxy tới một địa chỉ khác. Mỗi website có user Linux, thư mục và tài khoản SFTP (không bắt buộc) riêng, và có thể được nhân bản, sao lưu hay xóa ngay trong phần cài đặt của nó.

Trang này nói về web server, cách tạo website và mọi cài đặt của một website: tên miền, PHP, quy tắc rewrite, tùy chọn truy cập, SFTP, thống kê truy cập, nhật ký, nhân bản và xóa. Chứng chỉ có trang riêng, [Chứng chỉ SSL](./ssl), và [chặn quốc gia cùng khóa file](./website-protection) cũng vậy.

## Nên chọn web server nào? {#choose-the-web-server}

Mỗi máy chủ chỉ chạy một web server tại một thời điểm, và Nefo cấu hình mọi website cho web server đó. Trước khi thêm website đầu tiên, trang **Website** sẽ yêu cầu bạn chọn:

| Web server | Nên chọn khi | RAM khi rảnh |
|---|---|---|
| Nginx | Website của bạn không cần tệp `.htaccess`. Đây là lựa chọn nhẹ nhất. | khoảng 15 MB |
| Apache | Website dựa vào tệp `.htaccess`, ví dụ plugin WordPress hoặc website chuyển từ shared hosting sang. | khoảng 40 MB |
| OpenLiteSpeed | Bạn muốn dùng quy tắc rewrite trong `.htaccess` và plugin LiteSpeed Cache cho WordPress. Hỗ trợ cả HTTP/3. | khoảng 26 MB |

Cách cài:

1. Mở **Website**.
2. Trong phần **Cài web server**, chọn Nginx, Apache hoặc OpenLiteSpeed.
3. Bấm **Cài Nginx** (nút ghi tên web server bạn đã chọn).

Nefo cài web server cùng phiên bản PHP của hệ điều hành (chạy bằng PHP-FPM), rồi mở cổng 80 và 443 trên tường lửa ufw, thêm cổng 443/udp cho HTTP/3 nếu là OpenLiteSpeed. Việc này mất khoảng một đến ba phút. OpenLiteSpeed được cài từ kho phần mềm riêng của LiteSpeed, và trang quản trị WebAdmin của nó luôn bị tắt.

Cả ba web server đều chạy PHP trong một pool PHP-FPM riêng cho từng website. Chỉ Nginx giới hạn được số request của mỗi khách; các cài đặt còn lại hoạt động như nhau trên cả ba.

### Đổi sang web server khác {#switch-to-another-web-server}

Nefo không chuyển được website đang có sang web server khác, vì cấu hình của từng website được viết riêng cho web server đang cài. Muốn đổi:

1. Xóa hết website (xem [Xóa website](#delete-a-website)). Nếu còn cần, hãy sao lưu trước: xem [Sao lưu](./backups).
2. Mở **Phần mềm** và tìm thẻ **Nginx, Apache hoặc OpenLiteSpeed**.
3. Bấm **Gỡ cài đặt** ở web server hiện tại, gõ tên của nó (`nginx`, `apache` hoặc `openlitespeed`) rồi xác nhận.
4. Bấm **Cài** ở web server bạn muốn dùng.

Khi gỡ, các thư mục website trong `/var/www`, chứng chỉ và tài khoản Let's Encrypt vẫn được giữ. PHP vẫn còn. Adminer bị gỡ theo vì cần web server, và các luật tường lửa mà Nefo đã thêm cho website (cổng 80 và 443) bị xóa.

Nefo cũng từ chối cài web server khi máy chủ đang có một web server khác, kể cả web server bạn tự cài: cả hai đều cần cổng 80 và 443. Trang **Phần mềm** khi đó ghi nó là "đã cài, panel không dùng". Hãy gỡ nó trước.

## Tạo website {#create-a-website}

1. Trỏ bản ghi A của tên miền về IP của máy chủ, ví dụ `203.0.113.10`.
2. Mở **Website** và bấm **Thêm website**.
3. Ở ô **Tên miền**, gõ tên không kèm `http://` hay `www.`, ví dụ `example.com`.
4. Giữ dấu chọn **Nhận cả www.example.com** nếu website cũng cần mở được bằng `www`. Nefo tự chọn ô này khi tên miền có hai phần, như `example.com`.
5. Chọn **Loại** (xem bảng dưới).
6. Bấm **Tạo website**.

Nefo báo "Đã tạo xong example.com. Hãy tải file của bạn lên /var/www/example.com/public." Một trang chào sẽ hiện cho tới khi bạn thay bằng website của mình.

| Loại | Website làm gì |
|---|---|
| **PHP 8.3** (mỗi phiên bản PHP đã cài là một lựa chọn) | Phục vụ file của bạn và chạy file `.php` bằng phiên bản PHP đó. |
| **Tĩnh (chỉ HTML)** | Phục vụ HTML, CSS, JavaScript và các file khác. Không có PHP. |
| **Ứng dụng (Node.js, Python, Java…) qua proxy** | Chuyển mọi request tới một chương trình đang nghe ở một cổng của máy chủ này. |
| **Reverse proxy tới một địa chỉ khác** | Chuyển mọi request tới địa chỉ bạn nhập: một máy chủ khác, hoặc một dịch vụ trên máy chủ này mà Nefo không chạy. Xem [Website reverse proxy](#reverse-proxy-websites). |

Sau khi tạo, website không đổi loại được: website tĩnh không thành website PHP, website ứng dụng không thành website PHP hay tĩnh, và website reverse proxy cũng không thành loại khác được. Khi cần, hãy tạo website mới. Muốn cài WordPress hay ứng dụng khác, xem [Kho ứng dụng](./apps). Ứng dụng Docker cũng có website riêng được tạo từ đó.

Nefo từ chối tên miền không hợp lệ (tên miền tiếng Việt có dấu phải viết dạng punycode, `xn--…`), tên đã có website khác dùng, và tên miền mà thư mục của nó đã có sẵn trong `/var/www`.

### Website ứng dụng (Node.js, Python, Java) {#app-websites-node-js-python}

Với loại **Ứng dụng (Node.js, Python, Java…) qua proxy**, form hỏi thêm hai mục:

- **Cổng ứng dụng lắng nghe (127.0.0.1)**: một cổng từ 1024 đến 65535. Nefo không nhận cổng của chính panel, 3306 (MariaDB), 6379 (Redis), 7809 (bộ nhận webhook Git) và cổng mà một website ứng dụng khác đang dùng.
- **Lệnh khởi động (không bắt buộc)**: một dòng, tối đa 1000 ký tự, ví dụ `node server.js`. Nefo chạy lệnh trong thư mục `app` của website bằng user Linux của website, với các biến `PORT`, `HOST=127.0.0.1` và `NODE_ENV=production`. Lệnh chạy dưới dạng service systemd `panel-app-<user>.service` và tự chạy lại sau 3 giây nếu bị dừng. Để trống nếu chương trình được chạy bằng cách khác, ví dụ trong Docker.

Đặt mã nguồn vào `/var/www/example.com/app`, qua [SFTP](#sftp-access) hoặc bằng [Triển khai từ Git](./git-deploy), tính năng triển khai website ứng dụng vào đúng thư mục này. Sau đó, trong cài đặt của website:

1. Ở tab **Runtime**, chọn **Runtime**: **Mặc định của server (node trong /usr/local/bin)**, một phiên bản Node.js bạn đã cài ở trang **Phần mềm**, Python (`app/.venv`), cần cài Python ở trang **Phần mềm**, hoặc một phiên bản Java bạn đã cài ở trang **Phần mềm**, hiện ra ví dụ như "Java 21 (OpenJDK 21.0.9)". Cũng trong tab này, bạn đổi được **Lệnh khởi động (không bắt buộc)**. Bấm **Lưu**.
2. Ở tab **Tổng quan**, trong thẻ **Chương trình ứng dụng**, bấm **Cài phụ thuộc**. Với `package.json`, Nefo chạy `npm ci` (hoặc `npm install` nếu không có `package-lock.json`). Với Python, Nefo tạo môi trường ảo `app/.venv` và cài `requirements.txt` bằng pip. Việc này chạy bằng user của website, tối đa 20 phút.
3. Bấm **Khởi động lại** (hoặc **Bật**) để chạy ứng dụng với các phụ thuộc vừa cài.

Thẻ **Chương trình ứng dụng** hiện khi website có lệnh khởi động. Thẻ này còn có **Dừng** và **Log ứng dụng**. Ở tab **Reverse proxy** bạn đổi được cổng. Web server chuyển request tới `127.0.0.1` ở cổng đó, kèm địa chỉ của khách trong `X-Real-IP` và `X-Forwarded-For`, và kết nối WebSocket vẫn hoạt động. Muốn chuyển tiếp tới một máy chủ khác, hoặc tới một dịch vụ trên máy chủ này mà Nefo không khởi động, hãy tạo [website reverse proxy](#reverse-proxy-websites).

Danh sách website có bộ lọc **Java** khi đã có một website ứng dụng Java, bên cạnh bộ lọc cho các loại khác.

#### Website ứng dụng Java {#java-app-websites}

Với chương trình Java, đặt **Lệnh khởi động (không bắt buộc)** là lệnh chạy chương trình, ví dụ `java -jar app.jar`, và đặt `app.jar` vào `/var/www/example.com/app`. Chọn phiên bản Java làm **Runtime**. Hãy cài Java trước ở trang **Phần mềm** (xem [Java](./software#java)).

- Service của ứng dụng nhận `JAVA_HOME` của phiên bản đó, và thư mục `bin` của nó đứng đầu `PATH`, nên `java` chính là phiên bản đó. Ngoài `PORT` và `HOST=127.0.0.1` mà mọi website ứng dụng đều có, service còn nhận `SERVER_PORT` (cùng cổng) và `SERVER_ADDRESS=127.0.0.1`, hai biến mà Spring Boot đọc. Nhờ vậy ứng dụng Spring Boot nghe đúng cổng, chỉ trên máy chủ này, mà không cần cài đặt riêng. Các framework khác đọc `PORT` hoặc cần đặt nó trong cài đặt riêng của chúng.
- **Giới hạn bộ nhớ (MB)** hiện ra với runtime Java. Ô này đặt heap tối đa của Java (`-Xmx`), từ 32 đến 65536 MB, qua `JDK_JAVA_OPTIONS`, biến mà mọi lệnh `java` do lệnh khởi động chạy đều đọc, kể cả khi chạy qua một script khởi động. Để trống thì Java dùng tối đa một phần tư RAM của máy chủ. Tiến trình dùng nhiều bộ nhớ hơn heap của nó một chút. Java in `NOTE: Picked up JDK_JAVA_OPTIONS: -Xmx512m` ở đầu **Log ứng dụng**.
- Các lệnh [Triển khai từ Git](./git-deploy) của website chạy với cùng `JAVA_HOME` và `PATH`, nên `./mvnw package` hay `./gradlew build` build bằng phiên bản Java của website. Giới hạn bộ nhớ không áp dụng cho chúng.
- Website Java không có **Cài phụ thuộc**: hãy build file jar trong lệnh sau khi triển khai, hoặc trước khi tải lên.
- Không gỡ được ở trang **Phần mềm** một phiên bản Java đang có website dùng. Hãy chọn runtime khác trước, hoặc xóa website.

## Website reverse proxy {#reverse-proxy-websites}

Website reverse proxy chuyển mọi request tới một địa chỉ bạn chọn (gọi là đích, hay upstream): một website hoặc ứng dụng trên máy chủ khác, hoặc một dịch vụ trên máy chủ này mà Nefo không chạy, như chương trình bạn tự chạy hay một container trong dự án Compose của riêng bạn. Website có tên miền, chứng chỉ HTTPS, tùy chọn truy cập và tường lửa web riêng, như mọi website khác. Loại này chạy được với Nginx, Apache và OpenLiteSpeed.

1. Mở **Website** và bấm **Thêm website**.
2. Gõ **Tên miền** và chọn loại **Reverse proxy tới một địa chỉ khác**.
3. Ở ô **Chuyển tiếp tới**, gõ địa chỉ: `http://` hoặc `https://`, tên máy hoặc địa chỉ IP, có thể kèm cổng và đường dẫn, ví dụ `http://198.51.100.7:8080` hoặc `https://backend.example.com/app`.
4. Chọn **Header Host gửi đi**: tên miền của website này, hoặc tên máy của đích.
5. Bấm **Tạo website**.

Nefo báo "Đã tạo example.com. Website chuyển tiếp tới … khi tên miền đã trỏ về server này." Địa chỉ được lưu kèm cổng, nên `https://backend.example.com/app` hiện thành `https://backend.example.com:443/app`.

Đường dẫn của request được nối sau đường dẫn của địa chỉ: với `http://198.51.100.7:8080/app`, một lượt truy cập `https://example.com/shop?page=2` sẽ tới `http://198.51.100.7:8080/app/shop?page=2`.

### Các tùy chọn {#the-options}

Đổi các tùy chọn ở tab **Reverse proxy** của website rồi bấm **Lưu**. Website reverse proxy mới có WebSocket và các header X-Forwarded-\* đang bật, với thời gian chờ 60 giây.

| Tùy chọn | Tác dụng |
|---|---|
| **Chuyển tiếp tới** | Địa chỉ, như trên. |
| **Header Host gửi đi** | Tên miền của website: gửi tên miền mà khách truy cập yêu cầu, điều mà đa số ứng dụng chạy sau proxy cần. **Tên máy của đích** gửi tên riêng của đích (kèm cổng khi không phải 80 hay 443), cho máy chủ chỉ trả lời đúng tên của nó. Khi đó địa chỉ của chính đích trong các chuyển hướng (`Location`) được đổi thành địa chỉ của website. |
| **WebSocket** | Chuyển tiếp các kết nối WebSocket (chat, cập nhật trực tiếp, terminal). |
| **Header X-Forwarded-\*** | Báo cho đích biết địa chỉ của khách truy cập và request có đi qua HTTPS hay không: `X-Forwarded-For`, `X-Forwarded-Proto`, `X-Forwarded-Host` và `X-Real-IP`. Khi tắt, các header này bị xóa, kể cả header do khách tự gửi. |
| **Thời gian chờ (giây)** | Thời gian tối đa đích được phép trả lời, từ 5 đến 3600 giây. Riêng việc mở kết nối phải xong trong 30 giây (hoặc trong thời gian chờ, nếu ngắn hơn). Nếu đích chậm hơn, khách nhận trang lỗi: 504 với Nginx, 502 với Apache. |
| **Bỏ qua chứng chỉ của đích (tự ký)** | Hiện với địa chỉ `https://`. Chấp nhận mọi chứng chỉ của đích. |

::: warning
**Bỏ qua chứng chỉ của đích (tự ký)** vẫn giữ kết nối tới đích được mã hóa, nhưng ai chặn được kết nối đó có thể giả làm đích. Chỉ dùng tùy chọn này cho đích trong mạng riêng của bạn có chứng chỉ tự ký.
:::

Không bật tùy chọn đó, Nginx và Apache kiểm tra chứng chỉ của đích `https://` theo các tổ chức cấp chứng chỉ (CA) của máy chủ (`/etc/ssl/certs/ca-certificates.crt`), cho tên trong header Host: tên miền của website, hoặc tên máy của đích nếu bạn chọn nó. Chứng chỉ tự ký hoặc đã hết hạn khiến khách nhận trang lỗi (502 với Nginx, 500 với Apache).

### Nefo từ chối những gì {#what-nefo-refuses}

- địa chỉ không phải `http://` hay `https://` kèm tên máy hoặc địa chỉ IP, có thể kèm cổng (1 đến 65535) và đường dẫn, ví dụ địa chỉ có tên người dùng, chuỗi truy vấn (`?`), phân đoạn (`#`), khoảng trắng, `..` trong đường dẫn, hoặc các địa chỉ `0.0.0.0` và `::`;
- tên máy không tìm thấy ("Không tìm thấy tên máy …. Hãy kiểm tra lại, hoặc dùng địa chỉ IP.");
- chính panel: cổng của panel hoặc `7809` (bộ nhận webhook Git) trên máy chủ này, dù bạn viết `127.0.0.1`, `localhost`, một địa chỉ IP của chính máy chủ hay một tên trỏ về các địa chỉ đó;
- cổng 80 và 443 trên máy chủ này: website sẽ chuyển tiếp ngược về chính web server;
- **Thời gian chờ (giây)** dưới 5 hoặc trên 3600.

### Nên biết {#good-to-know}

- Với Nginx, tên máy được tra khi có request tới, bằng các DNS server trong `/etc/resolv.conf`, nên một tên chưa phân giải được lúc khởi động không bao giờ làm Nginx không chạy được. Các lần tra này không đọc `/etc/hosts`: với tên chỉ có trong `/etc/hosts`, hãy dùng địa chỉ IP của nó.
- HTTPS, chuyển hướng sang HTTPS và HSTS hoạt động như với các website khác: xem [Chứng chỉ SSL](./ssl).
- **Tạm ngưng**, **Chuyển hướng**, bảo vệ bằng mật khẩu, địa chỉ IP bị chặn, giới hạn request, [chặn quốc gia](./website-protection#block-or-allow-countries) và tường lửa web được áp dụng trước khi request tới đích.
- Các tab quy tắc rewrite, trang 404 riêng, chống hotlink và khóa file không áp dụng cho website reverse proxy nên bị ẩn. Chỉ thị riêng của bạn (tab cấu hình web server) vẫn có tác dụng.
- Website vẫn có thư mục `/var/www/example.com` và user Linux, cho SFTP và sao lưu, nhưng không phục vụ các file đó.

### Với OpenLiteSpeed {#with-openlitespeed}

OpenLiteSpeed chuyển tiếp khác với Nginx và Apache. Tab **Reverse proxy** có một ghi chú về điều này, và không hiện **Bỏ qua chứng chỉ của đích (tự ký)**:

- OpenLiteSpeed không bao giờ kiểm tra chứng chỉ của đích `https://`: chứng chỉ tự ký vẫn chạy mà không cần tùy chọn đó.
- **Thời gian chờ (giây)** không được áp dụng: OpenLiteSpeed không giới hạn thời gian cho câu trả lời qua proxy.
- OpenLiteSpeed không gửi địa chỉ của khách trong `X-Forwarded-For`: nó gửi `X-Forwarded-Host`, và `X-Forwarded-Proto` với request HTTPS. Khi tắt **Header X-Forwarded-\***, Nefo xóa `X-Forwarded-For` (còn `X-Real-IP` luôn bị xóa); OpenLiteSpeed vẫn gửi `X-Forwarded-Host`.
- Kết nối WebSocket tới đích không kèm đường dẫn của địa chỉ (`/live`, không phải `/app/live`), và không chạy với đích `https://`.
- Đường dẫn, header Host của đích, mật khẩu và địa chỉ bị chặn hoạt động như với các web server khác.

## Mở cài đặt của website {#open-a-website-s-settings}

Bấm vào tên miền trong danh sách, hoặc nút **Cài đặt** ở cuối dòng. Cài đặt mở ra trong một khung với các tab chia thành bốn nhóm. Một số tab chỉ hiện với một số website:

| Nhóm | Các tab |
|---|---|
| **Cơ bản** | **Tổng quan**, **Tên miền**, **PHP** (website PHP), **Runtime** (website ứng dụng), **Reverse proxy** (website reverse proxy) |
| **Bảo mật** | **SSL / HTTPS**, **Tường lửa WAF**, **Chặn truy cập**, **Chặn quốc gia**, **Khóa file** (trừ ứng dụng Docker và website reverse proxy), **Chống hotlink** (trừ website reverse proxy), **Quét mã độc** |
| **Cấu hình** | **Rewrite** (website PHP và tĩnh), **Bộ nhớ đệm trang** (website PHP trên Nginx và OpenLiteSpeed), **Cấu hình Nginx** (theo tên web server của bạn), **Chuyển hướng**, **Reverse proxy** (website ứng dụng: cổng), **Trang 404** (trừ website reverse proxy) |
| **Vận hành** | **WordPress** (website WordPress), **Triển khai Git** (trừ ứng dụng Docker), **SFTP**, **Sao lưu**, **Theo dõi**, **Thống kê**, **Nhật ký**, **Hạn dùng & tạm ngưng** |

Tab **Tổng quan** cho biết **Thư mục** của website (kèm **Mở trong trình quản lý tệp**), **Người dùng Linux**, tên miền, phiên bản PHP hoặc runtime, web server, tình trạng HTTPS và tường lửa. Bấm **Thêm ghi chú** để viết một dòng (tối đa 200 ký tự) hiện dưới tên miền trong danh sách, ví dụ tên khách hàng.

## User Linux và thư mục của website {#the-website-s-linux-user-and-folders}

Mỗi website có một user Linux riêng, nên một website bị hack không đọc hay sửa được các website khác. Tên user lấy từ tên miền: `example.com` thành `example_com`, `blog.example.com` thành `blog_example_com` (thêm `_2`, `_3`… nếu tên đã có). User này không có mật khẩu, không có shell và không đăng nhập SSH được, trừ khi bạn bật [SFTP](#sftp-access). PHP và ứng dụng của website chạy bằng user này, và PHP chỉ mở được file bên trong thư mục của website.

| Đường dẫn | Chứa gì |
|---|---|
| `/var/www/example.com/public` | Thư mục web (document root). File website của bạn đặt ở đây. |
| `/var/www/example.com/logs` | `php-error.log`, lỗi PHP của website. |
| `/var/www/example.com/tmp` | File tạm của PHP, file đang tải lên và session. |
| `/var/www/example.com/app` | Chỉ có ở website ứng dụng: mã nguồn của chương trình. |
| `/var/log/nginx/example.com.access.log` và `.error.log` | Log của web server cho website (`/var/log/apache2` với Apache, `/var/log/openlitespeed` với OpenLiteSpeed). |
| `/var/lib/nefo/certs/example.com` | Chứng chỉ của website. |
| `/var/cache/nginx/nefo/example.com` | Chỉ có trên Nginx: [bộ nhớ đệm trang](#page-cache), khi được bật. |

Các file cấu hình web server và PHP-FPM do Nefo viết đều mở đầu bằng "Managed by Nefo. Do not edit". Nefo sẽ ghi đè chúng, nên hãy thay đổi trong panel.

## Tên miền và tên miền phụ {#domains-and-aliases}

Ở tab **Tên miền**, ô **Tên miền phụ** liệt kê các tên khác cùng mở website này, mỗi dòng một tên, ví dụ `www.example.com`. Mỗi website có tối đa 20 tên. Mỗi tên phải trỏ về máy chủ này và chỉ thuộc về một website.

Tên wildcard như `*.example.com` nhận mọi tên miền con. Chứng chỉ cho nó cần xác thực DNS qua Cloudflare: xem [Chứng chỉ SSL](./ssl).

Nếu DNS của tên miền đặt ở Cloudflare, nút **Trỏ DNS về đây…** ở cùng tab sẽ tạo hoặc cập nhật bản ghi A (và AAAA) cho mọi tên này, sau khi cho bạn xem trước: xem [DNS (Cloudflare)](./dns#point-a-website-to-this-server).

Nếu website đã có HTTPS mà bạn đổi tên miền, Nefo báo "Tên miền đã thay đổi. Hãy xin chứng chỉ mới để HTTPS bao phủ tất cả tên miền." Hãy bấm **Xin lại chứng chỉ**.

## Phiên bản và cài đặt PHP {#php-version-and-settings}

Tab **PHP** của website PHP có các cài đặt sau:

| Cài đặt | Mặc định | Cho phép |
|---|---|---|
| **Phiên bản PHP** | phiên bản chọn lúc tạo | mọi phiên bản PHP đã cài |
| **Giới hạn bộ nhớ (MB)** | 256 | 32 đến 4096 |
| **Dung lượng tải lên tối đa (MB)** | 64 | 1 đến 4096 (giới hạn kích thước request của web server theo giá trị này) |
| **Thời gian chạy tối đa (giây)** | 300 | 10 đến 3600 |
| **Số worker PHP** | 5 | 1 đến 200 |
| **Chế độ worker** | **Khởi động khi cần (tiết kiệm RAM)** | hoặc **Giữ sẵn vài worker (nhanh hơn cho site đông)** |

**Số worker PHP** là số request PHP mà website xử lý cùng lúc. Mỗi worker có thể dùng tới giới hạn bộ nhớ, nên số worker × bộ nhớ phải vừa với RAM của máy chủ.

Để an toàn, Nefo tắt `exec`, `passthru`, `shell_exec`, `system`, `proc_open`, `popen` và `pcntl_exec` trên mọi website, vì chúng cho phép website bị hack chạy chương trình trên máy chủ. Trong mục **Cho phép các hàm đang bị tắt vì an toàn**, chỉ chọn hàm nào ứng dụng thật sự cần.

Bấm **Lưu**. Nếu PHP-FPM hoặc web server từ chối cấu hình mới, Nefo đặt lại cài đặt cũ. Muốn cài thêm phiên bản PHP, extension, ionCube Loader hay sửa `php.ini`, xem [Phần mềm](./software).

## Quy tắc rewrite và chỉ thị riêng {#rewrite-rules-and-your-own-directives}

Ở tab **Rewrite**, ô **Quy tắc rewrite** có sẵn quy tắc cho các ứng dụng phổ biến: **Mặc định (front controller: index.php hoặc index.html)**, WordPress, Laravel, Symfony, CodeIgniter 4, Drupal, Joomla, OpenCart, Nextcloud, Moodle và phpBB cho website PHP, và Ứng dụng một trang (SPA) cùng Website tĩnh (URL không có .html) cho mọi website. Tab hiện nội dung quy tắc bạn chọn. Trên Nginx, quy tắc này thay cho khối `location /` mặc định. Trên Apache, tệp `.htaccess` của website có thể thay front controller của quy tắc có sẵn; trên OpenLiteSpeed, front controller chạy sau các quy tắc `.htaccess` của website. Trên cả hai, tệp `.htaccess` không gỡ bỏ được các quy tắc bảo vệ của quy tắc có sẵn.

**Website tĩnh (URL không có .html)** trả về trang mà không cần đuôi `.html`, đúng như cách các trình tạo website tĩnh như VitePress (với `cleanUrls`) tạo liên kết: `/docs/install` trả về `docs/install.html`, còn `/docs/` trả về `docs/index.html`. Các tệp khác vẫn được trả về như cũ, và trang không tồn tại vẫn nhận lỗi 404.

Quy tắc phpBB dành cho phpBB bạn tự cài. Quy tắc này chuyển các địa chỉ không phải file tới `app.php` của phpBB, và không bao giờ trả về các file cài đặt của phpBB (`config.php`, `common.php`), các thư mục `cache`, `files` (file đính kèm), `store` (bản sao lưu), `includes`, `phpbb` và `vendor` hay `images/avatars/upload`: các đường dẫn đó trả về 403.

Tab kế tiếp mang tên web server của bạn, ví dụ **Cấu hình Nginx**. Trong ô **Chỉ thị Nginx** (hoặc **Chỉ thị Apache**, **Chỉ thị OpenLiteSpeed**), bạn thêm được chỉ thị riêng theo cú pháp của web server, tối đa 32 KB:

- Nginx thêm chúng vào trong khối `server` của website, trước các quy tắc rewrite.
- Apache thêm chúng vào cuối khối `VirtualHost` của website.
- OpenLiteSpeed thêm chúng vào cuối khối `virtualHost` của website.

Web server kiểm tra cấu hình mới trước khi thay đổi. Nếu bị từ chối, Nefo hiện lỗi và website vẫn chạy như cũ. Bấm **Xem toàn bộ file cấu hình** để xem file Nefo tạo ra.

Về tệp `.htaccess`: Apache đọc chúng. OpenLiteSpeed chỉ dùng các quy tắc rewrite trong đó, và Nefo chép chúng vào cấu hình của website trong vài giây sau mỗi lần tệp thay đổi. Nginx bỏ qua chúng.

## Bộ nhớ đệm trang {#page-cache}

Trên Nginx, tab **Bộ nhớ đệm trang** của website PHP bật FastCGI cache của Nginx cho website đó: Nginx giữ lại mỗi trang do PHP tạo và trả lại trang đó mà không phải chạy PHP hay hỏi database. Hãy bật công tắc **Bộ nhớ đệm trang**.

- Mỗi trang được giữ 10 phút, trang không tồn tại (404) được giữ 1 phút. Nếu PHP lỗi hoặc không trả lời, Nginx trả bản sao cuối cùng mà nó còn giữ.
- Nginx luôn hỏi PHP với các request không phải GET hay HEAD (form gửi đi), địa chỉ có dấu `?`, khách có cookie đăng nhập hoặc giỏ hàng (WordPress `wordpress_logged_in_`, `wp-postpass_`, `comment_author_`, WooCommerce `woocommerce_items_in_cart`, `wp_woocommerce_session_`, và `PHPSESSID` của PHP), cùng các trang dưới `/wp-admin/`, `/wp-login.php`, `/xmlrpc.php`, `/wp-json/`, `/wp-cron.php`, `/cart/`, `/checkout/` và `/my-account/`. Trang mà PHP đánh dấu là riêng tư hoặc gửi kèm cookie cũng không được giữ.
- Mỗi website có bộ nhớ đệm riêng, tối đa 256 MB, trong `/var/cache/nginx/nefo/example.com`. Trang không ai mở trong một giờ sẽ bị bỏ.

Sau khi sửa một trang, hãy bấm **Xóa bộ nhớ đệm**: mọi khách sẽ thấy bản mới ngay. Nếu không, khách chưa đăng nhập có thể thấy trang cũ tới 10 phút. Tắt bộ nhớ đệm hoặc xóa website thì thư mục của nó cũng bị xóa.

Để kiểm tra, mở một trang hai lần trong cửa sổ ẩn danh, hoặc chạy `curl -sI https://example.com/` hai lần: header `X-Cache-Status` sẽ là `MISS`, rồi `HIT` (`BYPASS` với các request kể trên). File tĩnh không có header này: Nginx vốn đã trả chúng trực tiếp.

Trên OpenLiteSpeed, tab cho biết web server này có sẵn bộ nhớ đệm trang riêng là LSCache: với WordPress hãy cài plugin LiteSpeed Cache, ứng dụng khác bật nó bằng quy tắc cache trong `.htaccess`. Trên Apache, Nefo không có bộ nhớ đệm trang và không có tab này.

## Tạm ngưng, chuyển hướng hoặc khóa website bằng mật khẩu {#suspend-redirect-or-password-protect-a-website}

**Tạm ngưng.** Tắt công tắc ở cột **Hoạt động** trong danh sách, hoặc công tắc **Website đang chạy** ở tab **Tổng quan**. Khách truy cập khi đó nhận HTTP 503 với dòng "This website is temporarily unavailable." File và cài đặt vẫn giữ nguyên, chứng chỉ vẫn tự gia hạn. Muốn tạm ngưng nhiều website, hãy chọn chúng, chọn **Tạm ngưng** trong ô **Thao tác hàng loạt** rồi bấm **Áp dụng**; **Chạy lại** bật chúng lên lại.

**Chuyển hướng.** Ở tab **Chuyển hướng**, chọn **Chuyển hướng mọi truy cập sang địa chỉ khác** và nhập **Địa chỉ**, ví dụ `https://example.org`. Đây là chuyển hướng vĩnh viễn (301) và giữ nguyên đường dẫn: `/page` sẽ chuyển tới địa chỉ mới kèm `/page`. Chuyển hướng chỉ hoạt động khi website đang chạy.

**Mật khẩu.** Ở tab **Chặn truy cập**, trong phần **Bảo vệ bằng mật khẩu**, chọn **Bảo vệ bằng mật khẩu** rồi nhập **Tên đăng nhập** và **Mật khẩu** dài 8 đến 72 ký tự. Trình duyệt sẽ hỏi hai thông tin này trước khi hiện website. Về sau, để trống mật khẩu là giữ mật khẩu hiện tại. Việc xác thực của Let's Encrypt và webhook Git vẫn tới được website.

Cũng trong tab này, bạn chặn được địa chỉ IP và, trên Nginx, giới hạn số request của mỗi khách: xem [Bảo mật](./security#rate-limit-and-blocked-addresses). Muốn chặn cả một quốc gia, xem [Chặn quốc gia](./website-protection#block-or-allow-countries).

## Trang 404 riêng và chống hotlink {#custom-404-page-and-hotlink-protection}

**Trang 404.** Ở tab **Trang 404**, chọn **Dùng trang 404 của riêng tôi** và tải trang của bạn lên thành `public/404.html`. Các trang do ứng dụng tự xử lý, như trang của WordPress hay Laravel, vẫn dùng trang 404 riêng của chúng.

**Chống hotlink.** Ở tab **Chống hotlink**, chọn **Chống hotlink ảnh và media**. Website khác nhúng file `jpg`, `jpeg`, `png`, `gif`, `webp`, `avif`, `svg`, `ico`, `bmp`, `mp4`, `webm`, `mp3`, `ogg` và `pdf` của bạn sẽ nhận HTTP 403. Chính website của bạn, các công cụ tìm kiếm (Google, Bing, DuckDuckGo và Yahoo) và người mở file trực tiếp luôn xem được. Ghi tối đa 20 website khác vào ô **Các site khác được phép hiển thị**, mỗi dòng một tên.

## Ngày hết hạn {#expiry-date}

Ở tab **Hạn dùng & tạm ngưng**, đặt **Ngày hết hạn** rồi bấm **Lưu**. Website chạy đến hết ngày đó (theo giờ máy chủ); sau đó Nefo tự tạm ngưng website. Nefo kiểm tra mỗi giờ một lần. Bấm **Không bao giờ** để bỏ ngày hết hạn. Trong 7 ngày cuối, danh sách hiện "hết hạn" kèm ngày dưới tên miền. Cách này tiện cho website của khách hàng trả tiền đến một ngày nhất định.

Tab này còn có **Tạm ngừng website này** để tạm ngưng website ngay.

## Truy cập SFTP {#sftp-access}

SFTP cho phép bạn hoặc khách hàng tải file lên bằng FileZilla, WinSCP hay bất kỳ ứng dụng SFTP nào. File tải lên sẽ thuộc về user của website, đúng như cần. Tính năng này cần cài OpenSSH server.

1. Mở tab **SFTP** của website và chọn **Cho phép đăng nhập SFTP cho website này**.
2. Chọn cách đăng nhập:
   - **Tạo mật khẩu mạnh mới**
   - **Tự đặt mật khẩu (ít nhất 12 ký tự)**, tối đa 128 ký tự
   - **Giữ mật khẩu hiện tại** (khi đã có mật khẩu)
   - **Không dùng mật khẩu: chỉ đăng nhập bằng khóa SSH**
3. Không bắt buộc: dán **Khóa SSH công khai (tùy chọn)**, mỗi dòng một khóa giống `authorized_keys`, tối đa 50 khóa. Các tùy chọn như `command=` sẽ bị bỏ.
4. Bấm **Lưu**. Mật khẩu do panel tạo chỉ hiện một lần: hãy sao chép ngay.
5. Kết nối bằng **Máy chủ**, **Cổng**, **Tên đăng nhập** và **Thư mục** được hiển thị. Máy chủ là địa chỉ bạn dùng để mở panel, cổng là cổng SSH của máy chủ. Tab cũng hiện địa chỉ đầy đủ, ví dụ `sftp://example_com@panel.example.com/var/www/example.com`.

Tài khoản bị khóa trong thư mục của website. Nó chỉ thấy `/var/www/example.com`, ở đúng đường dẫn như trong panel và trong PHP, và không chạy được lệnh hay chuyển tiếp cổng. Chính sách mật khẩu của máy chủ có thể từ chối mật khẩu bạn tự đặt; khi đó Nefo cho biết lý do. Trong lúc [file của website đang bị khóa](./website-protection#lock-a-website-s-files), tài khoản SFTP chỉ sửa được các thư mục được ghi.

Tắt SFTP sẽ xóa mật khẩu, khóa SSH và ngắt các phiên đang mở.

## Thống kê truy cập {#traffic-statistics}

Tab **Thống kê** đọc log truy cập của web server cho website. Chọn **Khoảng thời gian**: **Hôm nay**, **7 ngày**, **14 ngày** hoặc **30 ngày**. Tab hiện:

- **Yêu cầu**, **Khách truy cập** (số địa chỉ IP khác nhau), **Băng thông**, **Tỷ lệ lỗi** (4xx và 5xx) và **Bot**
- số yêu cầu và khách truy cập theo giờ hoặc theo ngày
- **Trang được xem nhiều nhất**, **Địa chỉ IP truy cập nhiều nhất**, **Nguồn giới thiệu**, **Trình duyệt** và **Bot và script**

Các yêu cầu kiểm tra chứng chỉ và các lần [theo dõi website](#monitoring) của chính panel không được tính. Máy chủ giữ log khoảng 14 ngày, nên không có dữ liệu cũ hơn. Với log rất lớn, chỉ 5.000.000 dòng mới nhất được đọc. Cột **Truy cập 7 ngày** trong danh sách website lấy từ cùng các log này.

## Nhật ký {#logs}

Tab **Nhật ký** hiện những dòng mới nhất của:

- **Log truy cập (Nginx)** và **Log lỗi (Nginx)** (theo tên web server của bạn)
- **Lỗi PHP**, với website PHP (`/var/www/example.com/logs/php-error.log`)
- **Log ứng dụng**, với website ứng dụng và website Docker

## Theo dõi website {#monitoring}

Tab **Theo dõi** kiểm tra vài phút một lần xem website có phản hồi không, và gửi cảnh báo cho bạn khi website không phản hồi. Bật **Theo dõi website này**; lần kiểm tra đầu tiên chạy ngay.

Panel truy cập địa chỉ công khai của website, bằng `https://` nếu website có chứng chỉ (nếu không thì `http://`), qua DNS như mọi khách truy cập. Vì vậy bản ghi DNS sai, trang lỗi của Cloudflare hay chứng chỉ hết hạn cũng hiện ra. Panel đi theo các chuyển hướng tới tên miền của chính website (như từ `http://` sang `https://`, hoặc sang `www.`). Chuyển hướng tới tên miền khác được tính là có phản hồi.

| Cài đặt | Tác dụng | Mặc định |
|---|---|---|
| **Chu kỳ kiểm tra** | bao lâu một lần: mỗi phút, hoặc mỗi 2, 5 hay 10 phút | mỗi 5 phút |
| **Đường dẫn** | trang cần truy cập, như `/health`; có thể kèm chuỗi truy vấn | `/` |
| **Chữ phải có trên trang** | không bắt buộc; không phân biệt chữ hoa, chữ thường | trống |
| **Coi mã 4xx (như 404) là không phản hồi** | câu trả lời 4xx làm lần kiểm tra bị lỗi | tắt |

Một lần kiểm tra bị lỗi khi:

- không có câu trả lời trong 10 giây, tên miền không phân giải được, kết nối bị từ chối, hoặc chứng chỉ đã hết hạn, không được tin cậy hay cấp cho tên miền khác
- câu trả lời là lỗi HTTP từ 500 trở lên. Câu trả lời 4xx vẫn được tính là hoạt động, trừ khi bạn chọn coi mã 4xx là không phản hồi
- chữ phải có trên trang không nằm trong 256 KB đầu của trang

Website bị coi là **Không phản hồi** sau 2 lần kiểm tra lỗi liên tiếp, và hoạt động trở lại sau một lần kiểm tra thành công. Khi đó panel gửi cảnh báo tới mọi kênh trong **Cài đặt** → **Cảnh báo** (xem [Cảnh báo](./mail-alerts#alerts)):

- khi website ngừng phản hồi, kèm lý do, ví dụ "example.com is down (HTTP 502, 2 checks in a row)", và báo lại mỗi 6 giờ nếu vẫn chưa phản hồi
- khi website hoạt động trở lại, kèm thời gian đã ngừng
- khi chứng chỉ website đang dùng còn hạn dưới 14 ngày, tối đa một lần mỗi ngày

Tab hiện trạng thái và lần kiểm tra gần nhất, thời gian hoạt động trong 24 giờ và 7 ngày, thời gian phản hồi trung bình, biểu đồ thời gian phản hồi trong 24 giờ qua, và các lần lỗi gần đây kèm lý do. Thời gian phản hồi được tính tới khi nhận được phần đầu (header) của câu trả lời, gồm cả thời gian kết nối. Trong danh sách website, website được theo dõi mà không phản hồi hiện chữ **Không phản hồi** màu đỏ ở cột **Hoạt động**, và chuông thông báo cũng liệt kê nó.

- Các lần kiểm tra gửi user agent `Nefo-Monitor/1.0`. Chúng không được tính trong tab **Thống kê**.
- Kết quả được giữ 7 ngày. Xóa website thì kết quả cũng bị xóa.
- Website đang tạm ngưng không được kiểm tra.
- Các lần kiểm tra chạy trên chính máy chủ này. Chúng không báo được khi cả máy chủ ngừng hoạt động, hay khi có sự cố trên đường mạng giữa khách truy cập và máy chủ: với trường hợp đó, hãy dùng thêm một dịch vụ theo dõi bên ngoài.

## Tường lửa web, quét mã độc, chặn quốc gia, khóa file và sao lưu {#web-firewall-malware-scan-country-blocking-file-lock-and-backups}

- **Tường lửa WAF**: ModSecurity với bộ luật OWASP Core Rule Set cho website này: **Tắt**, **Chỉ phát hiện: ghi lại tấn công, không chặn** hoặc **Chặn tấn công**. Hãy cài tường lửa ở trang **Phần mềm** trước. Xem [Bảo mật](./security#web-application-firewall), trang này cũng nói về tab **Quét mã độc**.
- **Chặn quốc gia**: từ chối khách từ một số quốc gia, hoặc chỉ cho một số quốc gia vào. **Khóa file**: đặt mã nguồn của website ở chế độ chỉ đọc với PHP và tài khoản SFTP của nó, trừ các thư mục mà website phải ghi vào. Xem [Chặn quốc gia và khóa file](./website-protection).
- **Sao lưu**: **Sao lưu ngay**, hoặc đặt lịch **Hằng ngày lúc 03:00**. Xem [Sao lưu](./backups).

## Nhân bản website {#clone-a-website}

**Nhân bản** sao chép một website sang tên miền mới: để bắt đầu một website mới từ website có sẵn, hoặc để thử thay đổi trên một bản sao. Tính năng này dùng được cho website PHP, tĩnh và reverse proxy, website ứng dụng (Node.js, Python, Java) và ứng dụng Docker từ [Kho ứng dụng](./apps).

1. Mở tab **Tổng quan** của website, bấm **Nhân bản…** ở mục **Nhân bản website này**.
2. Gõ **Tên miền của bản sao**.
3. Giữ dấu chọn **Sao chép các database gắn với website này** để chép cả chúng. Ô này không hiện với WordPress, vì database của nó luôn được chép, và với ứng dụng Docker, vì dữ liệu của chúng luôn được chép.
4. Bấm **Nhân bản**. Website vẫn chạy trong lúc sao chép; việc sao chép chạy như một tác vụ có nhật ký. Ứng dụng Docker tạm dừng một lát trong lúc chép dữ liệu.
5. Trỏ DNS của tên miền mới về máy chủ này, rồi bật HTTPS ở tab **SSL / HTTPS** của nó. Bản sao bắt đầu bằng `http://`.

### Website PHP, tĩnh và reverse proxy {#php-static-and-reverse-proxy-websites}

Bản sao có:

- một website mới cùng loại (cùng phiên bản PHP, tĩnh, hoặc reverse proxy tới cùng địa chỉ với cùng tùy chọn) với user Linux riêng, cùng ứng dụng, cài đặt PHP, quy tắc rewrite và chỉ thị riêng. Trong chỉ thị riêng của bạn, thư mục của website cũ được thay bằng thư mục mới;
- bản sao thư mục website `/var/www/example.com`, thuộc user mới, không kèm `logs`, `tmp` và khóa triển khai Git (`.ssh`). Liên kết tượng trưng (symlink) được chép nguyên là liên kết; liên kết trỏ tới một chỗ bên trong thư mục của website cũ giờ trỏ tới đúng chỗ đó trong thư mục mới. Trong `public/.user.ini` thư mục cũ được thay bằng thư mục mới;
- với mỗi cơ sở dữ liệu đang gắn, MariaDB, MySQL hay PostgreSQL, một cơ sở dữ liệu và user mới cùng loại chứa bản sao dữ liệu, gắn với website mới;
- các chương trình nền của website, ở trạng thái dừng (xem bên dưới).

Khi bản sao đã sẵn sàng, Nefo hiện thông tin kết nối của các cơ sở dữ liệu MariaDB và MySQL mới (cơ sở dữ liệu PostgreSQL thì xem ở trang **Cơ sở dữ liệu**). Các file cấu hình của chính ứng dụng (như `.env` hay `config.php`) vẫn ghi cơ sở dữ liệu và user cũ: hãy đổi sang cái mới. Các file ghi thư mục cũ, như bộ nhớ đệm cấu hình của ứng dụng, cũng vẫn ghi thư mục cũ.

### Website ứng dụng {#app-websites}

Bản sao của website ứng dụng có thêm:

- một cổng riêng do Nefo chọn, ứng dụng nhận cổng này qua `PORT`. Ứng dụng tự đặt cổng của mình thì phải sửa để dùng cổng mới;
- cùng **Runtime** và **Lệnh khởi động (không bắt buộc)**; trong lệnh, thư mục của website cũ được thay bằng thư mục mới. Service `panel-app-<user>.service` của bản sao khởi động khi file và cơ sở dữ liệu đã được chép xong;
- `node_modules` cùng các liên kết của nó (`node_modules/.bin`), và môi trường ảo Python `app/.venv` chạy được ở thư mục mới: đường dẫn trong các script, `pyvenv.cfg` và các file `.pth` của nó ghi thư mục của bản sao. Các phụ thuộc không được cài lại;
- các chương trình nền của website, cùng lệnh, thư mục và biến (đường dẫn tới thư mục cũ đã được thay), nhưng ở trạng thái **đã dừng**: chừng nào bản sao chưa dùng cơ sở dữ liệu và hàng đợi riêng, chúng sẽ làm việc trên dữ liệu của bản gốc. Hãy bật chúng ở trang **Chương trình nền**.

Runtime vẫn phải đang được cài: nếu phiên bản Node.js, Python hay Java đã bị gỡ ở trang **Phần mềm**, Nefo từ chối ngay trước khi bắt đầu sao chép.

### Ứng dụng Docker {#docker-apps}

Bản sao của website ứng dụng Docker là một website mới với một bản cài mới của ứng dụng:

- cùng phiên bản và image với bản gốc (kể cả khi Kho ứng dụng đã có bản mới hơn), với container và cổng riêng, cùng giới hạn bộ nhớ;
- bản sao dữ liệu của ứng dụng từ `/var/lib/panel-apps/<name>`, chép trong lúc bản gốc dừng: Nefo dừng nó, chép dữ liệu rồi chạy lại ngay. Trong bản sao, file thuộc user của website gốc được chuyển sang user của website mới;
- cùng tài khoản và mật khẩu như bản gốc (dữ liệu được chép đã tạo bằng chúng), hiện ở **Kho ứng dụng** → **Đã cài** như mọi ứng dụng khác;
- địa chỉ của ứng dụng được đổi sang tên miền mới, với những ứng dụng lấy địa chỉ từ panel (`url`, `DOMAIN` và các cài đặt tương tự của ứng dụng). Nếu ứng dụng giữ địa chỉ trong cài đặt riêng của nó, hãy đổi ở đó.

Trước tiên Nefo kiểm tra bộ nhớ và dung lượng đĩa mà bản sao cần (khoảng bằng dung lượng dữ liệu của ứng dụng, cộng thêm 512 MB).

Nefo từ chối nhân bản:

| Ứng dụng | Lý do |
|---|---|
| n8n | Bản sao sẽ chạy các workflow đang bật (lịch và trigger) thêm một lần nữa. Hãy xuất các workflow rồi nhập vào một n8n mới. |
| Odoo | Bản sao cũng sẽ chạy các tác vụ theo lịch và máy chủ thư: nó sẽ gửi đi cùng các email, và tải về rồi xóa thư của cùng hộp thư. Hãy sao lưu nó, hoặc cài một Odoo mới. |
| Ứng dụng cài từ trước khi có Kho ứng dụng | Hãy cập nhật nó trước (**Kho ứng dụng** → **Đã cài**), rồi mới nhân bản. |

Bản sao của một ứng dụng giám sát như Uptime Kuma kiểm tra cùng các website và gửi cùng các thông báo như bản gốc: hãy tạm dừng những gì không cần ở một trong hai bản.

### WordPress {#wordpress}

Website WordPress được chép như một [bản staging](./wordpress#make-a-staging-copy): cơ sở dữ liệu luôn được chép, `wp-config.php` trỏ tới bản sao và địa chỉ website được thay ở mọi nơi, nên bản sao chạy được ngay. Công cụ tìm kiếm được yêu cầu không lập chỉ mục bản sao; nếu bản sao là một website riêng, hãy đổi lại trong WordPress ở **Cài đặt → Đọc** (Settings → Reading). Khác với bản staging, bản nhân bản không đẩy ngược lên website gốc được.

### Không sao chép {#not-copied}

Tên miền phụ, chứng chỉ, bảo vệ bằng mật khẩu, chuyển hướng và cài đặt tường lửa, chặn quốc gia, khóa file, cron job, cài đặt triển khai Git, truy cập SFTP, theo dõi website và lịch sao lưu.

Nếu một bước bị lỗi, Nefo xóa mọi thứ đã tạo cho bản sao: website, user Linux của nó, các cơ sở dữ liệu mới, các chương trình nền đã chép và, với ứng dụng Docker, các container mới cùng dữ liệu của chúng. Bản gốc vẫn chạy bình thường.

## Xóa website {#delete-a-website}

1. Mở tab **Tổng quan** của website.
2. Trong phần **Xóa website này**, bấm **Xóa**.
3. Hộp thoại liệt kê những gì bị xóa theo website dưới dòng **Bị xóa cùng website:** (ứng dụng Docker và dữ liệu của nó, tài khoản SFTP, cron job, chương trình nền, theo dõi website, cài đặt triển khai Git, lịch sao lưu).
4. Giữ dấu chọn **Sao lưu website trước** (mặc định), hoặc bỏ chọn.
5. Chọn **Xóa luôn các database của website** để xóa cả các cơ sở dữ liệu đang gắn. Nếu không chọn (mặc định), chúng vẫn nằm ở trang **Cơ sở dữ liệu**, không gắn với website nào.
6. Gõ tên miền để xác nhận rồi bấm **Sao lưu rồi xóa** (hoặc **Xóa** nếu không sao lưu).

### Bản sao lưu tạo trước khi xóa {#the-backup-made-first}

Với **Sao lưu website trước**, Nefo tạo trước một bản sao lưu thông thường trên máy chủ này: file của website và các cơ sở dữ liệu MariaDB, MySQL và PostgreSQL của nó, và với ứng dụng Docker, dữ liệu của ứng dụng thành một bản sao lưu thứ hai loại **Ứng dụng**. Website chỉ bị xóa nếu sao lưu thành công. Nếu sao lưu lỗi, tác vụ dừng với dòng "the backup failed, so example.com was not deleted" và không có gì bị xóa.

- Bản sao lưu có tên `example.com_before-delete_<date>-<time>.tar.gz` (và `<user>_before-delete_….tar.gz` cho dữ liệu của ứng dụng Docker), nằm trong `/var/backups/panel/site/example.com/`.
- Như mọi bản sao lưu bạn tạo bằng tay, nó được giữ đến khi bạn xóa ở trang **Sao lưu**. Lịch sao lưu chỉ xóa các bản do chính nó tạo, và lịch sao lưu của website bị xóa cùng website, nên không có gì xóa bản này.
- Bản này chỉ nằm trên máy chủ này: nó không được tải lên nơi lưu trữ từ xa. Nếu muốn có một bản ở nơi khác, hãy tải nó lên từ trang **Sao lưu**.
- Việc xóa chạy như một tác vụ có nhật ký. Nhật ký ghi tên bản sao lưu.

::: warning
Không có bản sao lưu, file của website (và các cơ sở dữ liệu, nếu bạn chọn **Xóa luôn các database của website**) không lấy lại được. Hộp thoại báo điều này khi bạn bỏ chọn **Sao lưu website trước**.
:::

### Có lấy lại được website đã xóa không? {#bring-a-deleted-website-back}

Được, bằng bản sao lưu tạo trước khi xóa (hoặc bất kỳ bản sao lưu nào khác của website). Ở trang **Sao lưu**, khôi phục bản `…_before-delete_…` và gõ tên miền để xác nhận. Việc khôi phục tạo lại website với tên miền, tên miền phụ, phiên bản PHP, quy tắc rewrite, file và cơ sở dữ liệu của nó. Cơ sở dữ liệu đã bị xóa được tạo lại với mật khẩu cũ, đúng mật khẩu mà các file được khôi phục ghi; cơ sở dữ liệu bạn đã giữ lại được gắn lại với website.

Không được tạo lại: chứng chỉ (hãy lấy chứng chỉ mới ở tab **SSL / HTTPS**), cron job, truy cập SFTP, chương trình nền, chuyển hướng và bảo vệ bằng mật khẩu. Xem [Khôi phục trên máy chủ mới](./restore-new-server#restoring-a-website-that-doesn-t-exist-here) về những gì việc khôi phục tạo ra.

Bản sao lưu tạo bằng Nefo 0.24.1 trở lên ghi lại loại của website, nên website ứng dụng (Node.js, Python, Java) được khôi phục kèm môi trường chạy, lệnh khởi động, cổng và giới hạn bộ nhớ, còn website reverse proxy kèm địa chỉ đích và các tùy chọn; hãy cài môi trường chạy của ứng dụng ở trang **Phần mềm** trước. Bản sao lưu tạo bằng các bản cũ hơn chỉ ghi phiên bản PHP, nên website như vậy được khôi phục thành website tĩnh: hãy tạo lại website với cùng tên miền và cùng loại trước, rồi mới khôi phục. Với ứng dụng Docker, hãy cài lại ứng dụng trên cùng tên miền từ **Kho ứng dụng**, rồi khôi phục bản sao lưu loại **Ứng dụng** của nó.

### Nefo xóa và giữ lại những gì {#what-nefo-removes-and-keeps}

Nefo xóa:

- thư mục website `/var/www/example.com` cùng toàn bộ file trong đó, kể cả kho Git và khóa triển khai
- user và group Linux, cùng quyền truy cập SFTP
- cấu hình web server, file mật khẩu và chỉ thị riêng của bạn
- pool PHP-FPM, service của ứng dụng, hoặc ứng dụng Docker cùng container, dữ liệu và image không còn dùng
- chứng chỉ, bộ nhớ đệm trang và log web server của website
- các chương trình nền của website, cron job chạy bằng user của nó, lịch sao lưu, theo dõi website và cài đặt triển khai Git
- thùng rác của website trong **Tệp tin** và các file bị trình quét mã độc cách ly
- các cơ sở dữ liệu gắn với website, nếu bạn đã chọn **Xóa luôn các database của website**

Nefo giữ lại các bản sao lưu đã có: chúng vẫn nằm ở trang **Sao lưu**.

Bản staging WordPress: xóa một website chính đang có bản staging thì bản staging vẫn còn, nhưng không đẩy lên website chính nào được nữa; hộp thoại có báo điều này. Xóa bản staging không làm thay đổi website chính của nó.

Khi website đang được triển khai từ Git, Nefo từ chối xóa ("Máy chủ đang chạy một tác vụ khác. Hãy chờ tác vụ đó xong."). Khi có sao lưu, tác vụ chờ lần triển khai đã bắt đầu trong lúc sao lưu chạy xong. Nếu một bước dọn dẹp bị lỗi, website vẫn bị gỡ khỏi panel và Nefo liệt kê các bước lỗi dưới dòng "Một vài bước dọn dẹp bị lỗi:" (trong nhật ký của tác vụ, nếu đã sao lưu trước).

Qua API: `DELETE /api/sites/{id}` với `{"confirm": "example.com", "delete_databases": false, "backup_first": true}`. Với `backup_first`, API trả về 202 kèm tác vụ. Không có nó, website bị xóa ngay (200, kèm các cảnh báo).
