---
title: Tệp tin
description: Duyệt, sửa, tải lên, tải về, tìm kiếm, nén và giải nén file ngay trên trình duyệt, và khôi phục các mục đã xóa từ thùng rác.
---

# Tệp tin

Trang **Tệp tin** của Nefo là trình quản lý file ngay trên trình duyệt: duyệt thư mục, sửa file văn bản tối đa 2 MB, tải lên tối đa 4 GB mỗi lần, tải về, tải file từ URL, tìm theo tên hoặc nội dung, nén và giải nén, và khôi phục các mục đã xóa từ thùng rác giữ trong 30 ngày. Bên trong thư mục của một website, trình quản lý file làm việc bằng user của website và không ra được ngoài thư mục đó.

Trang này hướng dẫn từng việc trên. Panel chạy bằng quyền root, nên trang này cũng giải thích bạn được vào đâu và panel từ chối thay đổi những gì.

## Di chuyển giữa các thư mục {#find-your-way}

Trình quản lý file mở ở `/var/www`, nơi mỗi website có một thư mục riêng, ví dụ `/var/www/example.com`.

- Chọn tên một thư mục để mở nó. **Lên** đưa bạn về thư mục cha, **Làm mới** đọc lại thư mục.
- Mỗi phần của đường dẫn ở trên cùng đều bấm được. Nút hình ngôi nhà (**Website**) đưa bạn về `/var/www`.
- Để tới thẳng một thư mục bất kỳ, chọn nút cây bút (**Gõ đường dẫn**), gõ đường dẫn tuyệt đối như `/etc/nginx`, rồi chọn **Đi**.
- Bảng có các cột **Tên**, **Dung lượng**, **Quyền**, **Chủ sở hữu** và **Sửa lần cuối**. Thư mục có hơn 5.000 mục chỉ hiện 5.000 mục đầu tiên.

## Bạn được vào đâu {#what-you-can-reach}

Bên ngoài thư mục website, bạn làm việc với quyền root trên toàn bộ hệ thống file.

Bên trong thư mục của một website, ví dụ `/var/www/example.com`, mọi thao tác chỉ giới hạn trong thư mục đó, và một nhãn ghi tên miền hiện cạnh đường dẫn:

- Đường dẫn không thể leo ra ngoài bằng `..`, và liên kết tượng trưng không thể trỏ ra ngoài thư mục. Một site bị hack có cài sẵn liên kết tới `/etc` cũng không lừa được panel động vào file hệ thống.
- File và thư mục mới thuộc về user của website, để PHP của site dùng được chúng.
- Không thể di chuyển, nén hay giải nén giữa thư mục của một website và thư mục khác. Thay vào đó, hãy tải file về rồi tải lên chỗ bạn cần.
- Khi file của website đang [bị khóa](./website-protection#lock-a-website-s-files), bạn chỉ thay đổi được các thư mục cho phép ghi của nó và các thư mục `tmp` và `logs`. Ở những chỗ khác trong thư mục website, trình quản lý file từ chối và báo rằng file đang bị khóa.

Các thư mục sau không thể xóa, đổi tên hay đổi quyền:

- `/`, `/bin`, `/boot`, `/dev`, `/etc`, `/home`, `/lib`, `/lib64`, `/opt`, `/proc`, `/root`, `/run`, `/sbin`, `/srv`, `/sys`, `/tmp`, `/usr`, `/var`;
- `/var/www`, `/var/lib`, `/var/log`, `/var/lib/nefo` và `/etc/nefo`;
- thư mục gốc của từng website, ví dụ `/var/www/example.com` (các file bên trong vẫn thay đổi được);
- thư mục thùng rác `/var/lib/panel-trash` và các thư mục chứa nó.

## Sửa file {#edit-a-file}

1. Chọn tên file. File mở trong trình soạn thảo.
2. Sửa nội dung. Ctrl+S để lưu, phím Tab chèn một ký tự tab.
3. Chọn **Lưu**, rồi **Đóng**.

Trình soạn thảo mở được file văn bản tối đa 2 MB. File nhị phân và file lớn hơn bị từ chối: hãy tải chúng về. File sau khi lưu vẫn giữ nguyên quyền và chủ sở hữu. Nếu bạn đóng trình soạn thảo khi còn thay đổi chưa lưu, panel sẽ hỏi trước.

Để tạo file, chọn **File mới**, nhập tên ở ô **Tên file mới** rồi chọn **OK**. Để tạo thư mục, chọn **Thư mục mới**. File mới có quyền `0644`, thư mục mới có quyền `0755`.

## Tải file lên {#upload-files}

1. Mở thư mục bạn muốn tải file vào.
2. Chọn **Tải lên** và chọn một hoặc nhiều file.
3. Nếu đã có file cùng tên, panel hỏi trước khi ghi đè.

Thanh tiến trình cho biết quá trình tải lên. Mỗi lần tải lên tối đa 4 GB tổng cộng. File tải lên có quyền `0644`. Trong thư mục website, file thuộc về user của website; ở nơi khác, file thuộc về root.

## Tải file về {#download-a-file}

Mở menu của file và chọn **Tải về**. Chỉ tải về được file. Muốn tải cả thư mục, hãy nén nó trước.

## Tải file từ URL {#download-from-a-url}

Máy chủ có thể tải một file từ Internet thẳng vào thư mục, không cần qua máy tính của bạn.

1. Mở thư mục đích và chọn **Tải từ URL**.
2. Nhập **Địa chỉ file (URL)** và chọn **Tải về**.
3. Log hiện tiến trình. Bạn có thể chọn **Ẩn (vẫn tiếp tục tải)**.

Quy tắc:

- Chỉ địa chỉ `http` và `https`, tối đa 5 lần chuyển hướng.
- File tối đa 2 GB.
- Panel không bao giờ tải từ mạng nội bộ của máy chủ: địa chỉ loopback, địa chỉ mạng riêng và link-local (như `127.0.0.1`, `10.x.x.x` và `169.254.169.254`), cùng các địa chỉ của chính máy chủ, đều bị từ chối. Panel kiểm tra lại ở mỗi lần chuyển hướng.
- Tên file lấy từ máy chủ nguồn hoặc từ URL. File đã có không bao giờ bị ghi đè: panel thêm ` (1)`, ` (2)` và tiếp tục như vậy.
- File thuộc về chủ sở hữu của thư mục, với quyền `0640`.

## Tìm kiếm {#search}

1. Mở thư mục cần tìm.
2. Gõ vào ô tìm kiếm (**Tìm trong thư mục này**) và nhấn Enter.
3. Trong hộp thoại, chỉnh ô **Tên có chứa**. Để tìm chữ bên trong file, đánh dấu **Tìm cả nội dung bên trong file** và điền **Đoạn chữ trong file**.
4. Chọn **Tìm**.

Việc tìm không phân biệt chữ hoa, chữ thường. Chọn một kết quả để mở thư mục chứa nó. Nếu là file văn bản, file cũng mở trong trình soạn thảo.

Các giới hạn giúp một lần tìm không làm máy chủ quá tải:

| Giới hạn | Giá trị |
|---|---|
| Số mục được xem | 20.000 |
| Số kết quả | 200 |
| Thời gian | 20 giây |
| Tìm nội dung | Bỏ qua file nhị phân và file lớn hơn 2 MB |
| Độ dài từ khóa | 200 ký tự |

Liên kết tượng trưng được liệt kê nhưng không bao giờ được đi theo. Khi tìm từ `/`, panel bỏ qua `/proc`, `/sys` và `/dev`. Khi chạm giới hạn, panel sẽ báo: hãy tìm trong thư mục nhỏ hơn hoặc dùng từ khóa cụ thể hơn.

## Nén và giải nén {#compress-and-extract}

### Nén {#compress}

1. Đánh dấu các file và thư mục cần nén.
2. Chọn **Nén** trên thanh lựa chọn.
3. Nhập **Tên file nén (.zip hoặc .tar.gz)** và chọn **OK**.

File nén được tạo trong thư mục hiện tại. Liên kết tượng trưng và file đặc biệt bị bỏ qua.

### Giải nén {#extract}

1. Mở menu của file nén và chọn **Giải nén**.
2. Ô nhập điền sẵn thư mục hiện tại. Đổi nó nếu muốn giải nén vào chỗ khác, rồi chọn **OK**.

Panel giải nén được file `.zip`, `.tar.gz`, `.tgz` và `.tar`:

- File đã có cùng tên sẽ bị ghi đè.
- Liên kết tượng trưng và file đặc biệt trong file nén bị bỏ qua.
- File nhận quyền `0644`, hoặc `0755` nếu vốn là file thực thi. Trong thư mục website, file thuộc về user của website; ở nơi khác, file thuộc về root.
- File nén có nội dung trên 20 GB hoặc hơn 500.000 file bị từ chối.

Nén và giải nén chạy dưới dạng tác vụ có log trực tiếp, nên vẫn tiếp tục khi bạn rời trang.

## Đổi tên hoặc di chuyển {#rename-or-move}

Mở menu của một mục và chọn **Đổi tên**, nhập tên mới rồi chọn **OK**. Tên mới được hiểu tương đối so với thư mục hiện tại, nên một đường dẫn như `old/index.html` hoặc `../index.html` sẽ di chuyển mục đó trong cùng website, hoặc trong cùng phần còn lại của máy chủ.

## Đổi quyền {#change-permissions}

Chọn giá trị quyền của một mục ở cột **Quyền**, hoặc mở menu của nó và chọn **Quyền**. Nhập giá trị mới ở hệ bát phân, ví dụ `0644` cho file hoặc `0755` cho thư mục, rồi chọn **OK**.

Giá trị tối đa là `0777`. Không đặt được bit setuid, setgid và sticky ở đây.

Cột **Chủ sở hữu** hiện chủ sở hữu và nhóm. Trình quản lý file không đổi chủ sở hữu. Hãy dùng [Terminal](./system#terminal) cho việc đó, ví dụ `chown -R example_com:example_com /var/www/example.com/public` với user riêng của website.

## Xóa file và thùng rác {#delete-files-and-the-trash}

1. Đánh dấu các mục, hoặc mở menu của một mục, rồi chọn **Xóa**.
2. Mặc định các mục được chuyển vào thùng rác: chọn **Chuyển vào thùng rác**.
3. Để bỏ qua thùng rác, đánh dấu **Xóa vĩnh viễn (không qua thùng rác)** và chọn **Xóa vĩnh viễn**.

::: danger
Mục bị xóa vĩnh viễn, kể cả thư mục cùng toàn bộ nội dung, không thể lấy lại.
:::

### Khôi phục từ thùng rác {#restore-from-the-trash}

1. Mở một thư mục của website có file cần lấy lại, hoặc một thư mục bất kỳ ngoài các website với phần còn lại của máy chủ.
2. Chọn **Thùng rác**.
3. Chọn **Khôi phục** cạnh mục cần lấy lại.

Mỗi website có thùng rác riêng, còn phần còn lại của máy chủ dùng chung một thùng rác. Mục được trả về đúng đường dẫn cũ, cùng chủ sở hữu và quyền. Thư mục cha còn thiếu sẽ được tạo. Nếu hiện đã có thứ khác ở đường dẫn đó, việc khôi phục bị từ chối: hãy chuyển nó đi trước.

Trong thùng rác, bạn cũng có thể **Xóa** vĩnh viễn từng mục, hoặc chọn **Dọn sạch thùng rác**. Danh sách hiện 1.000 mục bị xóa gần đây nhất.

### Các mục được giữ bao lâu {#how-long-items-stay}

Các mục được giữ trong thùng rác **30 ngày**. Mỗi giờ panel kiểm tra một lần và xóa vĩnh viễn các mục cũ hơn.

- Thùng rác nằm ở `/var/lib/panel-trash/<domain>`, hoặc `/var/lib/panel-trash/_server` cho phần còn lại của máy chủ. Chỉ root đọc được, nên user của website không truy cập được file đã xóa của nó.
- Các mục trong thùng rác vẫn chiếm dung lượng ổ đĩa. Hãy xóa vĩnh viễn các file rất lớn.
- Một mục lớn hơn 1 GB nằm trên ổ đĩa khác với thùng rác thì không chuyển vào thùng rác được. Hãy xóa vĩnh viễn nó.
- Khi xóa một website, thùng rác của website đó cũng bị xóa.
