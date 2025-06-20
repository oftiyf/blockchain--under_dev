package tx

import (
	"blockchain/common"
	"blockchain/stateDB"
	"fmt"
	"log"
	"math"
	"math/big"
	"sort"
	"sync"
	"time"
)

// 常量定义
const (
	chainHeadChanSize   = 10
	statsReportInterval = 8 * time.Second
	evictionInterval    = 10 * time.Minute
)

// 错误定义
var (
	ErrOversizedData      = fmt.Errorf("transaction data too large")
	ErrNegativeValue      = fmt.Errorf("transaction value cannot be negative")
	ErrGasLimit           = fmt.Errorf("transaction gas limit exceeds block limit")
	ErrInvalidSender      = fmt.Errorf("invalid transaction sender")
	ErrUnderpriced        = fmt.Errorf("transaction gas price too low")
	ErrNonceTooLow        = fmt.Errorf("transaction nonce too low")
	ErrInsufficientFunds  = fmt.Errorf("insufficient funds for transaction")
	ErrIntrinsicGas       = fmt.Errorf("transaction gas limit below intrinsic gas")
	ErrReplaceUnderpriced = fmt.Errorf("replacement transaction underpriced")
)

// 事件定义
type ChainHeadEvent struct {
	Block *Block
}

type TxPreEvent struct {
	Tx *Transaction
}

// 接口定义
type blockChain interface {
	CurrentBlock() *Block
	StateAt(root []byte) (stateDB.StateDB, error)
	SubscribeChainHeadEvent(ch chan ChainHeadEvent) Subscription
	GetBlock(hash common.Hash, number uint64) *Block
}

type Subscription interface {
	Err() <-chan error
	Unsubscribe()
}

type Block interface {
	Header() *Header
	Transactions() []*Transaction
	NumberU64() uint64
	Hash() common.Hash
	ParentHash() common.Hash
}

type Header struct {
	Number     *big.Int
	Hash       common.Hash
	ParentHash common.Hash
	Root       []byte
	GasLimit   *big.Int
}

// 辅助类型
type addressByHeartbeat struct {
	address   common.Address
	heartbeat time.Time
}

type addresssByHeartbeat []addressByHeartbeat

func (a addresssByHeartbeat) Len() int           { return len(a) }
func (a addresssByHeartbeat) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a addresssByHeartbeat) Less(i, j int) bool { return a[i].heartbeat.Before(a[j].heartbeat) }

// TxPool 包含所有当前已知的交易
// 交易从网络接收或本地提交时进入池中，被包含在区块链中时退出池
// 池将可处理的交易（可应用于当前状态）和未来交易分开
// 交易随着时间推移在接收和处理过程中在这两种状态之间移动
type TxPool struct {
	config       TxPoolConfig
	chainconfig  *ChainConfig
	chain        blockChain
	gasPrice     *big.Int
	txFeed       *Feed
	scope        *SubscriptionScope
	chainHeadCh  chan ChainHeadEvent // 订阅区块头消息，当新区块头生成时会在此接收通知
	chainHeadSub Subscription
	signer       Signer // 封装交易签名处理
	mu           sync.RWMutex

	currentState  stateDB.StateDB // 区块链头部的当前状态
	pendingState  *ManagedState   // 跟踪虚拟nonce的待处理状态
	currentMaxGas *big.Int        // 交易上限的当前gas限制

	locals  *accountSet // 本地交易集合，豁免驱逐规则
	journal *txJournal  // 本地交易日志，备份到磁盘

	pending map[common.Address]*txList   // 所有当前可处理的交易
	queue   map[common.Address]*txList   // 排队但不可处理的交易
	beats   map[common.Address]time.Time // 每个已知账户的最后心跳
	all     map[common.Hash]*Transaction // 所有交易，允许查找
	priced  *txPricedList                // 按价格排序的所有交易

	wg sync.WaitGroup // 用于关闭同步

	homestead bool // homestead版本
}

// ChainConfig 链配置
type ChainConfig struct {
	ChainId *big.Int
}

// ManagedState 管理状态
type ManagedState struct {
	state stateDB.StateDB
}

// Feed 事件源
type Feed struct{}

func (f *Feed) Send(event interface{}) int { return 0 }

type SubscriptionScope struct{}

// NewTxPool 创建一个新的交易池来收集、排序和过滤来自网络的入站交易
func NewTxPool(config TxPoolConfig, chainconfig *ChainConfig, chain blockChain) *TxPool {
	// 清理输入以确保没有设置易受攻击的gas价格
	config = config.sanitize()

	// 使用其初始设置创建交易池
	pool := &TxPool{
		config:      config,
		chainconfig: chainconfig,
		chain:       chain,
		signer:      NewEIP155Signer(chainconfig.ChainId),
		pending:     make(map[common.Address]*txList),
		queue:       make(map[common.Address]*txList),
		beats:       make(map[common.Address]time.Time),
		all:         make(map[common.Hash]*Transaction),
		chainHeadCh: make(chan ChainHeadEvent, chainHeadChanSize),
		gasPrice:    new(big.Int).SetUint64(config.PriceLimit),
		txFeed:      &Feed{},
		scope:       &SubscriptionScope{},
	}
	pool.locals = newAccountSet(pool.signer)
	pool.priced = newTxPricedList(&pool.all)
	pool.reset(nil, chain.CurrentBlock().Header())

	// 如果启用了本地交易和日志记录，从磁盘加载
	// 如果允许本地交易且配置的Journal目录不为空，从指定目录加载日志
	// 然后轮转交易日志。由于旧交易可能已过期，在调用add方法后将接收到的交易写入日志
	if !config.NoLocals && config.Journal != "" {
		pool.journal = newTxJournal(config.Journal)

		if err := pool.journal.load(pool.AddLocal); err != nil {
			log.Printf("Failed to load transaction journal: %v", err)
		}
		if err := pool.journal.rotate(pool.local()); err != nil {
			log.Printf("Failed to rotate transaction journal: %v", err)
		}
	}
	// 订阅区块链事件
	pool.chainHeadSub = pool.chain.SubscribeChainHeadEvent(pool.chainHeadCh)

	// 启动事件循环并返回
	pool.wg.Add(1)
	go pool.loop()

	return pool
}

// reset 检索区块链的当前状态并确保交易池的内容相对于当前区块链状态有效
func (pool *TxPool) reset(oldHead, newHead *Header) {
	// 如果我们正在重组旧状态，重新注入所有丢弃的交易
	var reinject []*Transaction

	if oldHead != nil && oldHead.Hash != newHead.ParentHash {
		// 如果重组太深，避免执行（在快速同步期间会发生）
		oldNum := oldHead.Number.Uint64()
		newNum := newHead.Number.Uint64()

		if depth := uint64(math.Abs(float64(oldNum) - float64(newNum))); depth > 64 {
			// 如果旧头和新头相距太远，则取消重建
			log.Printf("Skipping deep transaction reorg, depth: %d", depth)
		} else {
			// 重组似乎足够浅，可以将所有交易拉入内存
			var discarded, included []*Transaction

			var (
				rem = pool.chain.GetBlock(oldHead.Hash, oldHead.Number.Uint64())
				add = pool.chain.GetBlock(newHead.Hash, newHead.Number.Uint64())
			)
			// 如果旧高度大于新高度，则需要删除所有内容
			for rem.NumberU64() > add.NumberU64() {
				discarded = append(discarded, rem.Transactions()...)
				if rem = pool.chain.GetBlock(rem.ParentHash(), rem.NumberU64()-1); rem == nil {
					log.Printf("Unrooted old chain seen by tx pool, block: %d, hash: %s", oldHead.Number, oldHead.Hash)
					return
				}
			}
			// 如果新高度大于旧高度，则需要增加
			for add.NumberU64() > rem.NumberU64() {
				included = append(included, add.Transactions()...)
				if add = pool.chain.GetBlock(add.ParentHash(), add.NumberU64()-1); add == nil {
					log.Printf("Unrooted new chain seen by tx pool, block: %d, hash: %s", newHead.Number, newHead.Hash)
					return
				}
			}
			// 高度相同。如果哈希不同，则需要向后查找并找到具有相同哈希根的节点
			for rem.Hash() != add.Hash() {
				discarded = append(discarded, rem.Transactions()...)
				if rem = pool.chain.GetBlock(rem.ParentHash(), rem.NumberU64()-1); rem == nil {
					log.Printf("Unrooted old chain seen by tx pool, block: %d, hash: %s", oldHead.Number, oldHead.Hash)
					return
				}
				included = append(included, add.Transactions()...)
				if add = pool.chain.GetBlock(add.ParentHash(), add.NumberU64()-1); add == nil {
					log.Printf("Unrooted new chain seen by tx pool, block: %d, hash: %s", newHead.Number, newHead.Hash)
					return
				}
			}
			// 找出所有存在于discard中但不在included中的值
			// 需要等待这些交易重新插入到池中
			reinject = TxDifference(discarded, included)
		}
	}
	// 将内部状态初始化为当前头
	if newHead == nil {
		newHead = pool.chain.CurrentBlock().Header() // 测试期间的特殊情况
	}
	statedb, err := pool.chain.StateAt(newHead.Root)
	if err != nil {
		log.Printf("Failed to reset txpool state: %v", err)
		return
	}
	pool.currentState = statedb
	pool.pendingState = &ManagedState{state: statedb}
	pool.currentMaxGas = newHead.GasLimit

	// 注入由于重组而丢弃的任何交易
	log.Printf("Reinjecting stale transactions, count: %d", len(reinject))
	pool.addTxsLocked(reinject, false)

	// 验证待处理交易池，这将删除已包含在区块中或由于另一个交易而失效的任何交易（例如更高的gas价格）
	pool.demoteUnexecutables()

	// 将所有账户更新为最新的已知待处理nonce
	for addr, list := range pool.pending {
		txs := list.Flatten() // 很重但会被缓存，矿工无论如何都需要
		pool.pendingState.SetNonce(addr, txs[len(txs)-1].Nonce+1)
	}
	// 检查队列并在可能的情况下将交易移动到待处理状态，或删除那些已失效的交易
	pool.promoteExecutables(nil)
}

// addTx 如果有效，将单个交易排队到池中
func (pool *TxPool) addTx(tx *Transaction, local bool) error {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	// 尝试注入交易并更新任何状态
	replace, err := pool.add(tx, local)
	if err != nil {
		return err
	}
	// 如果我们添加了新交易，运行提升检查并返回
	if !replace {
		from, _ := pool.signer.Sender(tx) // 已经验证
		pool.promoteExecutables([]common.Address{from})
	}
	return nil
}

// addTxsLocked 尝试在假设交易池锁已被持有的情况下将一批交易排队（如果它们有效）
func (pool *TxPool) addTxsLocked(txs []*Transaction, local bool) error {
	// 添加交易批次，跟踪接受的交易
	dirty := make(map[common.Address]struct{})
	for _, tx := range txs {
		if replace, err := pool.add(tx, local); err == nil {
			if !replace { // replace：如果不是替换，则意味着状态已更新，在下一步中有可能被处理
				from, _ := pool.signer.Sender(tx) // 已经验证
				dirty[from] = struct{}{}
			}
		}
	}
	// 只有在实际添加了内容时才重新处理内部状态
	if len(dirty) > 0 {
		addrs := make([]common.Address, 0, len(dirty))
		for addr := range dirty {
			addrs = append(addrs, addr)
		}
		// 修改的地址
		pool.promoteExecutables(addrs)
	}
	return nil
}

// demoteUnexecutables 从池的可执行/待处理队列中删除无效和已处理的交易，任何变得不可执行的后续交易都会移回未来队列
func (pool *TxPool) demoteUnexecutables() {
	// 遍历所有账户并降级任何不可执行的交易
	for addr, list := range pool.pending {
		nonce := pool.currentState.GetNonce(addr)

		// 删除所有被认为太旧的交易（低nonce）
		for _, tx := range list.Forward(nonce) {
			hash, _ := tx.GetHash()
			log.Printf("Removed old pending transaction, hash: %s", hash)
			delete(pool.all, hash)
			pool.priced.Removed()
		}
		// 删除所有成本过高的交易（余额低或gas不足），并将任何无效交易排队等待稍后处理
		drops, invalids := list.Filter(pool.currentState.GetBalance(addr), pool.currentMaxGas)
		for _, tx := range drops {
			hash, _ := tx.GetHash()
			log.Printf("Removed unpayable pending transaction, hash: %s", hash)
			delete(pool.all, hash)
			pool.priced.Removed()
		}
		for _, tx := range invalids {
			hash, _ := tx.GetHash()
			log.Printf("Demoting pending transaction, hash: %s", hash)
			pool.enqueueTx(hash, tx)
		}
		// 如果前面有间隙，警告（不应该发生）并推迟所有交易
		// 这一步确实不应该发生，因为Filter已经处理了invalids。应该没有invalids的交易，也就是说没有漏洞
		if list.Len() > 0 && list.txs[0].Nonce != nonce {
			for _, tx := range list.Cap(0) {
				hash, _ := tx.GetHash()
				log.Printf("Demoting invalidated transaction, hash: %s", hash)
				pool.enqueueTx(hash, tx)
			}
		}
		// 如果整个队列条目变为空，则删除它
		if list.Empty() {
			delete(pool.pending, addr)
			delete(pool.beats, addr)
		}
	}
}

// enqueueTx 将新交易插入到不可执行交易队列中
// 注意，此方法假设池锁已被持有！
func (pool *TxPool) enqueueTx(hash common.Hash, tx *Transaction) (bool, error) {
	// 尝试将交易插入未来队列
	from, _ := pool.signer.Sender(tx) // 已经验证
	if pool.queue[from] == nil {
		pool.queue[from] = newTxList(false)
	}
	inserted, old := pool.queue[from].Add(tx, pool.config.PriceBump)
	if !inserted {
		// 较旧的交易更好，丢弃这个
		return false, ErrReplaceUnderpriced
	}
	// 丢弃任何先前的交易并标记这个
	if old != nil {
		oldHash, _ := old.GetHash()
		delete(pool.all, oldHash)
		pool.priced.Removed()
	}
	pool.all[hash] = tx
	pool.priced.Put(tx)
	return old != nil, nil
}

// promoteExecutables 将已变得可处理的交易从未来队列移动到待处理交易集合。在此过程中，所有失效的交易（低nonce，低余额）都会被删除
func (pool *TxPool) promoteExecutables(accounts []common.Address) {
	// 收集所有可能需要更新的账户
	// accounts存储所有需要更新的账户。如果传入的account为nil，则表示所有已知账户
	if accounts == nil {
		accounts = make([]common.Address, 0, len(pool.queue))
		for addr := range pool.queue {
			accounts = append(accounts, addr)
		}
	}
	// 遍历所有账户并提升任何可执行的交易
	for _, addr := range accounts {
		list := pool.queue[addr]
		if list == nil {
			continue // 以防有人用不存在的账户调用
		}
		// 删除所有被认为太旧的交易（低nonce）
		for _, tx := range list.Forward(pool.currentState.GetNonce(addr)) {
			hash, _ := tx.GetHash()
			log.Printf("Removed old queued transaction, hash: %s", hash)
			delete(pool.all, hash)
			pool.priced.Removed()
		}
		// 删除所有成本过高的交易（余额低或gas不足）
		drops, _ := list.Filter(pool.currentState.GetBalance(addr), pool.currentMaxGas)
		for _, tx := range drops {
			hash, _ := tx.GetHash()
			log.Printf("Removed unpayable queued transaction, hash: %s", hash)
			delete(pool.all, hash)
			pool.priced.Removed()
		}
		// 收集所有可执行的交易并提升它们
		for _, tx := range list.Ready(pool.pendingState.GetNonce(addr)) {
			hash, _ := tx.GetHash()
			log.Printf("Promoting queued transaction, hash: %s", hash)
			pool.promoteTx(addr, hash, tx)
		}
		// 删除超过允许限制的所有交易
		if !pool.locals.contains(addr) {
			for _, tx := range list.Cap(int(pool.config.AccountQueue)) {
				hash, _ := tx.GetHash()
				delete(pool.all, hash)
				pool.priced.Removed()
				log.Printf("Removed cap-exceeding queued transaction, hash: %s", hash)
			}
		}
		// 如果整个队列条目变为空，则删除它
		if list.Empty() {
			delete(pool.queue, addr)
		}
	}
	// 如果待处理限制溢出，开始平衡配额
	pending := uint64(0)
	for _, list := range pool.pending {
		pending += uint64(list.Len())
	}
	// 如果待处理总数超过系统配置
	if pending > pool.config.GlobalSlots {
		pendingBeforeCap := pending
		// 组装垃圾邮件顺序以首先惩罚大额交易者
		spammers := make([]common.Address, 0)
		for addr, list := range pool.pending {
			// 只从高额交易者中驱逐交易
			// 首先记录所有大于AccountSlots最小值的账户，将从这些账户中删除一些交易
			// 注意spammers是一个优先级队列，按交易数量从大到小排序
			if !pool.locals.contains(addr) && uint64(list.Len()) > pool.config.AccountSlots {
				spammers = append(spammers, addr)
			}
		}
		// 按交易数量排序
		sort.Slice(spammers, func(i, j int) bool {
			return pool.pending[spammers[i]].Len() > pool.pending[spammers[j]].Len()
		})

		// 逐渐从违规者中删除交易
		offenders := []common.Address{}
		for pending > pool.config.GlobalSlots && len(spammers) > 0 {
			// 模拟违规者队列中账户交易数量的变化
			// 第一个周期 [10] 周期结束 [10]
			// 第二个周期 [10, 9] 周期结束 [9,9]
			// 第三个周期 [9, 9, 7] 周期结束 [7, 7, 7]
			// 第四个周期 [7, 7 , 7 , 2] 周期结束 [2, 2 , 2, 2]

			// 如果不是本地地址，检索下一个违规者
			offender := spammers[0]
			spammers = spammers[1:]
			offenders = append(offenders, offender)

			// 平衡余额直到全部相同或低于阈值
			if len(offenders) > 1 { // 第一次进入此循环时，违规者队列中有两个交易数量最大的账户
				// 计算所有当前违规者的平衡阈值
				// 最后添加账户的交易数量是成本阈值
				threshold := pool.pending[offender].Len()

				// 迭代减少所有违规者直到低于限制或达到阈值
				for pending > pool.config.GlobalSlots && pool.pending[offenders[len(offenders)-2]].Len() > threshold {
					// 遍历除最后一个之外的所有账户，并将它们的交易数量减1
					for i := 0; i < len(offenders)-1; i++ {
						list := pool.pending[offenders[i]]
						for _, tx := range list.Cap(list.Len() - 1) {
							// 也从全局池中删除交易
							hash, _ := tx.GetHash()
							delete(pool.all, hash)
							pool.priced.Removed()

							// 将账户nonce更新为删除的交易
							if nonce := tx.Nonce; pool.pendingState.GetNonce(offenders[i]) > nonce {
								pool.pendingState.SetNonce(offenders[i], nonce)
							}
							log.Printf("Removed fairness-exceeding pending transaction, hash: %s", hash)
						}
						pending--
					}
				}
			}
		}
		// 如果仍然高于阈值，减少到限制或最小配额
		// 在上述循环之后，所有超过AccountSlots的账户的交易数量已更改为之前的最小值
		// 如果仍然超过阈值，则继续每次从违规者中删除一个
		if pending > pool.config.GlobalSlots && len(offenders) > 0 {
			for pending > pool.config.GlobalSlots && uint64(pool.pending[offenders[len(offenders)-1]].Len()) > pool.config.AccountSlots {
				for _, addr := range offenders {
					list := pool.pending[addr]
					for _, tx := range list.Cap(list.Len() - 1) {
						// 也从全局池中删除交易
						hash, _ := tx.GetHash()
						delete(pool.all, hash)
						pool.priced.Removed()

						// 将账户nonce更新为删除的交易
						if nonce := tx.Nonce; pool.pendingState.GetNonce(addr) > nonce {
							pool.pendingState.SetNonce(addr, nonce)
						}
						log.Printf("Removed fairness-exceeding pending transaction, hash: %s", hash)
					}
					pending--
				}
			}
		}
		log.Printf("Pending rate limit counter: %d", pendingBeforeCap-pending)
	}
	// 结束 if pending > pool.config.GlobalSlots
	// 如果我们排队的交易超过硬限制，删除最旧的
	// 我们已经处理了待处理的限制，我们需要处理未来队列的限制
	queued := uint64(0)
	for _, list := range pool.queue {
		queued += uint64(list.Len())
	}
	if queued > pool.config.GlobalQueue {
		// 按心跳对所有有排队交易的账户进行排序
		addresses := make(addresssByHeartbeat, 0, len(pool.queue))
		for addr := range pool.queue {
			if !pool.locals.contains(addr) { // 不删除本地交易
				addresses = append(addresses, addressByHeartbeat{addr, pool.beats[addr]})
			}
		}
		sort.Sort(addresses)

		// 删除交易直到总数低于限制或只剩下本地交易
		// 从现在开始，心跳越新，删除的越多
		for drop := queued - pool.config.GlobalQueue; drop > 0 && len(addresses) > 0; {
			addr := addresses[len(addresses)-1]
			list := pool.queue[addr.address]

			addresses = addresses[:len(addresses)-1]

			// 如果它们小于溢出，删除所有交易
			if size := uint64(list.Len()); size <= drop {
				for _, tx := range list.Flatten() {
					hash, _ := tx.GetHash()
					pool.removeTx(hash)
				}
				drop -= size
				continue
			}
			// 否则只删除最后几个交易
			txs := list.Flatten()
			for i := len(txs) - 1; i >= 0 && drop > 0; i-- {
				hash, _ := txs[i].GetHash()
				pool.removeTx(hash)
				drop--
			}
		}
	}
}

// promoteTx 将交易添加到待处理（可处理）交易列表
// 注意，此方法假设已获得锁！
func (pool *TxPool) promoteTx(addr common.Address, hash common.Hash, tx *Transaction) {
	// 尝试将交易插入待处理队列
	if pool.pending[addr] == nil {
		pool.pending[addr] = newTxList(true)
	}
	list := pool.pending[addr]

	inserted, old := list.Add(tx, pool.config.PriceBump)
	if !inserted {
		// 如果没有替换，则已经有旧交易。删除
		// 较旧的交易更好，丢弃这个
		delete(pool.all, hash)
		pool.priced.Removed()
		return
	}
	// 否则丢弃任何先前的交易并标记这个
	if old != nil {
		oldHash, _ := old.GetHash()
		delete(pool.all, oldHash)
		pool.priced.Removed()
	}
	// 故障保护以解决直接待处理插入（测试）
	if pool.all[hash] == nil {
		pool.all[hash] = tx
		pool.priced.Put(tx)
	}
	// 设置潜在的新待处理nonce并通知任何子系统新交易
	// 将交易添加到队列并向所有订阅者发送消息，订阅者在eth协议内部。它将接收消息并在网络上广播消息
	pool.beats[addr] = time.Now()
	pool.pendingState.SetNonce(addr, tx.Nonce+1)
	go pool.txFeed.Send(TxPreEvent{tx})
}

// removeTx 从队列中删除单个交易，将所有后续交易移回未来队列
func (pool *TxPool) removeTx(hash common.Hash) {
	// 获取我们想要删除的交易
	tx, ok := pool.all[hash]
	if !ok {
		return
	}
	addr, _ := pool.signer.Sender(tx) // 插入期间已经验证

	// 从已知交易列表中删除它
	delete(pool.all, hash)
	pool.priced.Removed()

	// 从待处理列表中删除交易并重置账户nonce
	// 从待处理中删除交易，并将由于删除此交易而失效的交易放入未来队列，然后更新pendingState的状态
	if pending := pool.pending[addr]; pending != nil {
		if removed, invalids := pending.Remove(tx); removed {
			// 如果没有更多交易，删除列表
			if pending.Empty() {
				delete(pool.pending, addr)
				delete(pool.beats, addr)
			} else {
				// 否则推迟任何失效的交易
				for _, tx := range invalids {
					hash, _ := tx.GetHash()
					pool.enqueueTx(hash, tx)
				}
			}
			// 如果需要，更新账户nonce
			if nonce := tx.Nonce; pool.pendingState.GetNonce(addr) > nonce {
				pool.pendingState.SetNonce(addr, nonce)
			}
			return
		}
	}
	// 交易在未来队列中
	if future := pool.queue[addr]; future != nil {
		future.Remove(tx)
		if future.Empty() {
			delete(pool.queue, addr)
		}
	}
}

// loop 是交易池的主要事件循环，等待并响应外部区块链事件以及各种报告和交易驱逐事件
func (pool *TxPool) loop() {
	defer pool.wg.Done()

	// 启动统计报告和交易驱逐计时器
	var prevPending, prevQueued, prevStales int

	report := time.NewTicker(statsReportInterval)
	defer report.Stop()

	evict := time.NewTicker(evictionInterval)
	defer evict.Stop()

	journal := time.NewTicker(pool.config.Rejournal)
	defer journal.Stop()

	// 跟踪用于交易重组的先前头头
	head := pool.chain.CurrentBlock()

	// 继续等待并响应各种事件
	for {
		select {
		// 处理ChainHeadEvent
		// 监听区块头事件并获取新区块头
		case ev := <-pool.chainHeadCh:
			if ev.Block != nil {
				pool.mu.Lock()
				pool.reset(head.Header(), ev.Block.Header())
				head = ev.Block
				pool.mu.Unlock()
			}
		// 由于系统停止而被取消订阅
		case <-pool.chainHeadSub.Err():
			return

		// 处理统计报告计时器
		case <-report.C:
			pool.mu.RLock()
			pending, queued := pool.stats()
			stales := pool.priced.stales
			pool.mu.RUnlock()

			if pending != prevPending || queued != prevQueued || stales != prevStales {
				log.Printf("Transaction pool status report, executable: %d, queued: %d, stales: %d", pending, queued, stales)
				prevPending, prevQueued, prevStales = pending, queued, stales
			}

		// 处理非活动账户交易驱逐
		case <-evict.C:
			pool.mu.Lock()
			for addr := range pool.queue {
				// 从驱逐机制中跳过本地交易
				if pool.locals.contains(addr) {
					continue
				}
				// 任何足够旧的本地交易都应该被删除
				if time.Since(pool.beats[addr]) > pool.config.Lifetime {
					for _, tx := range pool.queue[addr].Flatten() {
						hash, _ := tx.GetHash()
						pool.removeTx(hash)
					}
				}
			}
			pool.mu.Unlock()

		// 处理本地交易日志轮转
		// rotate根据交易池的当前内容重新生成交易日志
		case <-journal.C:
			if pool.journal != nil {
				pool.mu.Lock()
				if err := pool.journal.rotate(pool.local()); err != nil {
					log.Printf("Failed to rotate local tx journal: %v", err)
				}
				pool.mu.Unlock()
			}
		}
	}
}

// add 验证交易并将其插入到不可执行队列中以供稍后待处理提升和执行。如果交易是已待处理或排队交易的替换，它会覆盖前一个并返回这个，以便外部代码不会无用地调用promote
//
// 如果新添加的交易被标记为本地，其发送账户将被列入白名单，防止任何相关交易因价格限制而被驱逐出池
func (pool *TxPool) add(tx *Transaction, local bool) (bool, error) {
	// 如果交易已经已知，丢弃它
	hash, _ := tx.GetHash()
	if pool.all[hash] != nil {
		log.Printf("Discarding already known transaction, hash: %s", hash)
		return false, fmt.Errorf("known transaction: %s", hash)
	}
	// 如果交易基本验证失败，丢弃它
	if err := pool.validateTx(tx, local); err != nil {
		log.Printf("Discarding invalid transaction, hash: %s, err: %v", hash, err)
		return false, err
	}
	// 如果交易池已满，丢弃价格过低的交易
	if uint64(len(pool.all)) >= pool.config.GlobalSlots+pool.config.GlobalQueue {
		// 如果新交易价格过低，不接受它
		if pool.priced.Underpriced(tx, pool.locals) {
			log.Printf("Discarding underpriced transaction, hash: %s, price: %s", hash, tx.GasPrice)
			return false, ErrUnderpriced
		}
		// 新交易比我们最差的更好，为它腾出空间
		drop := pool.priced.Discard(len(pool.all)-int(pool.config.GlobalSlots+pool.config.GlobalQueue-1), pool.locals)
		for _, tx := range drop {
			hash, _ := tx.GetHash()
			log.Printf("Discarding freshly underpriced transaction, hash: %s, price: %s", hash, tx.GasPrice)
			pool.removeTx(hash)
		}
	}
	// 如果交易正在替换已经待处理的交易，直接执行
	from, _ := pool.signer.Sender(tx) // 已经验证
	if list := pool.pending[from]; list != nil && list.Overlaps(tx) {
		// Nonce已经待处理，检查是否满足所需的价格提升
		inserted, old := list.Add(tx, pool.config.PriceBump)
		if !inserted {
			return false, ErrReplaceUnderpriced
		}
		// 新交易更好，替换旧交易
		if old != nil {
			oldHash, _ := old.GetHash()
			delete(pool.all, oldHash)
			pool.priced.Removed()
		}
		pool.all[hash] = tx
		pool.priced.Put(tx)
		pool.journalTx(from, tx)

		log.Printf("Pooled new executable transaction, hash: %s, from: %s, to: %s", hash, from, tx.To)
		return old != nil, nil
	}
	// 新交易没有替换待处理的交易，推入队列
	replace, err := pool.enqueueTx(hash, tx)
	if err != nil {
		return false, err
	}
	// 标记本地地址并记录本地交易
	if local {
		pool.locals.add(from)
	}
	// 如果是本地交易，将记录到journalTx
	pool.journalTx(from, tx)

	log.Printf("Pooled new future transaction, hash: %s, from: %s, to: %s", hash, from, tx.To)
	return replace, nil
}

// validateTx 根据共识规则检查交易是否有效，并遵守本地节点的一些启发式限制（价格和大小）
func (pool *TxPool) validateTx(tx *Transaction, local bool) error {
	// 启发式限制，拒绝超过32KB的交易以防止DOS攻击
	if tx.Size() > 32*1024 {
		return ErrOversizedData
	}
	// 交易不能为负。使用RLP解码的交易可能永远不会发生这种情况，但如果您使用RPC创建交易，可能会发生
	if tx.Value.Sign() < 0 {
		return ErrNegativeValue
	}
	// 确保交易不超过当前区块限制gas
	// currentMaxGas是currentBlock的GasLimit
	if pool.currentMaxGas.Cmp(tx.GasLimit) < 0 {
		return ErrGasLimit
	}
	// 确保交易签名正确
	from, err := pool.signer.Sender(tx)
	if err != nil {
		return ErrInvalidSender
	}
	// 删除低于我们自己最低接受gas价格的非本地交易
	local = local || pool.locals.contains(from) // 即使交易来自网络，账户也可能是本地的
	// 如果不是本地交易且GasPrice低于我们的设置，将不会接收
	if !local && pool.gasPrice.Cmp(tx.GasPrice) > 0 {
		return ErrUnderpriced
	}
	// 确保交易遵守nonce排序
	if pool.currentState.GetNonce(from) > tx.Nonce {
		return ErrNonceTooLow
	}
	// 交易者应该有足够的资金来支付成本
	// cost == V - Value + GP - GasPrice * GL - GasLimit
	if pool.currentState.GetBalance(from).Cmp(tx.Cost()) < 0 {
		return ErrInsufficientFunds
	}
	intrGas := IntrinsicGas(tx.Data, tx.To == nil, pool.homestead)
	// 如果交易是合约创建或调用。然后看看是否有足够的初始Gas
	if tx.GasLimit < intrGas {
		return ErrIntrinsicGas
	}
	return nil
}

// 辅助方法
func (pool *TxPool) stats() (int, int) {
	pending := 0
	for _, list := range pool.pending {
		pending += list.Len()
	}
	queued := 0
	for _, list := range pool.queue {
		queued += list.Len()
	}
	return pending, queued
}

func (pool *TxPool) local() []*Transaction {
	var txs []*Transaction
	for addr := range pool.locals.accounts {
		if list := pool.pending[addr]; list != nil {
			txs = append(txs, list.Flatten()...)
		}
		if list := pool.queue[addr]; list != nil {
			txs = append(txs, list.Flatten()...)
		}
	}
	return txs
}

func (pool *TxPool) journalTx(from common.Address, tx *Transaction) {
	if pool.journal != nil {
		pool.journal.insert(tx)
	}
}

func (pool *TxPool) AddLocal(tx *Transaction) error {
	return pool.addTx(tx, true)
}

// 辅助函数
func TxDifference(a, b []*Transaction) []*Transaction {
	keep := make(map[common.Hash]struct{})
	for _, tx := range b {
		hash, _ := tx.GetHash()
		keep[hash] = struct{}{}
	}
	var result []*Transaction
	for _, tx := range a {
		hash, _ := tx.GetHash()
		if _, exists := keep[hash]; !exists {
			result = append(result, tx)
		}
	}
	return result
}

func IntrinsicGas(data []byte, isContractCreation bool, homestead bool) uint64 {
	gas := uint64(21000) // 基础gas
	if isContractCreation {
		gas += 53000 // 合约创建额外gas
	}
	if len(data) > 0 {
		gas += uint64(len(data)) * 68 // 数据gas
	}
	return gas
}

// 辅助类型和函数
func NewEIP155Signer(chainId *big.Int) Signer {
	return &eip155Signer{chainId: chainId}
}

type eip155Signer struct {
	chainId *big.Int
}

func (s *eip155Signer) Sender(tx *Transaction) (common.Address, error) {
	return tx.GetSender()
}

// 扩展StateDB接口
func (s *ManagedState) SetNonce(addr common.Address, nonce uint64) {
	// 实现设置nonce的逻辑
}

func (s *ManagedState) GetNonce(addr common.Address) uint64 {
	// 实现获取nonce的逻辑
	return 0
}

func (s stateDB.StateDB) GetNonce(addr common.Address) uint64 {
	// 实现获取nonce的逻辑
	return 0
}

func (s stateDB.StateDB) GetBalance(addr common.Address) *big.Int {
	// 实现获取余额的逻辑
	return big.NewInt(0)
}

func (tx *Transaction) Size() int {
	// 实现获取交易大小的逻辑
	return 0
}

func (tx *Transaction) Cost() *big.Int {
	// 实现计算交易成本的逻辑
	return big.NewInt(0)
}
