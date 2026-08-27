package g

import "testing"

func TestAddMeshNodePreservesExistingName(t *testing.T) {
	Cfg = Config{
		Addr: "10.100.1.8",
		Network: map[string]NetworkMember{
			"10.100.1.4": {Name: "CAN-XXG", Addr: "10.100.1.4", Pingmesh: true, Ping: []string{}, Topology: []map[string]string{}},
		},
	}
	AddMeshNode("can-xxg", "10.100.1.4")
	if got := Cfg.Network["10.100.1.4"].Name; got != "CAN-XXG" {
		t.Fatalf("existing name preserved: got %q want CAN-XXG", got)
	}
}

func TestAddMeshNodeUsesJoinNameForNewNode(t *testing.T) {
	Cfg = Config{
		Addr:    "10.100.1.8",
		Network: map[string]NetworkMember{},
	}
	AddMeshNode("CAN-XXG", "10.100.1.4")
	if got := Cfg.Network["10.100.1.4"].Name; got != "CAN-XXG" {
		t.Fatalf("new node name: got %q want CAN-XXG", got)
	}
}
