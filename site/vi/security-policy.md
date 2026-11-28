---
title: Chính sách bảo mật
description: Cách báo cáo lỗ hổng bảo mật của Nefo, cách bản phát hành được ký và cách bạn tự kiểm tra, cùng tình trạng bảo mật hiện tại.
---

# Chính sách bảo mật

Nefo chạy với quyền root và quản lý cả một máy chủ, nên mọi báo cáo lỗ hổng đều được xử lý khẩn cấp. Trang này hướng dẫn cách báo cáo, cho biết bản phát hành được ký ra sao, cách bạn tự kiểm tra một bản phát hành, và Nefo đang ở đâu về bảo mật. Nội dung theo tệp [SECURITY.md](https://github.com/nefocp/nefo/blob/main/SECURITY.md) trong kho mã.

## Báo cáo lỗ hổng {#report-a-vulnerability}

Đừng mở issue, discussion hay pull request công khai. Hãy báo riêng qua GitHub:

1. Mở [kho mã Nefo](https://github.com/nefocp/nefo) và vào tab **Security**.
2. Chọn **Report a vulnerability**, hoặc dùng [liên kết trực tiếp](https://github.com/nefocp/nefo/security/advisories/new). Chỉ những người duy trì dự án thấy được báo cáo của bạn.

Hãy ghi kèm:

- phiên bản bị ảnh hưởng (`nefo version`) và hệ điều hành;
- kẻ tấn công làm được gì, và cần có quyền gì trước: không cần gì, một tài khoản panel, một tài khoản shell…;
- các bước tái hiện lỗi, hoặc một bản chứng minh (proof of concept);
- bạn có định công bố phát hiện của mình không, và khi nào.

### Sau khi bạn báo cáo {#what-happens-next}

| Bước | Mục tiêu |
|---|---|
| Dự án xác nhận đã nhận được báo cáo của bạn | trong vòng 3 ngày làm việc |
| Dự án cho bạn biết có xác nhận lỗi hay không và mức độ nghiêm trọng của nó | trong vòng 7 ngày |
| Dự án phát hành bản vá cho lỗi mức nghiêm trọng hoặc mức cao | trong vòng 14 ngày kể từ khi xác nhận |
| Công bố công khai | sau khi bản vá được phát hành, hoặc chậm nhất 90 ngày sau ngày bạn báo cáo, có phối hợp với bạn |

Mỗi lỗ hổng đã xác nhận đều có một GitHub Security Advisory kèm mã CVE. Tên bạn được ghi nhận, trừ khi bạn muốn ẩn danh.

### Phạm vi {#scope}

Trong phạm vi: chương trình `nefo`, API và giao diện của nó, trình cài đặt, và cấu hình mà nó tạo trên máy chủ.

Ngoài phạm vi:

- cảnh báo của trình duyệt về chứng chỉ tự ký mà panel tạo ở lần chạy đầu (đây là điều bình thường);
- các cuộc tấn công cần quyền root trên máy chủ, hoặc cần tiếp cận máy chủ vật lý;
- tấn công từ chối dịch vụ chỉ bằng lưu lượng lớn;
- lỗ hổng trong phần mềm bên thứ ba mà panel cài, như Nginx, PHP hay MariaDB. Hãy báo cho chính dự án đó, nhưng cũng báo cho Nefo nếu panel cấu hình chúng không an toàn.

### Cam kết không truy cứu {#safe-harbour}

Dự án sẽ không có hành động pháp lý nào với bạn vì nghiên cứu bảo mật làm đúng theo chính sách này: bạn chỉ thử trên máy chủ của mình hoặc máy chủ được phép thử, tránh xâm phạm quyền riêng tư và phá hủy dữ liệu, và cho dự án đủ thời gian sửa lỗi trước khi công bố.

## Phiên bản được hỗ trợ {#supported-versions}

Cho tới bản 1.0, chỉ bản phát hành mới nhất được vá lỗi bảo mật. Hãy cập nhật trong panel (**Cài đặt** → **Panel** → **Kiểm tra cập nhật**) hoặc bằng lệnh `nefo update` trên máy chủ. Xem [Cập nhật](./docs/update).

## Bản phát hành được ký thế nào {#how-releases-are-signed}

Mỗi bản phát hành do GitHub Actions dựng từ một tag phiên bản trong kho mã. Một bản phát hành gồm:

- `nefo-linux-amd64` và `nefo-linux-arm64`, tức chương trình;
- `install.sh`, trình cài đặt;
- `VERSION`;
- `checksums.txt`, mã SHA-256 của hai chương trình và trình cài đặt;
- `checksums.txt.sig`, chữ ký Ed25519 của `checksums.txt`;
- `appstore-catalog.json` và `appstore-catalog.json.sig`, danh mục kho ứng dụng với chữ ký riêng.

Khóa công khai của dự án được gắn sẵn vào mọi chương trình Nefo và được chép vào `install.sh`. Khóa này cũng có trong kho mã, ở tệp [`internal/update/release.pub`](https://github.com/nefocp/nefo/blob/main/internal/update/release.pub):

```
-----BEGIN PUBLIC KEY-----
MCowBQYDK2VwAyEA2c6nGagX7yp8Y8YftuOqW2WVprKe6h0hv+3s9iB+KJA=
-----END PUBLIC KEY-----
```

Ba nơi kiểm tra chữ ký trước khi dùng thứ đã tải về:

- **Trình cài đặt** kiểm tra chữ ký của `checksums.txt` và checksum của chương trình. Sai một trong hai là nó dừng lại: "invalid signature: checksums.txt was not signed by the project. Do not install this file."
- **`nefo update`**, và nút **Cài bản cập nhật** trong panel, làm y như vậy với khóa gắn sẵn trong chương trình bạn đang chạy, rồi chạy thử chương trình mới một lần trước khi thay `/usr/local/bin/nefo`.
- **Kho ứng dụng** tải danh mục mỗi ngày một lần, và bỏ qua danh mục nếu chữ ký không hợp lệ.

### Chữ ký chứng minh được gì, và không chứng minh được gì {#what-the-signature-does-and-doesn-t-prove}

Chữ ký cho biết các tệp đến từ quy trình dựng bản phát hành của dự án và không bị sửa sau đó, dù trên GitHub hay trên đường tới máy chủ của bạn.

- **Khi cập nhật**, chữ ký được kiểm bằng khóa nằm trong bản Nefo bạn đang có, nên một bản phát hành bị sửa trên GitHub không thể tự cài vào máy bạn.
- **Khi cài lần đầu** thì khác: chính trình cài đặt bạn tải về mang theo khóa để kiểm tra. Ai sửa được `install.sh` thì cũng sửa được khóa đó. Để loại trừ khả năng này, hãy tự kiểm tra `install.sh` trước khi chạy, theo hướng dẫn bên dưới.
- Khóa ký được lưu dưới dạng secret của kho mã trên GitHub, và GitHub Actions ký bản phát hành ngay trên đó. Ai chiếm được tài khoản GitHub của dự án thì có thể ký một bản phát hành.

## Tự kiểm tra một bản phát hành {#check-a-release-yourself}

Bạn cần `curl`, `sha256sum` và OpenSSL 3 (có sẵn trên Ubuntu 22.04 trở lên và Debian 12 trở lên). Ví dụ dưới đây kiểm tra bản phát hành mới nhất trên máy chủ amd64, với quyền root; muốn kiểm tra bản khác, hãy đặt `V` thành tag của bản phát hành đó.

1. Tải danh sách checksum, chữ ký và trình cài đặt:

   ```sh
   V=$(curl -fsSL https://github.com/nefocp/nefo/releases/latest/download/VERSION)
   cd /tmp
   for f in checksums.txt checksums.txt.sig install.sh; do
     curl -fsSLO "https://github.com/nefocp/nefo/releases/download/$V/$f"
   done
   ```

2. Lưu khóa công khai từ trang này:

   ```sh
   cat > nefo-release.pub <<'EOF'
   -----BEGIN PUBLIC KEY-----
   MCowBQYDK2VwAyEA2c6nGagX7yp8Y8YftuOqW2WVprKe6h0hv+3s9iB+KJA=
   -----END PUBLIC KEY-----
   EOF
   ```

3. Kiểm tra chữ ký, rồi kiểm tra checksum của trình cài đặt:

   ```sh
   openssl pkeyutl -verify -pubin -inkey nefo-release.pub -rawin \
     -in checksums.txt -sigfile checksums.txt.sig
   grep " install.sh$" checksums.txt | sha256sum -c -
   ```

   Lệnh đầu phải in ra `Signature Verified Successfully`, lệnh sau phải in `install.sh: OK`. Nếu một trong hai thất bại, đừng chạy trình cài đặt, và hãy báo cáo theo hướng dẫn ở trên.

4. Chạy trình cài đặt bạn vừa kiểm tra. Nó sẽ tải chương trình cho CPU của bạn và kiểm tra bằng chính khóa đó:

   ```sh
   NEFO_VERSION=$V bash install.sh
   ```

Nếu bạn tự tải một chương trình, như `nefo-linux-amd64`, hãy chạy `grep " nefo-linux-amd64$" checksums.txt | sha256sum -c -` sau bước 3 để kiểm tra.

## Nefo bảo vệ máy chủ thế nào {#how-nefo-protects-the-server}

Tóm tắt ngắn; chi tiết có ở trang [Bảo mật](./docs/security):

- Panel chỉ trả lời dưới một đường dẫn bí mật, trên một cổng ngẫu nhiên. Mọi địa chỉ khác đều trả về `404`.
- Panel dùng HTTPS ngay từ lần chạy đầu, với TLS 1.2 trở lên.
- Đăng nhập hỗ trợ xác thực hai lớp. Sai 5 lần trong 15 phút thì địa chỉ đó bị khóa 15 phút. Mật khẩu được lưu dưới dạng mã băm bcrypt.
- Các lần đăng nhập và thay đổi được ghi vào nhật ký hoạt động, giữ trong 90 ngày.
- Mọi trang đều có Content Security Policy chặt chẽ, và mọi thay đổi đều cần một header mà website khác không gửi được.
- [Quy tắc viết mã](https://github.com/nefocp/nefo/blob/main/CONTRIBUTING.md#security-rules-for-code) của dự án cấm ghép lệnh shell từ dữ liệu nhập vào: mỗi giá trị được đưa cho chương trình thành một tham số riêng, và dữ liệu nhập được kiểm tra theo danh sách cho phép trước khi chạm tới đường dẫn tệp, tệp cấu hình hay câu lệnh. Lệnh do chính bạn viết, như cron job hay script triển khai, chạy trong shell đúng như bạn viết.
- Quy tắc tường lửa và cài đặt SSH làm bạn mất quyền truy cập đều bị từ chối.
- Mỗi website chạy dưới một user Linux riêng, và tài khoản SFTP chỉ vào được thư mục của website đó.

## Tình trạng hiện tại {#current-status}

Nefo đang ở giai đoạn beta. Bản phát hành công khai đầu tiên là v0.20.0, vào tháng 11/2026; bản hiện tại có trong [nhật ký thay đổi](./changelog). Nefo chưa được bên ngoài kiểm toán bảo mật.

Hãy thử Nefo trên một máy chủ có thể cài lại từ đầu trước khi dùng cho website quan trọng, giữ bản sao lưu ở ngoài máy chủ, và luôn cập nhật Nefo. Một đợt rà soát bảo mật độc lập là một trong các bước trước bản 1.0 (xem [lộ trình](./roadmap)). Nếu bạn đọc mã nguồn và phát hiện vấn đề, hãy báo cáo theo cách ở trên.
