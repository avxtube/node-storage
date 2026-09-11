# Server log maintenance

คู่มือนี้ใช้เมื่อตรวจพบว่า Storage Node ลบไฟล์ media แล้ว แต่พื้นที่ดิสก์ยังไม่ลด หรือ `/var/log` ใช้พื้นที่มากผิดปกติ

## ตรวจสอบพื้นที่

```bash
df -h
sudo du -xhd1 /var/log | sort -h
sudo find /var/log -xdev -type f -printf '%s\t%p\n' \
  | sort -nr \
  | head -30 \
  | numfmt --field=1 --to=iec
journalctl --disk-usage
```

ตรวจหาไฟล์ที่ถูกลบแล้ว แต่ process ยังเปิดใช้งานอยู่:

```bash
sudo lsof +L1
```

หาก nginx-vod ยังเปิดไฟล์ media ที่ถูกลบอยู่ ให้ restart เฉพาะ nginx-vod เพื่อคืนพื้นที่:

```bash
sudo systemctl restart nginx-vod
sudo lsof +L1
df -h
```

## ปัญหา nginx-vod เขียน log ต่อในไฟล์ `.log.1`

ไฟล์ `/etc/logrotate.d/nginx` ปกติจับคู่ `/var/log/nginx/*.log` ซึ่งรวมทั้ง log ของ nginx ปกติและ nginx-vod แต่คำสั่ง `postrotate` ของระบบส่งสัญญาณเปิด log ใหม่ให้เฉพาะ `nginx.service` เท่านั้น

ผลคือ nginx-vod อาจยังเขียนลงไฟล์ที่ถูกหมุนเป็น `vod-access.log.1` หรือ `vod-error.log.1` ทำให้ไฟล์เหล่านี้โตต่อเนื่องและยังบีบอัดไม่ได้

แก้ส่วน `postrotate` ใน `/etc/logrotate.d/nginx` เป็น:

```conf
postrotate
        invoke-rc.d nginx rotate >/dev/null 2>&1 || true

        if [ -s /run/nginx-vod.pid ]; then
                kill -USR1 "$(cat /run/nginx-vod.pid)" 2>/dev/null || true
        fi
endscript
```

`SIGUSR1` สั่งให้ nginx-vod เปิดไฟล์ log ชุดใหม่โดยไม่หยุดให้บริการหรือยกเลิก request ที่กำลังทำงาน

ทดสอบ configuration ก่อนบังคับหมุน log:

```bash
sudo logrotate -d /etc/logrotate.d/nginx
sudo logrotate -f /etc/logrotate.d/nginx
sudo lsof /var/log/nginx/*
sudo du -sh /var/log/nginx
```

หลังหมุน log process ของ nginx-vod ควรเปิด `vod-access.log` และ `vod-error.log` ชุดใหม่ ไม่ควรเปิดเขียน `.log.1` ต่อ

## จำกัดขนาด nginx log

ตัวอย่างค่าที่เหมาะกับเครื่อง Storage Node:

```conf
/var/log/nginx/*.log {
        daily
        maxsize 100M
        missingok
        rotate 3
        compress
        delaycompress
        notifempty
        create 0640 www-data adm
        sharedscripts
        prerotate
                if [ -d /etc/logrotate.d/httpd-prerotate ]; then
                        run-parts /etc/logrotate.d/httpd-prerotate
                fi
        endscript
        postrotate
                invoke-rc.d nginx rotate >/dev/null 2>&1 || true

                if [ -s /run/nginx-vod.pid ]; then
                        kill -USR1 "$(cat /run/nginx-vod.pid)" 2>/dev/null || true
                fi
        endscript
}
```

ถ้าไม่ต้องใช้ access log ของ nginx-vod สามารถปิดใน nginx-vod configuration เพื่อลดทั้ง disk I/O และพื้นที่:

```nginx
access_log off;
```

ควรเก็บ `error_log` ไว้สำหรับวิเคราะห์ปัญหา โดยปรับระดับเป็น `warn` หรือ `error` ตามความเหมาะสม

## จำกัด systemd journal

ลด journal ที่มีอยู่ให้เหลือไม่เกิน 500 MB:

```bash
sudo journalctl --vacuum-size=500M
```

สร้าง `/etc/systemd/journald.conf.d/limits.conf`:

```ini
[Journal]
SystemMaxUse=500M
SystemKeepFree=2G
RuntimeMaxUse=100M
MaxRetentionSec=7day
```

จากนั้นโหลดค่าใหม่:

```bash
sudo systemctl restart systemd-journald
journalctl --disk-usage
```

## ตรวจสอบหลังแก้ไข

```bash
sudo logrotate -d /etc/logrotate.d/nginx
sudo systemctl status nginx nginx-vod --no-pager
sudo lsof +L1
sudo du -sh /var/log/nginx /var/log/journal
df -h
```

พื้นที่จากไฟล์ที่ถูกลบจะคืนให้ระบบเมื่อ process ปิด file descriptor แล้ว ส่วนพื้นที่ของ log จะลดลงหลังการหมุน บีบอัด หรือลบตาม retention ที่ตั้งไว้
