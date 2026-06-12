// Package dnsbench 提供对 DNS 服务器的并发基准测试引擎：
// 并发查询、连接复用、结果聚合与评分。它不包含任何命令行/终端展示逻辑。
package dnsbench

// 支持的协议。
const (
	UDP = "udp"
	DOT = "dot"
	DOH = "doh"
)

// Server 描述一个待测试的 DNS 服务器。
type Server struct {
	Name, Address, Protocol string
	IsSystem                bool // 是否为检测到的系统当前默认 DNS
}

// 域名分类。
const (
	CategoryDomestic = "国内"
	CategoryForeign  = "国外"
	CategoryCustom   = "自定义"
)

// Domain 是一个带分类的测试域名。
type Domain struct{ Name, Category string }

// DefaultServers 是内置的默认 DNS 服务器列表。
var DefaultServers = []Server{
	{Name: "AliDNS 1 (UDP)", Address: "223.5.5.5", Protocol: UDP},
	{Name: "AliDNS 2 (UDP)", Address: "223.6.6.6", Protocol: UDP},
	{Name: "BaiduDNS (UDP)", Address: "180.76.76.76", Protocol: UDP},
	{Name: "DNSPod 1 (UDP)", Address: "119.28.28.28", Protocol: UDP},
	{Name: "DNSPod 2 (UDP)", Address: "119.29.29.29", Protocol: UDP},
	{Name: "114DNS 1 (UDP)", Address: "114.114.114.114", Protocol: UDP},
	{Name: "114DNS 2 (UDP)", Address: "114.114.115.115", Protocol: UDP},
	{Name: "114DNS Safe 1 (UDP)", Address: "114.114.114.119", Protocol: UDP},
	{Name: "114DNS Safe 2 (UDP)", Address: "114.114.115.119", Protocol: UDP},
	{Name: "114DNS Family 1 (UDP)", Address: "114.114.114.110", Protocol: UDP},
	{Name: "114DNS Family 2 (UDP)", Address: "114.114.115.110", Protocol: UDP},
	{Name: "Bytedance 1 (UDP)", Address: "180.184.1.1", Protocol: UDP},
	{Name: "Bytedance 2 (UDP)", Address: "180.184.2.2", Protocol: UDP},
	{Name: "Google 1 (UDP)", Address: "8.8.8.8", Protocol: UDP},
	{Name: "Google 2 (UDP)", Address: "8.8.4.4", Protocol: UDP},
	{Name: "Cloudflare 1 (UDP)", Address: "1.1.1.1", Protocol: UDP},
	{Name: "Cloudflare 2 (UDP)", Address: "1.0.0.1", Protocol: UDP},
	{Name: "Freenom 1 (UDP)", Address: "80.80.80.80", Protocol: UDP},
	{Name: "Freenom 2 (UDP)", Address: "80.80.81.81", Protocol: UDP},

	{Name: "AliDNS (DoT)", Address: "dns.alidns.com", Protocol: DOT},
	{Name: "DNSPod (DoT)", Address: "dot.pub", Protocol: DOT},
	{Name: "Google (DoT)", Address: "dns.google", Protocol: DOT},
	{Name: "Cloudflare 1 (DoT)", Address: "1.1.1.1", Protocol: DOT},
	{Name: "Cloudflare 2 (DoT)", Address: "one.one.one.one", Protocol: DOT},

	// 统一使用 RFC 8484 标准的 /dns-query 端点（wire-format，application/dns-message）。
	{Name: "AliDNS 1 (DoH)", Address: "https://dns.alidns.com/dns-query", Protocol: DOH},
	{Name: "AliDNS 2 (DoH)", Address: "https://223.5.5.5/dns-query", Protocol: DOH},
	{Name: "AliDNS 3 (DoH)", Address: "https://223.6.6.6/dns-query", Protocol: DOH},
	{Name: "DNSPod (DoH)", Address: "https://doh.pub/dns-query", Protocol: DOH},
	{Name: "Cloudflare 1 (DoH)", Address: "https://cloudflare-dns.com/dns-query", Protocol: DOH},
	{Name: "Cloudflare 2 (DoH)", Address: "https://1.1.1.1/dns-query", Protocol: DOH},
	{Name: "Cloudflare 3 (DoH)", Address: "https://1.0.0.1/dns-query", Protocol: DOH},
	{Name: "Google (DoH)", Address: "https://dns.google/dns-query", Protocol: DOH},
}

// DefaultDomains 是内置的默认测试域名列表（按分类均衡精选，去除同公司重复域名）。
var DefaultDomains = []Domain{
	{"baidu.com", CategoryDomestic},
	{"qq.com", CategoryDomestic},
	{"taobao.com", CategoryDomestic},
	{"jd.com", CategoryDomestic},
	{"bilibili.com", CategoryDomestic},
	{"douyin.com", CategoryDomestic},
	{"weibo.com", CategoryDomestic},
	{"163.com", CategoryDomestic},
	{"zhihu.com", CategoryDomestic},
	{"aliyun.com", CategoryDomestic},

	{"google.com", CategoryForeign},
	{"youtube.com", CategoryForeign},
	{"github.com", CategoryForeign},
	{"facebook.com", CategoryForeign},
	{"x.com", CategoryForeign},
	{"apple.com", CategoryForeign},
	{"chatgpt.com", CategoryForeign},
	{"bing.com", CategoryForeign},
	{"tiktok.com", CategoryForeign},
	{"cloudflare.com", CategoryForeign},
}
