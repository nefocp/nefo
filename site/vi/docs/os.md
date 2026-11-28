---
title: Hệ thống
description: Cài bản cập nhật hệ điều hành, bật tự động cập nhật bảo mật, thêm swap, đặt múi giờ, đồng bộ đồng hồ và tên máy, bật TCP BBR và giải phóng ổ đĩa.
---

# Hệ thống

Trang **Hệ thống** cài bản cập nhật hệ điều hành, được kiểm tra vài giờ một lần và có đánh dấu bản cập nhật bảo mật, bật tự động cập nhật bảo mật, tạo file swap, đặt múi giờ, đồng bộ đồng hồ và tên máy, bật TCP BBR, và giải phóng dung lượng ổ đĩa. Mọi thay đổi trên trang này đều được ghi vào nhật ký hoạt động.

Trang nằm trong nhóm **Máy chủ** ở menu bên trái. Trang có năm thẻ: **Cập nhật**, **Bộ nhớ swap**, **Thời gian & tên máy**, **Mạng** và **Dọn dẹp**. Nhật ký hoạt động nằm ở **Tài khoản** → **Hoạt động gần đây**.

Các thay đổi mất thời gian, như cài bản cập nhật hay tạo file swap, chạy dưới dạng tác vụ nền: trang hiện tiến trình của chúng, và mỗi lúc chỉ chạy một tác vụ.

## Cập nhật {#updates}

Thẻ **Cập nhật** liệt kê các bản cập nhật đang chờ của hệ điều hành: từng gói, phiên bản đang cài và phiên bản mới, cùng nguồn của phiên bản mới (suite, như `noble-updates`). Bản cập nhật từ suite bảo mật (`noble-security`, `bookworm-security`…) được đánh dấu **Bảo mật** và xếp lên đầu. Gói ghi **mới** là gói được cài thêm cùng bản cập nhật, ví dụ một kernel mới.

### Panel kiểm tra cập nhật thế nào {#how-the-panel-checks-for-updates}

- Panel tự kiểm tra: mỗi giờ nó xem lần kiểm tra gần nhất, và khi lần đó đã quá 6 giờ, nó chạy `apt-get update` rồi hỏi apt xem một lần nâng cấp sẽ cài những gì. Panel bỏ qua lượt kiểm tra khi đang có tác vụ khác chạy, và trong 30 phút đầu sau khi panel khởi động.
- Mở trang không bao giờ chạy apt: trang hiện kết quả của lần kiểm tra gần nhất, kèm thời điểm.
- **Kiểm tra ngay** kiểm tra ngay lập tức, dưới dạng một tác vụ.
- **Tạm giữ lại** liệt kê những bản cập nhật apt chưa cài: các gói apt giữ lại vì cần thay đổi thêm, và bản cập nhật theo đợt (phased update) của Ubuntu, được đưa dần tới các máy chủ trong vài ngày.

Khi có bản cập nhật bảo mật đang chờ, mục **Cần xử lý** trên trang chủ và chuông thông báo hiện "Có N bản cập nhật bảo mật".

### Cài bản cập nhật {#install-updates}

- **Cài tất cả bản cập nhật** chạy `apt-get upgrade --with-new-pkgs`. Lệnh này cài thêm các gói mới mà bản cập nhật cần, như kernel mới, và không bao giờ gỡ gói nào.
- **Chỉ cài bản cập nhật bảo mật** chỉ cập nhật các gói đã cài có phiên bản mới đến từ suite bảo mật (`apt-get install --only-upgrade` kèm tên các gói đó).

Cả hai đều làm mới danh sách gói trước, chạy không hỏi gì, và giữ nguyên các file cấu hình bạn đã sửa. Khi xong, danh sách được kiểm tra lại.

::: warning Dịch vụ khởi động lại trong giây lát
Cập nhật Nginx, Apache, OpenLiteSpeed, PHP, MariaDB hoặc Redis sẽ khởi động lại dịch vụ đó, nên website có thể không phản hồi vài giây. Cập nhật Docker sẽ khởi động lại Docker và các container của nó. Trang và hộp xác nhận đều nêu tên các dịch vụ mà bản cập nhật đang chờ sẽ khởi động lại. Hãy cài những bản cập nhật này vào lúc ít truy cập.
:::

### Cần khởi động lại {#restart-required}

Một số bản cập nhật, như kernel mới hay thư viện C mới, chỉ hoàn tất sau khi máy chủ khởi động lại. Thẻ này báo "Máy chủ cần khởi động lại để hoàn tất việc cài bản cập nhật." khi:

- có file `/var/run/reboot-required` (Ubuntu tạo file này, và liệt kê các gói trong `/var/run/reboot-required.pkgs`), hoặc
- trong `/boot` đã có kernel mới hơn kernel đang chạy (Debian không phải lúc nào cũng tạo file trên).

**Khởi động lại trong Cài đặt** mở **Cài đặt** → **Panel** → **Nguồn**, nơi bạn khởi động lại máy chủ. Chuông thông báo cũng hiện "Máy chủ cần khởi động lại" cho tới khi bạn làm việc đó.

### Tự động cập nhật bảo mật {#automatic-security-updates}

Công tắc **Tự động cập nhật bảo mật** cho máy chủ tự cài các bản cập nhật bảo mật mỗi ngày:

- Nếu chưa có `unattended-upgrades`, panel cài nó trước, dưới dạng tác vụ. Chương trình phụ của nó chờ máy chủ tắt dùng khoảng 20 MB bộ nhớ.
- Panel đặt `APT::Periodic::Update-Package-Lists "1";` và `APT::Periodic::Unattended-Upgrade "1";` trong `/etc/apt/apt.conf.d/20auto-upgrades`, giữ nguyên các dòng khác của file, và bật các timer hằng ngày của apt.
- Tắt công tắc sẽ đặt `APT::Periodic::Unattended-Upgrade "0";`.

Những bản cập nhật nào được cài là do cài đặt riêng của unattended-upgrades trong `/etc/apt/apt.conf.d/50unattended-upgrades` quyết định: trên Ubuntu và Debian, chỉ bản cập nhật bảo mật. Nó không khởi động lại máy chủ, trừ khi bạn bật `Unattended-Upgrade::Automatic-Reboot` ở đó. Nếu một file khác trong `/etc/apt/apt.conf.d` ghi đè thiết lập này, panel báo cho bạn và để file như cũ.

## Bộ nhớ swap {#swap}

Swap là phần ổ đĩa mà kernel dùng khi hết bộ nhớ. Với máy chủ có 1 hoặc 2 GB bộ nhớ, 1 đến 2 GB swap giúp MariaDB và PHP không bị kernel dừng khi lúc đông khách cần nhiều bộ nhớ hơn máy chủ có.

Thẻ **Bộ nhớ swap** hiện bộ nhớ, lượng swap đang dùng, và từng file hoặc phân vùng swap trong `/proc/swaps`.

### Tạo file swap {#create-a-swap-file}

1. Chọn **Dung lượng**: 512 MB, 1 GB, 2 GB, 4 GB hoặc 8 GB. Chỉ những dung lượng còn chừa lại 1 GB trống trên ổ đĩa mới được hiện.
2. Chọn **Tạo file swap**.

Panel tạo `/panel.swap`, chỉ root đọc được, bằng `fallocate`. Khi hệ thống file không hỗ trợ cách này, hoặc `swapon` từ chối file đó, panel ghi file bằng `dd`, mất khoảng một phút cho mỗi vài GB. Trên btrfs, file được đánh dấu không copy-on-write trước. Sau đó panel chạy `mkswap` và `swapon`, rồi thêm `/panel.swap none swap sw 0 0` vào `/etc/fstab`, để swap vẫn bật sau khi khởi động lại.

Panel từ chối tạo file swap:

- khi ổ đĩa không còn trống bằng dung lượng file cộng thêm 1 GB;
- khi máy chủ đã có một file swap không do panel tạo, như `/swapfile` hoặc `/swap.img`. Panel giữ nguyên file đó. Phân vùng swap thì không ngăn việc tạo;
- trên ZFS, nơi file swap không hoạt động;
- trong container (OpenVZ, LXC, Docker), nơi kernel không cho phép `swapon`: "Máy chủ này không cho phép dùng swap". Hãy hỏi nhà cung cấp về swap, hoặc dùng máy chủ ảo đầy đủ.

Nếu có bước nào lỗi, panel xóa file đi.

### Xóa file swap {#remove-the-swap-file}

**Xóa file swap** tắt swap bằng `swapoff`, xóa dòng của nó trong `/etc/fstab` và xóa `/panel.swap`. Dữ liệu trong swap được chuyển lại vào bộ nhớ trước, nên panel từ chối khi bộ nhớ còn trống không chứa đủ.

### Swappiness {#swappiness}

**Swappiness** (`vm.swappiness`, từ 0 đến 100) quyết định kernel chuyển bộ nhớ sang swap dễ đến mức nào. Mặc định là 60; máy chủ nên dùng 10, khi đó swap chỉ được dùng lúc thiếu bộ nhớ. Panel ghi giá trị vào `/etc/sysctl.d/90-panel-swap.conf` và áp dụng ngay.

## Thời gian và tên máy {#time-and-host-name}

Thẻ **Thời gian & tên máy** hiện giờ của máy chủ, múi giờ và đồng hồ đã được đồng bộ hay chưa.

### Múi giờ {#time-zone}

Chọn múi giờ trong danh sách (`timedatectl list-timezones`) rồi chọn **Lưu**. Panel đặt múi giờ bằng `timedatectl set-timezone` và khởi động lại cron, để cron job chạy theo múi giờ mới. Các dịch vụ khác đang chạy, như MariaDB, dùng múi giờ mới sau khi chúng khởi động lại.

### Đồng bộ đồng hồ {#clock-synchronization}

**Đồng bộ đồng hồ (NTP)** bật hoặc tắt việc đồng bộ thời gian (`timedatectl set-ntp`). Hãy để bật: chứng chỉ và mã xác thực hai lớp cần đồng hồ đúng. Tính năng này cần một dịch vụ thời gian như `systemd-timesyncd`. Nếu chưa có, công tắc bị làm mờ: hãy cài bằng `apt install systemd-timesyncd`.

### Tên máy {#host-name}

Nhập **Tên máy** mới rồi chọn **Lưu**. Tên máy gồm chữ cái, chữ số và dấu gạch ngang, các phần cách nhau bằng dấu chấm, tối đa 64 ký tự, và không bắt đầu hay kết thúc bằng dấu gạch ngang: `web1` hoặc `web1.example.com`.

Panel sẽ:

1. đặt tên bằng `hostnamectl set-hostname`;
2. trỏ dòng `127.0.1.1` trong `/etc/hosts` về tên này (tên đầy đủ và phần đầu của nó), hoặc thêm dòng đó;
3. trên máy chủ có cloud-init, ghi `/etc/cloud/cloud.cfg.d/99-panel-hostname.cfg` với `preserve_hostname: true`, để cloud-init không đặt lại tên của nhà cung cấp ở lần khởi động sau.

Tên máy không làm đổi địa chỉ của panel hay chứng chỉ nào. Khi khởi động lại máy chủ từ **Cài đặt** → **Panel** → **Nguồn**, bạn xác nhận bằng cách gõ tên máy chủ: sau khi đổi tên, hãy gõ tên mới.

## Mạng: TCP BBR {#network-tcp-bbr}

Thẻ **Mạng** hiện thuật toán điều khiển tắc nghẽn TCP đang dùng (thường là `cubic`), hàng đợi mặc định (`net.core.default_qdisc`, thường là `fq_codel`) và các thuật toán kernel có.

BBR là thuật toán điều khiển tắc nghẽn của Google. Nó thường giúp website và việc tải file nhanh hơn trên kết nối xa hoặc hay mất gói, ví dụ tới khách truy cập ở châu lục khác hoặc dùng mạng di động. Bật **TCP BBR** sẽ:

1. nạp module `tcp_bbr`, khi kernel có BBR dưới dạng module;
2. ghi `/etc/modules-load.d/panel-bbr.conf` (`tcp_bbr`) và `/etc/sysctl.d/90-panel-bbr.conf` (`net.core.default_qdisc = fq` và `net.ipv4.tcp_congestion_control = bbr`);
3. chạy `sysctl --system`.

Kết nối mới dùng BBR ngay. Hàng đợi `fq` áp dụng cho card mạng sau lần khởi động lại tới; trong lúc đó BBR vẫn chạy tốt.

Panel từ chối khi kernel đang chạy không có BBR, và trong container, nơi không đổi được thiết lập kernel. Khi đó panel không giữ lại gì. Tắt BBR sẽ xóa cả hai file và quay về `cubic` cùng `fq_codel`. Khi BBR được thiết lập bên ngoài panel, công tắc bị làm mờ.

## Dọn dẹp ổ đĩa {#disk-cleanup}

Thẻ **Dọn dẹp** cho biết mỗi mục dọn dẹp giải phóng được bao nhiêu. **Dọn dẹp** hỏi bạn xác nhận, chạy việc dọn dẹp dưới dạng tác vụ, rồi cho biết đã giải phóng được bao nhiêu.

| Mục | Những gì bị xóa |
|---|---|
| **Gói đã tải về** | Các file gói mà apt giữ lại trong `/var/cache/apt/archives` sau khi cài (`apt-get clean`). |
| **Nhật ký hệ thống** | Các mục cũ của nhật ký hệ thống (journal), thu nhỏ xuống 100 MB (`journalctl --rotate`, rồi `journalctl --vacuum-size=100M`). |
| **Log cũ** | Các log trong `/var/log` mà logrotate đã xoay vòng: tên kết thúc bằng `.gz` hoặc một con số, như `syslog.1` hoặc `auth.log.2.gz`. |
| **Gói không dùng** | Các gói không còn cần đến, như kernel cũ (`apt-get autoremove`). |
| **Docker** | Image không container nào dùng, và bộ nhớ đệm build (`docker image prune --all`, `docker builder prune --all`). Chỉ hiện khi đã cài Docker. |

- **Log cũ** giữ lại log của web server trong `/var/log/nginx`, `/var/log/apache2` và `/var/log/openlitespeed`, vì thống kê truy cập của website đọc chúng. Mục này chỉ xóa file trong các thư mục của root và không bao giờ đi theo liên kết.
- Với **Gói không dùng**, hộp xác nhận liệt kê các gói. Nếu tới lúc bạn xác nhận mà apt định gỡ thêm gói nào khác, sẽ không gói nào bị gỡ: "Danh sách gói không dùng đã thay đổi".
- **Docker** không bao giờ xóa container, dù đang chạy hay đã dừng, và không xóa volume, nơi container giữ dữ liệu.

## Nhật ký hoạt động {#activity-log}

Các thao tác sau xuất hiện trong **Tài khoản** → **Hoạt động gần đây**: cài bản cập nhật hệ thống, bật hoặc tắt tự động cập nhật bảo mật, tạo hoặc xóa file swap, đổi swappiness, đổi múi giờ, bật hoặc tắt đồng bộ đồng hồ, đổi tên máy, bật hoặc tắt TCP BBR, và dọn dẹp ổ đĩa.
