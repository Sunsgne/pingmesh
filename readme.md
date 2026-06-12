<p align="center">
    <img src="html/assets/img/logo.png" width="96" alt="ZENLENET PingMesh">
</p>
<h1 align="center">ZENLENET PingMesh</h1>
<p align="center"><b>全网互 PING · 开箱即用的网络质量监控平台</b></p>
<p align="center">
    任意多台服务器之间两两互 PING，一张矩阵看清全网延迟与丢包；<br>
    异常自动告警到钉钉 / 企业微信 / 飞书 / Telegram 等通道，并附 MTR 路由快照辅助定位。
</p>

---

## 目录

- [功能介绍](#功能介绍)
- [快速部署（Ubuntu 24.04）](#快速部署ubuntu-2404)
- [手动部署教程](#手动部署教程)
- [Docker 部署](#docker-部署)
- [集群组网教程（多节点互 PING）](#集群组网教程多节点互-ping)
- [使用教程](#使用教程)
- [命令行参数](#命令行参数)
- [常见问题](#常见问题)

---

## 功能介绍

| 模块 | 说明 |
| --- | --- |
| **Pingmesh 矩阵** | 行=探测源、列=监测目标的全网互 PING 热力网格，五级健康度着色，完全不通显示 ✕；顶部汇总节点/链路/异常数与全网平均延迟；时间窗口支持 15分钟~7天预设与自定义起止时间；点击任意色块查看历史曲线 |
| **正向 / 反向 PING** | 本节点 → 各目标、各节点 → 本节点的实时延迟与丢包曲线（ECharts 5，支持任意时间段回查） |
| **网络拓扑** | 互 PING 状态拓扑图，绿色正常、红色触发阈值；异常时柔和声音提醒（可关闭） |
| **多通道告警** | 邮件(SMTP)、钉钉(支持加签)、企业微信、飞书(支持签名)、Telegram、Slack、Discord、通用 Webhook；**告警与恢复均通知**(仅状态翻转时各发一次, 不刷屏)，附实测指标、触发规则与 MTR 路由快照 |
| **告警治理** | 时间范围/关键字查询；**确认**(填写原因, 并抑制该故障的重复提醒)；**屏蔽**(1小时~7天, 到期仍异常自动补通知)；可选**持续故障定期提醒**(30分钟~24小时)；恢复通知附故障时长 |
| **双网口分路监控** | 每条监测链路可指定**探测源IP**(公网口/专线口)，公网与专线两条路径完全分开监控、分开告警 |
| **毫秒级探测引擎** | 探测间隔/每轮包数/单包超时/包大小全部可配(毫秒级)，全局默认 + **按链路单独覆盖**(专线链路高频小超时、普通链路保持默认)；采集**抖动(Jitter)**指标并支持独立抖动告警阈值 |
| **域名与 ASN** | 节点地址支持填域名(探测时自动解析)；内置 ASN 查询(RIPE NCC 权威数据)，节点编辑时自动显示所属 AS 与归属机构 |
| **告警阈值** | 每条链路独立配置：延迟阈值(ms)、丢包阈值(%)、检测窗口(秒)、触发次数 |
| **一键组网** | 主节点 + Agent 架构：新节点一条命令加入，自动全互联组网，配置每分钟自动从主节点同步；数据去中心化存储在各节点 |
| **集群容灾** | 去中心化多主：每个节点持有完整配置副本（主挂配置永不丢失）；主节点不可达时下一优先级节点**自动接管**写入；配置带纪元(Epoch)，任意节点的改动全网 LWW 收敛；「集群容灾」页可视化主从态势、在线状态与配置版本，并自动留存配置快照供回滚 |
| **用户与权限** | 登录会话、管理员/只读两种角色、用户管理；兼容旧版 IP 白名单 |
| **全球延迟地图** | 世界地图按地区/线路延迟着色 + 搜索与延迟列表，地区与线路完全自定义（可整体开启/关闭） |
| **检测工具** | 从所有节点同时发起 **ICMP PING / TCP Ping / HTTP(curl分段计时) / MTR 路由追踪 / DNS 解析**，对比各地连通性与质量 |
| **可视化配置** | 基础设置 / 节点管理 / 节点接入 / 报警通道 / 全国延迟 / 高级 JSON 六个页签全部页面化 |
| **单二进制** | 前端资源与默认配置内嵌，纯 Go 实现（无 CGO），首次启动自动初始化，支持交叉编译 |

> 技术栈：Go 1.25（构建时按 go.mod 自动拉取工具链）· SQLite(纯Go驱动, WAL) · Bootstrap 5 · ECharts 5。基于开源项目 SmartPing 深度重构。
> 安全加固：HTTP 安全响应头(CSP/X-Frame-Options 等) + 服务端读超时(防慢速攻击) + 登录失败限速 + 优雅停机；内置 `/healthz` 健康检查端点。

---

## 快速部署（Ubuntu 24.04）

一条命令完成 依赖安装 → 编译 → systemd 服务 → 防火墙放行（兼容 Ubuntu 22.04 / Debian 12）。
**克隆仓库后直接运行无参数脚本会进入交互向导**，新手按提示选主/从、填端口与名称即可：

```bash
git clone https://github.com/Sunsgne/smartping && cd smartping
sudo ./deploy/install.sh          # 进入交互式部署向导(主/从、端口、名称、令牌)
```

也可一行远程安装主节点（非交互, 走默认值）：

```bash
# 部署主节点
curl -fsSL https://raw.githubusercontent.com/Sunsgne/smartping/master/deploy/install.sh | sudo bash
```

```bash
# 部署 Agent 节点并自动加入主节点（接入令牌在主节点「系统配置 → 节点接入」页面查看复制）
curl -fsSL https://raw.githubusercontent.com/Sunsgne/smartping/master/deploy/install.sh \
  | sudo bash -s -- --join http://<主节点IP>:8899 --token <接入令牌> --name 北京机房
```

完成后访问 `http://<服务器IP>:8899`，默认账号 **admin / admin123**（登录后请立即修改密码）。

脚本支持的参数：

```bash
sudo ./deploy/install.sh                  # 无参数 → 交互向导(新手推荐)
sudo ./deploy/install.sh --yes            # 非交互, 主节点默认值
sudo ./deploy/install.sh --port 9000      # 自定义端口
sudo ./deploy/install.sh --name 北京主节点 # 节点名称(主节点首次安装即生效)
sudo ./deploy/install.sh --dir /data/pm   # 自定义安装目录(默认 /opt/pingmesh)
sudo ./deploy/install.sh --join URL --token XXX --name 节点名   # Agent 模式
sudo ./deploy/install.sh --join URL --token XXX --name 节点名 --masters 10.0.0.2:8899  # Agent + 容灾备选
sudo ./deploy/install.sh --update         # 在线升级(保留启动参数与数据)
sudo ./deploy/install.sh --uninstall      # 卸载(保留数据)
```

> **启动参数写入 systemd 环境文件**：脚本把端口、节点名、接入令牌、容灾备选等必要初始化参数写入 `/opt/pingmesh/pingmesh.env`，由 systemd 单元的 `EnvironmentFile` 引用。日后改参数只需编辑该文件再 `systemctl restart pingmesh`，无需重跑脚本。生成的 systemd 单元已做安全加固（`CAP_NET_RAW` 最小权限、`ProtectSystem`/`ProtectHome`/`PrivateTmp`/`NoNewPrivileges` 等）。
>
> 重复执行脚本(`--update`)即为**原地升级**：自动拉取最新代码、重新编译并重启服务，数据与配置不受影响。
>
> 若系统自带 Go 版本过旧，脚本会自动从 [go.dev](https://go.dev/dl) 安装匹配的官方工具链。

---

## 手动部署教程

适用于想了解每一步细节、或非 apt 系发行版的场景（以 Ubuntu 24.04 为例）。

**1. 安装依赖**

```bash
sudo apt update
sudo apt install -y git golang-go libcap2-bin   # Ubuntu 24.04 自带 Go 1.22+
go version                                       # 需 >= go1.21；构建时会按 go.mod 自动拉取 Go 1.25 工具链
```

> 若发行版自带 Go 过旧（< 1.21），从 [go.dev/dl](https://go.dev/dl) 安装官方工具链即可；一键脚本会自动处理。

**2. 获取源码并编译**

```bash
git clone https://github.com/Sunsgne/smartping && cd smartping
CGO_ENABLED=0 go build -ldflags="-s -w -X main.GitCommit=$(git rev-parse --short HEAD) -X main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o pingmesh ./src
```

**3. 安装与授权**

```bash
sudo mkdir -p /opt/pingmesh
sudo install -m 755 pingmesh /opt/pingmesh/pingmesh
sudo setcap cap_net_raw+ep /opt/pingmesh/pingmesh   # ICMP 探测所需, 免 root 运行
```

**4. 配置 systemd 服务**

启动参数集中放在环境文件中，单元通过 `EnvironmentFile` 引用（与一键脚本一致，便于后续修改）：

```bash
# 启动参数(可留空 → 主节点默认; Agent 示例见注释)
sudo tee /opt/pingmesh/pingmesh.env <<'EOF'
# -p 端口  -name 节点名  -addr 本机IP  -join 主节点  -token 令牌  -masters 容灾备选(逗号分隔)
PINGMESH_OPTS=-p 8899
EOF
sudo chmod 600 /opt/pingmesh/pingmesh.env

sudo tee /etc/systemd/system/pingmesh.service <<'EOF'
[Unit]
Description=ZENLENET PingMesh - network quality monitor & DR cluster
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=/opt/pingmesh
EnvironmentFile=-/opt/pingmesh/pingmesh.env
ExecStart=/opt/pingmesh/pingmesh $PINGMESH_OPTS
Restart=always
RestartSec=3
LimitNOFILE=65536
AmbientCapabilities=CAP_NET_RAW
CapabilityBoundingSet=CAP_NET_RAW
NoNewPrivileges=true
ProtectSystem=full
ProtectHome=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
EOF
sudo systemctl daemon-reload
sudo systemctl enable --now pingmesh
```

**5. 放行防火墙并访问**

```bash
sudo ufw allow 8899/tcp        # 如启用了 ufw
```

浏览器打开 `http://<服务器IP>:8899`，使用 **admin / admin123** 登录。
首次启动会在 `/opt/pingmesh` 下自动生成 `conf/`（配置）、`db/`（数据）、`logs/`（日志）、`html/`（页面资源）。

---

## Docker 部署

```bash
git clone https://github.com/Sunsgne/smartping && cd smartping
docker compose up -d                # 主节点, host 网络 + NET_RAW
```

Agent 节点：编辑 `docker-compose.yml` 中 `pingmesh-agent` 服务的 `-join/-token/-name` 参数后：

```bash
docker compose --profile agent up -d
```

> ICMP 探测与节点互访建议使用 `network_mode: host`（容器直接使用宿主机 IP）。

---

## 安全

- 页面访问：登录会话 + 角色权限（管理员/只读）
- 节点间接口：**HMAC-SHA256 请求签名**（基于集群接入令牌，时间戳±5分钟 + nonce 防重放），开启「API 签名与加密」后仅凭 IP 互信的请求一律拒绝
- 配置同步：**AES-256-GCM 加密**传输（含邮箱密码、告警通道密钥等敏感字段）
- 节点加入：HMAC 签名认证，令牌不再明文传输；join 成功后集群令牌自动统一
- Web 加固：统一注入安全响应头（CSP / X-Frame-Options / X-Content-Type-Options / Referrer-Policy）；服务端读超时防慢速攻击；登录失败限速（同一来源连续失败自动短时锁定）；收到 SIGTERM 优雅停机
- 配置容灾：每次保存自动留存配置快照（`conf/backups/`，保留最近 30 份），误改可回滚；支持一键导出配置备份
- 公网部署建议：开启 API 签名与加密（系统配置 → 节点接入），修改默认令牌与默认密码，防火墙限制 8899 端口来源

## 集群组网教程（多节点互 PING）

架构：**任选一台作为主节点**（保存全量配置与用户），其余机器以 Agent 身份加入。

1. 在 A 机器部署主节点（见上文）
2. 登录 A 的页面 → 「系统配置 → 节点接入」→ 复制接入令牌与加入命令
3. 在 B/C/D... 机器上执行（或用一键脚本的 `--join` 参数）：

```bash
./pingmesh -join http://<A的IP>:8899 -token <接入令牌> -name 上海机房
```

加入后自动完成：

- **全互联组网**：新节点监测所有既有节点，所有探测节点反向监测新节点（默认阈值：延迟 200ms / 丢包 30%）
- **配置同步**：Agent 进入 cloud 模式，每分钟从主节点拉取最新配置——日常只需在主节点页面操作，告警通道、阈值等改动自动下发
- **数据去中心化**：监测数据存储在各自节点，任意节点页面都能汇总查看全网矩阵

> IP 无法自动识别（如 NAT 环境）时，加 `-addr <本机IP>` 显式指定。
> 跨公网部署请在防火墙限制 8899 端口来源, 并修改默认接入令牌。

---

## 集群容灾（主挂不丢数据、自动接管）

去中心化多主设计，目标是**主节点宕机后配置不丢、集群继续读写、自动收敛**：

- **配置永不丢失**：每个节点本地都持有一份完整配置副本（join 后即同步），任意单点（含主节点）宕机都不会丢失配置。
- **主挂自动接管**：每条配置带单调递增的纪元(Epoch)。主节点不可达时，下一优先级的在线候选自动成为「代理主节点」，继续接受配置变更与新节点加入。
- **全网收敛**：任意候选节点上的权威改动（管理员保存 / 节点加入）都会自增纪元，并在下一同步周期（≤60 秒）被全网按 LWW（纪元高者胜出）采纳。
- **可视化与回滚**：「系统管理 → 集群容灾」页展示当前代理主节点、各节点在线/纪元/是否收敛；每次保存都会在 `conf/backups/` 留存配置快照（保留最近 30 份），并可一键导出配置备份。

默认开启**自动容灾**（`MasterAuto`）：所有探测节点都是主候选，按地址优先级自动接管，无需人工指定备选。也可在页面或用 `-masters host:port,host:port` 显式指定有序的备选主节点。

```bash
# Agent 加入并指定容灾备选(主挂时按顺序接管)
./pingmesh -join http://<主IP>:8899 -token <令牌> -name 上海机房 -masters 10.0.0.2:8899,10.0.0.3:8899
```

> 健康检查 / 探活：`curl http://<节点>:8899/healthz` 返回节点版本、模式、配置纪元与是否代理主节点。

---

## 使用教程

| 操作 | 步骤 |
| --- | --- |
| 修改登录密码 | 右上角头像 → 修改密码（首次登录必做） |
| 添加监测目标 | 系统配置 → 节点管理 → 添加节点：填名称与 IP；普通目标(如 DNS、网关)关闭「主动探测」，部署了本程序的机器开启 |
| 配置监测关系与阈值 | 节点管理 → 编辑某个探测节点 → 勾选监测目标并设置延迟/丢包阈值 → 保存配置 |
| 查看全网质量 | 「Pingmesh 矩阵」：切换时间窗口与延迟/丢包指标，点击色块看历史曲线 |
| 配置告警通道 | 系统配置 → 报警通道 → 添加通道（钉钉/企微/飞书/Telegram/Slack/Discord/Webhook）→ 发送测试 → 保存配置 |
| 邮件告警 | 同页左侧填写 SMTP 服务器、发件账号密码、收件人列表 → 发送测试邮件 |
| 新增登录用户 | 用户管理 → 新建用户（管理员=全部功能；只读=仅查看与检测工具） |
| 开关全球延迟 | 系统配置 → 基础设置 → 全球延迟功能开关；各地区/线路探测 IP 在「全球延迟」页签维护（地区用英文国家名才能在地图着色） |
| 多节点检测 | 「检测工具」：输入任意域名/IP，从所有节点同时发起 PING 对比 |
| 查看告警历史 | 「报警记录」：按日期筛选，点 MTR 查看告警时刻的路由快照 |

通用 Webhook 告警格式（POST JSON），便于对接自有平台：

```json
{
  "event": "alert | recovery | test",
  "title": "【告警】本机 → 百度 网络质量异常",
  "content": "时间/源/目标/触发规则...",
  "fromname": "本机", "fromip": "10.0.0.1",
  "targetname": "百度", "targetip": "1.2.3.4",
  "time": "2026-06-12 12:00"
}
```

---

## 命令行参数

| 参数 | 说明 |
| --- | --- |
| `-p 8899` | 覆盖配置中的 HTTP 端口 |
| `-l 0.0.0.0:8899` | 指定监听地址 |
| `-w /data/pingmesh` | 指定工作目录（默认二进制所在目录） |
| `-join http://主节点:8899` | 以 Agent 身份加入主节点 |
| `-token / -name / -addr` | 接入令牌 / 节点名称 / 本机IP（留空自动识别） |
| `-masters host:port,...` | 容灾备选主节点（有序，主挂自动接管，可选） |
| `-v` | 显示版本（含构建提交与时间） |

---

## 常见问题

**页面打不开？**
1. 确认服务在运行：`systemctl status pingmesh`
2. 确认监听地址：默认 `:8899` 监听所有网卡；若用 `-l 127.0.0.1:8899` 启动则只能本机访问
3. 放行防火墙/安全组的 `8899/tcp`（云服务器还需检查控制台安全组）

**PING 全部丢包 / 无数据？**
ICMP 需要 raw socket 权限：`sudo setcap cap_net_raw+ep ./pingmesh` 或以 root 运行；安装脚本与 systemd 单元已自动处理。

**忘记管理员密码？**
删除 `db/database.db` 中的 users 表记录后重启（会重建默认 admin/admin123）：
`sqlite3 db/database.db "delete from users"` 后 `systemctl restart pingmesh`。

**如何升级？**
重新执行一键安装脚本即可原地升级；手动部署则重新编译并替换二进制后重启服务。数据与配置目录不受影响。

**如何卸载？**
`sudo ./deploy/install.sh --uninstall`（保留数据目录，彻底清除再执行 `rm -rf /opt/pingmesh`）。

---

## 致谢与许可

- 基于开源项目 [SmartPing](https://github.com/smartping/smartping) 深度重构（Go 模块路径沿用以保持兼容）
- Pingmesh 矩阵交互参考快猫星云 Flashcat Pingmesh 与微软 Pingmesh 论文
- License: Apache-2.0
