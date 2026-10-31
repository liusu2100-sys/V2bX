package panel

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/InazumaV/V2bX/conf"
)

type recordedRequest struct {
	Method string
	Path   string
	Query  url.Values
}

// newRecordingServer 模拟面板接口，记录每次请求的方法、路径和查询参数
func newRecordingServer(t *testing.T) (*httptest.Server, func() []recordedRequest) {
	t.Helper()
	var (
		mu   sync.Mutex
		reqs []recordedRequest
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		reqs = append(reqs, recordedRequest{Method: r.Method, Path: r.URL.Path, Query: r.URL.Query()})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/server/UniProxy/config", "/api/v2/server/config":
			w.Write([]byte(`{"server_port":443,"cipher":"aes-128-gcm","base_config":{"push_interval":60,"pull_interval":60}}`))
		case "/api/v1/server/UniProxy/user", "/api/v2/server/user":
			w.Write([]byte(`{"users":[{"id":1,"uuid":"u1","speed_limit":0,"device_limit":0}]}`))
		case "/api/v1/server/UniProxy/alivelist", "/api/v2/server/alivelist":
			w.Write([]byte(`{"alive":{"1":2}}`))
		case "/api/v1/server/UniProxy/push", "/api/v2/server/push",
			"/api/v1/server/UniProxy/alive", "/api/v2/server/alive":
			w.Write([]byte(`{"data":true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, func() []recordedRequest {
		mu.Lock()
		defer mu.Unlock()
		return append([]recordedRequest(nil), reqs...)
	}
}

// callAllAPIs 依次调用全部面板接口，任一失败即终止测试
func callAllAPIs(t *testing.T, c *Client) {
	t.Helper()
	node, err := c.GetNodeInfo()
	if err != nil {
		t.Fatalf("GetNodeInfo: %v", err)
	}
	if node == nil || node.Shadowsocks == nil || node.Shadowsocks.Cipher != "aes-128-gcm" {
		t.Fatalf("GetNodeInfo: unexpected node %+v", node)
	}
	users, err := c.GetUserList()
	if err != nil {
		t.Fatalf("GetUserList: %v", err)
	}
	if len(users) != 1 || users[0].Uuid != "u1" {
		t.Fatalf("GetUserList: unexpected users %+v", users)
	}
	alive, err := c.GetUserAlive()
	if err != nil {
		t.Fatalf("GetUserAlive: %v", err)
	}
	if alive[1] != 2 {
		t.Fatalf("GetUserAlive: unexpected alive %+v", alive)
	}
	if err := c.ReportUserTraffic([]UserTraffic{{UID: 1, Upload: 10, Download: 20}}); err != nil {
		t.Fatalf("ReportUserTraffic: %v", err)
	}
	if err := c.ReportNodeOnlineUsers(&map[int][]string{1: {"1.1.1.1"}}); err != nil {
		t.Fatalf("ReportNodeOnlineUsers: %v", err)
	}
}

func checkRequests(t *testing.T, got []recordedRequest, want []recordedRequest, wantQuery url.Values, forbidden ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d requests, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i].Method != want[i].Method || got[i].Path != want[i].Path {
			t.Errorf("request %d: got %s %s, want %s %s", i, got[i].Method, got[i].Path, want[i].Method, want[i].Path)
		}
		for k := range wantQuery {
			if got[i].Query.Get(k) != wantQuery.Get(k) {
				t.Errorf("request %d (%s): query %s = %q, want %q", i, got[i].Path, k, got[i].Query.Get(k), wantQuery.Get(k))
			}
		}
		for _, k := range forbidden {
			if got[i].Query.Has(k) {
				t.Errorf("request %d (%s): unexpected query %s", i, got[i].Path, k)
			}
		}
	}
}

func TestClient_V1ServerToken(t *testing.T) {
	srv, requests := newRecordingServer(t)
	c, err := New(&conf.ApiConfig{
		APIHost:  srv.URL,
		Key:      "server-token",
		NodeType: "Shadowsocks",
		NodeID:   7,
	})
	if err != nil {
		t.Fatal(err)
	}
	callAllAPIs(t, c)
	checkRequests(t, requests(), []recordedRequest{
		{Method: http.MethodGet, Path: "/api/v1/server/UniProxy/config"},
		{Method: http.MethodGet, Path: "/api/v1/server/UniProxy/user"},
		{Method: http.MethodGet, Path: "/api/v1/server/UniProxy/alivelist"},
		{Method: http.MethodPost, Path: "/api/v1/server/UniProxy/push"},
		{Method: http.MethodPost, Path: "/api/v1/server/UniProxy/alive"},
	}, url.Values{
		"node_type": {"shadowsocks"},
		"node_id":   {"7"},
		"token":     {"server-token"},
	}, "machine_id")
}

func TestClient_V2MachineToken(t *testing.T) {
	srv, requests := newRecordingServer(t)
	c, err := New(&conf.ApiConfig{
		APIHost:   srv.URL,
		Key:       "machine-token",
		NodeType:  "shadowsocks",
		NodeID:    7,
		MachineID: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	callAllAPIs(t, c)
	checkRequests(t, requests(), []recordedRequest{
		{Method: http.MethodGet, Path: "/api/v2/server/config"},
		{Method: http.MethodGet, Path: "/api/v2/server/user"},
		{Method: http.MethodGet, Path: "/api/v2/server/alivelist"},
		{Method: http.MethodPost, Path: "/api/v2/server/push"},
		{Method: http.MethodPost, Path: "/api/v2/server/alive"},
	}, url.Values{
		"machine_id": {"3"},
		"node_id":    {"7"},
		"token":      {"machine-token"},
	}, "node_type")
}

func TestClient_V2MachineTokenRejected(t *testing.T) {
	// 模拟 Xboard ServerV2 中间件：机器或令牌不匹配时返回 401
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"Machine not found or invalid token"}`))
	}))
	defer srv.Close()
	c, err := New(&conf.ApiConfig{
		APIHost:   srv.URL,
		Key:       "wrong",
		NodeType:  "shadowsocks",
		NodeID:    7,
		MachineID: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.GetNodeInfo(); err == nil {
		t.Fatal("GetNodeInfo: expected error on 401")
	}
	if _, err := c.GetUserList(); err == nil {
		t.Fatal("GetUserList: expected error on 401")
	}
}
