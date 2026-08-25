#!/bin/sh
# deb/rpm 安装后刷新 systemd;apk(Alpine)无 systemd,忽略错误
systemctl daemon-reload >/dev/null 2>&1 || true
