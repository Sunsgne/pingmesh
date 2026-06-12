package g

// 新节点加入时的默认报警阈值
var defaultTopoRule = map[string]string{
	"Thdavgdelay": "200",
	"Thdchecksec": "900",
	"Thdloss":     "30",
	"Thdoccnum":   "3",
}

func newTopoEntry(addr, name string) map[string]string {
	e := map[string]string{"Addr": addr, "Name": name}
	for k, v := range defaultTopoRule {
		e[k] = v
	}
	return e
}

func hasPing(list []string, addr string) bool {
	for _, a := range list {
		if a == addr {
			return true
		}
	}
	return false
}

func hasTopo(list []map[string]string, addr string) bool {
	for _, t := range list {
		if t["Addr"] == addr {
			return true
		}
	}
	return false
}

// AddMeshNode 将节点加入全互联 Pingmesh:
// 新节点监测所有既有节点, 所有开启探测的既有节点监测新节点。
func AddMeshNode(name, addr string) {
	member, ok := Cfg.Network[addr]
	if !ok {
		member = NetworkMember{Name: name, Addr: addr, Pingmesh: true, Ping: []string{}, Topology: []map[string]string{}}
	}
	// 显式 join 提供的名字生效(重启重放旧参数的场景由 agent 侧参数指纹拦截)
	member.Name = name
	member.Addr = addr
	member.Pingmesh = true
	if member.Ping == nil {
		member.Ping = []string{}
	}
	if member.Topology == nil {
		member.Topology = []map[string]string{}
	}
	for a, m := range Cfg.Network {
		if a == addr {
			continue
		}
		if !hasPing(member.Ping, a) {
			member.Ping = append(member.Ping, a)
		}
		if !hasTopo(member.Topology, a) {
			member.Topology = append(member.Topology, newTopoEntry(a, m.Name))
		}
		if m.Pingmesh {
			changed := false
			if !hasPing(m.Ping, addr) {
				m.Ping = append(m.Ping, addr)
				changed = true
			}
			if !hasTopo(m.Topology, addr) {
				m.Topology = append(m.Topology, newTopoEntry(addr, name))
				changed = true
			}
			if changed {
				Cfg.Network[a] = m
			}
		}
	}
	Cfg.Network[addr] = member
	SelfCfg = Cfg.Network[Cfg.Addr]
}
