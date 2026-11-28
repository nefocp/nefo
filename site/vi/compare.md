---
title: Nefo và các panel khác
description: Nefo khác các panel quản trị phổ biến ở điểm nào, và khi nào một panel khác hợp với bạn hơn.
---

# Nefo và các panel khác

Nefo khác phần lớn các panel quản trị hosting ở vài điểm: mọi tính năng đều miễn phí, không cần khóa bản quyền hay tài khoản với nhà phát triển; bản thân panel dùng khoảng 34 MB bộ nhớ; trang đăng nhập nằm trên một cổng ngẫu nhiên, sau một đường dẫn bí mật; và Nefo cố ý không có máy chủ mail, DNS, FTP lẫn tài khoản đại lý.

Trang này so sánh Nefo với các panel quản trị hosting mà người dùng VPS hay cài nhất. Trang không nêu tên panel nào. Mỗi panel một khác, nên cột bên phải ghi có bao nhiêu panel làm như vậy: phần lớn, nhiều, một số hay rất ít.

Phần so sánh được đối chiếu với trang web chính thức của các panel vào tháng 11/2026. Giá và tính năng của chúng thay đổi theo thời gian, nên hãy xem lại panel bạn định dùng trước khi quyết định.

## So sánh nhanh {#at-a-glance}

| | Nefo | Các panel khác |
|---|---|---|
| Giá | Miễn phí. Mọi tính năng, không có bản trả phí | Phần lớn có thứ phải trả tiền: giấy phép cho từng máy chủ, thường tính theo số tài khoản hoặc số website, hoặc một bản Pro hay tiện ích trả phí |
| Khóa bản quyền hoặc tài khoản với nhà phát triển | Không cần | Một số cần ngay từ đầu mới chạy được, và nhiều panel cần cho các tính năng trả phí |
| Dữ liệu gửi về nhà phát triển | Không có: không thu thập dữ liệu sử dụng, không báo về khi cài, không kiểm tra bản quyền | Tùy panel. Một số mặc định thu thập thống kê sử dụng, và một số trình cài đặt báo thông tin máy chủ về cho nhà phát triển |
| Bộ nhớ | Bản thân panel dùng khoảng 34 MB (đo trên một máy chủ thử nghiệm chạy v0.24.0). Thiết kế cho VPS 1 GB RAM | Phần lớn yêu cầu tối thiểu 1 GB, nhiều panel khuyên dùng 4 GB |
| Trang đăng nhập | Cổng ngẫu nhiên và đường dẫn bí mật. Mọi địa chỉ khác trả về `404` | Phần lớn dùng một cổng cố định ai cũng biết. Rất ít panel dùng cổng ngẫu nhiên hoặc đường dẫn bí mật |
| Cập nhật panel | Có chữ ký Ed25519, được kiểm tra trước khi cài. Chỉ cập nhật khi bạn yêu cầu | Một số có tài liệu về gói cập nhật được ký; số còn lại không nói rõ |
| Chống tự khóa mình | Từ chối quy tắc tường lửa và cài đặt SSH làm bạn mất quyền truy cập | Rất ít panel có cơ chế này cho tường lửa, chẳng hạn tự trả lại quy tắc cũ nếu bạn không xác nhận. Cho cài đặt SSH thì càng hiếm |
| Mã nguồn | Toàn bộ, theo giấy phép AGPLv3 | Phần lớn là mã đóng hoặc chỉ công khai mã để xem. Một số là mã nguồn mở |
| Máy chủ mail và DNS | Không có, theo thiết kế | Phần lớn cài cả hai |
| Quản lý nhiều máy chủ từ một panel | Không. Mỗi máy chủ một panel | Rất ít panel quản lý được nhiều máy chủ từ một panel, dù nhiều panel đồng bộ được zone DNS giữa các máy chủ |
| Tài khoản đại lý và khách hàng | Không. Chỉ một tài khoản quản trị | Phần lớn có tài khoản đại lý (reseller), và gần như tất cả có tài khoản đăng nhập cho khách hàng hoặc người dùng |
| Hệ điều hành | Ubuntu và Debian, trên x86-64 và ARM64 | Phần lớn chỉ chạy trên Linux, và phần lớn hỗ trợ nhiều bản phân phối Linux hơn hai bản này. Rất ít panel chạy trên Windows |
| Tuổi đời | Phát hành công khai lần đầu vào tháng 11/2026. Đang beta | Nhiều panel đã có mặt từ 20 năm trở lên |

## Nefo khác ở đâu {#what-nefo-does-differently}

### Gọn nhẹ {#it-stays-small}

- Nefo là một chương trình duy nhất, `/usr/local/bin/nefo`, khoảng 25 MB, có sẵn giao diện web bên trong. Trên một máy chủ thử nghiệm chạy v0.24.0, nó dùng khoảng 34 MB bộ nhớ. Các tệp của giao diện được gửi đi ở dạng nén gzip, và mỗi ngôn ngữ trong bảy ngôn ngữ của giao diện chỉ được tải về khi dùng tới.
- Nefo không cài thêm gì cho tới khi bạn chọn. Ở lần đăng nhập đầu tiên, trang thiết lập hỏi bạn cần web server, phiên bản PHP, máy chủ cơ sở dữ liệu và thành phần thêm nào, rồi cho biết lựa chọn đó tốn thêm khoảng bao nhiêu RAM và dung lượng đĩa. Lệnh `nefo setup` làm việc tương tự trên dòng lệnh.
- Thành phần nào cũng gỡ ra được, và menu ẩn những gì chưa cài. Nefo từ chối gỡ thành phần mà website, cơ sở dữ liệu hay container còn đang dùng.
- Không có máy chủ mail, DNS hay FTP nào chạy nền, vì Nefo không kèm chúng.

### Mọi tính năng miễn phí, không gửi gì về {#every-feature-is-free-and-nothing-phones-home}

- Không có bản trả phí, không phải tạo tài khoản, không có khóa bản quyền.
- Không thu thập dữ liệu sử dụng, không gửi thống kê. Tự nó, panel chỉ kết nối tới Let's Encrypt để xin và gia hạn chứng chỉ, và mỗi ngày một lần tới trang phát hành của dự án trên GitHub để tải danh mục kho ứng dụng đã được ký. Xem [Quyền riêng tư](./docs/#privacy).
- Nefo không bao giờ tự cập nhật, và chỉ kiểm tra phiên bản mới khi bạn yêu cầu.

### Trang đăng nhập khó tìm {#the-sign-in-is-hard-to-find}

- Trình cài đặt chọn một cổng ngẫu nhiên từ 20000 đến 49999 và một đường dẫn bí mật gồm 12 chữ cái và chữ số. Mọi địa chỉ khác trên cổng đó đều trả về `404`.
- Panel dùng HTTPS ngay từ lần chạy đầu. Đăng nhập hỗ trợ xác thực hai lớp, và sai 5 lần trong 15 phút thì địa chỉ đó bị khóa 15 phút. Các lần đăng nhập và thay đổi đều được ghi vào nhật ký hoạt động.
- Khóa API dùng cho script có thể đặt chỉ đọc, chỉ nhận các địa chỉ IP bạn liệt kê, và có hạn dùng. Ngay cả khóa toàn quyền cũng không mở được terminal, công cụ cơ sở dữ liệu hay phần cài đặt tài khoản.

Xem [Bảo mật](./docs/security) và [Khóa API](./docs/api).

### Từ chối thay đổi làm bạn mất quyền truy cập hoặc gây hỏng {#it-refuses-changes-that-would-lock-you-out-or-break-things}

- **Tường lửa:** không xóa được quy tắc cuối cùng cho phép địa chỉ của bạn vào SSH hoặc panel.
- **SSH:** từ chối cài đặt khiến không ai đăng nhập được nữa, ví dụ tắt đăng nhập bằng mật khẩu trong khi chưa tài khoản nào có khóa SSH.
- **Phần mềm:** không gỡ được web server, phiên bản PHP hay máy chủ cơ sở dữ liệu đang được dùng.
- **Docker:** dự án Compose được kiểm tra trước khi lưu hoặc chạy. Container đặc quyền (privileged), cổng mở ra Internet và việc gắn thư mục hệ thống vào container đều bị từ chối.

### Tự kiểm tra máy chủ của mình {#it-checks-its-own-server}

[Kiểm tra bảo mật](./docs/security-check) chạy 22 mục kiểm tra nhanh máy chủ khi bạn yêu cầu hoặc mỗi ngày một lần, chẳng hạn đăng nhập SSH bằng mật khẩu, tường lửa, bản cập nhật bảo mật đang chờ, cơ sở dữ liệu mở ra Internet, cổng Docker đi vòng qua ufw và tệp mà website vô tình để lộ ra ngoài, rồi gắn mỗi phát hiện với trang dùng để khắc phục nó. Tính năng này không gửi gì ra ngoài máy chủ.

### Bản sao lưu vẫn còn khi máy chủ mất {#backups-survive-the-server}

Bản sao lưu được lưu trên chính máy chủ và gửi tới kho tương thích S3, một máy chủ SFTP hoặc Google Drive, có mã hóa bằng passphrase nếu bạn muốn. Một máy chủ mới tìm thấy chúng trên kho lưu trữ và khôi phục các website, cùng cơ sở dữ liệu của chúng. Xem [Khôi phục trên máy chủ mới](./docs/restore-new-server).

### Bản phát hành có chữ ký {#releases-are-signed}

Danh sách mã kiểm tra (checksum) của mỗi bản phát hành được ký bằng khóa Ed25519 của dự án. Trình cài đặt và `nefo update` kiểm tra chữ ký và checksum trước khi cài bất cứ thứ gì. Trang [chính sách bảo mật](./security-policy#check-a-release-yourself) hướng dẫn bạn tự kiểm tra một bản phát hành.

### Chuyển website từ panel khác sang {#you-can-move-websites-in}

Nefo nhập được website từ bản sao lưu của các panel khác: tệp tin, cơ sở dữ liệu cùng user và mật khẩu, chứng chỉ và cron job. Xem [Nhập website](./docs/import) để biết Nefo đọc được những định dạng nào.

## Khi nào panel khác hợp với bạn hơn {#when-another-panel-fits-you-better}

Nefo cố ý bỏ qua một số thứ (lý do có ở trang [lộ trình](./roadmap#not-planned-and-why)). Hãy chọn panel khác nếu bạn cần một trong những điều sau:

- **Mail trên cùng máy chủ.** Nefo không có máy chủ mail: không hộp thư, không webmail, không lọc thư rác. Website và cảnh báo gửi mail qua SMTP relay của nhà cung cấp mail. Phần lớn các panel khác có kèm máy chủ mail.
- **Máy chủ DNS của riêng bạn.** Nefo không chạy name server. Nefo quản lý bản ghi DNS của tên miền trên Cloudflare.
- **FTP.** Nefo không có máy chủ FTP. Mỗi website có thể có một tài khoản SFTP, chỉ vào được thư mục của website đó.
- **Quản lý nhiều máy chủ ở một chỗ.** Mỗi máy chủ chạy một Nefo riêng, và bạn đăng nhập vào từng máy.
- **Kinh doanh hosting.** Mỗi máy chủ Nefo chỉ có một tài khoản quản trị. Không có đại lý, không có tài khoản cho khách hàng, không có gói hosting hay hạn mức dung lượng (quota).
- **Phần mềm đã chạy nhiều năm, hoặc đã được kiểm toán bảo mật độc lập.** Nefo phát hành công khai lần đầu vào tháng 11/2026, đang ở giai đoạn beta và chưa được bên ngoài kiểm toán bảo mật.
- **Windows, hoặc bản Linux khác ngoài Ubuntu và Debian.** Nefo chỉ chạy trên Ubuntu 22.04, 24.04, 26.04 và Debian 12, 13.

Nếu không điều nào ở trên quan trọng với bạn, hãy [cài Nefo](./docs/install) lên một máy chủ có thể cài lại từ đầu, và luôn giữ một bản sao lưu ở ngoài máy chủ.
