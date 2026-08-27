package nettools

import (
	"bytes"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

// 共享 ICMP socket + 应答分发器。
// 旧实现每次 ping 都新建 raw socket, 且每个 socket 都会收到整机全部 ICMP
// 回包, N 个目标并发时包处理量是 O(N^2)。现在全进程仅一个探测 socket,
// 读循环按 (id,seq) 把应答派发给等待者, 包处理量回到 O(N)。

var (
	pingSeq uint32 // 全局自增, 生成唯一 (id,seq)

	// 按源IP维护探测 socket(""=系统默认路由), 支持多网口(公网/专线)分开探测
	socketsMu sync.Mutex
	sockets   = map[string]net.PacketConn{}

	pendingMu sync.Mutex
	pending   = map[uint64]chan struct{}{}

	chPool = sync.Pool{New: func() any { return make(chan struct{}, 1) }}
)

func pingKey(id, seq int) uint64 {
	return uint64(uint16(id))<<16 | uint64(uint16(seq))
}

// getPingSocket 获取(或创建)绑定到指定源IP的探测 socket
func getPingSocket(src string) (net.PacketConn, error) {
	socketsMu.Lock()
	defer socketsMu.Unlock()
	if c, ok := sockets[src]; ok {
		return c, nil
	}
	bind := "0.0.0.0"
	if src != "" {
		bind = src
	}
	conn, err := net.ListenPacket("ip4:icmp", bind)
	if err != nil {
		return nil, err
	}
	// 放大收发缓冲, 全网状高 pps 时避免 socket 队列溢出被内核丢掉回包(表现为齐丢)
	if sc, ok := conn.(syscall.Conn); ok {
		if rc, err := sc.SyscallConn(); err == nil {
			_ = rc.Control(func(fd uintptr) {
				_ = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_RCVBUF, 4<<20)
				_ = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_SNDBUF, 1<<20)
			})
		}
	}
	sockets[src] = conn
	// 双读循环: 降低单读协程被调度饿死时整批 in-flight 同时超时的概率
	go pingReadLoop(conn)
	go pingReadLoop(conn)
	return conn, nil
}

func pingReadLoop(conn net.PacketConn) {
	buf := make([]byte, 1600)
	for {
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				continue
			}
			return
		}
		msg, err := icmp.ParseMessage(1, buf[:n])
		if err != nil || msg.Type != ipv4.ICMPTypeEchoReply {
			continue
		}
		rply, ok := msg.Body.(*icmp.Echo)
		if !ok {
			continue
		}
		key := pingKey(rply.ID, rply.Seq)
		pendingMu.Lock()
		ch := pending[key]
		delete(pending, key)
		pendingMu.Unlock()
		if ch != nil {
			select {
			case ch <- struct{}{}:
			default:
			}
		}
	}
}

// RunPing 发送一个 ICMP echo 并等待应答, 返回毫秒级延迟。
// 保持旧签名兼容; seq 参数仅作参考, 内部使用全局唯一序号防串包。
func RunPing(IpAddr *net.IPAddr, maxrtt time.Duration, maxttl int, seq int) (float64, error) {
	return RunPingSize(IpAddr, maxrtt, 56)
}

// RunPingSize 指定 payload 大小(字节)的探测
func RunPingSize(IpAddr *net.IPAddr, maxrtt time.Duration, size int) (float64, error) {
	return RunPingFrom(IpAddr, maxrtt, size, "")
}

// RunPingFrom 指定源IP的探测: 双网口机器(公网口/专线口)可按链路
// 强制从对应网口发包, 实现两条路径分开监控。srcip 为空走系统默认路由。
func RunPingFrom(IpAddr *net.IPAddr, maxrtt time.Duration, size int, srcip string) (float64, error) {
	conn, err := getPingSocket(srcip)
	if err != nil {
		return 0, err
	}
	if size < 8 {
		size = 8
	}
	if size > 1472 {
		size = 1472
	}
	// 唯一 (id,seq): 高 16 位做 id, 低 16 位做 seq
	v := atomic.AddUint32(&pingSeq, 1)
	id := int(uint16(v >> 16)) | 0x1
	sq := int(uint16(v))

	payload := bytes.Repeat([]byte("ZENLENET-PingMesh!"), size/18+1)[:size]
	msg := icmp.Message{Type: ipv4.ICMPTypeEcho, Code: 0,
		Body: &icmp.Echo{ID: id, Seq: sq, Data: payload}}
	wire, err := msg.Marshal(nil)
	if err != nil {
		return 0, err
	}

	ch := chPool.Get().(chan struct{})
	// drain if previous use left a signal
	select {
	case <-ch:
	default:
	}
	key := pingKey(id, sq)
	pendingMu.Lock()
	pending[key] = ch
	pendingMu.Unlock()
	cleanup := func() {
		pendingMu.Lock()
		if pending[key] == ch {
			delete(pending, key)
		}
		pendingMu.Unlock()
		select {
		case <-ch:
		default:
		}
		chPool.Put(ch)
	}

	sendOn := time.Now()
	if _, err := conn.WriteTo(wire, IpAddr); err != nil {
		cleanup()
		return 0, err
	}
	timer := time.NewTimer(maxrtt)
	defer timer.Stop()
	select {
	case <-ch:
		// success: return channel to pool without double-drain issues
		pendingMu.Lock()
		delete(pending, key)
		pendingMu.Unlock()
		chPool.Put(ch)
		return float64(time.Since(sendOn).Nanoseconds()) / 1e6, nil
	case <-timer.C:
		cleanup()
		return 0, errors.New("timeout")
	}
}
