// Package browserless 提供基于 Browserless v2 的远程浏览器集群 Provider 实现。
//
// 支持多节点负载均衡 (least-load / round-robin)、健康检查、容量预检、
// stealth 模式、代理透传等能力。适合生产环境大规模部署。
//
// 用法:
//
//	provider := browserless.NewProvider(browserless.Config{
//	    Stealth:       true,
//	    RouteStrategy: "least-load",
//	    Endpoints: []browserless.EndpointConfig{
//	        {URL: "http://chrome-1:3000?token=xxx"},
//	        {URL: "http://chrome-2:3000?token=xxx"},
//	    },
//	})
//	pool.Register(provider)
package browserless
