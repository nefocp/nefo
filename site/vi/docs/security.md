---
title: Bảo mật
description: Cách Nefo bảo vệ việc đăng nhập vào chính nó, và cách dùng tường lửa, fail2ban, cài đặt SSH, tường lửa ứng dụng web và trình quét mã độc.
---

# Bảo mật

Nefo có quyền root trên máy chủ của bạn, nên trước hết nó tự bảo vệ việc đăng nhập vào panel. Ngoài ra, Nefo có sẵn các công cụ để bạn bảo vệ máy chủ và website. Trang này gồm:

- lớp bảo vệ của chính panel: địa chỉ ẩn, HTTPS, giới hạn đăng nhập, xác thực hai lớp, phiên đăng nhập và nhật ký hoạt động;
- tường lửa (ufw), fail2ban và cài đặt SSH của máy chủ, ở trang **Tường lửa & SSH**;
- bảo mật website: tường lửa ứng dụng web, giới hạn tốc độ, chặn địa chỉ IP và quét mã độc.

## Lớp bảo vệ của chính panel

### Đường dẫn bí mật

Trình cài đặt chọn ngẫu nhiên một cổng từ 20000 đến 49999 và một đường dẫn bí mật gồm 12 chữ thường và chữ số. Panel chỉ trả lời dưới đường dẫn đó, ví dụ `https://203.0.113.10:28443/k3x9p2qa7m1z/`. Mọi URL khác trên cổng này đều trả về `404`, nên công cụ dò quét không biết có panel đang chạy.

Nếu quên địa chỉ, hãy chạy lệnh sau trên máy chủ:

```sh
nefo info
```

Lệnh in ra URL với địa chỉ IP đầu tiên của máy chủ, và cả với tên miền của panel khi đã có (xem bên dưới). Nếu máy chủ có một địa chỉ công khai riêng, hãy chạy `nefo info --host 203.0.113.10` để in URL với địa chỉ đó.

Cổng và đường dẫn được lưu ở mục `listen` và `entrance` trong `/etc/nefo/config.json`. Panel không có chỗ nào để đổi chúng. Muốn đổi đường dẫn bằng tay, hãy đặt `entrance` thành 8 đến 32 chữ thường và chữ số, rồi chạy `systemctl restart nefo`.

### HTTPS ngay từ lần chạy đầu {#https-from-the-first-start}

Panel dùng HTTPS (TLS 1.2 trở lên) ngay từ lần chạy đầu tiên. Nó tự tạo một chứng chỉ tự ký trong `/var/lib/nefo/tls/`, nên lần đầu trình duyệt sẽ cảnh báo. Để hết cảnh báo, hãy xin chứng chỉ Let's Encrypt miễn phí cho một tên miền trỏ về máy chủ này:

1. Cài web server ở trang **Website** nếu chưa cài. Let's Encrypt kiểm tra tên miền qua cổng 80, và web server sẽ trả lời.
2. Trỏ bản ghi A của tên miền về máy chủ, không qua proxy hay CDN.
3. Vào **Cài đặt** → **Chứng chỉ**.
4. Nhập **Tên miền của panel**, ví dụ `panel.example.com`, rồi chọn **Xin chứng chỉ**.
5. Mở panel tại `https://panel.example.com:28443/k3x9p2qa7m1z/`, thay bằng cổng và đường dẫn của bạn.

Chứng chỉ tự động gia hạn như chứng chỉ của website; nếu gia hạn lỗi, panel thử lại sau mỗi 12 giờ. Muốn bỏ chứng chỉ, chọn **Xóa**: panel quay lại dùng chứng chỉ tự ký. Xem thêm về chứng chỉ ở trang [Chứng chỉ SSL](./ssl).

### Giới hạn đăng nhập {#sign-in-limits}

- Sai 5 lần trong 15 phút thì địa chỉ đó bị khóa 15 phút. Trang đăng nhập sẽ báo "Bạn đã nhập sai quá nhiều lần. Hãy thử lại sau … giây."
- Sai mật khẩu và sai mã xác thực hai lớp đều bị tính. Nhập sai mật khẩu hiện tại trong các form đổi mật khẩu và 2FA ở trang **Tài khoản** cũng bị tính.
- Địa chỉ IPv6 được tính theo dải /64, vì một người thường dùng cả dải /64.
- Đăng nhập thành công thì số lần sai được xóa.
- Sai tên đăng nhập và sai mật khẩu cho cùng một thông báo và mất cùng thời gian, nên trang đăng nhập không để lộ tên đăng nhập nào có thật.
- Panel tính theo địa chỉ kết nối trực tiếp tới nó và bỏ qua `X-Forwarded-For`. Nếu đặt proxy trước panel, mọi người truy cập sẽ dùng chung một bộ đếm.

Mật khẩu được lưu dưới dạng mã băm bcrypt, không bao giờ ở dạng rõ.

::: tip
Nếu tự khóa mình, hãy chờ 15 phút. Bộ đếm nằm trong bộ nhớ, nên khởi động lại panel trên máy chủ (`systemctl restart nefo`) cũng xóa được nó.
:::

### Phiên đăng nhập và đăng xuất

- Mỗi lần đăng nhập có hiệu lực 12 giờ. Hết thời gian đó bạn phải đăng nhập lại, kể cả khi đang dùng.
- Để đăng xuất, mở menu tài khoản ở góc trên bên phải và chọn **Đăng xuất**.
- Đổi mật khẩu sẽ đăng xuất bạn trên mọi thiết bị. Lệnh `nefo reset-password` trên máy chủ cũng vậy.
- Cookie phiên không đọc được bằng script, chỉ gửi qua HTTPS và chỉ gửi tới đường dẫn của panel.

### Header bảo mật

Mọi trang đều kèm Content Security Policy chặt, chỉ cho phép script từ chính panel. Panel cũng gửi `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer` (để đường dẫn bí mật không lọt vào log của website khác) và `Permissions-Policy` tắt camera, micro và vị trí. Mọi thao tác thay đổi đều cần một header mà website khác không gửi được, nhờ vậy chặn được tấn công giả mạo request (CSRF).

## Xác thực hai lớp

Xác thực hai lớp (2FA) yêu cầu mã 6 số từ ứng dụng xác thực mỗi lần đăng nhập, nên lộ mật khẩu thôi là chưa đủ để vào panel. Ứng dụng nào hỗ trợ mã theo thời gian chuẩn cũng dùng được, như Google Authenticator, Authy hoặc 2FAS. Khi 2FA còn tắt, panel coi "Chưa bật xác thực hai lớp" là vấn đề nghiêm trọng.

### Bật 2FA

1. Vào **Tài khoản** ở menu bên trái. Bạn cũng có thể mở menu tài khoản và chọn **Mật khẩu và 2FA**.
2. Ở mục **Xác thực hai lớp**, chọn **Thiết lập**.
3. Quét mã QR bằng ứng dụng, hoặc nhập **Khóa thiết lập** vào ứng dụng. Tài khoản sẽ hiện với tên Nefo.
4. Nhập mã 6 số mà ứng dụng hiển thị rồi chọn **Bật**.

Từ đó, trang đăng nhập sẽ hỏi **Mã xác thực hai lớp** sau khi bạn nhập mật khẩu.

Mã đổi sau mỗi 30 giây. Panel chấp nhận cả mã ngay trước và ngay sau, phòng khi đồng hồ hai bên lệch nhau một chút. Mỗi mã chỉ dùng được một lần: nếu mã bị từ chối, hãy chờ mã tiếp theo.

### Tắt 2FA

Ở trang **Tài khoản**, mục **Xác thực hai lớp**, nhập **Mật khẩu** và một **Mã** hiện tại, rồi chọn **Tắt**.

### Khi mất điện thoại

Nefo không có mã dự phòng. Bạn lấy lại quyền vào panel qua máy chủ: đăng nhập SSH bằng root và chạy:

```sh
nefo reset-password --disable-2fa
```

Lệnh này đặt mật khẩu ngẫu nhiên mới cho tài khoản `admin`, tắt 2FA, đăng xuất mọi phiên và in ra mật khẩu mới. Hãy đăng nhập bằng mật khẩu đó, đổi mật khẩu và thiết lập lại 2FA. Với tài khoản khác, thêm `--user` và tên tài khoản. Xem trang [Dòng lệnh](./cli).

::: warning
Khi không còn điện thoại, quyền root trên máy chủ là cách duy nhất để vào lại panel. Hãy giữ cẩn thận SSH key hoặc mật khẩu root, hoặc quyền vào web console của nhà cung cấp VPS.
:::

## Đổi mật khẩu panel

1. Vào **Tài khoản** → **Đổi mật khẩu**.
2. Nhập **Mật khẩu hiện tại**, rồi nhập **Mật khẩu mới** hai lần (**Nhập lại mật khẩu mới**). Mật khẩu phải dài từ 12 đến 72 ký tự.
3. Chọn **Đổi mật khẩu**.

Bạn sẽ bị đăng xuất trên mọi thiết bị. Hãy đăng nhập lại bằng mật khẩu mới.

## Nhật ký hoạt động

**Tài khoản** → **Hoạt động gần đây** hiện 100 mục mới nhất, gồm thời gian, hành động, người dùng, địa chỉ IP và chi tiết. Chọn **Tất cả** hoặc chỉ **Lỗi**, và gõ vào ô lọc để tìm. Đầu trang hiện lần đăng nhập gần nhất và số lần đăng nhập lỗi có trong nhật ký.

Nhật ký ghi lại:

- đăng nhập, đăng xuất và đăng nhập thất bại, kèm lý do (sai tên đăng nhập hoặc mật khẩu, hoặc sai mã 2FA);
- đổi mật khẩu, đặt lại mật khẩu từ dòng lệnh, bật hoặc tắt 2FA;
- các thay đổi làm trong panel: website, chứng chỉ, cơ sở dữ liệu, tệp tin, sao lưu, cron job, dịch vụ, luật tường lửa, cài đặt SSH và SSH key, phần mềm và Docker;
- mở và đóng terminal, kèm thời gian đã mở;
- khởi động lại panel và khởi động lại máy chủ.

Các lần tự động gia hạn chứng chỉ được ghi với người dùng `system`. Nhật ký được giữ 90 ngày. Mỗi mục cũng được ghi vào log dịch vụ của panel, xem bằng `journalctl -u nefo`. fail2ban đọc các lần đăng nhập thất bại từ đây.

## Tường lửa (ufw) {#firewall-ufw}

Nefo quản lý tường lửa của máy chủ bằng ufw. Vào **Tường lửa & SSH** → **Tường lửa (ufw)**.

Nếu trang báo máy chủ chưa cài ufw, hãy cài trên máy chủ. Panel không tự cài ufw:

```sh
apt install ufw
```

### Bật tường lửa

1. Chọn **Bật tường lửa** và xác nhận.
2. Panel mở cổng SSH (ghi chú `SSH`) và cổng của panel (ghi chú `Nefo`) trước. Nếu đã cài web server, panel mở thêm cổng 80 và 443 cho website (ghi chú `Websites`), và cổng 443/UDP khi OpenLiteSpeed trả lời HTTP/3. Sau đó panel mới bật ufw. Panel vẫn thêm các luật này kể cả khi bạn đã thêm luật cho một trong các cổng đó lúc tường lửa còn tắt, ví dụ luật chỉ cho địa chỉ của bạn, để việc bật tường lửa không bao giờ khóa bạn ở ngoài. Muốn chỉ giữ luật của bạn, hãy xóa luật cho mọi nơi sau khi bật (xem cuối mục [Những gì tường lửa từ chối](#nhung-gi-tuong-lua-tu-choi)).

Bạn có thể [thêm luật](#them-luat) trước khi bật tường lửa, ví dụ cho một cổng mà chương trình khác trên máy chủ cần. Khi tường lửa còn tắt, trang hiện các luật này bên dưới nút **Bật tường lửa** dưới dạng lệnh ufw, như `ufw allow 8080/tcp`, ở cột **Luật (lệnh ufw)**, mỗi cặp luật IPv4 và IPv6 chỉ hiện một lần. Các luật có hiệu lực khi bạn bật tường lửa. ufw chỉ đánh số luật khi đang bật, nên muốn xóa một luật trong số đó, hãy bật tường lửa trước.

Mọi cổng vào khác đều bị đóng, trừ khi có luật cho phép. Nếu sau này bạn cài web server khi tường lửa đang bật, Nefo mở cổng 80 và 443 ngay lúc cài.

Nếu nhà cung cấp VPS có tường lửa cloud (security group), hãy mở cùng các cổng đó ở đấy. Nefo không đổi được tường lửa cloud.

### Thêm luật

1. Chọn **Thêm luật**.
2. Chọn **Cho phép** hoặc **Chặn**.
3. Ở ô **Cổng, ví dụ 8080**, nhập một cổng hoặc một dải như `6000:6010`.
4. Chọn **Giao thức**: TCP, UDP hoặc TCP + UDP. Dải cổng cần một giao thức: nếu chọn TCP + UDP thì dải sẽ dùng TCP.
5. Ở ô **Từ**, nhập một địa chỉ IP hoặc dải mạng như `198.51.100.0/24`. Để trống nếu luật áp dụng cho mọi nơi.
6. Có thể thêm **Ghi chú**, tối đa 60 ký tự, không có dấu nháy.
7. Chọn **Thêm**.

Phía trên danh sách luật có hiện IP của bạn, tiện khi chỉ muốn mở một cổng cho riêng mình.

ufw xét các luật từ trên xuống và dùng luật đầu tiên khớp. Luật **Chặn** có địa chỉ ở ô **Từ** được đặt lên đầu danh sách, nên vẫn có tác dụng với cổng mà một luật phía dưới cho mọi nơi truy cập. Các luật khác được thêm vào cuối.

::: tip
Muốn chặn một địa chỉ khỏi riêng một website, hãy dùng mục **Chặn truy cập** của website đó (xem bên dưới).
:::

Danh sách cũng hiện các luật bạn thêm bằng ufw trên dòng lệnh. **Giới hạn** cho phép cổng, nhưng tạm từ chối địa chỉ nào mở 6 kết nối trong 30 giây (`ufw limit`). **Từ chối** chặn như **Chặn**, nhưng báo ngay cho phía bên kia.

### Xóa luật

Chọn biểu tượng thùng rác ở cuối dòng của luật và xác nhận. Nếu danh sách luật đã thay đổi kể từ lúc bạn mở trang, panel sẽ từ chối và yêu cầu tải lại trang, để không bao giờ xóa nhầm luật.

### Những gì tường lửa từ chối

Các chốt chặn sau giúp bạn không tự khóa mình ở ngoài:

- Luật cho phép cổng SSH hoặc cổng của panel có nhãn **được bảo vệ** khi đó là đường vào cuối cùng từ địa chỉ bạn đang dùng (hiện phía trên danh sách), và không xóa được ở đây. Khi một luật khác cũng cho địa chỉ của bạn vào, luật đó xóa được.
- Luật **Chặn** cổng SSH hoặc cổng của panel mà để trống ô **Từ** sẽ bị từ chối. Luật **Chặn** cho một địa chỉ hoặc một dải mạng thì được phép.

Muốn chỉ địa chỉ của bạn vào được panel: thêm luật **Cho phép** cho cổng của panel với địa chỉ của bạn ở ô **Từ**, rồi xóa luật cho cổng đó từ mọi nơi (và bản IPv6 của nó). Nếu sau này địa chỉ của bạn đổi, hãy dùng console web của nhà cung cấp để chạy `ufw allow <cổng>/tcp`.

Luật cho truy cập cơ sở dữ liệu từ xa (cổng 3306) được thêm và xóa từ trang **Cơ sở dữ liệu**. Hãy sửa chúng ở đó. Xem trang [Cơ sở dữ liệu](./databases).

## Chống dò mật khẩu (fail2ban) {#brute-force-protection-fail2ban}

fail2ban chặn các địa chỉ đăng nhập sai liên tục vào SSH và vào panel.

### Cài fail2ban

1. Vào **Tường lửa & SSH** → **Chống dò mật khẩu (fail2ban)**.
2. Chọn **Cài fail2ban**. Việc cài đặt chạy nền, có log trực tiếp.

Nefo tạo hai jail trong `/etc/fail2ban/jail.d/panel.conf`:

| Jail | Theo dõi | Chặn khi | Thời gian chặn |
|---|---|---|---|
| `sshd` | đăng nhập SSH, kể cả SFTP | sai 5 lần trong 10 phút | 1 giờ |
| `panel` | lần đăng nhập panel thất bại, hoặc bị từ chối khi địa chỉ đang bị khóa | 10 lần trong 10 phút | 1 giờ |

Địa chỉ bị chặn sẽ không vào được cổng mà jail đó bảo vệ. Giới hạn đăng nhập riêng của panel (sai 5 lần trong 15 phút) có tác dụng trước. Địa chỉ nào vẫn cố thử trong lúc bị khóa sẽ bị jail `panel` chặn hẳn ở tường lửa.

Panel tự ghi lại file này và bộ lọc của nó trong `/etc/fail2ban/filter.d/panel.conf`, nên đừng sửa bằng tay. Khi bạn đổi cổng SSH, jail `sshd` cũng chuyển theo.

### Xem và bỏ chặn địa chỉ

Bảng liệt kê từng **Dịch vụ được bảo vệ**, số **Lần sai gần đây** và các địa chỉ **Đang bị chặn**, kèm tổng số lần đã chặn. Để bỏ chặn một địa chỉ, chọn **Bỏ chặn** bên cạnh nó.

Nếu fail2ban đã cài nhưng đang dừng, hãy bật nó ở trang **Dịch vụ**.

::: tip
Nếu bạn tự chặn mình khỏi SSH, hãy đăng nhập panel từ mạng hiện tại và chọn **Bỏ chặn**. Bạn cũng có thể chạy `fail2ban-client set sshd unbanip 203.0.113.10` trong **Terminal** của panel.
:::

## Cài đặt SSH

Vào **Tường lửa & SSH** → **SSH**.

| Cài đặt | Lựa chọn |
|---|---|
| **Cổng** | từ 1 đến 65535 |
| **Đăng nhập root** | **Chỉ bằng SSH key (khuyên dùng)**, **Bằng key hoặc mật khẩu**, **Không cho phép** |
| **Cho phép đăng nhập bằng mật khẩu** | bật hoặc tắt, cho mọi tài khoản |

Chọn **Lưu**. Panel ghi cài đặt vào `/etc/ssh/sshd_config.d/00-panel.conf`, file này được ưu tiên hơn các file cài đặt SSH khác. Panel cũng tắt kiểu đăng nhập keyboard-interactive. Trước khi khởi động lại SSH, panel kiểm tra cài đặt mới bằng `sshd -t`. Nếu kiểm tra thất bại, file cũ được trả lại và không có gì thay đổi.

::: warning
Sau khi lưu, hãy giữ phiên SSH hiện tại. Mở một kết nối mới và kiểm tra là đăng nhập được rồi mới đóng phiên cũ.
:::

### Những gì cài đặt SSH từ chối

- Cài đặt khiến không ai đăng nhập SSH được nữa. Ví dụ: tắt đăng nhập bằng mật khẩu khi chưa tài khoản nào có SSH key; đặt đăng nhập root **Chỉ bằng SSH key** khi root chưa có key; hoặc root **Không cho phép** khi không tài khoản nào khác đăng nhập được.
- Cổng đã có chủ: cổng của panel, 80, 443, hoặc bất kỳ cổng nào đang có chương trình khác dùng.

Để quyết định, panel xét root và các tài khoản người dùng (UID từ 1000 trở lên) có shell đăng nhập: key trong `~/.ssh/authorized_keys` (hoặc các file đặt bằng `AuthorizedKeysFile`), mật khẩu, cùng `AllowUsers`, `DenyUsers`, `AllowGroups` và `DenyGroups`. Panel không đọc các khối `Match`. Trang liệt kê những ai đang đăng nhập được và bằng cách nào (key hoặc mật khẩu). Khi root chưa có key, trang cũng hiện cảnh báo kèm nút **Thêm key** (xem [SSH key](#ssh-keys)).
- Đổi cổng khi SSH được khởi động bằng socket của systemd. Khi đó trang hiện nhãn **kích hoạt qua socket** và ô cổng bị khóa. Hãy đổi cổng bằng dòng lệnh.

Không thể dừng SSH ở trang **Dịch vụ**, chỉ khởi động lại hoặc nạp lại được.

::: warning
Nếu đặt đăng nhập root là **Không cho phép**, bạn cần một tài khoản khác dùng được `sudo`, nếu không bạn vào được SSH nhưng không quản trị được máy chủ. Nefo không tạo tài khoản đó, và không kiểm tra nó có dùng được `sudo` hay không.
:::

### Đổi cổng SSH

1. Nhập **Cổng** mới, chọn **Lưu** và xác nhận.
2. Nếu tường lửa đang bật, panel mở cổng mới (ghi chú `SSH`) trước khi chuyển SSH sang đó. Nếu đã cài fail2ban, jail `sshd` cũng chuyển sang cổng mới.
3. Nếu nhà cung cấp VPS có tường lửa cloud, hãy mở cổng mới ở đó.
4. Thử một kết nối mới, ví dụ `ssh -p 2222 root@203.0.113.10`.
5. Luật tường lửa cho cổng cũ vẫn còn và không còn được bảo vệ. Khi cổng mới đã chạy ổn, hãy xóa luật đó ở **Tường lửa (ufw)**.

Tài khoản SFTP của các website cũng kết nối qua cổng này.

### SSH key {#ssh-keys}

Mục **SSH key**, bên dưới cài đặt SSH, liệt kê public key của root và của các tài khoản người dùng (UID từ 1000 trở lên, có shell đăng nhập), nhóm theo tài khoản. Nefo đọc chúng từ `~/.ssh/authorized_keys`, hoặc từ các file đặt bằng `AuthorizedKeysFile`. Mỗi key hiện loại và kích thước (như `ed25519` hoặc `rsa 4096`), ghi chú, fingerprint SHA256, và các tùy chọn nếu có, như `from="…"` hoặc `command="…"`. Key DSA, hoặc key RSA dưới 2048 bit, được đánh dấu **yếu**: OpenSSH hiện nay từ chối chúng. Hãy thêm một key mới, rồi xóa key yếu.

Tài khoản SFTP của website không có trong danh sách này: key của chúng nằm trong cài đặt SFTP của từng website.

#### Thêm key {#add-a-key}

1. Trên máy của bạn, tạo một cặp key nếu chưa có. Lệnh này chạy giống hệt trong PowerShell trên Windows 10 trở lên:
   ```sh
   ssh-keygen -t ed25519
   ```
   Private key được lưu vào `~/.ssh/id_ed25519` và ở lại trên máy của bạn. Public key được lưu vào `~/.ssh/id_ed25519.pub`.
2. In public key ra và sao chép nguyên dòng, bắt đầu bằng `ssh-ed25519`:
   ```sh
   cat ~/.ssh/id_ed25519.pub
   ```
3. Trong panel, ở mục **SSH key**, chọn **Thêm key**.
4. Chọn **Tài khoản**, dán dòng đó vào ô **Public key** rồi chọn **Thêm key**.
5. Thử bằng một terminal mới, ví dụ `ssh root@203.0.113.10`.

Nefo thêm key vào cuối file `~/.ssh/authorized_keys` của tài khoản. Nếu thư mục hoặc file chưa có, panel tạo chúng, thuộc về tài khoản đó, với quyền 700 và 600. Panel từ chối:

- private key, ví dụ key bắt đầu bằng `-----BEGIN OPENSSH PRIVATE KEY-----`. Nếu lỡ dán private key vào đâu đó, hãy tạo cặp key mới;
- nhiều hơn một dòng. Với PuTTYgen, hãy sao chép ô **Public key for pasting into OpenSSH authorized_keys file**;
- key DSA, và key RSA dưới 2048 bit;
- key mà tài khoản đã có;
- file `authorized_keys` hoặc thư mục `.ssh` là liên kết tượng trưng, hoặc không phải file hay thư mục thường. Tài khoản đó có thể trỏ nó tới một file khác của máy chủ, nên Nefo không đọc và không sửa nó.

#### Xóa key {#remove-a-key}

Chọn biểu tượng thùng rác cạnh key và xác nhận. Hộp xác nhận ghi rõ key theo loại, fingerprint và ghi chú. Nếu thiếu key này mà với cài đặt hiện tại không ai đăng nhập SSH được nữa, panel sẽ từ chối: hãy thêm key khác trước, hoặc cho phép đăng nhập bằng mật khẩu.

Việc thêm và xóa key được ghi vào nhật ký hoạt động, kèm tài khoản và fingerprint của key.

### Tắt đăng nhập bằng mật khẩu

1. Ở mục [SSH key](#ssh-keys), thêm public key của bạn cho root, hoặc cho tài khoản bạn dùng để đăng nhập.
2. Mở một terminal mới và đăng nhập bằng key. Nếu SSH hỏi mật khẩu thì key chưa được dùng: hãy kiểm tra là bạn đã thêm nó cho đúng tài khoản bạn đăng nhập.
3. Tắt **Cho phép đăng nhập bằng mật khẩu** rồi chọn **Lưu**. Nếu chưa ai có key, panel sẽ từ chối (xem mục **Những gì cài đặt SSH từ chối** ở trên).
4. Giữ phiên hiện tại, và thử lại một kết nối mới trước khi đóng nó.

Tài khoản SFTP của website vẫn giữ cách đăng nhập riêng (mật khẩu hoặc key), kể cả khi SSH đã tắt đăng nhập bằng mật khẩu.

Khi root còn đăng nhập được bằng mật khẩu, panel coi "Root đăng nhập SSH bằng mật khẩu" là vấn đề nghiêm trọng.

## Bảo mật website

Mỗi website có cài đặt bảo mật riêng. Vào **Website**, mở website bằng cách chọn tên của nó hoặc nút **Cài đặt**, rồi dùng các mục trong nhóm **Bảo mật**.

### Tường lửa ứng dụng web

Tường lửa ứng dụng web (WAF) là ModSecurity với bộ luật OWASP Core Rule Set, cả hai đều miễn phí và mã nguồn mở. Nó kiểm tra mọi request và chặn SQL injection, XSS, chèn file, công cụ dò quét và các kiểu tấn công phổ biến khác. WAF dùng được với Nginx, Apache và OpenLiteSpeed.

1. Cài một lần: **Phần mềm** → **Tường lửa web (ModSecurity)** → **Cài**. Các website vẫn tắt tường lửa cho đến khi bạn bật.
2. Vào **Website** → (website) → **Tường lửa WAF**.
3. Chọn chế độ:
   - **Tắt**
   - **Chỉ phát hiện: ghi lại tấn công, không chặn**
   - **Chặn tấn công**: khách nhận HTTP 403.
4. Chọn **Lưu**.

Nên dùng chế độ chỉ phát hiện vài ngày trước, xem các sự kiện để tìm báo nhầm, rồi mới chuyển sang chặn. Website chạy WordPress hoặc dùng mẫu rewrite WordPress hay Drupal được tự áp ngoại lệ của bộ luật dành riêng cho các ứng dụng đó.

**Sự kiện tường lửa gần đây** liệt kê 50 request mới nhất bị bộ luật đánh dấu trên website này: thời gian, địa chỉ IP, request, các luật đã khớp, và request đó **đã chặn** hay chỉ **đã ghi**. Dữ liệu lấy từ `/var/log/panel-waf/audit.log`, file này được xoay vòng mỗi ngày và giữ 7 ngày.

Chỉ 13 MB đầu của mỗi request được kiểm tra, nên tải file lớn vẫn chạy bình thường. Nếu bạn gỡ WAF ở trang **Phần mềm**, mỗi website vẫn giữ chế độ đã chọn, và chế độ đó có tác dụng lại khi bạn cài lại.

### Giới hạn tốc độ và chặn địa chỉ

Vào **Website** → (website) → **Chặn truy cập**.

- **Giới hạn tốc độ (số request mỗi giây mỗi khách, 0 = tắt)**: chỉ tính request PHP và ứng dụng, không tính ảnh, CSS hay script. Cho phép dồn tối đa gấp năm lần giới hạn; vượt quá thì khách nhận HTTP 429. Mức 10 hợp với đa số website, tối đa là 1000. Chỉ Nginx có cài đặt này: với Apache và OpenLiteSpeed, ô này không hiện.
- **Địa chỉ IP bị chặn**: mỗi dòng một địa chỉ hoặc dải IPv4 hay IPv6, ví dụ `203.0.113.7` hoặc `198.51.100.0/24`, tối đa 200 cho mỗi website. Khách bị chặn nhận HTTP 403.

Chọn **Lưu**. Hai cài đặt này hoạt động độc lập, không cần tường lửa ứng dụng web. Cùng mục này còn có **Bảo vệ bằng mật khẩu**, xem ở trang [Website](./websites).

### Quét mã độc

Trình quét tìm web shell, backdoor, mã ẩn và script bị chèn trong file của website. Nó không thay đổi gì cho đến khi bạn cách ly một file.

1. Vào **Website** → (website) → **Quét mã độc**.
2. Chọn **Quét ngay**. Việc quét chạy nền, có log trực tiếp.

Trình quét đọc thư mục của website, ví dụ `/var/www/example.com`. Nó bỏ qua thư mục `.git`, các file lớn hơn 5 MB, và không đi theo liên kết tượng trưng (symlink). Nó kiểm tra file PHP (cả mã PHP nằm trong file có đuôi khác), file JavaScript, HTML và ảnh.

| Phát hiện | Mức độ |
|---|---|
| Web shell đã biết | nghiêm trọng |
| Chạy mã do khách gửi lên | nghiêm trọng |
| Gọi hàm theo tên do khách gửi lên | nghiêm trọng |
| Chạy lệnh shell do khách gửi lên | nghiêm trọng |
| Chạy mã ẩn (đã mã hóa) | cao |
| Giải mã dữ liệu ẩn do khách gửi lên | cao |
| Tạo hàm từ dữ liệu khách gửi lên | cao |
| Trình tải file cho phép khách chọn nơi lưu | cao |
| Script bị chèn và làm rối | cao |
| Mã PHP giấu trong ảnh | cao |
| Chạy mã qua preg_replace /e | cần xem |
| Dữ liệu mã hóa dài kèm đoạn mã giải mã nó | cần xem |
| Chuỗi bị làm rối nặng kèm đoạn mã chạy nó | cần xem |
| File PHP trong thư mục uploads | cần xem |

Mỗi phát hiện hiện tên file, số dòng và một đoạn trích ngắn. File PHP trong thư mục uploads mà chỉ chứa chú thích, như file `index.php` "Silence is golden" của WordPress, sẽ không bị báo.

Panel giữ kết quả quét gần nhất của mỗi website. Panel không quét theo lịch: bạn tự bắt đầu khi cần.

### Cách ly và khôi phục

Chọn **Cách ly** bên cạnh một phát hiện để chuyển file ra khỏi website, vào `/var/lib/panel-quarantine`, thư mục chỉ root đọc được. Panel ghi lại chủ sở hữu và quyền của file.

Trong mục **File đã cách ly**:

- **Khôi phục** đưa file về chỗ cũ, với chủ sở hữu và quyền như trước. Thao tác bị từ chối nếu chỗ đó đã có file khác, hoặc thư mục không còn nữa.
- **Xóa** xóa vĩnh viễn file, sau khi bạn xác nhận.

Khi bạn xóa một website, các file đã cách ly của nó cũng bị xóa.

::: warning
Cách ly một file mà website cần, ví dụ file của plugin, có thể làm website lỗi. Hãy xem đoạn trích trước, và khôi phục file nếu đó là báo nhầm.
:::

### Quét bằng ClamAV

ClamAV là bộ quét bổ sung, không bắt buộc.

1. Cài **Phần mềm diệt virus ClamAV** ở trang **Phần mềm**. Sau khi cài, cơ sở dữ liệu virus của nó tải về ở chế độ nền trong vài phút.
2. Ở mục **Quét mã độc**, đánh dấu **Quét thêm bằng ClamAV (chậm hơn, cần khoảng 1,4 GB RAM trống)** rồi chọn **Quét ngay**.

ClamAV cần khoảng 1,4 GB RAM trống trong lúc quét, nên hợp với máy chủ từ 2 GB trở lên. Nếu RAM trống ít hơn, panel bỏ qua ClamAV và ghi rõ trong kết quả. ClamAV chạy với mức ưu tiên CPU thấp nhất và bỏ qua file lớn hơn 25 MB. Phát hiện của ClamAV được xếp mức cao.

## Lịch sử đăng nhập SSH

Danh sách đăng nhập SSH và các lần thử thất bại nằm ở trang **Tiến trình**. Xem mục [Đăng nhập SSH](./system#ssh-sign-ins).
