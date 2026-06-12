<p align="center">
    <img src="html/assets/img/logo.png" width="96" alt="ZENLENET PingMesh">
    <h3 align="center">ZENLENET PingMesh | 全网互 PING · 网络质量监控平台</h3>
    <p align="center">
        开箱即用的网络质量(PING)监控平台：Pingmesh 互PING矩阵、正/反向PING曲线、拓扑告警、
        多通道告警(钉钉/企微/飞书/Telegram/Slack/Discord/Webhook/邮件)、全国延迟地图与多节点检测工具
    </p>
</p>

> 基于开源项目 SmartPing 深度重构（Go 模块路径沿用 `github.com/smartping/smartping`）。

## 功能 ##

- **Pingmesh 互PING矩阵**：行=源节点、列=目标的紧凑热力网格，预设/自定义时间窗口，延迟/丢包切换，点击查看历史曲线
- **用户与权限**：登录会话、管理员/只读角色、用户管理（默认账号 `admin / admin123`，请尽快修改）
- **多通道告警**：邮件 / 钉钉(加签) / 企业微信 / 飞书(签名) / Telegram / Slack / Discord / 通用Webhook，告警与恢复均通知，页面内一键测试
- **一键组网**：主节点 + `-join` 一条命令加入，自动全互联组网，配置每分钟自动同步
- 正/反向 PING 曲线（ECharts 5 现代化图表）、互PING拓扑（柔和声音告警）、阈值报警与 MTR 快照
- 全国PING延迟地图（电信/联通/移动三线路，可在配置中开启/关闭）
- 多节点检测工具、可视化系统配置（Bootstrap 5 + 自研设计系统）
- **单二进制部署**：前端资源内嵌，首次启动自动初始化；提供 Docker 与 systemd 一键安装

## 快速开始 ##

### 方式一：单二进制 ###

```bash
go build -o pingmesh ./src
sudo setcap cap_net_raw+ep ./pingmesh    # ICMP 需要 raw socket 权限(或用 root 运行)
./pingmesh                               # 默认端口 8899, 首次启动自动生成 conf/db/html/logs
```

### 方式二：Docker ###

```bash
docker compose up -d                     # 或 docker build -t pingmesh . && docker run --net=host --cap-add=NET_RAW pingmesh
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
./pingmesh -join http://<主节点IP>:8899 -token <接入令牌> -name 北京机房
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
| `-w /data/pingmesh` | 指定工作目录(默认二进制所在目录) |
| `-join / -token / -name / -addr` | 以Agent身份加入主节点(addr留空自动识别) |

## 设计思路 ##

本系统的定位为轻量级工具，组成互Ping网络时遵守无中心化原则：所有数据均存储在自身节点中，每个节点提供出方向的数据，从任意节点查询数据均会通过Ajax请求关联节点的API接口获取并组装全部数据。主节点仅承担配置分发与节点注册职责。

## 项目贡献 ##

欢迎参与项目贡献！比如提交PR修复一个bug，或者新建 Issue 讨论新特性或者变更。

## 致谢 ##

- 原始项目：[SmartPing](https://github.com/smartping/smartping) (Apache-2.0)
- Pingmesh 矩阵交互参考：快猫星云 Flashcat Pingmesh 与微软 Pingmesh 论文
