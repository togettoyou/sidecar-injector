package main

import (
	"fmt"
	"net/http"
)

const (
	// hostname 为 API Server 的请求域名，根据实际情况更改
	hostname = "host.docker.internal"
	port     = 9443
	crt      = "tls.crt"
	key      = "tls.key"
)

func main() {
	// 1.为 webhook 服务自建 https 证书
	caPEM, err := createCert()
	if err != nil {
		panic(err)
	}
	// 2.创建 MutatingWebhookConfiguration
	err = createMutatingWebhookConfiguration(caPEM)
	if err != nil {
		panic(err)
	}
	// 3.注册 /inject 和 /inject/ 路由
	http.HandleFunc("/inject", inject)
	http.HandleFunc("/inject/", inject)
	// 4.启动 webhook 服务
	panic(http.ListenAndServeTLS(fmt.Sprintf(":%d", port), crt, key, nil))
}
