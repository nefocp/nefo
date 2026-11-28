---
title: Yêu cầu hệ thống
description: Hệ điều hành, loại CPU, bộ nhớ, cổng mạng và tường lửa mà Nefo cần.
---

# Yêu cầu hệ thống

Nefo cần một máy chủ 64-bit, x86-64 hoặc ARM64, chạy Ubuntu 22.04, 24.04 hoặc 26.04, hay Debian 12 hoặc 13, có quyền root và systemd. Nefo được làm cho VPS có 1 GB RAM: bản thân panel dùng khoảng 34 MB bộ nhớ, đo trên một máy chủ thử nghiệm sau khi cập nhật lên v0.24.0.

Trang này liệt kê những gì máy chủ cần có trước khi bạn [cài Nefo](./install): hệ điều hành, loại CPU, bộ nhớ và ổ đĩa, cùng các cổng mạng.

## Hệ điều hành {#operating-system}

Trình cài đặt hỗ trợ các hệ điều hành sau:

| Bản phân phối | Phiên bản |
|---|---|
| Ubuntu | 22.04, 24.04, 26.04 |
| Debian | 12, 13 |

Trình cài đặt đã được thử trên máy chủ mới cài của từng hệ điều hành này, trên x86-64. Bản ARM64 chưa được thử trên phần cứng ARM thật.

Trên hệ điều hành khác, trình cài đặt dừng lại và báo hệ điều hành "is not supported yet" (chưa được hỗ trợ). Bạn có thể đặt `NEFO_FORCE=1` để thử (xem [Cài đặt](./install#environment-variables)), nhưng phần quản lý website cần `apt-get`, `systemctl` và `useradd`. Nếu thiếu, trang **Website** sẽ báo "Quản lý website cần máy chủ Debian hoặc Ubuntu có systemd, nên không dùng được trên máy này."

Trình cài đặt còn cần:

- quyền root (đăng nhập bằng root, hoặc dùng `sudo`);
- systemd;
- `curl` và `sha256sum`;
- `openssl`, để kiểm tra chữ ký của bản phát hành.

Nếu máy chưa có `curl`, hãy cài trước bằng `apt install curl`.

## CPU

Nefo chạy trên x86 64-bit (`x86_64`, còn gọi là amd64) và ARM 64-bit (`aarch64`, còn gọi là arm64). Với CPU khác, trình cài đặt dừng lại và báo "unsupported CPU architecture".

## Bộ nhớ và ổ đĩa {#memory-and-disk}

Nefo được làm cho VPS nhỏ có 1 GB RAM. Bản thân panel là một chương trình khoảng 25 MB, và dùng khoảng 34 MB bộ nhớ (đo trên một máy chủ thử nghiệm sau khi cập nhật lên v0.24.0). Bạn chỉ cài những thành phần mình cần. Trang **Phần mềm** cho biết mỗi thành phần tốn khoảng bao nhiêu RAM. Ví dụ, khi không có tải, web server dùng khoảng 15 MB (Nginx), 26 MB (OpenLiteSpeed) hoặc 40 MB (Apache).

Trên VPS là một container (LXC, OpenVZ), Nefo tính theo giới hạn bộ nhớ của chính container đó, không phải bộ nhớ của máy chủ vật lý chứa nó (xem [VPS là một container](./first-steps#a-vps-that-is-a-container)).

Có một thành phần cần nhiều RAM hơn hẳn: phần mềm diệt virus ClamAV (tùy chọn) cần khoảng 1,4 GB RAM trống khi quét, nên chỉ hợp với máy chủ từ 2 GB trở lên.

Với máy chủ 1 GB RAM, hãy thêm 1 đến 2 GB swap, để lúc đông khách kernel không phải dừng MariaDB hay PHP. Nhiều bản cài VPS không có sẵn swap. **Hệ thống** → **Bộ nhớ swap** tạo file swap chỉ với một cú bấm (xem [Bộ nhớ swap](./os#swap)). Trong container (OpenVZ, LXC), kernel không cho phép dùng swap: hãy hỏi nhà cung cấp.

Dung lượng ổ đĩa cần bao nhiêu tùy vào website, database và bản sao lưu của bạn. Bản sao lưu giữ trên máy chủ nằm trong `/var/backups/panel`, cùng ổ đĩa với website. Trang chủ của panel hiện mức dùng ổ đĩa, và panel cảnh báo khi ổ đĩa đầy 90%.

## Nên dùng máy chủ mới cài {#a-fresh-server}

Hãy cài Nefo lên máy chủ vừa cài mới. Trình cài đặt không kiểm tra máy đã có những gì, nhưng panel tự thiết lập web server, PHP, MariaDB, tường lửa và SSH:

- Panel chỉ chạy một web server (Nginx, Apache hoặc OpenLiteSpeed). Panel từ chối cài web server khi máy đã có một web server khác, vì cả hai đều cần cổng 80 và 443.
- Panel tự viết cấu hình cho các phần mềm nó quản lý. Đừng cài Nefo chung với một panel quản trị khác.

Trình cài đặt không chạy lần thứ hai trên cùng máy chủ: khi đã có `/etc/nefo/config.json`, nó dừng lại và báo "the panel is already installed".

## Cổng mạng {#ports}

| Cổng | Dùng cho | Khi nào |
|---|---|---|
| Một cổng TCP ngẫu nhiên từ 20000 đến 49999 | Panel | Ngay từ khi cài. Trình cài đặt in cổng này ra. |
| 80 và 443 (TCP) | Web server | Khi bạn cài web server từ panel |
| 443 (UDP) | HTTP/3 của OpenLiteSpeed | Khi web server là OpenLiteSpeed |
| Cổng SSH của bạn (mặc định 22) | SSH | Luôn luôn |
| 3306 (TCP) | MariaDB hoặc MySQL | Chỉ khi bạn cho phép truy cập database từ xa, và chỉ cho các địa chỉ bạn liệt kê |
| 5432 (TCP) | PostgreSQL | Chỉ khi bạn cho phép truy cập từ xa vào một database PostgreSQL, và chỉ cho các địa chỉ bạn liệt kê |

Một số dịch vụ chỉ lắng nghe trên chính máy chủ (127.0.0.1) nên bạn không cần mở: bộ nhận webhook khi push Git (cổng 7809, đi qua web server), cổng của các container Docker, và bộ chuyển tiếp mail nội bộ.

## Tường lửa {#firewalls}

### Tường lửa trên máy chủ (ufw) {#the-firewall-on-the-server-ufw}

- Nếu ufw đang bật lúc cài, trình cài đặt mở cổng của panel trên ufw (hoặc trên firewalld nếu máy dùng firewalld).
- Khi bạn cài web server từ panel và ufw đang bật, panel mở cổng 80 và 443. Panel không quản lý firewalld: nếu máy chủ dùng firewalld, bạn tự mở hai cổng này.
- Trang **Tường lửa & SSH** quản lý ufw. Nếu máy chưa cài ufw, trang này báo "Máy chủ này chưa cài ufw." Hãy cài bằng `apt install ufw`.

Khi bạn bật tường lửa từ panel, SSH và cổng của panel vẫn được giữ mở, cùng cổng 80 và 443 nếu đã cài web server. Xem [Bảo mật](./security).

### Tường lửa cloud của nhà cung cấp {#your-provider-s-cloud-firewall}

Nhiều nhà cung cấp có tường lửa nằm ngoài máy chủ, thường gọi là security group hoặc cloud firewall. Trình cài đặt không đổi được tường lửa này. Bạn hãy tự cho phép:

1. Cổng SSH của bạn.
2. Cổng TCP của panel, được in ra ở cuối quá trình cài.
3. Cổng TCP 80 và 443, khi bạn chạy website. Let's Encrypt kiểm tra tên miền qua cổng 80, nên khi cổng này đóng thì không xin được chứng chỉ.

## Tên miền {#a-domain-name}

Bạn không cần tên miền để cài Nefo: bạn mở panel bằng địa chỉ IP của máy chủ. Bạn cần tên miền cho từng website, và có thể đặt cho panel một tên miền riêng kèm chứng chỉ miễn phí (xem [Những việc đầu tiên](./first-steps#give-the-panel-its-own-domain)). Hãy trỏ bản ghi A của tên miền về địa chỉ IP của máy chủ.
