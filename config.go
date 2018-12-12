package byterpc

import "time"

type ServerConfig struct {
	PSM                     string
	Port                    int           // 运行端口
	CodecType               string        // 编解码类型
	RequestTimeout          time.Duration // 方法调用超时
	ConnectionTimeout       time.Duration //
	Retries                 int           // 方法调用失败重试次数
	LoadBalance             string        // 负载均衡算法: random, roundrobin 等
	MaxConcurrencyOnMethod  int           // 消费者端对方法级别的最大并发调用限制，即当 Consumer 对一个方法的并发调用到上限后，新调用会阻塞直到超时
	MaxConcurrencyOnService int           // 消费者端对服务级别最大并发调用限制，即当 Consumer 对一个服务的并发调用到上限后，新调用会阻塞直到超时
}

func DefaultConfig() *ServerConfig {
	return &ServerConfig{
		CodecType:               "gob",
		Port:                    8888,
		PSM:                     "byterpc.default.basic",
		RequestTimeout:          time.Second * 30,
		Retries:                 3,
		LoadBalance:             "random",
		MaxConcurrencyOnMethod:  10000,
		MaxConcurrencyOnService: 10000 * 10,
	}

}

func (c *ServerConfig) check() error {
	return nil
}

type ClientConfig struct {
	CodecType string
}
