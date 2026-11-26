---
title: Chứng chỉ SSL
description: Lấy chứng chỉ Let's Encrypt miễn phí, chứng chỉ wildcard qua DNS Cloudflare, hoặc cài chứng chỉ riêng, và xử lý các lỗi thường gặp.
---

# Chứng chỉ SSL

Nefo lấy chứng chỉ miễn phí từ Let's Encrypt và tự gia hạn. Trang này nói về các yêu cầu, chuyển hướng sang HTTPS và HSTS, chứng chỉ wildcard qua DNS Cloudflare, cách cài chứng chỉ bạn đã có, cách tắt HTTPS, và cách xử lý khi xin chứng chỉ bị lỗi.

## Trước khi bắt đầu

Với chứng chỉ thông thường, Let's Encrypt kiểm tra qua HTTP rằng từng tên của website trỏ đúng về máy chủ của bạn. Mọi tên đều phải đạt: tên miền chính và từng [tên miền phụ](./websites).

- **DNS:** mỗi tên có bản ghi A trỏ về IP của máy chủ này, ví dụ `203.0.113.10`. Nếu tên đó còn có bản ghi AAAA (IPv6), bản ghi này cũng phải trỏ về máy chủ này, hoặc hãy xóa nó.
- **Cổng 80:** máy chủ phản hồi ở cổng 80 từ Internet. Nefo đã mở cổng 80 và 443 trên ufw khi cài web server. Nếu nhà cung cấp VPS có tường lửa riêng, hãy mở cổng 80 ở đó nữa.
- **Proxy:** nếu DNS của bạn ở Cloudflare và đang bật proxy (đám mây màu cam), hãy đọc phần [Proxy của Cloudflare](#cloudflare-proxy) trước.

Khi bật HTTPS, bạn đồng ý với [Thỏa thuận thuê bao (Subscriber Agreement)](https://letsencrypt.org/repository/) của Let's Encrypt.

## Lấy chứng chỉ miễn phí

1. Mở **Website**, rồi tab **SSL / HTTPS** của website. Trong danh sách website, nút **Bật HTTPS** ở cột **SSL** cũng mở tab này.
2. Trong phần **Chứng chỉ miễn phí từ Let's Encrypt**, để trống ô **Xác thực qua DNS (Cloudflare)**.
3. Bấm **Lấy chứng chỉ**.

Nhật ký hiện từng bước: Nefo kiểm tra mọi tên đã trỏ về máy chủ, xin chứng chỉ từ Let's Encrypt, xác thực từng tên rồi bật HTTPS trong web server. Dòng cuối là "Done. The certificate is valid until" kèm ngày hết hạn.

Khi website có chứng chỉ lần đầu, Nefo cũng chuyển website WordPress do panel cài sang địa chỉ `https://`, và khởi động lại ứng dụng Docker với địa chỉ `https://`. Chứng chỉ được lưu trong `/var/lib/nefo/certs/example.com/` dưới tên `fullchain.pem` và `privkey.pem`.

### Bước kiểm tra trước khi xin {#kiem-tra-truoc}

Trước khi gửi yêu cầu tới Let's Encrypt, Nefo đặt một file thử lên máy chủ rồi tải nó qua từng tên bằng HTTP. Nếu một tên không đạt, Nefo không gửi yêu cầu nào, nên lần thử hỏng không bị tính vào giới hạn của Let's Encrypt. Nhật ký khi đó ghi "these names do not reach this server yet, so no certificate was requested:" và mỗi tên một dòng:

| Thông báo | Cần sửa gì |
|---|---|
| `blog.example.com: no DNS record. Add an A record that points to this server's IP address.` | Thêm bản ghi A và chờ đến khi nó có hiệu lực. |
| `example.com (203.0.113.10): no answer over HTTP on port 80 (…). Check the DNS record and the firewall.` | Mở cổng 80 trên tường lửa của nhà cung cấp, và kiểm tra web server đang chạy. |
| `example.com (198.51.100.7) answered HTTP 404, but not from this server. Check that the DNS record points here.` | Tên miền đang trỏ về máy chủ khác: IP cũ, bản ghi AAAA trỏ nơi khác, hoặc một proxy như Cloudflare. |

Địa chỉ trong ngoặc là địa chỉ mà tên miền phân giải ra. Hãy so với IP của máy chủ.

## Chuyển HTTP sang HTTPS

Khi website đã có chứng chỉ, hãy bật **Chuyển HTTP sang HTTPS** ở tab **SSL / HTTPS** hoặc tab **Tổng quan**. Mọi địa chỉ `http://` sẽ được chuyển hướng vĩnh viễn (301) sang `https://`. Việc xác thực của Let's Encrypt và webhook Git vẫn hoạt động qua HTTP.

Để trình duyệt dùng HTTPS ngay cả trước khi được chuyển hướng, hãy bật thêm [HSTS](#hsts).

## HSTS {#hsts}

HSTS (header `Strict-Transport-Security`) báo cho trình duyệt chỉ mở site qua `https://`, kể cả khi ai đó gõ `http://` hay bấm vào một liên kết cũ. Hãy bật **HSTS (Strict-Transport-Security)** ở tab **SSL / HTTPS**. HSTS cần có chứng chỉ và **Chuyển HTTP sang HTTPS**; tắt chuyển hướng thì HSTS cũng tắt.

Khi đó Nefo gửi `Strict-Transport-Security: max-age=15552000` (180 ngày, khoảng 6 tháng) kèm các phản hồi HTTPS của site, trên Nginx, Apache và OpenLiteSpeed. Đánh dấu **Áp dụng cho cả tên miền con** để thêm `includeSubDomains`: trình duyệt sẽ dùng HTTPS cho mọi tên miền con của tên miền đó, nên chỉ đánh dấu khi tất cả chúng đều có HTTPS. Nefo không bao giờ thêm `preload`: rất khó ra khỏi danh sách preload của trình duyệt, nên nếu muốn thì bạn tự gửi tên miền vào đó.

::: warning
Trình duyệt đã nhận header sẽ từ chối HTTP thường cho site trong 6 tháng kể từ lần truy cập cuối, kể cả khi sau này bạn tắt HSTS hay HTTPS. Chỉ bật HSTS cho site sẽ giữ HTTPS lâu dài.
:::

Trên Nginx, header được đặt cho cả khối `server` của site. Nếu chỉ thị riêng của bạn (xem [Website](./websites#chi-thi-rieng)) thêm một `location` có `add_header` riêng, Nginx bỏ các header của site trong location đó: hãy lặp lại `add_header Strict-Transport-Security "max-age=15552000" always;` ở đó.

Để kiểm tra, chạy `curl -sI https://example.com/ | grep -i strict-transport`.

## Tự động gia hạn

Nefo kiểm tra chứng chỉ khi khởi động và sau đó mỗi giờ một lần. Chứng chỉ Let's Encrypt được gia hạn khi chỉ còn một phần ba thời hạn: 30 ngày trước khi hết hạn với chứng chỉ 90 ngày. Bạn không cần làm gì, và website đang tạm ngưng vẫn được gia hạn.

Nếu gia hạn lỗi, Nefo thử lại sau mỗi 12 giờ. Trong lúc đó:

- danh sách website ghi "Gia hạn lỗi" ở cột **SSL**;
- tab **SSL / HTTPS** hiện "Lần xin chứng chỉ gần nhất bị lỗi." kèm nội dung lỗi;
- một cảnh báo được gửi đi nếu bạn đã bật cảnh báo cho gia hạn lỗi: xem [Thư và cảnh báo](./mail-alerts).

Chứng chỉ của chính panel, lấy ở **Cài đặt** → **Chứng chỉ** (xem [Bảo mật](./security)), cũng được gia hạn như vậy: khi còn một phần ba thời hạn, và thử lại sau mỗi 12 giờ nếu bị lỗi. Mỗi lần gia hạn lỗi được ghi vào nhật ký hoạt động và gửi cùng loại cảnh báo đó.

Nefo đăng ký tài khoản Let's Encrypt không kèm địa chỉ email, nên Let's Encrypt sẽ không email cho bạn về chứng chỉ sắp hết hạn. Hãy dựa vào cảnh báo của panel.

Muốn gia hạn bằng tay, ví dụ sau khi đã sửa lỗi, hãy bấm **Xin lại chứng chỉ** ở tab **SSL / HTTPS**.

## Chứng chỉ wildcard qua DNS Cloudflare {#chung-chi-wildcard}

Tên wildcard như `*.example.com` chỉ xác thực được qua DNS. Nefo làm việc này qua API của Cloudflare, nên DNS của tên miền phải đặt ở Cloudflare. Xác thực DNS cũng dùng được cho website chưa truy cập được qua HTTP, ví dụ khi đứng sau proxy của Cloudflare. Khi xin chứng chỉ, Nefo chỉ thêm và xóa bản ghi TXT `_acme-challenge`.

### Tạo API token trên Cloudflare {#tao-api-token}

Token này cũng dùng cho [trang DNS](./dns). Các quyền của token được liệt kê ở một chỗ duy nhất, mục [API token](./dns#token): xin chứng chỉ chỉ cần Zone → DNS → Edit; trang DNS còn dùng thêm Zone → Zone → Read và Zone → Cache Purge → Purge.

1. Trong trang quản trị Cloudflare, mở **My Profile → API Tokens → Create Token**.
2. Dùng mẫu **Edit zone DNS**. Mẫu này cấp quyền Zone → DNS → Edit.
3. Ở phần zone resources, chọn các zone (tên miền) bạn dùng trên máy chủ này.
4. Tạo token và sao chép nó.

Xem hướng dẫn [tạo API token](https://developers.cloudflare.com/fundamentals/api/get-started/create-token/) của Cloudflare.

### Lưu token vào Nefo

1. Mở **Cài đặt**, rồi tab **Cloudflare**.
2. Dán token vào ô **API token** và bấm **Kiểm tra và lưu**.

Nefo kiểm tra Cloudflare chấp nhận token và token đang hoạt động, rồi hiện "Đã lưu token" kèm vài ký tự cuối của token và các **Zone** token dùng được. Dùng **Thay API token** để đổi token, hoặc **Xóa** để xóa nó.

### Lấy chứng chỉ

1. Với wildcard, mở tab **Tên miền** của website, thêm `*.example.com` vào ô **Tên miền phụ** rồi bấm **Lưu**.
2. Ở tab **SSL / HTTPS**, chọn **Xác thực qua DNS (Cloudflare)**. Với website có tên wildcard, ô này đã được chọn sẵn và không bỏ được: "Bắt buộc: site này có tên wildcard (*.domain)."
3. Bấm **Lấy chứng chỉ**.

Nefo thêm bản ghi TXT `_acme-challenge.example.com`, chờ tối đa 3 phút cho tới khi các name server của Cloudflare trả về bản ghi đó, xác thực, rồi xóa bản ghi. Website ghi nhớ lựa chọn này, nên các lần gia hạn sau cũng xác thực qua DNS.

Chứng chỉ chỉ bao phủ các tên; khách truy cập vẫn cần bản ghi DNS trỏ các tên miền con về máy chủ này.

## Dùng chứng chỉ riêng {#chung-chi-rieng}

Hãy cài chứng chỉ riêng khi bạn đã mua chứng chỉ từ nhà cung cấp khác, hoặc khi dùng chứng chỉ origin của Cloudflare.

1. Mở tab **SSL / HTTPS** của website, tới phần **Chứng chỉ của bạn**.
2. Dán **Chứng chỉ (PEM, kèm chứng chỉ trung gian)**: chứng chỉ rồi tới chain của nó. Bạn cũng có thể chọn file `.pem`, `.crt`, `.cer` hoặc `.txt`.
3. Dán **Khóa riêng (PEM, không mã hóa)**, hoặc chọn file `.pem`, `.key` hay `.txt`.
4. Bấm **Cài đặt**.

Nefo kiểm tra trước khi thay đổi bất cứ gì, và từ chối:

- file không phải chứng chỉ PEM, hoặc lớn hơn 64 KB;
- khóa riêng có mật khẩu (passphrase). Hãy gỡ trước, ví dụ bằng `openssl pkey -in key.pem -out plain.pem`;
- khóa không thuộc về chứng chỉ;
- chứng chỉ chưa tới ngày hiệu lực, hoặc đã hết hạn;
- chứng chỉ không bao gồm mọi tên của website. Tên wildcard của website phải có đúng như vậy trong chứng chỉ.

Nefo vẫn cài nhưng kèm cảnh báo ("Đã cài, có cảnh báo:") khi trình duyệt không tự tin cậy chứng chỉ, điều bình thường với chứng chỉ origin của Cloudflare dùng sau proxy của Cloudflare, hoặc khi chứng chỉ hết hạn trong vòng hai tuần.

Nefo không gia hạn chứng chỉ riêng. Trong hai tuần cuối, tab **SSL / HTTPS** ghi "Nefo không tự gia hạn chứng chỉ do bạn cài. Hãy cài chứng chỉ mới trước ngày" kèm ngày hết hạn, và mục **Cần xử lý** trên bảng điều khiển liệt kê website đó. Nefo cũng nhắc bạn mỗi ngày một lần: nhật ký hoạt động ghi lại như một lần gia hạn lỗi, và cảnh báo cho gia hạn lỗi được gửi đi. Hãy cài chứng chỉ mới kịp thời, hoặc chuyển lại Let's Encrypt bằng nút **Lấy chứng chỉ**. Danh sách website đánh dấu các website này bằng "cert riêng".

## Tắt HTTPS {#tat-https}

Để đưa website về HTTP thường, bấm **Tắt HTTPS** ở cuối tab **SSL / HTTPS** của website rồi xác nhận. Khi đó Nefo:

- chỉ phục vụ website qua `http://`: phần HTTPS trong cấu hình web server của nó bị bỏ;
- tắt **Chuyển HTTP sang HTTPS** và [HSTS](#hsts);
- xóa các file chứng chỉ trong `/var/lib/nefo/certs/example.com/`, dù từ Let's Encrypt hay của riêng bạn, nên không còn gì gia hạn hay nhắc bạn về chúng nữa. Tài khoản Let's Encrypt vẫn được giữ cho các website khác;
- chuyển website WordPress do Nefo cài về địa chỉ `http://`, và khởi động lại ứng dụng Docker với địa chỉ `http://`.

Khách mở một địa chỉ `https://` của site (từ bookmark, kết quả tìm kiếm hay một liên kết) sẽ gặp lỗi trình duyệt, vì máy chủ không còn chứng chỉ cho tên miền đó. Nếu site từng bật HSTS, trình duyệt đã nhận header sẽ tiếp tục từ chối HTTP thường tới 6 tháng, nên với họ site hoàn toàn không mở được cho đến khi có lại HTTPS. Bạn có thể lấy lại chứng chỉ bất cứ lúc nào bằng **Lấy chứng chỉ**.

Muốn thay chứng chỉ thì không cần tắt HTTPS: hãy lấy chứng chỉ Let's Encrypt mới hoặc cài chứng chỉ riêng đè lên. Xóa website thì chứng chỉ cũng bị xóa theo: xem [Website](./websites). Gỡ web server thì chứng chỉ vẫn được giữ.

## Khi xin chứng chỉ bị lỗi

Lỗi hiện ở tab **SSL / HTTPS** của website, dưới dòng "Lần xin chứng chỉ gần nhất bị lỗi.", và trong nhật ký của lần thử.

### DNS hoặc cổng 80

Xem [Bước kiểm tra trước khi xin](#kiem-tra-truoc). Phần lớn lỗi là do bản ghi DNS chưa trỏ về máy chủ này, hoặc cổng 80 bị đóng trên tường lửa của nhà cung cấp.

### Proxy của Cloudflare (đám mây màu cam) {#cloudflare-proxy}

Khi bật proxy của Cloudflare, tên miền phân giải ra địa chỉ của Cloudflare, và request tới Cloudflare trước rồi mới tới máy chủ. Các cài đặt của Cloudflare, như **Always Use HTTPS** (chuyển hướng sang HTTPS, ở **SSL/TLS → Edge Certificates**), có thể khiến bước kiểm tra không tới máy chủ nguyên vẹn, và Nefo báo tên đó "answered HTTP …, but not from this server". Xem thêm [Let's Encrypt khi bật đám mây màu cam](./dns#lets-encrypt). Bạn có ba cách:

- chọn **Xác thực qua DNS (Cloudflare)** và lấy chứng chỉ qua DNS. Proxy vẫn bật được, và việc gia hạn cũng chạy như vậy;
- cài chứng chỉ origin của Cloudflare làm [chứng chỉ riêng](#chung-chi-rieng);
- tắt proxy (DNS only, đám mây màu xám) cho tên đó. Việc gia hạn cũng kiểm tra qua HTTP, nên hãy để proxy tắt luôn.

### Bản ghi CAA

Nếu tên miền có bản ghi CAA trong DNS, một bản ghi phải cho phép `letsencrypt.org`. Nếu không, Let's Encrypt từ chối cấp chứng chỉ, và nhật ký hiện lỗi của Let's Encrypt có nhắc tới CAA. Xem [Let's Encrypt và CAA](https://letsencrypt.org/docs/caa/).

### Giới hạn tần suất

Let's Encrypt giới hạn số chứng chỉ và số lần xác thực lỗi cho mỗi tên miền và mỗi tài khoản. Khi chạm giới hạn, nhật ký hiện thông báo của Let's Encrypt kèm thời gian phải chờ. Xem [giới hạn của Let's Encrypt](https://letsencrypt.org/docs/rate-limits/).

::: tip
Muốn thử mà không tốn lượt, hãy thêm `"acme_directory": "https://acme-staging-v02.api.letsencrypt.org/directory"` vào `/etc/nefo/config.json` rồi khởi động lại panel bằng `systemctl restart nefo`. Chứng chỉ staging không được trình duyệt tin cậy. Xóa dòng đó và khởi động lại để lấy chứng chỉ thật.
:::

### Xác thực DNS qua Cloudflare

| Thông báo | Cần sửa gì |
|---|---|
| `DNS validation needs a Cloudflare API token: add one in Settings → Cloudflare DNS` | Lưu token ở trang **Cài đặt**, tab **Cloudflare**. |
| `Cloudflare không chấp nhận token này.` | Token sai, đã hết hạn hoặc không hoạt động. Hãy tạo token mới. |
| `none of the Cloudflare zones this API token can use holds example.com` | Thêm zone của tên miền vào phần zone resources của token trên Cloudflare. |
| `the DNS record _acme-challenge.example.com did not appear on … in time` | Name server của Cloudflare cập nhật chậm. Hãy thử lại sau vài phút. |
