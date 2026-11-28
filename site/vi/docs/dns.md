---
title: DNS (Cloudflare)
description: Quản lý bản ghi DNS của các zone Cloudflare ngay trong panel, trỏ các tên của website về máy chủ này, thêm bản ghi MX, SPF và DMARC của nhà cung cấp email, và xóa bộ nhớ đệm của Cloudflare.
---

# DNS (Cloudflare) {#dns-cloudflare}

Nefo không chạy máy chủ DNS. Nếu DNS của tên miền đặt ở Cloudflare, Nefo quản lý bản ghi của nó qua API của Cloudflare bằng một API token: thêm, sửa và xóa bản ghi, trỏ các tên của website về máy chủ này sau khi cho bạn xem trước, thêm bản ghi MX, SPF và DMARC của nhà cung cấp email, và xóa bộ nhớ đệm của Cloudflare.

Bạn lưu token ở **Cài đặt** → **Cloudflare**. Sau đó:

- trang **DNS**, trong nhóm **Lưu trữ** ở menu bên trái, liệt kê các zone và bản ghi của chúng, thêm, sửa, xóa bản ghi, và xóa bộ nhớ đệm của Cloudflare;
- nút **Trỏ DNS về đây…** ở tab **Tên miền** của website trỏ các tên của website về máy chủ này chỉ trong một bước;
- nút **Cài đặt email** ở trang DNS thêm bản ghi MX, SPF và DMARC của một nhà cung cấp email;
- chứng chỉ wildcard được xác thực bằng chính token này: xem [Chứng chỉ SSL](./ssl#wildcard-certificates-through-cloudflare-dns).

Nefo chỉ làm việc với API của Cloudflare, và token không bao giờ xuất hiện trong nhật ký hay thông báo của panel.

## API token {#the-api-token}

Một token dùng cho tất cả những việc trên. Token cần các quyền sau, cho những zone (tên miền) bạn dùng trên máy chủ này:

| Quyền | Dùng để |
|---|---|
| Zone → DNS → Edit | Liệt kê, thêm, sửa và xóa bản ghi, **Trỏ DNS về đây…**, và xác thực chứng chỉ qua DNS. |
| Zone → Zone → Read | Liệt kê các zone kèm trạng thái và gói. Thiếu quyền này, danh sách zone có thể trống, hoặc Cloudflare từ chối liệt kê các zone. |
| Zone → Cache Purge → Purge | Nút **Xóa bộ nhớ đệm** ở trang DNS. |

1. Trong trang quản trị Cloudflare, mở **My Profile → API Tokens → Create Token**.
2. Bắt đầu từ mẫu **Edit zone DNS**. Mẫu này có sẵn quyền Zone → DNS → Edit.
3. Bấm **+ Add more** và thêm Zone → Zone → Read cùng Zone → Cache Purge → Purge.
4. Ở mục **Zone Resources**, chọn các zone bạn dùng ở đây, hoặc mọi zone của tài khoản.
5. Tạo token, sao chép, rồi lưu vào Nefo: **Cài đặt** → **Cloudflare** → **API token** → **Kiểm tra và lưu**.

Khi token thiếu quyền, Nefo nói rõ quyền nào, ví dụ: "Cloudflare từ chối: API token thiếu quyền Zone → DNS → Edit cho zone này." Hãy sửa token trên Cloudflare (**My Profile → API Tokens**, rồi **Edit** cạnh token), thêm quyền cho zone và lưu. Sửa token không làm đổi giá trị của nó, nên không cần thay gì trong Nefo. Nếu bạn đã xóa hoặc đổi (roll) token, hãy lưu token mới ở **Cài đặt** → **Cloudflare**.

## Trang DNS {#the-dns-page}

Mỗi lần trang hiện một zone. Khi có nhiều zone, hãy chọn zone trong danh sách ở trên cùng; bên cạnh là trạng thái (**Đang hoạt động**, hoặc **Đang chờ** khi name server của tên miền chưa phải của Cloudflare) và gói của zone.

Bảng liệt kê các bản ghi của zone: **Loại**, **Tên** (`@` là chính zone, `www` là `www.example.com`), **Nội dung**, **Proxy** và **TTL**. Bạn có thể lọc theo loại, hoặc tìm theo tên và nội dung. Zone có hơn 5.000 bản ghi chỉ hiện 5.000 bản ghi đầu; hãy dùng trang quản trị của Cloudflare cho phần còn lại.

Nefo thêm và sửa được bản ghi A, AAAA, CNAME, TXT, MX và CAA. Các loại khác, như NS hay SRV, được liệt kê kèm chữ "trên Cloudflare": hãy sửa chúng trong trang quản trị của Cloudflare.

### Thêm hoặc sửa bản ghi {#add-or-change-a-record}

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

### Qua proxy hay Chỉ DNS {#proxied-or-dns-only}

Bản ghi A, AAAA và CNAME có thể **Qua proxy** (đám mây màu cam của Cloudflare): khách truy cập tới Cloudflare, Cloudflare chuyển tiếp yêu cầu web tới địa chỉ đó và giấu địa chỉ đi. Bản ghi qua proxy luôn dùng TTL tự động. **Chỉ DNS** (đám mây xám) trả lời thẳng bằng địa chỉ. Các loại bản ghi khác luôn là Chỉ DNS.

Proxy của Cloudflare chỉ chuyển tiếp HTTP và HTTPS ở cổng 80 và 443 (cùng vài cổng khác). Mọi thứ khác hãy để Chỉ DNS: SSH, thư, và panel.

::: warning Để tên của panel ở chế độ Chỉ DNS
Nếu bạn mở panel qua một tên như `panel.example.com`, hãy để bản ghi đó ở chế độ **Chỉ DNS**. Proxy của Cloudflare không chuyển tiếp cổng của panel, nên khi bật đám mây màu cam, panel không còn mở được qua tên đó nữa. Trang DNS và **Trỏ DNS về đây…** sẽ cảnh báo khi bản ghi này đang qua proxy.
:::

## Trỏ website về máy chủ này {#point-a-website-to-this-server}

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

### Let's Encrypt khi bật đám mây màu cam {#let-s-encrypt-behind-the-orange-cloud}

Let's Encrypt kiểm tra website qua HTTP thường. Khi bật proxy, tùy chọn **Always Use HTTPS** của Cloudflare (**SSL/TLS → Edge Certificates**) trả lời bước kiểm tra đó bằng một lệnh chuyển hướng trước khi nó tới máy chủ, và việc xin chứng chỉ thất bại. Hãy chọn một trong hai cách:

- tắt **Always Use HTTPS** trên Cloudflare, và bật [Chuyển HTTP sang HTTPS](./ssl#redirect-http-to-https) trong Nefo thay cho nó; hoặc
- tích **Xác thực qua DNS (Cloudflare)** ở tab **SSL / HTTPS** của website, và giữ nguyên cài đặt của Cloudflare.

Khi website đã có chứng chỉ, hãy đặt chế độ mã hóa **SSL/TLS** của Cloudflare là **Full (strict)**. Với **Flexible**, Cloudflare kết nối tới máy chủ qua HTTP, và **Chuyển HTTP sang HTTPS** lại đẩy nó về HTTPS, thành một vòng lặp chuyển hướng.

## Làm sao thiết lập bản ghi DNS cho email? {#set-up-email-records}

Nefo không lưu hộp thư (xem [Thư và cảnh báo](./mail-alerts)). Nếu email của bạn đặt ở Google Workspace, Microsoft 365, Zoho Mail hoặc một nhà cung cấp khác, hoặc được Cloudflare Email Routing chuyển tiếp, Nefo có thể thêm các bản ghi DNS mà nhà cung cấp cần:

| Bản ghi | Tác dụng |
|---|---|
| MX | Cho biết máy chủ nào nhận thư của tên miền. |
| SPF (một bản ghi TXT) | Cho biết máy chủ nào được phép gửi thư thay tên miền. |
| DMARC (một bản ghi TXT ở `_dmarc`) | Cho máy chủ nhận biết phải làm gì với thư không qua được bước kiểm tra SPF và DKIM, và gửi báo cáo về đâu. |
| DKIM (một bản ghi TXT hoặc CNAME) | Công bố khóa mà nhà cung cấp dùng để ký thư. |

Không có gì thay đổi cho đến khi bạn đã xem danh sách thay đổi và áp dụng nó.

1. Mở **DNS** và chọn zone.
2. Bấm **Cài đặt email**.
3. Ở mục **Nhà cung cấp email**, chọn **Google Workspace**, **Microsoft 365**, **Zoho Mail**, **Cloudflare Email Routing** hoặc **Nhà cung cấp khác**.
4. Tùy chọn: ở ô **Tên miền email**, nhập một tên trong zone, ví dụ `mail` cho `mail.example.com`. Để trống nếu dùng chính zone.
5. Tùy chọn: nhập **Địa chỉ nhận báo cáo DMARC (không bắt buộc)**, và dán bản ghi DKIM nếu bạn đã có (xem bên dưới).
6. Bấm **Xem các thay đổi**. Bảng liệt kê từng bản ghi với giá trị **Hiện tại** và **Sau khi đổi**, và việc sẽ xảy ra với nó: mới, thay đổi, xóa, giữ nguyên, hoặc nhà cung cấp khác.
7. Bấm nút áp dụng, ví dụ **Áp dụng 4 thay đổi**.

Nếu bạn sửa một ô sau khi xem trước, hãy bấm **Xem trước lại** trước khi áp dụng. Nếu bản ghi trên Cloudflare đã đổi trong lúc đó, Nefo từ chối và hiện bản xem trước mới.

### Mỗi nhà cung cấp nhận những gì {#what-each-provider-gets}

| Nhà cung cấp | Bản ghi MX | SPF include |
|---|---|---|
| **Google Workspace** | `smtp.google.com`, độ ưu tiên 1 | `include:_spf.google.com` |
| **Microsoft 365** | `example-com.mail.protection.outlook.com` (tên miền của bạn, dấu chấm đổi thành dấu gạch ngang), độ ưu tiên 0 | `include:spf.protection.outlook.com` |
| **Zoho Mail** | `mx.zoho.com` (10), `mx2.zoho.com` (20), `mx3.zoho.com` (50), với tên miền của **Trung tâm dữ liệu Zoho** đã chọn: zoho.com, zoho.eu, zoho.in hoặc zoho.com.au | `include:zohomail.com`, hoặc `zohomail.eu`, `zohomail.in`, `zohomail.com.au` |
| **Cloudflare Email Routing** | `route1.mx.cloudflare.net`, `route2.mx.cloudflare.net`, `route3.mx.cloudflare.net` | `include:_spf.mx.cloudflare.net` |
| **Nhà cung cấp khác** | Các máy chủ bạn nhập ở mục **Máy chủ thư (MX)**, tối đa 5, mỗi máy chủ có một độ ưu tiên | Tên miền ở ô **SPF include (không bắt buộc)** và các địa chỉ ở ô **Địa chỉ gửi thư (không bắt buộc)**; nếu không có cả hai thì là `mx` |

Một vài chi tiết:

- **Google Workspace**: nếu zone đã có các bản ghi MX cũ của Google (`aspmx.l.google.com` và các bản ghi khác), Nefo giữ chúng và không thêm gì.
- **Microsoft 365**: Microsoft cấp cho mỗi tên miền một máy chủ MX riêng. Hãy so giá trị trong bản xem trước với giá trị mà Microsoft 365 admin center hiện cho tên miền. Nếu khác, hãy chọn **Nhà cung cấp khác** và nhập giá trị đó. Bản ghi MX đã trỏ về Microsoft 365 được giữ nguyên.
- **Zoho Mail**: chọn trung tâm dữ liệu có địa chỉ mà bạn dùng để đăng nhập. Với một trung tâm dữ liệu Zoho khác, hãy chọn **Nhà cung cấp khác** và nhập các giá trị Zoho đưa ra.
- **Cloudflare Email Routing**: thư chỉ về tới sau khi bạn bật Email Routing cho zone trong trang quản trị Cloudflare và thêm một địa chỉ nhận ở đó. Các bản ghi định tuyến có sẵn của Cloudflare giữ nguyên độ ưu tiên mà Cloudflare đã đặt.

### SPF: luôn chỉ một bản ghi {#spf-always-one-record}

Một tên miền phải có đúng một bản ghi SPF; có hai bản ghi thì bước kiểm tra SPF sẽ lỗi. Nefo không bao giờ thêm bản ghi thứ hai:

- Nếu zone đã có bản ghi SPF, Nefo giữ mọi thứ trong đó, thêm include của nhà cung cấp, và giữ phần kết thúc của bản ghi (`-all`, `~all` hoặc `?all`). Bản ghi không có phần kết thúc sẽ được thêm `~all`.
- Nếu zone có nhiều bản ghi SPF, Nefo gộp chúng thành một: sửa bản ghi đầu tiên và xóa các bản ghi còn lại.
- Nếu bạn đã thiết lập SMTP relay ở **Cài đặt** → **Email**, bản ghi SPF cũng cho phép nhà cung cấp của relay (ví dụ `include:sendgrid.net` cho SendGrid, `include:mailgun.org` cho Mailgun, `include:amazonses.com` cho Amazon SES), để thư mà website gửi đi qua được bước kiểm tra. Bản xem trước có ghi rõ điều này.
- Bản ghi SPF cần hơn 10 lượt tra cứu DNS (`include`, `a`, `mx`, `ptr`, `exists`, `redirect`) sẽ bị lỗi ở nhiều máy chủ nhận. Bản xem trước sẽ cảnh báo bạn; hãy bỏ các include không còn dùng.

### Bản ghi MX của nhà cung cấp khác {#mx-records-of-another-provider}

Bản ghi MX trỏ tới nhà cung cấp khác được đánh dấu "nhà cung cấp khác". Thư sẽ bị chia cho cả hai nhà cung cấp, nên phải xóa các bản ghi này: tích **Thay các bản ghi MX hiện có** để xóa chúng. Nefo từ chối áp dụng thay đổi cho đến khi bạn tích ô này, hoặc tự xóa các bản ghi đó.

::: warning
Nhà cung cấp cũ không nhận được thư nữa ngay khi bản ghi MX của nó bị xóa. Hãy chuyển hộp thư đi trước.
:::

### DMARC {#dmarc}

Nefo thêm `v=DMARC1; p=none` ở `_dmarc`, kèm `rua=mailto:` và địa chỉ của bạn khi bạn nhập **Địa chỉ nhận báo cáo DMARC (không bắt buộc)**. Với `p=none`, không thư nào bị từ chối: máy chủ nhận chỉ gửi báo cáo. Khi báo cáo cho thấy mọi thư của bạn đều qua được, bạn có thể đặt chính sách chặt hơn (`p=quarantine`, rồi `p=reject`) bằng cách sửa bản ghi ở trang DNS.

Nếu zone đã có bản ghi DMARC, Nefo để nguyên nó.

### DKIM {#dkim}

Khóa DKIM do nhà cung cấp email tạo, nên Nefo không tạo được. Ở mục **Chữ ký DKIM**, hộp thoại cho biết chỗ lấy khóa ở nhà cung cấp của bạn:

- **Google Workspace**: trong Google Admin console, mở Apps → Google Workspace → Gmail → Authenticate email, rồi tạo bản ghi mới. Tên của bản ghi là `google._domainkey` và loại là TXT.
- **Microsoft 365**: trong Microsoft Defender portal, mở cài đặt DKIM của tên miền. Microsoft đưa ra hai bản ghi CNAME, `selector1._domainkey` và `selector2._domainkey`; hộp thoại có một dòng cho mỗi bản ghi.
- **Zoho Mail**: trong Zoho Mail Admin Console, mở cài đặt DKIM của tên miền và thêm một selector. Nhập tên selector theo sau là `._domainkey`, và giá trị TXT.
- **Cloudflare Email Routing** không cần bản ghi DKIM ở đây: Cloudflare tự thêm những gì cần khi bạn bật Email Routing.
- **Nhà cung cấp khác**: hãy hỏi nhà cung cấp bản ghi DKIM của họ.

Dán **Tên bản ghi** và **Giá trị**, chọn **Loại** (TXT hoặc CNAME), rồi xem trước. Một tên như `google._domainkey` nằm dưới tên miền email; tên đầy đủ cũng được chấp nhận. Bản ghi DKIM cùng tên sẽ bị thay. Để trống giá trị nếu muốn thêm bản ghi DKIM sau: khi đã có, hãy chạy lại **Cài đặt email** với cùng nhà cung cấp. Sau đó hoàn tất thiết lập ở nhà cung cấp (Google: **Start authentication**; Microsoft và Zoho: bật DKIM hoặc xác minh nó).

Chạy lại **Cài đặt email** không gây hại gì: bản ghi đã đúng sẽ hiện là "giữ nguyên".

Mọi bản ghi mà Nefo thêm, sửa hoặc xóa đều được ghi vào nhật ký hoạt động, như các thay đổi DNS khác.

## Xóa bộ nhớ đệm Cloudflare {#purge-cloudflare-s-cache}

Khi bật proxy, Cloudflare giữ bản sao các tệp của bạn. Sau khi sửa chúng, hãy bấm **Xóa bộ nhớ đệm** ở trang DNS:

- **Toàn bộ** xóa hết bộ nhớ đệm của Cloudflare cho cả zone. Sau đó Cloudflare tải lại mọi tệp từ máy chủ, nên website có thể chậm hơn trong vài phút;
- **Các URL này** xóa tối đa 30 địa chỉ của zone mỗi lần, mỗi dòng một địa chỉ, ví dụ `https://example.com/style.css`.

Đây là bộ nhớ đệm của Cloudflare, không phải [bộ nhớ đệm trang](./websites#page-cache) của website trên máy chủ này.

## Nhật ký hoạt động {#activity-log}

Mọi thay đổi đều được ghi vào nhật ký hoạt động (**Tài khoản** → **Hoạt động gần đây**): "Thêm bản ghi DNS", "Sửa bản ghi DNS", "Xóa bản ghi DNS" và "Xóa bộ nhớ đệm Cloudflare", kèm loại, tên và nội dung của bản ghi. **Trỏ DNS về đây…** ghi lại từng bản ghi nó thay đổi.

## Các thông báo {#messages}

| Thông báo | Cách xử lý |
|---|---|
| `Chưa lưu API token Cloudflare.` | Lưu token ở **Cài đặt** → **Cloudflare**. |
| `Cloudflare từ chối: API token thiếu quyền …` | Thêm quyền được nêu vào token: xem [API token](#the-api-token). |
| `Cloudflare không chấp nhận token này.` | Token sai, hết hạn hoặc đã bị xóa. Hãy tạo token mới. |
| `Không tên nào trong số này thuộc một zone Cloudflare mà API token thấy được` | Thêm zone vào mục **Zone Resources** của token, hoặc tạo bản ghi ở nơi đang quản lý DNS của tên miền. |
| `Cloudflare chưa trả lời cho example.com (pending).` | Đổi name server của tên miền ở nhà đăng ký sang hai name server mà Cloudflare đưa ra. |
| `Cloudflare từ chối: …` | Thông báo của chính Cloudflare, ví dụ khi một bản ghi CNAME và một bản ghi A trùng tên. |
