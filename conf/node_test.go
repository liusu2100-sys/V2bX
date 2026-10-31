package conf

import (
	"encoding/json"
	"testing"
)

func TestNodeConfig_MachineID(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want int
	}{
		{"flat", `{"ApiHost":"https://panel","ApiKey":"k","NodeID":1,"NodeType":"vless","MachineID":5}`, 5},
		{"nested", `{"ApiConfig":{"ApiHost":"https://panel","ApiKey":"k","NodeID":1,"NodeType":"vless","MachineID":6}}`, 6},
		{"omitted", `{"ApiHost":"https://panel","ApiKey":"k","NodeID":1,"NodeType":"vless"}`, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var n NodeConfig
			if err := json.Unmarshal([]byte(tc.raw), &n); err != nil {
				t.Fatal(err)
			}
			if n.ApiConfig.MachineID != tc.want {
				t.Fatalf("MachineID = %d, want %d", n.ApiConfig.MachineID, tc.want)
			}
		})
	}
}
