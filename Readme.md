
# 马工新玩具 通过网络连接无线电

## 服务器端功能描述

本系统可用于讲任意模式的设备进行各种形式的互联， 通过NRL硬件盒子将设备的音频重新编码后，通过网络转发。

比如，可以用于组建类似BM（数字DMR）的网络，将各地模拟中继台，手台， 公网台，相互链接

#
# 测试用服务器列表：

### web管理后台:

http://ham.bi4qzw.com

http://121.5.149.170:9998

http://bg6fcs.nrllink.net:9998


### 服务器语音等控制信号端口：udp 60050 
#

目前支持 BH4TDV出品的NRL系列盒子，比如NRL2100 2200 2300 3188 2600 等

https://item.taobao.com/item.htm?spm=a1z10.5-c-s.w4002-5392156393.12.c83a5e18os23Zo&id=583051536810

#

## 总功能描述（开发中）：

1. 系统基本组成，用户账号，设备，内置群组，公共群组
   1. 用户账号： 注册用户，可以修改个人的设置，可以关联设备，可以改变设备的群组
   2. 设备： NRL各型号的硬件设备，或者软设备：比如手机客户端（APP,小程序，或者浏览器）
   3. 内置群组：内部设备的集合，组内的设备可以相互通信
   4. 公共群组：和外部通信的桥梁，用户可以将自己的内置群组和外部公共群组关联，关联后，可以接收公共群组的语音，内置群组内的设备也可以向公共群组的设备转发语音
2. 设备缺省都在内置0号群组,系统为每个用户预设10个内置群组（包含0号组）设备可以从0号组迁移到其他群组，一个设备只能加入一个组，加入新组，自动从离开原来的组3. 
4. 系统为所有未登记注册认证的设备提供一个测试群组，群组内用户可以立即进行相互通联，这个操作自动完成。设备和用户认证关联后，自动离开测试群组，普通用户无法改变，系统管理员可以。
5. 加入同一个内置群组的设备，相互通联，系统只允许组内一个设备发送语音，其他设备接收。原则是，谁的语音先到服务器先转发，语音先到的设备抢占转发权，持续到本次语音结束后1秒。结束后，重新进入抢发言权阶段。
7. 可以扩展转发文本消息，语音留言，图片，视频，比如短消息，需要硬件设备支持。比如图片，需要设备有显示器
8. 服务器支持会议模式，支持多人同时说话，服务器将多路语音信号进行混音处理后转发给群组内所有设备。
9. BM网关功能，后期可以接入BM网关
10. 可以转发控制指令
11. 控制功能，可以对电台进行远程配置修改，比如频率调整

## WEB功能描述(开发中)：

1. 注册用户可以关联自己的设备
2. 可以将设备加入到任一内置群组
3. 可以改变设备的内置组
4. 可以将内置组和系统全局公共组关联
5. 提供语音对讲页面，样式类似于微信聊天界面，屏幕下面有大大的PTT通话按钮，按住实时对讲，双击可以保持讲话状态，可以发送文本，图片，等消息。
6. APP，小程序，WEB的操作方式风格保持一致
7. 可以修改自己的密码
8. 可以修改设备的安全KEY
9. 可以查看设备状态，电压，温度等
10. 
    

## centos 8 下安装方法：

1. 解压压缩包到 / 目录下 

         tar -zxf udphub.tgz 

2. 关闭防火墙

        systemctl disable firewalld.service
        systemctl stop firewalld.service


3. 安装数据库

        rpm -Uvh https://repo.huaweicloud.com/postgresql/repos/yum/reporpms/EL-8-x86_64/pgdg-redhat-repo-latest.noarch.rpm
        dnf -qy module disable postgresql
        dnf install -y postgresql13-server
        dnf install postgresql13-contrib.x86_64
        /usr/pgsql-13/bin/postgresql-13-setup initdb 

4. 修改 /var/lib/pgsql/13/data/pg_hba.conf 文件，信任127.0.0.1 

        # "local" is for Unix domain socket connections only
        local   all             all                                     trust
        # IPv4 local connections:
        host    all             all             127.0.0.1/32            trust

5. 重启数据库，并开机启动:

        systemctl enable postgresql-13
        systemctl start postgresql-13


6. 创建数据库:

        psql -U postgres 
        create database udphub
    
7. 初始化数据库:

        psql -U postgres udphub < udphub.sql

8. 启动程序:

        cd /udphub
        nohup ./udphub &





# 创建开机自动启动

1.  创建自启动配置文件

        vi /usr/lib/systemd/system/udphub.service

        [Unit]
        Description=udphub
        Documentation=https://Bi4Qzw.Com
        
        [Service]
        WorkingDirectory=/udphub/
        ExecStart=/udphub/udphub
        Restart=on-abnormal
        RestartSec=10s
        KillMode=mixed
    
        [Install]
        WantedBy=multi-user.target

2. 设置开机自启动 相关命令如下: 

    1. 更新配置:

            systemctl daemon-reload
    2. 启动服务:

            systemctl start udphub.service
    3. 设置开机启动:

            systemctl enable udphub.service
 