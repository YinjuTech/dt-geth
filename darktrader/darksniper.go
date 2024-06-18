package darktrader

import (
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/status-im/keycard-go/hexutils"
)

type DarkSniper struct {
	pairs      map[string]*DTPair
	tokens     []string
	pairsMutex sync.RWMutex

	tokensByPair      map[string]string
	tokensByPairMutex sync.RWMutex

	config *DTConfig

	conf        *Configuration
	erc20       *Erc20
	uniswapv2   *UniswapV2
	scamchecker *ScamChecker

	bc    blockChain
	bcApi *ethapi.BlockChainAPI
	txApi *ethapi.TransactionAPI

	darkSlayer *DarkSlayer
	darkJumper *DarkJumper
}

func NewDarkSniper(conf *Configuration, erc20 *Erc20, uniswapv2 *UniswapV2, scamchecker *ScamChecker, bc blockChain, bcApi *ethapi.BlockChainAPI, txApi *ethapi.TransactionAPI, darkSlayer *DarkSlayer, darkJumper *DarkJumper) *DarkSniper {
	sniper := DarkSniper{}

	sniper.Init(conf, erc20, uniswapv2, scamchecker, bc, bcApi, txApi, darkSlayer, darkJumper)

	return &sniper
}

func (this *DarkSniper) SetConf(conf *Configuration) {
	this.conf = conf
}

func (this *DarkSniper) Init(conf *Configuration, erc20 *Erc20, uniswapv2 *UniswapV2, scamchecker *ScamChecker, bc blockChain, bcApi *ethapi.BlockChainAPI, txApi *ethapi.TransactionAPI, darkSlayer *DarkSlayer, darkJumper *DarkJumper) {
	this.conf = conf
	this.darkSlayer = darkSlayer
	this.darkJumper = darkJumper

	this.erc20 = erc20
	this.uniswapv2 = uniswapv2
	this.scamchecker = scamchecker

	this.bc = bc
	this.bcApi = bcApi
	this.txApi = txApi

	this.pairs = make(map[string]*DTPair)
	this.tokens = []string{}
	this.pairsMutex = sync.RWMutex{}

	this.tokensByPair = make(map[string]string)
	this.tokensByPairMutex = sync.RWMutex{}
}

func (this *DarkSniper) SetConfig(config *DTConfig) {
	this.config = config
}

func (this *DarkSniper) ShouldUnWatchPair(pair *DTPair, head *types.Block) (bool, float64) {
	if pair == nil {
		return true, 0
	}
	if pair.buyTriggered || pair.bought {
		priceIncreaseTimes, _ := pair.priceIncreaseTimes.Float64()
		return true, priceIncreaseTimes
	}

	if new(big.Int).Sub(head.Number(), pair.firstSwapBlkNo).Int64() >= int64(this.config.darkSniper.validBlockCount) {
		priceIncreaseTimes, _ := pair.priceIncreaseTimes.Float64()
		return true, priceIncreaseTimes
	}

	baseReserve, tokenReserve, err := this.erc20.GetPairReserves(pair.address, pair.baseToken, pair.token, rpc.LatestBlockNumber)
	if err != nil {
		LogFbBlr.Println("[DarkSniper] - [CalcLatestReserve]", err)
		return false, 0
	}

	if baseReserve.Cmp(big.NewInt(0)) == 0 || tokenReserve.Cmp(big.NewInt(0)) == 0 || baseReserve.Cmp(this.config.minEthReserve) < 0 {
		fmt.Println("[DarkSniper] - [CalcPendingReserve]", "LP removed/too small", ViewableEthAmount(baseReserve), "[T]", pair.token.Hex(), "(", pair.symbol, ")")
		return true, 0
	}
	if pair.initialBaseReserve.Cmp(this.config.maxEthReserve) > 0 {
		fmt.Println("[DarkSniper] - LP too large", ViewableEthAmount(baseReserve), "[T]", pair.token.Hex(), "(", pair.symbol, ")")
		return true, 0
	}

	// check initial price up
	curPrice := new(big.Float).Quo(
		new(big.Float).SetInt(baseReserve),
		new(big.Float).SetInt(tokenReserve),
	)

	priceUp, _ := new(big.Float).Quo(curPrice, pair.initialPrice).SetPrec(5).Float64()

	if priceUp > this.config.darkSniper.maxPriceIncreaseTimes {
		if !pair.isHiddenSniperToken || priceUp > this.config.darkSniper.buyCondition4Params.maxPriceIncreasedTimes || new(big.Int).Sub(head.Number(), pair.firstSwapBlkNo).Int64() > int64(this.config.darkSniper.buyCondition4Params.watchBlockCount) {
			return true, priceUp
		}
	} else {
		if pair.isHiddenSniperToken && new(big.Int).Sub(head.Number(), pair.firstSwapBlkNo).Int64() > int64(this.config.darkSniper.buyCondition4Params.watchBlockCount) {
			pair.isHiddenSniperToken = false
		}
	}

	pair.pairMutex.Lock()
	pair.tokenReserve = tokenReserve
	pair.baseReserve = baseReserve
	pair.pairMutex.Unlock()
	return false, 0
}
func (this *DarkSniper) CheckEventLogs(head *types.Block, blkLogs []*types.Log) {
	if this.config == nil || this.config.darkSniper == nil || !this.config.isRunning {
		return
	}
	// Remove pairs from the list

	tokensToRemove := make([][]interface{}, 0)
	for _, token := range this.tokens {
		pair, exists := this.pairs[token]
		if exists {

			if shouldRemove, priceIncreaseTimes := this.ShouldUnWatchPair(pair, head); shouldRemove {
				// [token, name, symbol, baseReserve, uniqSwap, totalSwap, missingSwap, priceIncreaseTimes, firstBlkNumber]
				tokensToRemove = append(tokensToRemove, []interface{}{pair.token.Hex(), pair.name, pair.symbol, pair.baseReserve, pair.buyerCount, pair.buyCount, pair.totalActualSwapCount, priceIncreaseTimes, pair.firstSwapBlkNo, pair.totalActualBotSwapCount})

				this.darkJumper.OnDetectNewToken(pair, false)

				this.RemovePair(pair)
			} else {
				pair.swapTxs.RemoveMinedAndOldTxs(this.txApi)
			}
		}
	}
	if len(tokensToRemove) > 0 {
		go PrintDarkSniperPairsToRemove(tokensToRemove)
	}
}

func (this *DarkSniper) RemovePair(pair *DTPair) {
	token := pair.token.Hex()

	if _, exists := this.pairs[token]; exists {
		this.pairsMutex.Lock()
		delete(this.pairs, token)

		index := 0
		for _, t := range this.tokens {
			if t != token {
				this.tokens[index] = t
				index++
			}
		}
		this.tokens = this.tokens[:index]
		this.pairsMutex.Unlock()
	}
}
func (this *DarkSniper) AddPairs(newPairs []*DTPair, head *types.Block) {
	if this.config == nil || this.config.darkSniper == nil || !this.config.isRunning {
		return
	}

	for _, pair := range newPairs {
		this.onDetectNewToken(pair, head)
	}
}

func (this *DarkSniper) checkSpecialBuyMode(pair *DTPair) bool {
	return CheckSpecialBuyMode(pair, this.config, false, this.conf, this.bc.CurrentHeader(), this.bcApi, this.erc20, this.scamchecker, this.txApi, this.darkSlayer)
}

func (this *DarkSniper) onDetectNewToken(pair *DTPair, head *types.Block) bool {
	if this.config == nil || this.config.darkSniper == nil || !this.config.isRunning {
		return false
	}

	if pair.buyTriggered || pair.bought {
		return false
	}

	baseReserve, tokenReserve, err := this.erc20.GetPairReserves(pair.address, pair.baseToken, pair.token, rpc.LatestBlockNumber)

	if err != nil {
		fmt.Println("[DarkSniper] - AddPair->CalcReserve", err)
		return false
	}

	if baseReserve.Cmp(big.NewInt(0)) == 0 || tokenReserve.Cmp(big.NewInt(0)) == 0 {
		return false
	}

	if pair.baseReserve.Cmp(big.NewInt(0)) == 0 || pair.tokenReserve.Cmp(big.NewInt(0)) == 0 {
		return false
	}

	if pair.initialPrice == nil || pair.initialPrice.Cmp(big.NewFloat(0)) == 0 {
		pair.initialPrice = new(big.Float).Quo(
			new(big.Float).SetInt(pair.baseReserve),
			new(big.Float).SetInt(pair.tokenReserve),
		)
	}
	curPrice := new(big.Float).Quo(
		new(big.Float).SetInt(baseReserve),
		new(big.Float).SetInt(tokenReserve),
	)
	priceUp, _ := new(big.Float).Quo(curPrice, pair.initialPrice).SetPrec(5).Float64()
	pair.sniperDetectionPriceUp = priceUp
	pair.baseReserve = baseReserve
	pair.tokenReserve = tokenReserve

	hiddenCheck := this.checkHiddenBuyConditions(pair)
	if !hiddenCheck {
		this.checkSpecialBuyMode(pair)
		if priceUp > this.config.darkSniper.maxPriceIncreaseTimes {
			// Price up too high
			LogFg.Println("[DarkSniper] - [Ignore T]: ", pair.token, pair.symbol, priceUp, ">", this.config.darkSniper.maxPriceIncreaseTimes)

			go this.darkJumper.OnDetectNewToken(pair, false)
			return false
		}
	} else {
		LogFgr.Println(fmt.Sprintf("[DarkSniper] - [Found Hidden Buy] %s(%s) - %.3f <= %.3f <= %.3f, # Pending %d =< %d, Mined %d >= %d", pair.symbol, pair.token.Hex(), this.config.darkSniper.buyCondition4Params.minPriceIncreasedTimes, pair.sniperDetectionPriceUp, this.config.darkSniper.buyCondition4Params.maxPriceIncreasedTimes, len(pair.swapTxs.txs), this.config.darkSniper.buyCondition4Params.maxDetectedPendingTxCount, pair.totalActualSwapCount, this.config.darkSniper.buyCondition4Params.minMinedTxCount))

		currentHead := this.bc.CurrentHeader()
		maxBuyAmount := big.NewInt(0).Add(big.NewInt(0), this.config.maxBuyAmount)
		big.NewFloat(0).Mul(new(big.Float).SetInt(maxBuyAmount), new(big.Float).SetFloat64(pair.sniperDetectionPriceUp)).Int(this.config.maxBuyAmount)
		fmt.Println("Max buy amount eth: ", ViewableEthAmount(this.config.maxBuyAmount))
		buyInfo := CheckTokenTxs(currentHead, pair, nil, this.bcApi, this.erc20, this.scamchecker, this.txApi, this.config, true)
		this.config.maxBuyAmount = maxBuyAmount
		if buyInfo != nil {
			this.buyV2(nil, pair, 0, buyInfo)
		} else {
			this.checkSpecialBuyMode(pair)
		}

		return false
	}

	if _, exists := this.pairs[pair.token.Hex()]; exists {
		LogFg.Println("[DarkSniper] - [Existing T]: ", pair.token, pair.symbol, priceUp, "<=", this.config.darkSniper.maxPriceIncreaseTimes)
		return false
	}
	this.pairsMutex.Lock()
	this.tokens = append(this.tokens, pair.token.Hex())
	this.pairs[pair.token.Hex()] = pair
	this.pairsMutex.Unlock()

	pair.pairMutex.Lock()
	pair.pendingPriceUp, _ = pair.priceIncreaseTimes.Float64()
	pair.pendingPriceUp = pair.pendingPriceUp - priceUp
	if pair.pendingPriceUp < 0 {
		pair.pendingPriceUp = 0
	}
	pair.pendingSwapCount = pair.buyCount - pair.totalActualSwapCount
	if pair.pendingSwapCount < 0 {
		pair.pendingSwapCount = 0
	}
	pair.buyCount = 0
	pair.buyerCount = 0
	pair.totalActualSwapCount = 0
	pair.totalActualBotSwapCount = 0
	pair.uniqBuyAddresses = make(map[string]string)
	pair.uniqSwapAmtIns = make(map[string]string)
	pair.uniqSwapAmtOuts = make(map[string]string)
	pair.buyHoldOn = false
	pair.isHiddenSniperToken = false
	pair.triggerTx = nil
	pair.swapTxs.RemoveMinedAndOldTxs(this.txApi)
	pair.pairMutex.Unlock()

	this.tokensByPairMutex.Lock()
	this.tokensByPair[pair.address.Hex()] = pair.token.Hex()
	this.tokensByPairMutex.Unlock()

	fmt.Println("[DarkSniper] - [Watching T]: ", pair.token, pair.symbol, priceUp, "<=", this.config.darkSniper.maxPriceIncreaseTimes, "Pending swaps: $ - ", pair.pendingPriceUp, "# - ", pair.pendingSwapCount)

	// if hiddenCheck {
	// 	pair.isHiddenSniperToken = true
	// 	this.startHoldingOnBuyHiddenTx(pair)
	// }
	return true
}

func (this *DarkSniper) startHoldingOnBuyHiddenTx(pair *DTPair) {
	if pair.buyHoldOn {
		return
	}
	currentHead := this.bc.CurrentHeader()

	pair.buyHoldOn = true

	holdOnDuration := GetTimeDurationLeftToNextBlockMined(currentHead, this.config.buyTriggerInSeconds)

	LogFgr.Println(fmt.Sprintf("[DarkSniper] - [HiddenBuy] - Hold on checking in %3fs for %s(%s), $ %.3f <= %.3f, # %d", holdOnDuration.Seconds(), pair.symbol, pair.token.Hex(), pair.priceIncreaseTimesInitial, this.config.darkSniper.buyCondition4Params.maxPriceIncreasedTimes, len(pair.swapTxs.txs)))

	go func(holdOnDuration time.Duration, pair *DTPair) {
		time.AfterFunc(holdOnDuration, func() {
			buyInfo := CheckTokenTxs(currentHead, pair, nil, this.bcApi, this.erc20, this.scamchecker, this.txApi, this.config, true)
			if buyInfo == nil {
				pair.buyHoldOn = false
				return
			}
			if len(pair.swapTxs.txs) >= this.config.darkSniper.buyCondition4Params.minSwapCount && pair.buyCount >= this.config.darkSniper.buyCondition4Params.minBuyCount {
				this.buy(nil, pair, 0, buyInfo)
			} else {
				pair.buyHoldOn = false
				fmt.Println("[DarkSniper] - [HiddenBuy] - Dismiss token - not meet condition", pair.symbol, pair.token.Hex())
				fmt.Println(fmt.Sprintf("   $: %.4f x %.04f, #: %d / %d / %d", pair.sniperDetectionPriceUp, pair.priceIncreaseTimes, len(pair.swapTxs.txs), pair.buyerCount, pair.buyCount))
			}
		})
	}(holdOnDuration, pair)
}

func (this *DarkSniper) ProcessSwapTx(token *common.Address, tx *types.Transaction, swapInfo *SwapParserResult) {
	if this.config == nil || this.config.darkSniper == nil || !this.config.isRunning {
		return
	}
	var pair *DTPair = nil

	if token != nil {
		this.pairsMutex.RLock()
		pair, _ = this.pairs[token.Hex()]
		this.pairsMutex.RUnlock()
	}

	if swapInfo.router == "Contract" {
		this.tokensByPairMutex.RLock()
		hexInput := strings.ToLower(hexutils.BytesToHex(tx.Data()))
		for address, token := range this.tokensByPair {
			addr := strings.ToLower(address[2:])
			if strings.Contains(hexInput, addr) {
				swapInfo.address = tx.To()
				pair = this.pairs[token]
				break
			}
			addr = strings.ToLower(token[2:])
			if strings.Contains(hexInput, addr) {
				swapInfo.address = tx.To()
				pair = this.pairs[token]
				break
			}
		}
		this.tokensByPairMutex.RUnlock()
	}

	if pair == nil || pair.buyTriggered || pair.bought {
		return
	}
	currentHead := this.bc.CurrentHeader()

	if pair.buyTriggered == false {
		nextBaseFee := CalcNextBaseFee(currentHead)
		pair.pairMutex.Lock()
		// defer pair.pairMutex.Unlock()
		pair.swapTxs.append(NewDarkTx(tx, nextBaseFee, swapInfo, currentHead.Number.Uint64()))
		pair.pairMutex.Unlock()
		if !pair.buyHoldOn {
			if pair.isHiddenSniperToken && len(pair.swapTxs.txs) >= this.config.darkSniper.buyCondition4Params.minSwapCount && pair.swapTxs.HasAtLeastSwap(1) {
				this.startHoldingOnBuyHiddenTx(pair)
			} else if !pair.isHiddenSniperToken && len(pair.swapTxs.txs) >= this.config.darkSniper.buyCondition3Params.minBuyCount && pair.swapTxs.HasAtLeastSwap(1) {
				pair.buyHoldOn = true
				holdOnDuration := GetTimeDurationLeftToNextBlockMined(currentHead, this.config.buyTriggerInSeconds)

				LogFgr.Println(fmt.Sprintf("[DarkSniper] - Hold on checking in %3fs for %s(%s), #=%d", holdOnDuration.Seconds(), pair.symbol, pair.token.Hex(), len(pair.swapTxs.txs)))

				go func(holdOnDuration time.Duration, tx *types.Transaction, pair *DTPair) {
					time.AfterFunc(holdOnDuration, func() {
						buyInfo := CheckTokenTxs(currentHead, pair, tx, this.bcApi, this.erc20, this.scamchecker, this.txApi, this.config, true)
						if buyInfo == nil {
							pair.buyHoldOn = false
							return
						}
						buyCondition := this.checkBuyConditions(pair)
						if buyCondition > 0 {
							this.buy(tx, pair, buyCondition, buyInfo)
						} else {
							pair.buyHoldOn = false
							fmt.Println("[DarkSniper] - Dismiss token - not meet condition", pair.symbol, pair.token.Hex())
							fmt.Println(fmt.Sprintf("   $: %.4f x %.04f, #: %d / %d / %d", pair.sniperDetectionPriceUp, pair.priceIncreaseTimes, len(pair.swapTxs.txs), pair.buyerCount, pair.buyCount))
						}
					})
				}(holdOnDuration, tx, pair)
			}
		}
	}
}

func (this *DarkSniper) onDetectTokenSwap(log *types.Log, receipt map[string]interface{}, tx *types.Transaction) bool {
	return true
}

func (this *DarkSniper) getPairByAddress(address *common.Address) *DTPair {
	this.tokensByPairMutex.RLock()
	token, exists := this.tokensByPair[address.Hex()]
	var pair *DTPair = nil
	if exists {
		pair, _ = this.pairs[token]
	}
	this.tokensByPairMutex.RUnlock()
	return pair
}
func (this *DarkSniper) checkBuyConditions(pair *DTPair) int {
	// blkDiff := int(new(big.Int).Sub(this.bc.CurrentHeader().Number, pair.firstSwapBlkNo).Int64())
	// [condition 1]: check if increase in price hits the threshold
	// if pair.priceIncreaseTimes.Cmp(big.NewFloat(dt.config.buyCondition1Params.minPriceIncreaseTimes)) >= 0 {
	// 	return true
	// }

	// [condition 2]: check if buy count hits the thresholds
	currentHead := this.bc.CurrentHeader()
	// if currentHead.Number.Cmp(pair.firstSwapBlkNo) == 0 && pair.priceIncreaseTimes.Cmp(big.NewFloat(this.config.darkSniper.buyCondition2Params.minPriceIncreaseTimes-pair.pendingPriceUp)) >= 0 && pair.buyCount+pair.pendingSwapCount >= this.config.darkSniper.buyCondition2Params.minBuyCount {
	// 	return 2
	// }

	if pair.sniperDetectionPriceUp > this.config.darkSniper.maxPriceIncreaseTimes {
		return 0
	}

	if currentHead.Number.Cmp(pair.firstSwapBlkNo) == 0 && new(big.Float).Mul(pair.priceIncreaseTimes, big.NewFloat(pair.sniperDetectionPriceUp)).Cmp(big.NewFloat(this.config.darkSniper.buyCondition2Params.minPriceIncreaseTimes)) >= 0 && pair.buyCount >= this.config.darkSniper.buyCondition2Params.minBuyCount {
		return 2
	}

	// [condition 3]: check if buyer count and buy count hit the thresholds
	if currentHead.Number.Cmp(pair.firstSwapBlkNo) > 0 && pair.priceIncreaseTimes.Cmp(big.NewFloat(this.config.darkSniper.buyCondition3Params.minPriceIncreaseTimes)) >= 0 && pair.buyerCount >= this.config.darkSniper.buyCondition3Params.minBuyerCount && pair.buyCount >= this.config.darkSniper.buyCondition3Params.minBuyCount && pair.buyCount-pair.buyerCount >= this.config.darkSniper.buyCondition3Params.minDiff {
		return 1
	}

	return 0
}

func (this *DarkSniper) checkHiddenBuyConditions(pair *DTPair) bool {
	currentHead := this.bc.CurrentHeader()
	if currentHead.Number.Int64() != pair.firstSwapBlkNo.Int64() {
		return false
	}
	if len(pair.swapTxs.txs) > this.config.darkSniper.buyCondition4Params.maxDetectedPendingTxCount || pair.totalActualSwapCount < this.config.darkSniper.buyCondition4Params.minMinedTxCount || pair.sniperDetectionPriceUp > this.config.darkSniper.buyCondition4Params.maxPriceIncreasedTimes || pair.sniperDetectionPriceUp < this.config.darkSniper.buyCondition4Params.minPriceIncreasedTimes {
		return false
	}
	return true
}

func (this *DarkSniper) buy(tx *types.Transaction, pair *DTPair, buyCondition int, buyInfo *TokenBuyInfo) {
	if pair.buyTriggered {
		return
	}
	pair.buyTriggered = true

	// calc time diff from latest block time
	currentHead := this.bc.CurrentHeader()
	latestBlkTimestamp := currentHead.Time
	diffInSeconds := time.Now().Sub(time.Unix(int64(latestBlkTimestamp), 0)).Seconds()
	priceUp, _ := pair.priceIncreaseTimes.Float64()
	LogFgr.Println(fmt.Sprintf("Buy triggered in %3f secs. [S#]: %d -> %d, $: %.3f [T]: %s(%s)", diffInSeconds, pair.buyerCount, pair.buyCount, priceUp, pair.token, pair.symbol))

	customSlippage := uint64(0)
	if pair.priceIncreaseTimesInitial < this.config.darkSniper.buyCondition2Params.minPriceIncreaseTimes {
		// marginPriceUp := 1 - (this.config.darkSniper.maxPriceIncreaseTimes-pair.sniperDetectionPriceUp)/(this.config.darkSniper.maxPriceIncreaseTimes-1)
		marginPriceUp := float64(pair.priceIncreaseTimesInitial-1) / float64(this.config.darkSniper.buyCondition2Params.minPriceIncreaseTimes-1)
		customSlippage = 50 + uint64(marginPriceUp*50)
		if customSlippage > 98 {
			customSlippage = 98
		}
	} else {
		// if buyInfo.buyCount > 2 { // if we have jumped into the mid, we limit the buy count
		// 	buyInfo.buyCount = 2
		// }
		customSlippage = 90
	}
	// BuyToken(pair, this.config, currentHead, this.scamchecker, this.txApi, tx, this.conf, this.config.darkSniper.canBuy, this.darkSlayer, customSlippage)
	BuyTokenV2(pair, this.config, currentHead, this.scamchecker, this.txApi, buyInfo, this.conf, this.config.darkSniper.canBuy, this.darkSlayer, customSlippage, 0)
}

func (this *DarkSniper) buyV2(tx *types.Transaction, pair *DTPair, buyCondition int, buyInfo *TokenBuyInfo) {
	if pair.buyTriggered {
		return
	}
	pair.buyTriggered = true

	// calc time diff from latest block time
	currentHead := this.bc.CurrentHeader()
	latestBlkTimestamp := currentHead.Time
	diffInSeconds := time.Now().Sub(time.Unix(int64(latestBlkTimestamp), 0)).Seconds()
	priceUp := pair.sniperDetectionPriceUp
	LogFgr.Println(fmt.Sprintf("Buy triggered in %3f secs. [S#]: %d -> %d, $: %.3f [T]: %s(%s)", diffInSeconds, pair.buyerCount, pair.buyCount, priceUp, pair.token, pair.symbol))

	customSlippage := uint64(0)
	customSlippage = uint64(50 * priceUp)
	// BuyToken(pair, this.config, currentHead, this.scamchecker, this.txApi, tx, this.conf, this.config.darkSniper.canBuy, this.darkSlayer, customSlippage)
	BuyTokenV2(pair, this.config, currentHead, this.scamchecker, this.txApi, buyInfo, this.conf, this.config.darkSniper.canBuy, this.darkSlayer, customSlippage, 1)
}
