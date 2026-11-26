---
title: Cập nhật
description: Cập nhật Nefo từ panel hoặc bằng nefo update, cách bản phát hành được kiểm tra, và nên làm gì khi cập nhật lỗi.
---

# Cập nhật

Trang này hướng dẫn cách cập nhật Nefo, cách mỗi bản phát hành được kiểm tra trước khi cài, và nên làm gì nếu cập nhật lỗi. Cho tới phiên bản 1.0, chỉ bản phát hành mới nhất được vá lỗi bảo mật, vì vậy hãy luôn cập nhật Nefo.

## Cập nhật từ panel

1. Mở **Cài đặt → Panel**. Phiên bản hiện tại nằm ngay cạnh tên Nefo.
2. Bấm **Kiểm tra cập nhật**. Nếu bạn đang dùng bản mới nhất, panel báo "Đây là phiên bản mới nhất."
3. Nếu có bản mới hơn, panel báo "Đã có phiên bản v0.2.0." Bấm **Cài bản cập nhật** rồi xác nhận.

Tiến trình hiện ngay trên trang. Khi phiên bản mới đã vào chỗ, panel khởi động lại và trang tự tải lại sau khoảng 20 giây. Website của bạn vẫn chạy trong lúc panel khởi động lại.

Bạn cũng có thể cập nhật từ trang chủ: bấm **Kiểm tra cập nhật**, rồi **Cập nhật lên v0.2.0**.

## Cập nhật bằng dòng lệnh

```sh
nefo update
```

```
Installed: v0.1.0
Latest:    v0.2.0
Release signature verified
Downloading nefo-linux-amd64 v0.2.0
Checked: Nefo v0.2.0 (commit 5e6f7a8)
Installed. The panel restarts now; reload the page in a few seconds.
```

Nếu bạn đang dùng bản mới nhất, lệnh chỉ in hai phiên bản và không thay đổi gì. Chỉ muốn kiểm tra thì chạy `nefo update --check`.

## Điều gì diễn ra khi cập nhật

1. Panel đọc số phiên bản mới nhất từ trang phát hành của dự án trên GitHub.
2. Panel tải danh sách checksum cùng chữ ký của nó, rồi kiểm tra chữ ký bằng khóa của dự án được gắn sẵn trong panel.
3. Panel tải chương trình mới cho CPU của bạn và đối chiếu checksum SHA-256 với danh sách.
4. Panel chạy thử chương trình mới một lần để chắc chắn nó chạy được trên máy chủ này.
5. Panel thay `/usr/local/bin/nefo` trong một bước duy nhất, nên không bao giờ có chương trình ghi dở.
6. Panel khởi động lại dịch vụ `nefo`. Nếu đang có tác vụ chạy, như cài phần mềm, panel để tác vụ đó chạy xong trước, tối đa 75 giây.

Khi phiên bản mới khởi động, nó nâng cấp database của panel lên định dạng mới. Nó cũng cập nhật các tệp cấu hình do panel quản lý (web server, pool PHP-FPM) theo mẫu mới. Nếu web server từ chối các tệp mới, panel đặt lại các tệp cũ.

## Bản phát hành được kiểm tra thế nào

Mỗi bản phát hành trên GitHub gồm chương trình cho amd64 và arm64, `install.sh`, tệp `VERSION`, `checksums.txt` chứa checksum SHA-256 của các tệp, và `checksums.txt.sig` là chữ ký Ed25519 của `checksums.txt`.

Cả trình cài đặt lẫn trình cập nhật đều từ chối một `checksums.txt` không được ký bằng khóa của dự án, và một tệp tải về có checksum không khớp. Khi cập nhật, lỗi sẽ là:

- "checksums.txt has no valid signature from the project: the update was not installed" (không có chữ ký hợp lệ của dự án);
- "the download does not match checksums.txt: not installed" (tệp tải về không khớp checksum).

Nhờ vậy, kể cả người có thể sửa tệp trên GitHub hay trên đường truyền tới máy chủ của bạn cũng không cài được chương trình của họ.

Bản build phát triển (không có khóa của dự án) không kiểm tra được chữ ký. Nó chỉ kiểm tra checksum, và log ghi "Development build: the release signature is not checked".

## Những gì được giữ lại

Bản cập nhật chỉ thay chương trình `/usr/local/bin/nefo`. Mọi thứ khác giữ nguyên:

- tệp cấu hình `/etc/nefo/config.json`, gồm cả cổng và đường dẫn bí mật;
- thư mục dữ liệu `/var/lib/nefo`: tài khoản và xác thực hai lớp, cài đặt, website, lịch sao lưu, chứng chỉ và nhật ký hoạt động;
- website, database, tệp tin, bản sao lưu của bạn và các dịch vụ chạy chúng.

Chỉ panel khởi động lại. Website, database và các dịch vụ khác vẫn chạy.

## Nếu cập nhật lỗi {#if-an-update-fails}

**Cập nhật dừng trước khi cài chương trình mới.** Chuyện này xảy ra khi tải về lỗi, chữ ký hay checksum không khớp, hoặc chương trình mới không chạy được trên máy chủ này. Không có gì thay đổi: phiên bản cũ vẫn chạy. Lỗi hiện trong log tác vụ của panel hoặc trên terminal. Hãy thử lại sau, và báo lỗi nếu vẫn gặp.

**Phiên bản mới không khởi động được.** Kiểm tra dịch vụ và log của nó:

```sh
systemctl status nefo
journalctl -u nefo -n 50 --no-pager
```

Báo lỗi trên [kho GitHub](https://github.com/nefocp/nefo/issues), kèm kết quả trên. Muốn quay về bản trước, hãy cài chương trình của bản đó bằng tay. Ví dụ dưới đây dùng `v0.1.0` trên máy chủ amd64:

```sh
cd /tmp
curl -fsSLO https://github.com/nefocp/nefo/releases/download/v0.1.0/nefo-linux-amd64
curl -fsSLO https://github.com/nefocp/nefo/releases/download/v0.1.0/checksums.txt
grep " nefo-linux-amd64$" checksums.txt | sha256sum -c -
install -m 0755 nefo-linux-amd64 /usr/local/bin/nefo
systemctl restart nefo
```

Chỉ làm tiếp khi `sha256sum` in ra `OK`. Trên máy chủ ARM, dùng `nefo-linux-arm64`.

::: warning Quay về phiên bản cũ
Nếu phiên bản mới đã thay đổi database của panel, phiên bản cũ sẽ không chịu khởi động và báo "database schema version … is newer than this panel supports". Khi đó chỉ phiên bản mới hơn chạy được với dữ liệu đó. Trước một lần cập nhật quan trọng, bạn có thể giữ một bản sao thư mục dữ liệu: `systemctl stop nefo`, rồi `cp -a /var/lib/nefo /root/nefo-data-copy`, rồi `systemctl start nefo`.
:::

## Cập nhật tự động

Nefo không tự cập nhật, và cũng không tự kiểm tra bản cập nhật. Nó chỉ kiểm tra khi bạn bấm **Kiểm tra cập nhật** hoặc chạy `nefo update`. Để biết khi có bản mới, hãy theo dõi mục phát hành của [kho GitHub](https://github.com/nefocp/nefo/releases).

Điều này chỉ nói về bản thân panel. Các gói của hệ điều hành là chuyện riêng: panel kiểm tra chúng vài giờ một lần, hiện chúng ở **Hệ thống** → **Cập nhật**, và có thể tự động cài bản cập nhật bảo mật. Xem [Hệ thống](./os#updates).

## Bản cập nhật lấy từ đâu {#where-updates-come-from}

Mặc định, bản cập nhật lấy từ `https://github.com/nefocp/nefo/releases/latest/download`. Trường `update_url` trong `/etc/nefo/config.json` có thể trỏ tới một địa chỉ khác có cùng các tệp đó. Địa chỉ phải bắt đầu bằng `https://`, nếu không việc kiểm tra cập nhật sẽ lỗi với thông báo "update_url trong config.json phải là địa chỉ https://." Hãy để trống trừ khi bạn biết chắc mình cần.
