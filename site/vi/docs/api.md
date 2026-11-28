---
title: Khóa API
description: Điều khiển panel bằng script với khóa API, cho triển khai CI, sao lưu và giám sát, không cần phiên đăng nhập trình duyệt. Khóa làm được và không làm được gì, cách gọi API, ví dụ và mã lỗi.
---

# Khóa API

Khóa API cho phép script dùng panel Nefo mà không cần trình duyệt, ví dụ để triển khai website từ CI, chạy sao lưu từ một cron job trên máy chủ khác, hay đọc trạng thái các website từ công cụ giám sát. Mỗi khóa là chỉ đọc hoặc toàn quyền, có thể giới hạn theo các địa chỉ bạn liệt kê, và có thể hết hạn.

Script gửi khóa kèm mỗi yêu cầu, dưới dạng Bearer token, tới đúng địa chỉ và đường dẫn bí mật của panel.

Khóa API nằm ở **Cài đặt** → **Khóa API**.

## Khóa làm được gì {#what-a-key-can-do}

Mỗi khóa có một trong hai mức quyền:

- **Chỉ đọc**: chỉ các yêu cầu `GET` và `HEAD`. Khóa xem được danh sách website, database và bản sao lưu, đọc nhật ký tác vụ, thống kê và trạng thái máy chủ, nhưng không thay đổi được gì. Khóa cũng không đọc được những thứ cho nó thêm quyền: mật khẩu database và ứng dụng, bản xuất database, file (trình quản lý file), tải bản sao lưu về, giá trị Redis và khóa bí mật webhook Git. Các yêu cầu đó, và mọi yêu cầu khác `GET` hoặc `HEAD`, nhận `403` với `api_key_read_only`.
- **Toàn quyền**: mọi yêu cầu mà chính các trang của panel gửi, dưới tên bạn.

Ngay cả khóa toàn quyền cũng không thể:

- tạo, xem danh sách hay xóa khóa API;
- đổi mật khẩu hay xác thực hai lớp, hoặc đăng xuất (mọi thứ dưới `/api/auth/`);
- mở terminal trên web hay terminal của container;
- dùng các công cụ database nằm sau phần đăng nhập của panel, Adminer và phpMyAdmin.

Các yêu cầu này nhận `403` với `api_key_forbidden`: hãy đăng nhập vào panel để làm. Khóa cũng không đăng nhập được để lấy phiên.

::: warning
Khóa toàn quyền mạnh ngang bạn: nó ghi được file, thêm cron job chạy bằng root và cài phần mềm. Hãy giữ nó bí mật như mật khẩu root. Khóa chỉ đọc cũng là bí mật: nó đọc được cài đặt, nhật ký và danh sách của website và máy chủ.
:::

## Tạo khóa {#create-a-key}

1. Vào **Cài đặt** → **Khóa API** và chọn **Tạo khóa**.
2. Nhập **Tên**, ví dụ `deploy-bot`: tối đa 32 ký tự, gồm chữ, số, dấu cách, dấu chấm, gạch ngang và gạch dưới. Tên hiện trong nhật ký hoạt động.
3. Chọn **Chỉ đọc** hoặc **Toàn quyền**.
4. Không bắt buộc: ở **Địa chỉ được phép (tùy chọn)**, nhập các địa chỉ IP hoặc dải mạng được dùng khóa, như `203.0.113.7` hay `198.51.100.0/24`, IPv4 hoặc IPv6, cách nhau bằng dấu cách hoặc xuống dòng. Để trống nếu cho phép mọi địa chỉ.
5. Chọn khi nào khóa **Hết hạn**: sau 30, 90 hoặc 365 ngày, hoặc **Không hết hạn**.
6. Chọn **Tạo khóa**.

Panel hiện khóa một lần duy nhất, kèm nút **Sao chép**: hãy sao chép ngay, bạn sẽ không xem lại được. Khóa có dạng `nefo_` và 43 ký tự ngẫu nhiên. Panel chỉ giữ mã băm SHA-256 của khóa.

Danh sách hiện tên và các ký tự đầu của mỗi khóa (như `nefo_Ab12cd…`), quyền, các địa chỉ được phép, lần dùng cuối lúc nào và từ đâu (cập nhật tối đa mỗi phút một lần), và ngày hết hạn. Bạn có tối đa 20 khóa. Để thu hồi một khóa, chọn biểu tượng thùng rác của nó và xác nhận: yêu cầu tiếp theo dùng khóa đó sẽ bị từ chối.

## Gọi API {#call-the-api}

Gửi khóa trong header `Authorization`, dạng Bearer token. Địa chỉ là địa chỉ của panel kèm đường dẫn bí mật, rồi `api/` và route:

```sh
export NEFO_KEY='nefo_…'
export NEFO='https://panel.example.com:PORT/<entrance>'

curl -H "Authorization: Bearer $NEFO_KEY" "$NEFO/api/sites"
```

- Khóa chỉ được đọc từ header `Authorization`: không bao giờ từ địa chỉ (`?key=`) hay cookie.
- Yêu cầu có thân JSON cần `Content-Type: application/json`. Trường lạ bị từ chối.
- Yêu cầu dùng khóa không cần header `X-Requested-With` mà các trang của panel gửi.
- Hãy dùng panel qua HTTPS với chứng chỉ thật (**Cài đặt** → **Chứng chỉ**, xem [Bảo mật](./security#https-from-the-first-start)). Tránh `curl -k`: nó sẽ gửi khóa cho bất kỳ ai đứng ở giữa.

Tác vụ dài, như sao lưu, triển khai và chứng chỉ, chạy ở nền. Yêu cầu của chúng trả về `202` kèm một tác vụ:

```json
{ "id": "a1b2c3", "kind": "backup", "target": "example.com", "status": "running", "log": [], "started": "…" }
```

Gọi `GET /api/jobs/{id}` cho tới khi `status` là `succeeded` hoặc `failed`; `log` chứa các dòng nhật ký và `error` cho biết lý do thất bại. Panel chạy mỗi lần một tác vụ: khi tác vụ khác đang chạy, tác vụ mới nhận `409` với `busy`.

## Ví dụ {#examples}

Các route là những route mà trang của panel dùng. Thay `3` bằng `id` website của bạn.

### Liệt kê website {#list-the-websites}

```sh
curl -s -H "Authorization: Bearer $NEFO_KEY" "$NEFO/api/sites"
```

Kết quả là `{"sites": [{"id": 3, "domain": "example.com", "ssl": true, …}]}`. Khóa chỉ đọc là đủ.

### Sao lưu một website {#back-up-a-website}

```sh
curl -s -X POST -H "Authorization: Bearer $NEFO_KEY" -H "Content-Type: application/json" \
  -d '{"kind": "site", "target": "example.com", "remote": true}' \
  "$NEFO/api/backups"
```

- `kind` là `site` (`target` là tên miền), `database` (tên database), `folder` (đường dẫn tuyệt đối) hoặc `app` (tên ứng dụng trong Kho ứng dụng).
- `remote: true` tải bản sao lưu lên cả kho lưu trữ từ xa, nếu đã thiết lập (xem [Sao lưu](./backups#remote-storage)).

Yêu cầu trả về `202` kèm tác vụ. Để chờ tác vụ xong:

```sh
JOB=$(curl -s -X POST -H "Authorization: Bearer $NEFO_KEY" -H "Content-Type: application/json" \
  -d '{"kind": "database", "target": "shop", "remote": false}' "$NEFO/api/backups" | jq -r .id)
while [ "$(curl -s -H "Authorization: Bearer $NEFO_KEY" "$NEFO/api/jobs/$JOB" | jq -r .status)" = running ]; do
  sleep 5
done
```

### Triển khai từ Git {#deploy-from-git}

Trước hết hãy thiết lập kho ở tab **Triển khai Git** của website (xem [Triển khai từ Git](./git-deploy)). Sau đó, từ CI, khi kiểm thử đã qua:

```sh
curl -fsS -X POST -H "Authorization: Bearer $NEFO_KEY" "$NEFO/api/sites/3/git/deploy"
```

Yêu cầu không cần thân và trả về `202` kèm tác vụ. Trong workflow GitHub Actions, hãy để khóa và địa chỉ trong secrets của kho:

```yaml
- name: Deploy
  run: curl -fsS -X POST -H "Authorization: Bearer ${{ secrets.NEFO_KEY }}" "${{ secrets.NEFO_URL }}/api/sites/3/git/deploy"
```

### Cài chứng chỉ {#install-a-certificate}

Để lấy chứng chỉ miễn phí từ Let's Encrypt (xem [Chứng chỉ SSL](./ssl)):

```sh
curl -s -X POST -H "Authorization: Bearer $NEFO_KEY" "$NEFO/api/sites/3/ssl"
```

Yêu cầu trả về `202` kèm tác vụ. Để xác thực qua DNS Cloudflare thay vào đó, điều mà tên wildcard bắt buộc, hãy gửi thêm `-H "Content-Type: application/json" -d '{"dns": true}'`.

Để cài chứng chỉ riêng, ví dụ chứng chỉ được gia hạn trên máy chủ khác, hãy gửi chứng chỉ kèm chuỗi trung gian và khóa riêng, dạng PEM, tổng cộng tối đa 256 KB. `jq` tạo JSON từ các file:

```sh
jq -n --rawfile cert fullchain.pem --rawfile key privkey.pem '{cert: $cert, key: $key}' |
  curl -s -X POST -H "Authorization: Bearer $NEFO_KEY" -H "Content-Type: application/json" \
    --data-binary @- "$NEFO/api/sites/3/certificate"
```

Yêu cầu trả về `200` kèm website và danh sách `warnings`, hoặc `400` kèm lý do, như khóa riêng không khớp với chứng chỉ (xem [Dùng chứng chỉ riêng](./ssl#use-your-own-certificate)).

## Mã lỗi {#errors}

Lỗi trả về dạng JSON, như `{"error": "invalid_api_key"}`, đôi khi kèm `detail`.

| Mã HTTP | `error` | Ý nghĩa |
|---|---|---|
| 401 | `invalid_api_key` | Khóa không tồn tại, đã bị thu hồi hay đã hết hạn, hoặc được dùng từ địa chỉ không được phép. Câu trả lời không nói là trường hợp nào; nhật ký hoạt động thì có. |
| 403 | `api_key_read_only` | Khóa chỉ đọc gửi yêu cầu khác `GET` hoặc `HEAD`, hoặc xin mật khẩu, file, bản xuất hay tải bản sao lưu về. |
| 403 | `api_key_forbidden` | Chỉ phiên đã đăng nhập mới làm được việc này (xem [Khóa làm được gì](#what-a-key-can-do)). |
| 429 | `too_many_attempts` | Địa chỉ này thử sai quá nhiều lần. Hãy chờ `retry_after` giây, cũng có trong header `Retry-After`. |
| 400 | như `bad_request` | Yêu cầu sai; `detail` có thể nói sai ở đâu. |
| 404 | `not_found` | Mục này không tồn tại. Đường dẫn nằm ngoài đường dẫn bí mật nhận một trang 404 trống. |
| 409 | `busy` | Một tác vụ khác đang chạy. Hãy thử lại khi nó xong. |

## Thử sai và fail2ban {#failed-attempts-and-fail2ban}

Một lần dùng khóa thất bại được tính như một lần đăng nhập thất bại, chung một bộ đếm (xem [Giới hạn đăng nhập](./security#sign-in-limits)): 5 lần thất bại trong 15 phút sẽ khóa địa chỉ đó 15 phút, cả với khóa API lẫn trang đăng nhập. Một khóa hợp lệ không xóa bộ đếm.

Mỗi lần thất bại có trong **Tài khoản** → **Hoạt động gần đây** với tên "Đăng nhập thất bại", kèm lý do: `unknown API key`, `API key expired` hoặc `API key not allowed from this address`. Jail của panel trong fail2ban đọc các dòng này, nên nó chặn địa chỉ cứ tiếp tục đoán khóa (xem [Chống dò mật khẩu](./security#brute-force-protection-fail2ban)).

## Nhật ký hoạt động {#the-activity-log}

- Mỗi thay đổi làm bằng khóa (mọi yêu cầu trừ `GET` và `HEAD`) được ghi là "Thay đổi qua khóa API", kèm phương thức và đường dẫn, như `POST /api/sites/3/git/deploy`.
- Người dùng của các dòng này là tên đăng nhập của bạn kèm tên khóa, như `admin (key deploy-bot)`. Các dòng ghi việc mà yêu cầu đã làm, như "Triển khai từ Git", cũng vậy.
- Yêu cầu bị từ chối được ghi là "Từ chối khóa API"; việc tạo và xóa khóa được ghi là "Tạo khóa API" và "Xóa khóa API".
- Yêu cầu chỉ đọc không được ghi. Danh sách khóa cho biết mỗi khóa được dùng lần cuối lúc nào và từ đâu.

Bản thân khóa không bao giờ được ghi vào nhật ký nào.

## Lời khuyên bảo mật {#security-advice}

- **Dùng khóa chỉ đọc khi có thể.** Giám sát và báo cáo chỉ cần `GET`.
- **Mỗi script một khóa.** Tên khóa hiện trong nhật ký, và bạn thu hồi được một khóa mà không làm hỏng các script khác.
- **Chỉ cho phép những địa chỉ cần dùng**, như máy chạy CI hay máy chủ sao lưu của bạn, khi chúng có địa chỉ cố định.
- **Đặt hạn cho khóa và thay khóa định kỳ**: tạo khóa mới, đưa vào script, rồi xóa khóa cũ.
- **Đừng để khóa trong kho mã hay trong địa chỉ.** Hãy dùng kho secret của CI hoặc một file chỉ chủ sở hữu đọc được, và dùng biến trong lệnh để khóa không nằm lại trong lịch sử shell.
- **Xóa khóa ngay nếu nghi bị lộ**, rồi xem **Hoạt động gần đây** để biết khóa đã làm gì.
- Nếu [tường lửa](./security#firewall-ufw) chỉ mở cổng của panel cho một số địa chỉ, hãy cho phép cả địa chỉ của script.
