package g

import "encoding/json"

type Config struct {
	Ver        string
	Port       int
	Name       string
	Addr       string
	Mode       map[string]string
	Base       map[string]int
	Topology   map[string]string
	Alert      map[string]string
	Channels   []AlertChannel
	Network    map[string]NetworkMember
	Chinamap   map[string]map[string][]string
	Toollimit  int
	Authiplist string
	Password   string
}

// AlertChannel 告警通道配置
// Type: webhook | dingtalk | wecom | feishu | telegram | slack | discord
type AlertChannel struct {
	Type    string            `json:"Type"`
	Name    string            `json:"Name"`
	Enabled bool              `json:"Enabled"`
	Params  map[string]string `json:"Params"`
}

type NetworkMember struct {
	Name     string
	Addr     string
	Group    string // 分组(机房/地域), 便于大规模节点管理
	Pingmesh bool
	Ping     []string
	Topology []map[string]string
}

// UnmarshalJSON 兼容旧版配置中的 "Smartping" 字段名
func (n *NetworkMember) UnmarshalJSON(b []byte) error {
	type alias NetworkMember
	aux := struct {
		*alias
		Smartping *bool `json:"Smartping"`
	}{alias: (*alias)(n)}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	if aux.Smartping != nil && !n.Pingmesh {
		n.Pingmesh = *aux.Smartping
	}
	return nil
}

//Ping Struct
type PingSt struct {
	SendPk   int
	RevcPk   int
	LossPk   int
	MinDelay float64
	AvgDelay float64
	MaxDelay float64
	// Jitter 相邻样本RTT差的平均值(ms), 专线(IPLC/IEPL)质量核心指标
	Jitter float64
}

type PingLog struct {
	Logtime  string
	Maxdelay string
	Mindelay string
	Avgdelay string
	Losspk   string
	Jitter   string
}

type AlertLog struct {
	Id         int64
	Logtime    string
	Targetip   string
	Targetname string
	Tracert    string
	Fromip     string
	Fromname   string
	Ack        int
	Ackby      string
	Ackreason  string
	Acktime    string
}

type ChinaMp struct {
	Text     string              `json:"text"`
	Subtext  string              `json:"subtext"`
	Avgdelay map[string][]MapVal `json:"avgdelay"`
}

type MapVal struct {
	Value float64 `json:"value"`
	Name  string  `json:"name"`
}
