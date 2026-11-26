---
title: DNS (Cloudflare)
description: Quản lý bản ghi DNS của các zone Cloudflare ngay trong panel, trỏ các tên của website về máy chủ này, và xóa bộ nhớ đệm của Cloudflare.
---

# DNS (Cloudflare) {#dns-cloudflare}

Nefo không chạy máy chủ DNS. Nếu DNS của tên miền đặt ở Cloudflare, Nefo quản lý nó qua API của Cloudflare bằng token bạn lưu ở **Cài đặt** → **Cloudflare**:

- trang **DNS**, trong nhóm **Hosting** ở menu bên trái, liệt kê các zone và bản ghi của chúng, thêm, sửa, xóa bản ghi, và xóa bộ nhớ đệm của Cloudflare;
- nút **Trỏ DNS về đây…** ở tab **Tên miền** của website trỏ các tên của website về máy chủ này chỉ trong một bước;
- chứng chỉ wildcard được xác thực bằng chính token này: xem [Chứng chỉ SSL](./ssl#chung-chi-wildcard).

Nefo chỉ làm việc với API của Cloudflare, và token không bao giờ xuất hiện trong nhật ký hay thông báo của panel.

## API token {#token}

Một token dùng cho tất cả những việc trên. Token cần các quyền sau, cho những zone (tên miền) bạn dùng trên máy chủ này:

| Quyền | Dùng để |
|---|---|
| Zone → DNS → Edit | Liệt kê, thêm, sửa và xóa bản ghi, **Trỏ DNS về đây…**, và xác thực chứng chỉ qua DNS. |
| Zone → Zone → Read | Liệt kê các zone kèm trạng thái và gói. Hãy thêm quyền này nếu danh sách zone trống hoặc bị Cloudflare từ chối. |
| Zone → Cache Purge → Purge | Nút **Xóa bộ nhớ đệm** ở trang DNS. |

1. Trong trang quản trị Cloudflare, mở **My Profile → API Tokens → Create Token**.
2. Bắt đầu từ mẫu **Edit zone DNS**. Mẫu này có sẵn quyền Zone → DNS → Edit.
3. Bấm **+ Add more** và thêm Zone → Zone → Read cùng Zone → Cache Purge → Purge.
4. Ở mục **Zone Resources**, chọn các zone bạn dùng ở đây, hoặc mọi zone của tài khoản.
5. Tạo token, sao chép, rồi lưu vào Nefo: **Cài đặt** → **Cloudflare** → **API token** → **Kiểm tra và lưu**.

Khi token thiếu quyền, Nefo nói rõ quyền nào, ví dụ: "Cloudflare từ chối: API token thiếu quyền Zone → DNS → Edit cho zone này." Hãy sửa token trên Cloudflare (**My Profile → API Tokens**, rồi **Edit** cạnh token), thêm quyền cho zone và lưu. Sửa token không làm đổi giá trị của nó, nên không cần thay gì trong Nefo. Nếu bạn đã xóa hoặc đổi (roll) token, hãy lưu token mới ở **Cài đặt** → **Cloudflare**.

## Trang DNS {#dns-page}

Mỗi lần trang hiện một zone. Khi có nhiều zone, hãy chọn zone trong danh sách ở trên cùng; bên cạnh là trạng thái (**Đang hoạt động**, hoặc **Đang chờ** khi name server của tên miền chưa phải của Cloudflare) và gói của zone.

Bảng liệt kê các bản ghi của zone: **Loại**, **Tên** (`@` là chính zone, `www` là `www.example.com`), **Nội dung**, **Proxy** và **TTL**. Bạn có thể lọc theo loại, hoặc tìm theo tên và nội dung. Zone có hơn 5.000 bản ghi chỉ hiện 5.000 bản ghi đầu; hãy dùng trang quản trị của Cloudflare cho phần còn lại.

Nefo thêm và sửa được bản ghi A, AAAA, CNAME, TXT, MX và CAA. Các loại khác, như NS hay SRV, được liệt kê kèm chữ "trên Cloudflare": hãy sửa chúng trong trang quản trị của Cloudflare.

### Thêm hoặc sửa bản ghi {#records}

Bấm **Thêm bản ghi**, hoặc **Sửa** ở dòng của bản ghi. Không đổi được loại của bản ghi: hãy xóa nó rồi thêm bản ghi mới. Nefo kiểm tra bản ghi trước khi gửi sang Cloudflare:

| Loại | Cần nhập |
|---|---|
| A | Một địa chỉ IPv4, ví dụ `203.0.113.10`. |
| AAAA | Một địa chỉ IPv6, ví dụ `2001:db8::10`. |
| CNAME | Tên máy mà tên này là bí danh của nó, ví dụ `example.com`. Không được trỏ về chính tên của nó. |
| TXT | Một dòng chữ, tối đa 2.048 ký tự: chữ thường (Cloudflare tự tách thành các chuỗi 255 ký tự), hoặc các `"chuỗi trong ngoặc kép"` dài tối đa 255 ký tự mỗi chuỗi. Hãy đặt cả giá trị trong ngoặc kép hoặc không dùng ngoặc kép. |
| MX | Tên máy chủ thư, ví dụ `mail.example.com`, và **Độ ưu tiên** từ 0 đến 65535 (số nhỏ hơn được thử trước). |
| CAA | Một **Tag**: `issue` hoặc `issuewild` kèm tên miền của CA, ví dụ `letsencrypt.org` (`;` là không cho CA nào), hoặc `iodef` kèm địa chỉ `mailto:` hay `https://`. **Cờ (flags)** là 0, hoặc 128 cho critical. |

**TTL** là **Tự động** (Cloudflare tự chọn) hoặc một thời gian cố định từ 1 phút đến 1 ngày.

### Qua proxy hay Chỉ DNS {#proxy}

Bản ghi A, AAAA và CNAME có thể **Qua proxy** (đám mây màu cam của Cloudflare): khách truy cập tới Cloudflare, Cloudflare chuyển tiếp yêu cầu web tới địa chỉ đó và giấu địa chỉ đi. Bản ghi qua proxy luôn dùng TTL tự động. **Chỉ DNS** (đám mây xám) trả lời thẳng bằng địa chỉ. Các loại bản ghi khác luôn là Chỉ DNS.

Proxy của Cloudflare chỉ chuyển tiếp HTTP và HTTPS ở cổng 80 và 443 (cùng vài cổng khác). Mọi thứ khác hãy để Chỉ DNS: SSH, thư, và panel.

::: warning Để tên của panel ở chế độ Chỉ DNS
Nếu bạn mở panel qua một tên như `panel.example.com`, hãy để bản ghi đó ở chế độ **Chỉ DNS**. Proxy của Cloudflare không chuyển tiếp cổng của panel, nên khi bật đám mây màu cam, panel không còn mở được qua tên đó nữa. Trang DNS và **Trỏ DNS về đây…** sẽ cảnh báo khi bản ghi này đang qua proxy.
:::

## Trỏ website về máy chủ này {#point}

1. Mở **Website**, rồi tab **Tên miền** của website.
2. Ở phần **DNS trên Cloudflare**, bấm **Trỏ DNS về đây…**.
3. Kiểm tra địa chỉ và các lựa chọn:
   - **Địa chỉ IPv4 công khai của máy chủ**, đọc từ các card mạng của máy chủ;
   - **Thêm cả địa chỉ IPv6 (AAAA)**, được tích sẵn khi máy chủ có địa chỉ IPv6 công khai;
   - **Qua proxy của Cloudflare (đám mây màu cam)**.
4. Đọc bản xem trước: với mỗi tên và loại bản ghi là bản ghi **Hiện tại** và **Sau khi đổi**, đánh dấu **mới**, **thay đổi**, **xóa** hoặc **giữ nguyên**.
5. Bấm **Áp dụng**.

Nefo chỉ sửa bản ghi của các tên thuộc website, tức tên miền chính và các tên miền phụ; bản ghi của tên khác không bao giờ bị động tới. Với mỗi tên:

- một bản ghi A trỏ về máy chủ này: bản ghi A có sẵn được cập nhật, hoặc một bản ghi mới được tạo, và các bản ghi A khác của tên đó bị xóa;
- khi có IPv6, bản ghi AAAA cũng được xử lý như vậy. Khi không có, bản ghi AAAA trỏ về máy chủ khác bị xóa, vì khách dùng IPv6 và Let's Encrypt sẽ tới máy chủ kia thay vì máy chủ này;
- bản ghi CNAME được thay bằng bản ghi A, trừ khi nó trỏ về một tên khác của website (`www` → `example.com` được giữ nguyên);
- tên không thuộc zone nào mà token thấy được thì không bị động tới, và bản xem trước liệt kê chúng.

Khi bạn bấm **Áp dụng**, Nefo tính lại các thay đổi. Nếu bản ghi trên Cloudflare đã đổi từ lúc xem trước, không có gì bị sửa và Nefo hiện bản xem trước mới: "Bản ghi trên Cloudflare đã thay đổi từ lúc xem trước."

Nếu máy chủ nằm sau NAT và chỉ có địa chỉ IPv4 nội bộ, Nefo báo "Không tìm thấy địa chỉ IPv4 công khai trên các card mạng của máy chủ". Hãy nhập địa chỉ công khai dẫn tới máy chủ. Nefo không hỏi dịch vụ bên ngoài nào để tìm địa chỉ này.

### Let's Encrypt khi bật đám mây màu cam {#lets-encrypt}

Let's Encrypt kiểm tra website qua HTTP thường. Khi bật proxy, tùy chọn **Always Use HTTPS** của Cloudflare (**SSL/TLS → Edge Certificates**) trả lời bước kiểm tra đó bằng một lệnh chuyển hướng trước khi nó tới máy chủ, và việc xin chứng chỉ thất bại. Hãy chọn một trong hai cách:

- tắt **Always Use HTTPS** trên Cloudflare, và bật [Chuyển HTTP sang HTTPS](./ssl#chuyen-http-sang-https) trong Nefo thay cho nó; hoặc
- tích **Xác thực qua DNS (Cloudflare)** ở tab **SSL / HTTPS** của website, và giữ nguyên cài đặt của Cloudflare.

Khi website đã có chứng chỉ, hãy đặt chế độ mã hóa **SSL/TLS** của Cloudflare là **Full (strict)**. Với **Flexible**, Cloudflare kết nối tới máy chủ qua HTTP, và **Chuyển HTTP sang HTTPS** lại đẩy nó về HTTPS, thành một vòng lặp chuyển hướng.

## Xóa bộ nhớ đệm Cloudflare {#purge}

Khi bật proxy, Cloudflare giữ bản sao các tệp của bạn. Sau khi sửa chúng, hãy bấm **Xóa bộ nhớ đệm** ở trang DNS:

- **Toàn bộ** xóa hết bộ nhớ đệm của Cloudflare cho cả zone. Sau đó Cloudflare tải lại mọi tệp từ máy chủ, nên website có thể chậm hơn trong vài phút;
- **Các URL này** xóa tối đa 30 địa chỉ của zone mỗi lần, mỗi dòng một địa chỉ, ví dụ `https://example.com/style.css`.

Đây là bộ nhớ đệm của Cloudflare, không phải [bộ nhớ đệm trang](./websites#bo-nho-dem-trang) của website trên máy chủ này.

## Nhật ký hoạt động {#audit}

Mọi thay đổi đều được ghi vào nhật ký hoạt động (**Tài khoản** → **Hoạt động gần đây**): "Thêm bản ghi DNS", "Sửa bản ghi DNS", "Xóa bản ghi DNS" và "Xóa bộ nhớ đệm Cloudflare", kèm loại, tên và nội dung của bản ghi. **Trỏ DNS về đây…** ghi lại từng bản ghi nó thay đổi.

## Các thông báo {#messages}

| Thông báo | Cách xử lý |
|---|---|
| `Chưa lưu API token Cloudflare.` | Lưu token ở **Cài đặt** → **Cloudflare**. |
| `Cloudflare từ chối: API token thiếu quyền …` | Thêm quyền được nêu vào token: xem [API token](#token). |
| `Cloudflare không chấp nhận token này.` | Token sai, hết hạn hoặc đã bị xóa. Hãy tạo token mới. |
| `Không tên nào trong số này thuộc một zone Cloudflare mà API token thấy được` | Thêm zone vào mục **Zone Resources** của token, hoặc tạo bản ghi ở nơi đang quản lý DNS của tên miền. |
| `Cloudflare chưa trả lời cho example.com (pending).` | Đổi name server của tên miền ở nhà đăng ký sang hai name server mà Cloudflare đưa ra. |
| `Cloudflare từ chối: …` | Thông báo của chính Cloudflare, ví dụ khi một bản ghi CNAME và một bản ghi A trùng tên. |
