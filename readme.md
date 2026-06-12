<p align="center">
    <a href="http://smartping.org">
        <img src="http://smartping.org/logo.png">
    </a>
    <h3 align="center">SmartPing | 开源、高效、便捷的网络质量监控神器</h3>
    <p align="center">
        一个综合性网络质量(PING)检测工具，支持正/反向PING绘图、互PING拓扑绘图与报警、全国PING延迟地图与在线检测工具等功能
        <br>
        <a href="http://smartping.org"><strong>-- Browse website --</strong></a>
        <br>
        <br>
        <a href="https://www.travis-ci.org/smartping/smartping">
            <img src="https://www.travis-ci.org/smartping/smartping.svg?branch=master" >
        </a>
        <a href="https://goreportcard.com/report/github.com/smartping/smartping">
            <img src="https://goreportcard.com/badge/github.com/smartping/smartping" >
        </a>
         <a href="https://github.com/smartping/smartping/releases">
             <img src="https://img.shields.io/github/release/smartping/smartping.svg" >
         </a>
         <a href="https://github.com/smartping/smartping/blob/master/LICENSE">
             <img src="https://img.shields.io/hexpm/l/plug.svg" >
         </a>
    </p>    
</p>

## 功能 ##

- 用户登录与权限管理（管理员/只读用户，默认账号 `admin / admin123`，首次登录后请修改密码）
- Pingmesh 全网互PING矩阵（借鉴 Flashcat Pingmesh，行=源节点、列=目标，延迟/丢包热力着色，点击查看历史曲线）
- 多通道告警：邮件 / 钉钉 / 企业微信 / 飞书 / Telegram / Slack / Discord / 通用Webhook，告警与恢复均通知，支持页面内一键测试
- 主节点 + 一键加入组网：新节点一条命令加入，自动全互联组成 Pingmesh，配置每分钟自动从主节点同步
- 正向PING，反向Ping绘图（现代化界面，ECharts 实时曲线）
- 互PING间机器的状态拓扑，自定义延迟、丢包阈值报警，报警时MTR检测
- 全国PING延迟地图（各省份可分电信、联通、移动三条线路）
- 检测工具，支持使用SmartPing各节点进行网络相关检测
- 可视化系统配置（基础设置/节点管理/节点接入/报警通道/全国延迟/高级JSON）
- 单二进制部署：前端资源与默认配置内嵌，首次启动自动初始化，亦提供 Docker 与 systemd 一键安装

## 快速开始 ##

### 方式一：单二进制 ###

```bash
go build -o smartping ./src
sudo setcap cap_net_raw+ep ./smartping   # ICMP 需要 raw socket 权限(或用 root 运行)
./smartping                              # 默认端口 8899, 首次启动自动生成 conf/db/html/logs
```

### 方式二：Docker ###

```bash
docker compose up -d                     # 或 docker build -t smartping . && docker run --net=host --cap-add=NET_RAW smartping
```

### 方式三：systemd 一键安装 ###

```bash
sudo ./deploy/install.sh                 # 安装为系统服务并开机自启
```

浏览器访问 `http://<本机IP>:8899`，使用默认账号 `admin / admin123` 登录。

## 集群组网（主节点 + 自动加入） ##

任选一台作为**主节点**部署后，其余节点只需一条命令加入，自动完成全互联 Pingmesh 组网并持续从主节点同步配置：

```bash
# 接入令牌与完整命令可在主节点「系统配置 - 节点接入」页面直接复制
./smartping -join http://<主节点IP>:8899 -token <接入令牌> -name 北京机房
# 或使用安装脚本
sudo ./deploy/install.sh --join http://<主节点IP>:8899 --token <接入令牌> --name 北京机房
```

- 新节点会自动监测所有既有节点，所有探测节点自动监测新节点（全互联）
- 加入后节点进入 cloud 模式，每分钟从主节点拉取最新配置，日常只需在主节点页面操作
- 节点间数据依旧去中心化存储，任意节点页面均可汇总查看全网数据

## 常用命令行参数 ##

| 参数 | 说明 |
| --- | --- |
| `-p 8899` | 覆盖配置中的HTTP端口 |
| `-l 0.0.0.0:8899` | 指定监听地址 |
| `-w /data/smartping` | 指定工作目录(默认二进制所在目录) |
| `-join / -token / -name / -addr` | 以Agent身份加入主节点(addr留空自动识别) |

## 设计思路 ##

本系统的定位为轻量级工具，即使组多点成互Ping网络可以遵守无中心化原则，所有的数据均存储自身节点中，每个节点提供出方向的数据，从任意节点查询数据均会通过Ajax请求关联节点的API接口获取并组装全部数据。
## 项目截图 ##

![app-bg.jpg](http://smartping.org/assets/img/app-bg.png "")

## 技术交流

<a target="_blank" href="//shang.qq.com/wpa/qunwpa?idkey=dd689e43fd8ecfeb28bffc31d53cb058c6ea23263aa1a34fc032efaf91aae924"><img border="0" src="http://pub.idqqimg.com/wpa/images/group.png" alt="SmartPing" title="SmartPing"></a>

## 项目贡献

欢迎参与项目贡献！比如提交PR修复一个bug，或者新建 [Issue](https://github.com/smartping/smartping/issues/) 讨论新特性或者变更。

## 其他资料 ##

- 官网： http://smartping.org
- 文档： https://docs.smartping.org
- - 下载安装：https://docs.smartping.org/install/
- - API文档：https://docs.smartping.org/api/
