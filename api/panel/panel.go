package panel

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/InazumaV/V2bX/conf"
	"github.com/go-resty/resty/v2"
)

const (
	// v1 接口：使用面板全局通讯密钥（server_token）鉴权
	v1PathPrefix = "/api/v1/server/UniProxy/"
	// v2 接口：Xboard 机器令牌鉴权（machine_id + 机器 token），只能访问绑定在该机器上的节点
	v2PathPrefix = "/api/v2/server/"
)

// Panel is the interface for different panel's api.

type Client struct {
	client           *resty.Client
	APIHost          string
	APISendIP        string
	Token            string
	NodeType         string
	NodeId           int
	MachineID        int
	pathPrefix       string
	nodeEtag         string
	userEtag         string
	responseBodyHash string
	UserList         *UserListBody
	AliveMap         *AliveMap
}

func New(c *conf.ApiConfig) (*Client, error) {
	var client *resty.Client
	if c.APISendIP != "" {
		client = resty.NewWithLocalAddr(&net.TCPAddr{
			IP: net.ParseIP(c.APISendIP),
		})
	} else {	
		client = resty.New()
	}
	client.SetRetryCount(3)
	if c.Timeout > 0 {
		client.SetTimeout(time.Duration(c.Timeout) * time.Second)
	} else {
		client.SetTimeout(5 * time.Second)
	}
	client.OnError(func(req *resty.Request, err error) {
		var v *resty.ResponseError
		if errors.As(err, &v) {
			// v.Response contains the last response from the server
			// v.Err contains the original error
			logrus.Error(v.Err)
		}
	})
	client.SetBaseURL(c.APIHost)
	// Check node type
	c.NodeType = strings.ToLower(c.NodeType)
	switch c.NodeType {
	case "v2ray":
		c.NodeType = "vmess"
	case
		"vmess",
		"trojan",
		"shadowsocks",
		"hysteria",
		"hysteria2",
		"tuic",
		"anytls",
		"vless":
	default:
		return nil, fmt.Errorf("unsupported Node type: %s", c.NodeType)
	}
	// set params
	pathPrefix := v1PathPrefix
	if c.MachineID > 0 {
		// 机器令牌模式：ApiKey 填机器 token，节点类型由面板按 node_id 确定
		pathPrefix = v2PathPrefix
		client.SetQueryParams(map[string]string{
			"machine_id": strconv.Itoa(c.MachineID),
			"node_id":    strconv.Itoa(c.NodeID),
			"token":      c.Key,
		})
		logrus.WithFields(logrus.Fields{
			"machine_id": c.MachineID,
			"node_id":    c.NodeID,
		}).Info("Using Xboard machine token auth (api/v2)")
	} else {
		client.SetQueryParams(map[string]string{
			"node_type": c.NodeType,
			"node_id":   strconv.Itoa(c.NodeID),
			"token":     c.Key,
		})
	}
	return &Client{
		client:     client,
		Token:      c.Key,
		APIHost:    c.APIHost,
		APISendIP:  c.APISendIP,
		NodeType:   c.NodeType,
		NodeId:     c.NodeID,
		MachineID:  c.MachineID,
		pathPrefix: pathPrefix,
		UserList:   &UserListBody{},
		AliveMap:   &AliveMap{},
	}, nil
}

// apiPath 根据鉴权模式拼出接口路径，如 config、user、push、alive、alivelist
func (c *Client) apiPath(name string) string {
	return c.pathPrefix + name
}
