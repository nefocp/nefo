---
title: Docker
description: Chạy container và dự án Compose, mở terminal trong container, quản lý image, và đưa một container ra Internet qua website làm reverse proxy.
---

# Docker

Trang **Docker** hiển thị và điều khiển các container, dự án Compose và image trên máy chủ. Trang này hướng dẫn từng thẻ, các bước kiểm tra an toàn mà panel chạy với dự án Compose, cách các ứng dụng Docker từ Kho ứng dụng liên quan, và cách đưa một container ra Internet qua website.

## Cài Docker

Vào **Phần mềm** → **Docker** và chọn **Cài**. Panel cài `docker.io` từ kho phần mềm của hệ điều hành, kèm plugin Compose nếu có. Mục **Docker** hiện ra trên menu khi đã cài Docker.

Nếu sau này thiếu plugin Compose, panel sẽ tự cài nó vào lần đầu bạn khởi động một dự án Compose.

Để gỡ Docker, xem trang [Phần mềm](./software#khi-viec-go-bi-tu-choi). Việc gỡ bị từ chối khi còn container, dự án Compose hoặc ứng dụng Docker.

## Vì sao cổng chỉ mở trên 127.0.0.1

Cổng do Docker mở sẽ bỏ qua tường lửa ufw: một container mở cổng trên mọi địa chỉ sẽ lộ ra Internet, bất kể ufw cấu hình thế nào. Vì vậy panel:

- chỉ mở cổng của các container nó khởi động trên `127.0.0.1`;
- từ chối dự án Compose mở cổng trên bất kỳ địa chỉ nào khác.

Để đưa một container ra Internet, hãy đặt một website phía trước nó. Xem [Đưa container lên một website](#đua-container-len-mot-website).

## Container

Thẻ **Container** liệt kê mọi container trên máy chủ, đang chạy hay không, cùng image, trạng thái và cổng. Container của dự án Compose và của ứng dụng từ Kho ứng dụng cũng hiện ở đây.

Dòng của mỗi container có nút **Bật** hoặc **Dừng**, các thao tác khác nằm trong menu của nó:

| Thao tác | Tác dụng |
|---|---|
| **Bật** / **Dừng** | Khởi động hoặc dừng container |
| **Terminal** | Mở dòng lệnh bên trong container. Chỉ có với container đang chạy |
| **Khởi động lại** | Khởi động lại container |
| **Log** | Hiện 200 dòng output cuối cùng, kèm thời gian |
| **Xóa** | Xóa container, kể cả khi nó đang chạy |

::: warning
**Xóa** làm mất dữ liệu của container nếu dữ liệu không được lưu trong volume.
:::

### Chạy một container

1. Ở thẻ **Container**, chọn **Chạy container**.
2. Điền:
   - **Image**, ví dụ `nginx:alpine`;
   - **Tên container**, ví dụ `my-app`;
   - **Cổng trên máy chủ** (1024 đến 65535) và **Cổng trong container**, nếu container có phục vụ gì đó;
   - **Biến môi trường (mỗi dòng một TÊN=giá trị)**, ví dụ `TZ=UTC`.
3. Chọn **Chạy**.

Panel tải image nếu cần và khởi động container với chính sách khởi động lại `unless-stopped`, nên container sẽ tự chạy lại sau khi máy chủ khởi động lại. Cổng chỉ được mở trên `127.0.0.1`.

### Terminal trong container

Chọn **Terminal** để mở dòng lệnh bên trong một container đang chạy. Panel dùng `bash` nếu image có, nếu không thì dùng `sh`. Phiên kết thúc khi bạn đóng cửa sổ hoặc sau 30 phút không gõ gì.

## Dự án Compose

Mỗi dự án Compose là một tệp `compose.yaml` chạy nhiều container cùng nhau. Panel lưu mỗi dự án trong `/var/lib/panel-compose/<tên>/compose.yaml`, chỉ root đọc được, và chạy nó dưới tên dự án Compose `panel-<tên>`.

### Tạo dự án

1. Vào **Docker** → **Compose** và chọn **Dự án mới**.
2. Nhập **Tên dự án**: chữ thường, chữ số và dấu gạch ngang, tối đa 40 ký tự. Container sẽ có tên `panel-<tên>-<dịch-vụ>-1`.
3. Viết nội dung vào ô `compose.yaml`. **Chèn ví dụ** điền sẵn một ví dụ nhỏ chạy được.
4. Chọn **Tạo và khởi động**.

Panel kiểm tra tệp, lưu lại rồi khởi động các container, tải các image cần thiết. Đường dẫn tương đối trong tệp, ví dụ `./data:/data`, được tính từ thư mục của dự án.

Một dự án tối giản:

```yaml
services:
  web:
    image: traefik/whoami
    restart: unless-stopped
    ports:
      - "127.0.0.1:8081:80"
```

`compose.yaml` tối đa 64 KB.

### Sửa dự án

Mở menu của dự án và chọn **Sửa**. Sửa tệp rồi chọn **Lưu và áp dụng**. Panel kiểm tra tệp mới, lưu lại, và tạo lại các container có cấu hình thay đổi. Container của các dịch vụ bạn đã bỏ khỏi tệp cũng bị gỡ.

### Thao tác với dự án

| Thao tác | Tác dụng |
|---|---|
| **Bật** | Kiểm tra tệp và khởi động các container |
| **Dừng** | Dừng các container |
| **Khởi động lại** | Khởi động lại các container |
| **Cập nhật image** | Kiểm tra tệp, tải image mới hơn và tạo lại các container |
| **Gỡ container** | Dừng và gỡ các container. Volume và `compose.yaml` được giữ lại |
| **Log** | 200 dòng cuối của mỗi container |
| **Xóa** | Gỡ container, mạng và thư mục dự án cùng `compose.yaml`. Đánh dấu **Xóa luôn các volume (dữ liệu trong đó sẽ mất)** để xóa cả volume. Image được giữ lại |

### Kiểm tra an toàn

Trước khi lưu một dự án, và trước mỗi lần khởi động hay cập nhật, panel hỏi Compose cấu hình cuối cùng, sau khi đã xử lý include, extends và biến. Panel từ chối dự án có thể làm yếu máy chủ:

| Bị từ chối | Lý do |
|---|---|
| `privileged: true` | Container đặc quyền có toàn quyền trên máy chủ |
| `network_mode`, `pid`, `ipc`, `userns_mode` hoặc `cgroup` đặt là `host` | Container sẽ dùng chung namespace của máy chủ |
| Bất kỳ `cap_add` nào | Cấp thêm đặc quyền cho container |
| Bất kỳ `devices` hoặc `device_cgroup_rules` nào | Giao thiết bị của máy chủ cho container |
| `security_opt` khác `no-new-privileges` | Các tùy chọn như `seccomp=unconfined` hoặc `apparmor=unconfined` nới lỏng giới hạn bảo mật của container |
| Bất kỳ `sysctls` nào | Tham số nhân |
| Cổng không có `127.0.0.1` hoặc `::1` làm địa chỉ máy chủ, ví dụ `"8080:80"`, hoặc cổng không ghi số cổng mở ra | Docker sẽ lắng nghe trên mọi địa chỉ, vượt qua tường lửa |
| Gắn (bind mount) `/etc`, `/root`, `/boot`, `/proc`, `/sys`, `/dev`, hoặc bất cứ thứ gì bên trong chúng | Thư mục hệ thống cho container quyền điều khiển máy chủ |
| Gắn socket Docker, `/var/run/docker.sock` hoặc `/run/docker.sock`, hoặc một thư mục chứa nó, như `/`, `/var`, `/var/run` hoặc `/run` | Socket cho container quyền điều khiển Docker, tức là cả máy chủ |
| Named volume có tùy chọn `device` là một đường dẫn như trên, và secret hay config đọc từ đường dẫn như vậy | Cùng các thư mục đó, gắn theo cách khác |

Đường dẫn được kiểm tra cả như đã viết lẫn sau khi lần theo liên kết tượng trưng. `/etc/localtime` và `/etc/timezone` được phép gắn ở chế độ chỉ đọc (`:ro`), để container dùng múi giờ của máy chủ.

Khi một bước kiểm tra không đạt, trình soạn thảo cho biết dịch vụ và thiết lập nào bị từ chối. Không có gì được lưu hay khởi động.

## Image

Thẻ **Image** liệt kê các image trên máy, cùng dung lượng, ngày tạo và các container đang dùng chúng (**Được dùng bởi**).

- **Xóa** xóa một image. Nếu có container đang dùng, panel nêu tên các container đó và hỏi lại trước khi vẫn xóa image.
- **Xóa image không dùng** xóa mọi image không container nào dùng. Chúng sẽ được tải lại khi cần.

Không có nút tải image riêng. Image được tải khi bạn chạy container, khởi động dự án Compose, hoặc chọn **Cập nhật image**.

## Ứng dụng Docker từ Kho ứng dụng

[Kho ứng dụng](./apps) cài các ứng dụng Docker như Ghost, Uptime Kuma, n8n và Gitea. Chúng chạy dưới dạng dự án Compose mà panel viết từ công thức của từng ứng dụng:

- Tệp và dữ liệu của chúng nằm trong `/var/lib/panel-apps/<tên>`, và dự án Compose có tên `panelapp-<tên>`. Chúng không hiện ở thẻ **Compose**, nhưng container của chúng hiện ở thẻ **Container**.
- Chúng qua cùng các bước kiểm tra an toàn như dự án của bạn.
- Mỗi ứng dụng Docker có website riêng. Các ứng dụng dịch vụ như PostgreSQL, MongoDB và Ollama không có website và chỉ lắng nghe trên `127.0.0.1`.

Hãy quản lý các ứng dụng này ở **Kho ứng dụng** → **Đã cài**, nơi bạn khởi động, dừng, cập nhật và sao lưu chúng. Xem trang [Kho ứng dụng](./apps) về cách gỡ, và trang [Sao lưu](./backups#sao-luu-ung-dung) về bản sao lưu của chúng.

## Đưa container lên một website

Một website có thể chuyển khách truy cập tới cổng của container trên `127.0.0.1`. Khi đó web server đóng vai trò reverse proxy, và bạn có thể cấp chứng chỉ SSL cho site.

1. Mở cổng của container trên `127.0.0.1`, ví dụ `127.0.0.1:8081:80` trong `compose.yaml`, hoặc **Cổng trên máy chủ** là `8081` trong **Chạy container**.
2. Vào **Website** và chọn **Thêm website**.
3. Nhập **Tên miền**, ví dụ `app.example.com`.
4. Ở ô **Loại**, chọn **Ứng dụng (Node.js, Python…) qua proxy**.
5. Ở ô **Cổng ứng dụng lắng nghe (127.0.0.1)**, nhập đúng cổng đó, `8081`.
6. Để trống ô **Lệnh khởi động (không bắt buộc)**: Docker đã chạy ứng dụng.
7. Chọn **Tạo website**.

Cổng phải nằm trong khoảng 1024 đến 65535, và không được là cổng của panel, 3306 (MariaDB) hay 6379 (Redis). Hai website ứng dụng không dùng chung một cổng được.

Sau đó hãy lấy chứng chỉ cho site, xem [Chứng chỉ SSL](./ssl). Xem trang [Website](./websites) về các thiết lập khác của site.
