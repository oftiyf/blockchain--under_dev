package tx

import "time"

// TxPoolConfig 定义了交易池的配置参数
// 包括价格限制、队列长度、生命周期等
// 可根据实际需求调整
type TxPoolConfig struct {
	PriceLimit   uint64        // 最低Gas价格限制
	PriceBump    uint64        // 替换同Nonce交易所需的价格提升百分比
	AccountSlots uint64        // 每个账户pending队列的最小值
	GlobalSlots  uint64        // 全局pending队列的最大值
	AccountQueue uint64        // 每个账户queue队列的最小值
	GlobalQueue  uint64        // 全局queue队列的最大值
	Lifetime     time.Duration // queue中交易的最长等待时间
	NoLocals     bool          // 是否禁用本地交易特殊处理
	Journal      string        // 本地交易日志文件路径
	Rejournal    time.Duration // 日志轮转周期
}

// sanitize 用于校验和修正配置参数
func (cfg *TxPoolConfig) sanitize() TxPoolConfig {
	if cfg.PriceBump == 0 {
		cfg.PriceBump = 10 // 默认10%
	}
	if cfg.AccountSlots == 0 {
		cfg.AccountSlots = 16
	}
	if cfg.GlobalSlots == 0 {
		cfg.GlobalSlots = 4096
	}
	if cfg.AccountQueue == 0 {
		cfg.AccountQueue = 64
	}
	if cfg.GlobalQueue == 0 {
		cfg.GlobalQueue = 1024
	}
	if cfg.Lifetime == 0 {
		cfg.Lifetime = 3 * time.Hour
	}
	if cfg.Rejournal == 0 {
		cfg.Rejournal = time.Hour
	}
	return *cfg
}
