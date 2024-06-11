package darktrader

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/rpc"
)

type DarkJumperConfig struct {
	isRunning                 bool
	canBuy                    bool
	minPriceDown              float64
	minPickPriceUp            float64
	swapRatio                 float64
	txCountConsiderBlockCount int
	minTxCount                int
	maxWatchingBlockCount     int
	buyHoldOnThres            int
	maxBuyAmountInEth         float64
	maxBuyCount               int
	buyFee                    float64 //gei in fee
	slippage                  float64
	totalTokenFee             float64
}

type DarkJumperBuyInfo struct {
	pickPriceUp   float64
	curPriceUp    float64
	aggBuyCount   int
	aggSelllCount int
	buyCount      int
	sellCount     int
}
type DJPair struct {
	pair             *DTPair
	trackInfo        map[uint64]*PairTrackInfo
	watchBlockNumber uint64
	jBuyInfo         *DarkJumperBuyInfo
}

type DarkJumper struct {
	pairs      map[string]*DJPair
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
}

func NewDarkJumper(conf *Configuration, erc20 *Erc20, uniswapv2 *UniswapV2, scamchecker *ScamChecker, bc blockChain, bcApi *ethapi.BlockChainAPI, txApi *ethapi.TransactionAPI, darkSlayer *DarkSlayer) *DarkJumper {
	jumper := DarkJumper{}

	jumper.Init(conf, erc20, uniswapv2, scamchecker, bc, bcApi, txApi, darkSlayer)

	return &jumper
}

func (this *DarkJumper) SetConf(conf *Configuration) {
	this.conf = conf
}

func (this *DarkJumper) SetConfig(config *DTConfig) {
	this.config = config
}

func (this *DarkJumper) Init(conf *Configuration, erc20 *Erc20, uniswapv2 *UniswapV2, scamchecker *ScamChecker, bc blockChain, bcApi *ethapi.BlockChainAPI, txApi *ethapi.TransactionAPI, darkSlayer *DarkSlayer) {
	this.conf = conf

	this.erc20 = erc20
	this.uniswapv2 = uniswapv2
	this.scamchecker = scamchecker

	this.bc = bc
	this.bcApi = bcApi
	this.txApi = txApi

	this.pairs = make(map[string]*DJPair)
	this.tokens = []string{}
	this.pairsMutex = sync.RWMutex{}

	this.tokensByPair = make(map[string]string)
	this.tokensByPairMutex = sync.RWMutex{}

	this.darkSlayer = darkSlayer
}

func (this *DarkJumper) checkBuyConditions(jpair *DJPair, head *types.Header) bool {

	jBuyInfo := DarkJumperBuyInfo{}

	totalBuyTxCountInPast := 0
	totalSellTxCountInPast := 0
	blockNumber := head.Number.Uint64()
	for i := uint64(0); i < uint64(this.config.darkJumper.txCountConsiderBlockCount); i++ {
		trackInfo, exists := jpair.trackInfo[blockNumber-i]
		if !exists {
			continue
		}
		totalBuyTxCountInPast = totalBuyTxCountInPast + trackInfo.buyCount
		totalSellTxCountInPast = totalSellTxCountInPast + trackInfo.sellCount
	}
	if totalBuyTxCountInPast+totalSellTxCountInPast < this.config.darkJumper.minTxCount {
		// LogFwBrOb.Println("[DarkJumper]: LP not active enough!", jpair.pair.symbol, totalBuyTxCountInPast, "+", totalSellTxCountInPast, "<", this.config.darkJumper.minTxCount)
		return false
	}
	jBuyInfo.aggBuyCount = totalBuyTxCountInPast
	jBuyInfo.aggSelllCount = totalSellTxCountInPast

	curTrackInfo, exists := jpair.trackInfo[blockNumber]
	if !exists || curTrackInfo == nil {
		return false
	}

	lastTrackInfo, exists := jpair.trackInfo[blockNumber-1]
	if !exists || lastTrackInfo == nil {
		return false
	}

	pricePick := new(big.Float).Quo(lastTrackInfo.price, jpair.pair.initialPrice)
	if pricePick.Cmp(big.NewFloat(this.config.darkJumper.minPickPriceUp)) < 0 {

		// LogFwBrOb.Println("[DarkJumper]: Price pick not enough!", jpair.pair.symbol, pricePick, " < ", this.config.darkJumper.minPickPriceUp)
		return false
	}

	jBuyInfo.pickPriceUp, _ = pricePick.Float64()

	if curTrackInfo.price.Cmp(big.NewFloat(0)) > 0 && new(big.Float).Quo(lastTrackInfo.price, curTrackInfo.price).Cmp(big.NewFloat(this.config.darkJumper.minPriceDown)) < 0 {
		// LogFwBrOb.Println("[DarkJumper]: Price down not enough!", jpair.pair.symbol, lastTrackInfo.price, "/", curTrackInfo.price, " < ", this.config.darkJumper.minPriceDown)
		return false
	}

	jBuyInfo.curPriceUp, _ = new(big.Float).Quo(curTrackInfo.price, jpair.pair.initialPrice).Float64()

	if lastTrackInfo.sellCount > 0 {
		swapRatio := float64(lastTrackInfo.buyCount) / float64(lastTrackInfo.sellCount)
		if swapRatio < this.config.darkJumper.swapRatio {
			// LogFwBrOb.Println("[DarkJumper]: Past block swap ratio is low!", jpair.pair.symbol, swapRatio, "<", this.config.darkJumper.swapRatio)
			return false
		}
	}

	jBuyInfo.buyCount = lastTrackInfo.buyCount
	jBuyInfo.sellCount = lastTrackInfo.sellCount

	CheckTokenTxsForJumper(head, jpair.pair, this.bcApi, this.erc20, this.scamchecker, this.txApi, this.config)

	totalFee := 100 - float64(100-jpair.pair.buyInfo.buyFeeInPercent)*float64(100-jpair.pair.buyInfo.sellFeeInPercent)/100
	if totalFee > this.config.darkJumper.totalTokenFee {
		LogFwBrOb.Println("[DarkJumper]: Token fee too high!", jpair.pair.symbol, jpair.pair.buyInfo.buyFeeInPercent, jpair.pair.buyInfo.sellFeeInPercent, "=", totalFee)
		return false
	}
	// if jpair.pair.buyInfo.buyCount == 0 {
	// 	LogFwBrOb.Println("[DarkJumper]: No wallets available!", jpair.pair.symbol, len(this.config.wallets))
	// 	return false
	// }

	jpair.jBuyInfo = &jBuyInfo

	return true
}
func (this *DarkJumper) CheckEventLogs(head *types.Block, blkLogs []*types.Log) {
	if this.config == nil || this.config.darkSniper == nil || !this.config.isRunning || !this.config.darkJumper.isRunning {
		return
	}
	// track info
	this.trackPairInfo(head, nil)
	// Remove pairs from the list
	tokensToRemove := make([][]interface{}, 0)
	this.pairsMutex.RLock()
	for _, token := range this.tokens {
		jpair, exists := this.pairs[token]
		if exists {
			if shouldRemove := this.ShouldUnWatchPair(jpair); shouldRemove {
				// [token, name, symbol, baseReserve, firstBlkNumber]
				tokensToRemove = append(tokensToRemove, []interface{}{jpair.pair.token.Hex(), jpair.pair.name, jpair.pair.symbol, jpair.pair.baseReserve, jpair.pair.firstSwapBlkNo})
			} else {

				if !jpair.pair.buyHoldOn && this.checkBuyConditions(jpair, head.Header()) {
					jpair.pair.buyHoldOn = true

					latestBlkTimestamp := head.Time()
					diffIn := time.Now().Sub(time.Unix(int64(latestBlkTimestamp), 0))

					var holdOnDuration time.Duration = 0
					buyTriggerInSeconds := this.config.darkJumper.buyHoldOnThres
					if buyTriggerInSeconds == 0 {
						buyTriggerInSeconds = 10
					}
					if diffIn.Milliseconds() < int64(buyTriggerInSeconds)*1000 {
						holdOnDuration = time.Duration(buyTriggerInSeconds)*time.Second - time.Duration(diffIn.Nanoseconds())
					}

					LogFgr.Println(fmt.Sprintf("[DarkJumper] - Hold on checking in %3fs for %s(%s)", holdOnDuration.Seconds(), jpair.pair.symbol, jpair.pair.token.Hex()))

					go func(holdOnDuration time.Duration, jpair *DJPair) {
						time.AfterFunc(holdOnDuration, func() {
							this.buy(jpair, jpair.pair.buyInfo)
						})
					}(holdOnDuration, jpair)
				}
			}
		}
	}
	this.pairsMutex.RUnlock()
	if len(tokensToRemove) > 0 {
		for _, tt := range tokensToRemove {
			jpair, _ := this.pairs[tt[0].(string)]
			this.RemovePair(jpair)
		}

		go PrintDarkJumperPairsToRemove(tokensToRemove)
	}
}

func (this *DarkJumper) ShouldUnWatchPair(jpair *DJPair) bool {
	// LP should be greater than the configured min LP value
	// Token has not been purchased
	// Block number has been passed enough
	head := this.bc.CurrentHeader()
	if jpair.watchBlockNumber+uint64(this.config.darkJumper.maxWatchingBlockCount) <= head.Number.Uint64() {
		LogFg.Println(fmt.Sprintf("[DarkJumper] - Ignore %s(%s) - Passed %d blocks from %d", jpair.pair.symbol, jpair.pair.token.Hex(), this.config.darkJumper.maxWatchingBlockCount, jpair.watchBlockNumber))
		return true
	}

	if jpair.pair.baseReserve.Cmp(this.config.minEthReserve) < 0 || jpair.pair.tokenReserve.Cmp(big.NewInt(0)) <= 0 {
		LogFg.Println(fmt.Sprintf("[DarkJumper] - Ignore %s(%s) - LP removed/too small %.3f", jpair.pair.symbol, jpair.pair.token.Hex(), ViewableEthAmount(jpair.pair.baseReserve)))
		return true
	}
	for _, account := range this.config.wallets {
		tokenAmount, err := this.erc20.callBalanceOf(&account.address, jpair.pair.token, rpc.LatestBlockNumber, nil)
		if err != nil {
			continue
		}
		if tokenAmount.Cmp(big.NewInt(0)) > 0 {
			LogFg.Println(fmt.Sprintf("[DarkJumper] - Ignore %s(%s) - Token already bought", jpair.pair.symbol, jpair.pair.token.Hex()))
			return true
		}
	}
	return false
}

func (this *DarkJumper) RemovePair(jpair *DJPair) {
	token := jpair.pair.token.Hex()

	if _, exists := this.pairs[token]; exists {
		jpair.pair.pairMutex.Lock()
		delete(this.pairs, token)

		index := 0
		for _, t := range this.tokens {
			if t != token {
				this.tokens[index] = t
				index++
			}
		}
		this.tokens = this.tokens[:index]
		jpair.pair.pairMutex.Unlock()
	}

	this.tokensByPairMutex.Lock()
	_, exists := this.tokensByPair[jpair.pair.address.Hex()]
	if exists {
		delete(this.tokensByPair, jpair.pair.address.Hex())
	}
	this.tokensByPairMutex.Unlock()
}

func (this *DarkJumper) OnDetectNewToken(pair *DTPair, force bool) {
	// Check whether the token is watchable
	if pair == nil || !this.config.darkJumper.isRunning {
		return
	}
	if _, exists := this.pairs[pair.token.Hex()]; exists {
		return
	}
	jpair := DJPair{
		pair:             pair,
		trackInfo:        make(map[uint64]*PairTrackInfo),
		watchBlockNumber: this.bc.CurrentHeader().Number.Uint64(),
	}
	jpair.pair.baseReserve, jpair.pair.tokenReserve, _ = this.erc20.GetPairReserves(pair.address, pair.baseToken, pair.token, rpc.LatestBlockNumber)

	if !force && this.ShouldUnWatchPair(&jpair) {
		return
	}
	if pair.initialPrice == nil {
		pair.initialPrice = new(big.Float).Quo(
			new(big.Float).SetInt(pair.baseReserve),
			new(big.Float).SetInt(pair.tokenReserve),
		)
	}

	var (
		baseReserve  *big.Int
		tokenReserve *big.Int
		err          error
	)
	if force {
		err = nil
		baseReserve = jpair.pair.baseReserve
		tokenReserve = jpair.pair.tokenReserve
	} else {
		baseReserve, tokenReserve, err = this.erc20.GetPairReserves(jpair.pair.address, jpair.pair.baseToken, jpair.pair.token, rpc.LatestBlockNumber)
	}

	if baseReserve.Cmp(big.NewInt(0)) == 0 || tokenReserve.Cmp(big.NewInt(0)) == 0 || err != nil {
		return
	}

	priceUp, _ := new(big.Float).Quo(
		new(big.Float).Quo(
			new(big.Float).SetInt(baseReserve),
			new(big.Float).SetInt(tokenReserve),
		),
		pair.initialPrice,
	).Float64()

	if force || priceUp >= this.config.darkSniper.maxPriceIncreaseTimes {
		fmt.Println(fmt.Sprintf("[DarkJumper] - [Watching T] - %s(%s), %.3f > %.3f", pair.symbol, pair.token.Hex(), priceUp, this.config.darkSniper.maxPriceIncreaseTimes))
		this.AddPair(&jpair)

		// trackInfo for past blocks
		head := this.bc.CurrentHeader()
		blockNumber := head.Number.Uint64()
		for i := uint64(0); i < uint64(this.config.darkJumper.txCountConsiderBlockCount); i++ {
			block := this.bc.GetBlockByNumber(blockNumber - i)
			this.trackPairInfo(block, &jpair)
		}
	} else {

	}
}

func (this *DarkJumper) AddPair(jpair *DJPair) {
	this.pairsMutex.Lock()
	this.tokens = append(this.tokens, jpair.pair.token.Hex())
	this.pairs[jpair.pair.token.Hex()] = jpair
	this.pairsMutex.Unlock()

	jpair.pair.pairMutex.Lock()
	jpair.pair.buyHoldOn = false
	jpair.pair.triggerTx = nil
	jpair.pair.pairMutex.Unlock()

	this.tokensByPairMutex.Lock()
	this.tokensByPair[jpair.pair.address.Hex()] = jpair.pair.token.Hex()
	this.tokensByPairMutex.Unlock()
}

func (this *DarkJumper) trackPairInfo(head *types.Block, jpairCheck *DJPair) {
	txs := head.Transactions()
	blockNumber := head.NumberU64()
	if jpairCheck == nil {
		this.pairsMutex.Lock()
		for _, jpair := range this.pairs {
			baseReserve, tokenReserve, err := this.erc20.GetPairReserves(jpair.pair.address, jpair.pair.baseToken, jpair.pair.token, rpc.LatestBlockNumber)
			if err != nil || tokenReserve.Cmp(big.NewInt(0)) == 0 {
				LogFg.Println(fmt.Sprintf("[DarkJumper] - Ignore %s(%s) - Failed to get reserve", jpair.pair.symbol, jpair.pair.token.Hex()), err)
				continue
			}
			jpair.pair.baseReserve = baseReserve
			jpair.pair.tokenReserve = tokenReserve
			price := new(big.Float).Quo(
				new(big.Float).SetInt(baseReserve),
				new(big.Float).SetInt(tokenReserve),
			)
			jpair.trackInfo[blockNumber] = &PairTrackInfo{
				buyCount:  0,
				sellCount: 0,
				price:     price,
			}
		}
		this.pairsMutex.Unlock()
	} else {
		baseReserve, tokenReserve, err := this.erc20.GetPairReserves(jpairCheck.pair.address, jpairCheck.pair.baseToken, jpairCheck.pair.token, rpc.LatestBlockNumber)
		if err != nil || tokenReserve.Cmp(big.NewInt(0)) == 0 {
		} else {
			price := new(big.Float).Quo(
				new(big.Float).SetInt(baseReserve),
				new(big.Float).SetInt(tokenReserve),
			)
			jpairCheck.trackInfo[blockNumber] = &PairTrackInfo{
				buyCount:  0,
				sellCount: 0,
				price:     price,
			}
		}
	}
	for _, tx := range txs {
		receipt, err := this.txApi.GetTransactionReceipt(context.Background(), tx.Hash())
		if err != nil {
			continue
		}
		txStatus, exists := receipt["status"]
		if exists && uint(txStatus.(hexutil.Uint)) != 1 {
			continue
		}
		if receipt["logs"] != nil {
			logs := receipt["logs"].([]*types.Log)

			for _, log := range logs {
				if len(log.Topics) == 3 && log.Topics[0].Hex() == uniswapSwapTopic {
					jpair := this.getPairByAddress(&log.Address)
					if jpair != nil && (jpairCheck == nil || jpairCheck.pair.token.Hex() == jpair.pair.token.Hex()) {
						isBuy, _, _, _, _ := this.scamchecker.uniswapv2.ParseSwapTx(log, receipt, jpair.pair, tx)
						if _, exists := jpair.trackInfo[blockNumber]; !exists {
							fmt.Println(blockNumber, jpair.trackInfo)
							continue
						}
						if isBuy {
							jpair.trackInfo[blockNumber].buyCount++
						} else {
							jpair.trackInfo[blockNumber].sellCount++
						}
						break
					}
				}
			}
		}
	}
}

func (this *DarkJumper) getPairByAddress(address *common.Address) *DJPair {
	this.tokensByPairMutex.RLock()
	token, exists := this.tokensByPair[address.Hex()]
	this.tokensByPairMutex.RUnlock()
	var pair *DJPair = nil
	if exists {
		pair, _ = this.pairs[token]
	}
	return pair
}

func (this *DarkJumper) buy(jpair *DJPair, buyInfo *TokenBuyInfo) {
	currentHead := this.bc.CurrentHeader()
	latestBlkTimestamp := currentHead.Time
	diffInSeconds := time.Now().Sub(time.Unix(int64(latestBlkTimestamp), 0)).Seconds()

	LogFgr.Println(fmt.Sprintf("[DarkJumper] Buy triggered in %.3f secs. [T]: %s(%s)", diffInSeconds, jpair.pair.token, jpair.pair.symbol))
	fmt.Println(fmt.Sprintf("  Pick price up: %.3f, cur price up: %.3f, downPercent: %d", jpair.jBuyInfo.pickPriceUp, jpair.jBuyInfo.curPriceUp, int(jpair.jBuyInfo.pickPriceUp*100/jpair.jBuyInfo.curPriceUp)), "%")
	fmt.Println(fmt.Sprintf("  Txs: Latest %d/%d, Agg %d/%d", jpair.jBuyInfo.buyCount, jpair.jBuyInfo.sellCount, jpair.jBuyInfo.aggBuyCount, jpair.jBuyInfo.aggSelllCount))
	fmt.Println(fmt.Sprintf("  Buy amount in: %.3f, slippage: %.3f, tip: %.3f gwei", ViewableEthAmount(jpair.pair.buyInfo.buyAmount), this.config.darkJumper.slippage, this.config.darkJumper.buyFee))

	this.BuyToken(jpair, buyInfo)

	jpair.pair.boughtBlkNo = currentHead.Number.Uint64()
	this.darkSlayer.AddPair(jpair.pair)
}

func (this *DarkJumper) BuyToken(jpair *DJPair, buyInfo *TokenBuyInfo) {
	currentHead := this.bc.CurrentHeader()
	blockNumber := currentHead.Number.Int64()

	nextBaseFee := CalcNextBaseFee(currentHead)

	path, _ := BuildSwapPath(jpair.pair)
	if int(buyInfo.buyCount) > len(this.config.wallets) {
		buyInfo.buyCount = uint(len(this.config.wallets))
	}
	slippages := []uint64{uint64(this.config.darkJumper.slippage)}
	slippageTips := []float64{this.config.darkJumper.buyFee}
	buyCounts := []uint{buyInfo.buyCount}


	if jpair.pair.origin != nil {
		fmt.Println("Origin", jpair.pair.origin.Hex(), "(", jpair.pair.originName, ")")
	}

	if isBlockedOrigin(jpair.pair, this.config) {
		LogFwBrOb.Println("Origin blocked!")
		return
	}

	for slippageIdx, slippagePercent := range slippages {
		tip := slippageTips[slippageIdx]

		var (
			boughtAccounts []string
			txBundles      []*TxBundle
			tips           []*big.Int = CalculateTxTips(tip, buyInfo.buyCount)
		)

		bundleCount := 1
		txBundles = make([]*TxBundle, bundleCount)
		for i := 0; i < bundleCount; i++ {
			txBundles[i] = &TxBundle{
				txs:           [][]byte{},
				blkNumber:     blockNumber + 1,
				blkCount:      1,
				revertableTxs: []common.Hash{},
			}
		}

		maxAmountIn := new(big.Int).Div(
			new(big.Int).Mul(
				buyInfo.buyAmount,
				big.NewInt(100),
			),
			new(big.Int).SetUint64(slippagePercent),
		)

		LogFgr.Println("[Max amounts in]:", maxAmountIn, "(", slippagePercent, "%)", "Provided Tip", tip)

		// var nonceForBribeTx uint64 = 0

		var maxGas uint64 = 500000

		maxSlippageAmountIn := big.NewInt(0).Add(maxAmountIn, maxAmountIn)

		for i := 0; i < int(buyCounts[slippageIdx]); i++ {
			account := this.config.wallets[i]
			// account := this.scamchecker.testWallets[i]

			nonce, err := this.txApi.GetTransactionCount(context.Background(), account.address, rpc.BlockNumberOrHashWithNumber(rpc.PendingBlockNumber))
			if err != nil {
				fmt.Println("Error in get nonce", err)
				continue
			}

			buyTxPayloadInput, err := this.scamchecker.uniswapv2.abi.Pack("swapETHForExactTokens", buyInfo.tokenAmountExpected, path, account.address, big.NewInt(time.Now().Unix()+200))
			if err != nil {
				fmt.Println("Error in pack buy input", err)
			}

			amountToSend := maxSlippageAmountIn
			if i == 0 {
				amountToSend = maxAmountIn
			}
			gasTip := tips[i]
			// include buy tx to the bundle
			for j := 0; j < len(txBundles); j++ {
				dfBuyTx := types.DynamicFeeTx{
					ChainID:   CHAINID,
					Nonce:     uint64(*nonce),
					GasTipCap: gasTip,
					GasFeeCap: big.NewInt(1).Add(nextBaseFee, gasTip),
					Gas:       maxGas,
					To:        &v2RouterAddrObj,
					Value:     amountToSend,
					Data:      buyTxPayloadInput,
				}
				buyTx := types.NewTx(types.TxData(&dfBuyTx))
				signedBuyTx, err := types.SignTx(buyTx, types.LatestSignerForChainID(buyTx.ChainId()), account.key)
				if err != nil {
					fmt.Println("Error in signing tx", err)
					break
				}

				buyTxPayload, _ := signedBuyTx.MarshalBinary()
				txBundles[j].txs = append(txBundles[j].txs, buyTxPayload)
			}

			boughtAccounts = append(boughtAccounts, account.szAddress)
		}

		// bundling other txs
		// for i := 0; i < len(txBundles); i++ {
		// 	for _, bundleTx := range buyInfo.txsToBundle {
		// 		otherTxPayload, _ := bundleTx.MarshalBinary()
		// 		txBundles[i].txs = append(txBundles[i].txs, otherTxPayload)
		// 		txBundles[i].revertableTxs = append(txBundles[i].revertableTxs, bundleTx.Hash())
		// 	}
		// }

		if !this.config.darkJumper.canBuy {
			fmt.Println("[DarkJumper] - Buy not enabled")
			return
		} else {
			go SendRawTransaction(txBundles)
		}
	}
	initial_price, _ := jpair.pair.initialPrice.Float64()
	price_up, _ := jpair.pair.priceIncreaseTimes.Float64()
	msg := PairBoughtEvent{
		Token0:         jpair.pair.token.Hex(),
		Token1:         jpair.pair.baseToken.Hex(),
		Pair:           jpair.pair.address.Hex(),
		Owner:          "",
		LiquidityOwner: "",
		TokenName:      jpair.pair.name,
		TokenSymbol:    jpair.pair.symbol,
		TokenDecimals:  jpair.pair.decimals,
		AdditionalInfo: PairBoughtEventAdditionalInfo{
			Accounts:     []string{},
			BlockNumber:  int(blockNumber),
			TotalBuyers:  jpair.pair.buyerCount,
			TotalSwaps:   jpair.pair.buyCount,
			InitialPrice: initial_price,
			PriceUp:      price_up,
			BuyFee:       buyInfo.buyFeeInPercent,
			SellFee:      buyInfo.sellFeeInPercent,
			AmountIn:     buyInfo.buyAmount,
			Tip:          slippageTips,
			Slippage:     slippages,
		},
	}
	if jpair.pair.owner != nil {
		msg.Owner = jpair.pair.owner.Hex()
	}
	if jpair.pair.liquidityOwner != nil {
		msg.LiquidityOwner = jpair.pair.liquidityOwner.Hex()
	}
	if this.conf != nil {
		szMsg, err := json.Marshal(msg)
		if err == nil {
			go this.conf.Publish("channel:token-bought", string(szMsg))
		}
		fmt.Println()

	}

	jpair.pair.bought = true
	jpair.pair.status = 1

	// this.RemovePair(jpair)
}
