package darktrader

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	ethUnit "github.com/DeOne4eg/eth-unit-converter"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm/runtime"
	"github.com/ethereum/go-ethereum/eth/tracers"
	"github.com/ethereum/go-ethereum/eth/tracers/logger"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/status-im/keycard-go/hexutils"
)

type blockChain interface {
	CurrentBlock() *types.Header

	GetBlockByNumber(number uint64) *types.Block

	StateAt(root common.Hash) (*state.StateDB, error)

	SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription

	CurrentHeader() *types.Header
}

type DTPairBoughtInfo struct {
	wallet         *DTAccount
	tx             *types.Transaction
	amountIn       *big.Int
	amountOut      *big.Int
	fee            *big.Int
	tokenAmount    *big.Int
	blockNumber    uint64
	routerApproved bool
}

type DTPair struct {
	token                     *common.Address // Target token
	baseToken                 *common.Address // Base token - WETH, USDT, USDC, ...
	address                   *common.Address
	owner                     *common.Address
	liquidityOwner            *common.Address
	origin                    *common.Address
	originName                string
	tokenReserve              *big.Int // Target token amount in pair
	baseReserve               *big.Int // Base token amount in pair
	status                    int      // 0 - Liquidity added, 1 - Bought, 2 - Closed
	wallets                   []*DTAccount
	rescueWallets             []*DTAccount
	name                      string
	symbol                    string
	buyHoldOn                 bool
	buyTriggered              bool
	bought                    bool
	decimals                  uint8
	totalSupply               *big.Int
	buyCount                  int
	buyerCount                int
	totalActualSwapCount      int // exclusive fails
	totalActualBotSwapCount   int
	firstSwapBlkNo            *big.Int
	boughtBlkNo               uint64
	maxSwapGasFee             *big.Int
	maxSwapGasTip             *big.Int
	uniqSwapAmtIns            map[string]string
	uniqSwapAmtOuts           map[string]string
	uniqBuyAddresses          map[string]string
	triggerTx                 *types.Transaction
	triggerTxMutex            sync.RWMutex
	swapTxs                   *DarkTxs
	priceIncreaseTimes        *big.Float // price increase times from the lastest block
	priceIncreaseTimesInitial float64    // price increase times from the initial
	initialPrice              *big.Float
	initialTokenReserve       *big.Int
	initialBaseReserve        *big.Int
	sniperDetectionPriceUp    float64
	swapDetectedInBlock       bool
	pendingTokenReserve       *big.Int
	pendingBaseReserve        *big.Int
	pendingPriceUp            float64
	pendingSwapCount          int
	pairMutex                 sync.RWMutex
	pairBoughtInfo            []*DTPairBoughtInfo
	autoSellEnabled           bool
	buyInfo                   *TokenBuyInfo
	isHiddenSniperToken       bool
	isMarkedByDegen			  bool
}

type DTAccount struct {
	address   common.Address
	szAddress string
	key       *ecdsa.PrivateKey
}

type BuyCondition1Params struct {
	minPriceIncreaseTimes float64
}

type BuyCondition2Params struct {
	minPriceIncreaseTimes float64
	minBuyCount           int
}

type BuyCondition3Params struct {
	minPriceIncreaseTimes float64
	minBuyCount           int
	minBuyerCount         int
	minDiff               int
}

type BuyCondition4Params struct {
	minPriceIncreaseTimes float64
	minSwapCount          int
	maxSwapCount          int
	minBuyCount           int
}

type DSBuyCondition4Params struct {
	// mined
	maxDetectedPendingTxCount int
	minMinedTxCount           int
	minPriceIncreasedTimes    float64
	maxPriceIncreasedTimes    float64
	// pending
	minSwapCount    int
	minBuyCount     int
	watchBlockCount int
}

type DarkSniperConfig struct {
	canBuy                bool
	maxPriceIncreaseTimes float64
	validBlockCount       int

	buyCondition1Params BuyCondition1Params
	buyCondition2Params BuyCondition2Params
	buyCondition3Params BuyCondition3Params
	buyCondition4Params DSBuyCondition4Params
}

type MultiTipModeConfig struct {
	slippage            uint64
	tip                 float64
	buyCountPercent     uint64
	maxBuyFee           uint64
	maxSellFee          uint64
	maxTotalFee         uint64
	modes               []uint64
	maxEthAmountPercent uint64
}

type SpecialModeConfig struct {
	enabled                bool
	minBuyFee              uint64
	maxBuyFee              uint64
	minSellFee             uint64
	maxSellFee             uint64
	minTotalFee            uint64
	maxTotalFee            uint64
	origins                []*common.Address
	maxPriceIncreasedTimes float64
	tip                    float64
	minEthReserve          *big.Int
	maxEthReserve          *big.Int
	minEthIn               *big.Int
	maxEthIn               *big.Int
	maxBuyCount            uint64
}
type DegenTraderConfig struct {
	enabled                bool
	contractAddress		   string
}
type DTConfig struct {
	minEthReserve              *big.Int
	maxEthReserve              *big.Int
	maxBuyTokenAmountInPercent int64 // percentage
	minBuyAmount               *big.Int
	maxBuyAmount               *big.Int // Max eth to pay per account
	maxBuyFeeInPercent         uint64   // token max buy fee
	maxSellFeeInPercent        uint64   // token max sell fee
	maxTotalFeeInPercent       uint64   // total token fee
	defaultBaseTipMax          float64  // default base tip
	defaultBaseTipMin          float64

	// buy conditions
	buyCondition1Params BuyCondition1Params
	buyCondition2Params BuyCondition2Params
	buyCondition3Params BuyCondition3Params
	buyCondition4Params BuyCondition4Params

	maxBuySlippageInPercent uint64 // Buy Slippage percent
	minBuySlippageInPercent uint64 // BUy Slippage percent minimum
	buySellCheckCount       uint8  // number of tests to buy and sell
	testAccountAddress      common.Address
	// Flags
	isRunning         bool
	canBuy            bool
	hideImportantLogs bool
	// Accounts
	wallets    []*DTAccount
	subWallets []*DTAccount

	// dark sniper config
	darkSniper *DarkSniperConfig
	darkSlayer *DarkSlayerConfig
	darkJumper *DarkJumperConfig

	buyTriggerInSeconds int // How long shall we wait for pending tx
	/*
		tipMode
		0 - use defaultBaseTipMax * 10 as fee for all
		1 - use dynamic tip mode
		2 - use tipMode 1 when met with BuyConditionParams1, otherwise use Tipmode 0
		3 - use [defaultBaseTipMax * 10 - baseFee] as tip for all
		4 - use multi tip mode
	*/
	tipMode        int
	multiTipModes  []MultiTipModeConfig
	specialModes   []SpecialModeConfig
	broadcastCount int
	blockedOrigins []*common.Address
	degenTraderConfig *DegenTraderConfig
}

type DarkTrader struct {
	tokens                            []string
	pairs                             map[string]*DTPair
	pairsMutex                        sync.RWMutex
	tokensByPair                      map[string]string
	tokensByPairMutex                 sync.RWMutex
	config                            *DTConfig
	abiUniswapRouterV2                abi.ABI
	abiUniswapFactoryV2               abi.ABI
	abiUniswapUniversalRouter         abi.ABI
	abiUniswapUniversalRouterInternal abi.ABI
	abiFlashbotsCheckAndSend          abi.ABI
	abiErc20                          abi.ABI
	abiWeth                           abi.ABI
	abiDarkTrader                     abi.ABI

	conf        *Configuration
	erc20       *Erc20
	uniswapv2   *UniswapV2
	scamchecker *ScamChecker
	swapparser  *SwapParser

	db        ethdb.Database
	bc        blockChain
	rtConfig  *runtime.Config
	rpcApis   []rpc.API
	tracerApi *tracers.API
	bcApi     *ethapi.BlockChainAPI
	txApi     *ethapi.TransactionAPI

	txCallConfig *tracers.TraceCallConfig

	txCache      map[string]bool
	txCacheMutex sync.RWMutex

	resumeEnabled bool

	darkSniper   *DarkSniper
	darkSlayer   *DarkSlayer
	darkJumper   *DarkJumper
	darkInfinite *DarkInfinite
}

func (dt *DarkTrader) Init(_bc blockChain, _db ethdb.Database, genesis *core.Genesis, _rpcApis []rpc.API, _tracerApi *tracers.API) {
	fmt.Println("Init DarkTrader...")

	// Default values
	dt.pairs = make(map[string]*DTPair)
	dt.pairsMutex = sync.RWMutex{}
	dt.tokensByPair = make(map[string]string)
	dt.tokensByPairMutex = sync.RWMutex{}

	// set abis
	dt.abiUniswapRouterV2, _ = abi.JSON(strings.NewReader(ABI_UNISWAP_ROUTER_V2))
	dt.abiUniswapFactoryV2, _ = abi.JSON(strings.NewReader(ABI_UNISWAP_FACTORY_V2))
	dt.abiErc20, _ = abi.JSON(strings.NewReader(ABI_ERC20))
	dt.abiWeth, _ = abi.JSON(strings.NewReader(ABI_WETH))
	dt.abiDarkTrader, _ = abi.JSON(strings.NewReader(ABI_DARKTRADER))
	dt.abiUniswapUniversalRouter, _ = abi.JSON(strings.NewReader(ABI_UNISWAP_UNIVERSAL_ROUTER))
	dt.abiUniswapUniversalRouterInternal, _ = abi.JSON(strings.NewReader(ABI_UNISWAP_UNIVERSAL_ROUTER_INTERNAL))
	dt.abiFlashbotsCheckAndSend, _ = abi.JSON(strings.NewReader(ABI_FLASHBOTS_CHECK_AND_SEND))

	// set config
	dt.bc = _bc
	dt.db = _db
	dt.rpcApis = _rpcApis
	dt.bcApi = _rpcApis[1].Service.(*ethapi.BlockChainAPI)
	dt.tracerApi = _tracerApi
	dt.txApi = _rpcApis[2].Service.(*ethapi.TransactionAPI)

	tracer := "callTracer"
	dt.txCallConfig = &tracers.TraceCallConfig{
		TraceConfig: tracers.TraceConfig{
			Tracer: &tracer,
			Config: &logger.Config{
				EnableReturnData: true,
				EnableMemory:     true,
				DisableStorage:   true,
			},
		},
	}

	dt.txCache = make(map[string]bool)
	dt.txCacheMutex = sync.RWMutex{}

	// helper functions
	dt.erc20 = NewErc20(dt.bcApi)
	dt.uniswapv2 = NewUniswapV2()
	dt.scamchecker = NewScamChecker(dt.config, dt.erc20, dt.uniswapv2, dt.bcApi)
	dt.swapparser = NewSwapParser(dt.erc20, dt.uniswapv2)
	// resume

	dt.darkSlayer = NewDarkSlayer(dt.conf, dt.erc20, dt.uniswapv2, dt.scamchecker, dt.bc, dt.bcApi, dt.txApi)
	dt.darkJumper = NewDarkJumper(dt.conf, dt.erc20, dt.uniswapv2, dt.scamchecker, dt.bc, dt.bcApi, dt.txApi, dt.darkSlayer)
	dt.darkSniper = NewDarkSniper(dt.conf, dt.erc20, dt.uniswapv2, dt.scamchecker, dt.bc, dt.bcApi, dt.txApi, dt.darkSlayer, dt.darkJumper)
	dt.darkInfinite = NewDarkInfinite(dt.bcApi)
	dt.initRedis()

	dt.resumeEnabled = true
	if dt.resumeEnabled {
		go dt.loadTokens()
	}
	go dt.darkInfinite.Loop()
}

func (dt *DarkTrader) loadTokens() {
	// blockNumber, tokens := ReadTokensUnderWatchLogFromFile()

	// if tokens != nil {
	// 	fmt.Println("Load ", len(tokens), " saved tokens")
	// 	for _, token := range tokens {
	// 		dt.onDetectNewToken(common.HexToAddress(token), nil, "")
	// 	}
	// }
	// blockNumber = 18182461
	// if blockNumber != 0 {
	// 	for {
	// 		block := dt.bc.GetBlockByNumber(blockNumber)

	// 		if block != nil {
	// 			dt.CheckEventLogs(block, []*types.Log{}, false)
	// 		}

	// 		if blockNumber >= dt.bc.CurrentHeader().Number.Uint64() {
	// 			break
	// 		}
	// 		blockNumber++
	// 	}
	// }
	dt.setToRedis("channel:tokens_read", "")
}

func (dt *DarkTrader) initRedis() {
	dt.conf = NewConfiguration(dt)

	dt.conf.Init()

	dt.darkSniper.SetConf(dt.conf)
	dt.darkSlayer.SetConf(dt.conf)
	dt.darkJumper.SetConf(dt.conf)
}

func (dt *DarkTrader) setToRedis(key string, payload string) {
	dt.conf.Publish(key, payload)
}

func (dt *DarkTrader) getFromRedis(key string) string {
	return ""
}

func (dt *DarkTrader) start() {
	fmt.Println("DarkTrader has started...")

	dt.pairs = make(map[string]*DTPair)
}

func (dt *DarkTrader) stop() {
	fmt.Println("DarkTrader has stopped...")
}

func (dt *DarkTrader) setConfig(config *DTConfig) {
	fmt.Println("DarkTrader configuration has been updated")

	tipModeName := "Unknown"
	if config.tipMode == 1 {
		tipModeName = "Dynamic mode"
	} else if config.tipMode == 2 {
		tipModeName = "Combined mode"
	} else if config.tipMode == 3 {
		tipModeName = "Saving mode(fixed tip - baseFee)"
	} else if config.tipMode == 4 {
		tipModeName = "Multi mode"
	} else {
		tipModeName = fmt.Sprintf("Fixed mode %d gwei", int(config.defaultBaseTipMax*10))
		config.tipMode = 0
	}
	fmt.Println("          minEthReserve", ethUnit.NewWei(config.minEthReserve).Ether(), "maxEthReserve", ethUnit.NewWei(config.maxEthReserve).Ether())
	fmt.Println("          buyAmount", config.maxBuyTokenAmountInPercent, " %")
	fmt.Println("          buyAmount", ethUnit.NewWei(config.minBuyAmount).Ether(), "~", ethUnit.NewWei(config.maxBuyAmount).Ether())
	fmt.Println("          buySellCheckCount", config.buySellCheckCount)
	fmt.Println("          mempool broadcast #", config.broadcastCount)
	fmt.Println("          fee - buy ", config.maxBuyFeeInPercent, " sell ", config.maxSellFeeInPercent, " total", config.maxTotalFeeInPercent)
	fmt.Println("          tipMode ", tipModeName)
	if config.tipMode == 4 {
		for _, tipMode := range config.multiTipModes {
			modeStr := ""
			for _, mode := range tipMode.modes {
				if modeStr != "" {
					modeStr = modeStr + ", "
				}
				modeStr = modeStr + fmt.Sprintf("%d", mode)
			}
			fmt.Println(fmt.Sprintf("             %d%%/%.1f gwei x %d%% wallets, fee: %d%%+%d%%=%d%%, modes: %s, maxEthIn: %d%%", tipMode.slippage, tipMode.tip, tipMode.buyCountPercent, tipMode.maxBuyFee, tipMode.maxSellFee, tipMode.maxTotalFee, "["+modeStr+"]", tipMode.maxEthAmountPercent))
		}
	}
	// fmt.Println("          Special buy mode")
	// for spIdx, specialMode := range config.specialModes {
	// 	originSr := ""
	// 	for _, sr := range specialMode.origins {
	// 		if originSr != "" {
	// 			originSr = originSr + ", "
	// 		}
	// 		originSr = originSr + sr.Hex()
	// 	}
	// 	fmt.Println(fmt.Sprintf("             %d - Fee: %d~%d + %d~%d = %d~%d, $: %.3f, reserve: %.3f~%.3f, buyAmount: %.3f~%.3f, buyCount: %d%%, tip: %.3f gwei", spIdx+1, specialMode.minBuyFee, specialMode.maxBuyFee, specialMode.minSellFee, specialMode.maxSellFee, specialMode.minTotalFee, specialMode.maxTotalFee, specialMode.maxPriceIncreasedTimes, ViewableEthAmount(specialMode.minEthReserve), ViewableEthAmount(specialMode.maxEthReserve), ViewableEthAmount(specialMode.minEthIn), ViewableEthAmount(specialMode.maxEthIn), specialMode.maxBuyCount, specialMode.tip))
	// 	fmt.Println("                 origins: ", originSr)
	// 	if specialMode.enabled {
	// 		fmt.Println("                 Enabled")
	// 	} else {
	// 		fmt.Println("                 Not enabled")
	// 	}
	// }
	fmt.Println("          buyHoldOnThres ", config.buyTriggerInSeconds)
	fmt.Println("          slippage - buy ", config.minBuySlippageInPercent, "/", config.maxBuySlippageInPercent)
	fmt.Println("          defaultBaseTip", config.defaultBaseTipMin, "/", config.defaultBaseTipMax, "gwei")
	fmt.Println("          buyCondition1", "$:", config.buyCondition1Params.minPriceIncreaseTimes)
	fmt.Println("          buyCondition2", "$:", config.buyCondition2Params.minPriceIncreaseTimes, "#:", config.buyCondition2Params.minBuyCount)
	fmt.Println("          buyCondition3", "$:", config.buyCondition3Params.minPriceIncreaseTimes, "#[BUY,BYR,DIF]:", config.buyCondition3Params.minBuyCount, config.buyCondition3Params.minBuyerCount, config.buyCondition3Params.minDiff)
	fmt.Println("          buyCondition4", "$:", config.buyCondition4Params.minPriceIncreaseTimes, "#[SWP/BUY]:", config.buyCondition4Params.minSwapCount, "~", config.buyCondition4Params.maxSwapCount, config.buyCondition4Params.minBuyCount)
	fmt.Println("          isRunning", config.isRunning)
	fmt.Println("          canBuy", config.canBuy)
	fmt.Println("          blocked Origins", len(config.blockedOrigins))
	for i, blockedOrigin := range config.blockedOrigins {
		fmt.Println("            ", i+1, blockedOrigin.Hex())
	}
	fmt.Println("         - - - - - Degen Trader Config - - - - - ")
	fmt.Println("            enabled", config.degenTraderConfig.enable)
	fmt.Println("            address", config.degenTraderConfig.contractAddress)
	fmt.Println("          - - - - - DarkSniper Config - - - - -")
	fmt.Println("            canBuy", config.darkSniper.canBuy)
	fmt.Println("            validMaxPrice", config.darkSniper.maxPriceIncreaseTimes)
	fmt.Println("            validBlockCount", config.darkSniper.validBlockCount)
	fmt.Println("            buyCondition1", "$:", config.darkSniper.buyCondition1Params.minPriceIncreaseTimes)
	fmt.Println("            buyCondition2", "$:", config.darkSniper.buyCondition2Params.minPriceIncreaseTimes, "#", config.darkSniper.buyCondition2Params.minBuyCount)
	fmt.Println("            buyCondition3", "$:", config.darkSniper.buyCondition3Params.minPriceIncreaseTimes, "#[SWP,BYR,DIF]", config.darkSniper.buyCondition3Params.minBuyCount, config.darkSniper.buyCondition3Params.minBuyerCount, config.darkSniper.buyCondition3Params.minDiff)
	fmt.Println("            buyCondition4", "$: ", config.darkSniper.buyCondition4Params.minPriceIncreasedTimes, "~", config.darkSniper.buyCondition4Params.maxPriceIncreasedTimes, "#[PENDING,MINED]", config.darkSniper.buyCondition4Params.maxDetectedPendingTxCount, config.darkSniper.buyCondition4Params.minMinedTxCount, "#[SWP, BUY]", config.darkSniper.buyCondition4Params.minSwapCount, config.darkSniper.buyCondition4Params.minBuyCount, "[BLK]:", config.darkSniper.buyCondition4Params.watchBlockCount)
	fmt.Println("          - - - - - - - - - -  - - - - - - - -")
	fmt.Println("          - - - - - DarkSlayer Config - - - - -")
	fmt.Println("            AddBot", config.darkSlayer.isAddBotEnabled)
	fmt.Println("            LpRemoval", config.darkSlayer.isRemoveLpEnabled)
	fmt.Println("            Others", config.darkSlayer.isOtherEnabled)
	fmt.Println("            ScamActionTxMinFee", config.darkSlayer.scamTxMinFeeInGwei, "gwei")
	fmt.Println("            watchBlockCount", config.darkSlayer.watchBlockCount)
	fmt.Println("          - - - - - - - - - -  - - - - - - - -")
	// fmt.Println("          - - - - - DarkJumper Config - - - - -")
	// fmt.Println("            isRunning", config.darkJumper.isRunning)
	// fmt.Println("            canBuy", config.darkJumper.canBuy)
	// fmt.Println("            minPriceDown", config.darkJumper.minPriceDown)
	// fmt.Println("            minPickPriceUp", config.darkJumper.minPickPriceUp)
	// fmt.Println("            swapRatio", config.darkJumper.swapRatio)
	// fmt.Println("            txCountConsiderBlockCount", config.darkJumper.txCountConsiderBlockCount, "(", config.darkJumper.txCountConsiderBlockCount/5, "min)")
	// fmt.Println("            minTxCount", config.darkJumper.minTxCount)
	// fmt.Println("            maxWatchingBlockCount", config.darkJumper.maxWatchingBlockCount)
	// fmt.Println("            buyHoldOnThres", config.darkJumper.buyHoldOnThres)
	// fmt.Println("            maxBuyAmountInEth", config.darkJumper.maxBuyAmountInEth)
	// fmt.Println("            maxBuyCount", config.darkJumper.maxBuyCount)
	// fmt.Println("            buyFee", config.darkJumper.buyFee)
	// fmt.Println("            slippage", config.darkJumper.slippage)
	// fmt.Println("            totalTokenFee", config.darkJumper.totalTokenFee)
	// fmt.Println("          - - - - - - - - - -  - - - - - - - -")
	fmt.Println("          - - - - - Wallets - - - - -")
	for i, addr := range config.wallets {
		fmt.Println("          ", i, addr.szAddress)
	}
	fmt.Println("          - - - - - - - - - -  - - - - - - - -")
	fmt.Println("          - - - - - Sub Wallets - - - - -")
	for i, addr := range config.subWallets {
		fmt.Println("          ", i, addr.szAddress)
	}
	fmt.Println("          - - - - - - - - - -  - - - - - - - -")

	dt.config = config

	dt.darkSniper.SetConfig(config)
	dt.darkSlayer.SetConfig(config)
	dt.darkJumper.SetConfig(config)

	if (dt.config == nil || dt.config.isRunning) && !config.isRunning {
		dt.stop()
	} else if (dt.config == nil || !dt.config.isRunning) && config.isRunning {
		dt.start()
	}
	if (dt.config == nil || dt.config.isRunning) && !config.isRunning {
		dt.stop()
	} else if (dt.config == nil || !dt.config.isRunning) && config.isRunning {
		dt.start()
	}
}

func (dt *DarkTrader) ProcessTxs(txs []*types.Transaction) {
	if dt.config == nil || !dt.config.isRunning {
		return
	}
	for _, tx := range txs {
		dt.processTx(tx)()
	}
}

func (dt *DarkTrader) processPendingSwap(tx *types.Transaction, pair *DTPair, swapInfo *SwapParserResult) bool {
	currentHead := dt.bc.CurrentHeader()

	if !pair.buyTriggered && pair.firstSwapBlkNo == nil {
		nextBaseFee := CalcNextBaseFee(currentHead)
		pair.pairMutex.Lock()
		// defer pair.pairMutex.Unlock()
		pair.swapTxs.append(NewDarkTx(tx, nextBaseFee, swapInfo, currentHead.Number.Uint64()))
		pair.pairMutex.Unlock()
		dt.checkPairTrigger(pair, tx)
	}
	return true
}

func (dt *DarkTrader) checkPairTrigger(pair *DTPair, tx *types.Transaction) {
	currentHead := dt.bc.CurrentHeader()

	minSwapCount := dt.config.buyCondition3Params.minBuyCount
	if minSwapCount > dt.config.buyCondition2Params.minBuyCount {
		minSwapCount = dt.config.buyCondition2Params.minBuyCount
	}
	if minSwapCount > dt.config.buyCondition4Params.minSwapCount {
		minSwapCount = dt.config.buyCondition4Params.minSwapCount
	}
	if !pair.buyHoldOn && len(pair.swapTxs.txs) >= minSwapCount && pair.swapTxs.HasAtLeastSwap(1) {
		pair.buyHoldOn = true

		var holdOnDuration time.Duration = GetTimeDurationLeftToNextBlockMined(currentHead, dt.config.buyTriggerInSeconds)

		LogFgr.Println(fmt.Sprintf("Hold on checking in %3fs for %s(%s)", holdOnDuration.Seconds(), pair.symbol, pair.token.Hex()))

		go func(holdOnDuration time.Duration, tx *types.Transaction, pair *DTPair) {
			time.AfterFunc(holdOnDuration, func() {
				dt.updatePairInfo(pair)
				buyInfo := CheckTokenTxs(currentHead, pair, tx, dt.bcApi, dt.erc20, dt.scamchecker, dt.txApi, dt.config, true)
				if buyInfo == nil {
					pair.buyHoldOn = false
					dt.checkSpecialBuyMode(pair)
					return
				}
				if dt.checkBuyConditions(pair) {
					dt.buy(tx, pair, buyInfo)
				} else {
					pair.buyHoldOn = false
					fmt.Println("Dismiss token - not meet condition", pair.symbol, pair.token.Hex())
					fmt.Println(fmt.Sprintf("   $: %.04f, #: %d / %d / %d", pair.priceIncreaseTimes, len(pair.swapTxs.txs), pair.buyerCount, pair.buyCount))
					dt.checkSpecialBuyMode(pair)
				}
			})
		}(holdOnDuration, tx, pair)
	} else {
		dt.checkSpecialBuyMode(pair)
	}
}

func (dt *DarkTrader) checkSpecialBuyMode(pair *DTPair) bool {
	return CheckSpecialBuyMode(pair, dt.config, true, dt.conf, dt.bc.CurrentHeader(), dt.bcApi, dt.erc20, dt.scamchecker, dt.txApi, dt.darkSlayer)
}

func (dt *DarkTrader) checkBuyConditions(pair *DTPair) bool {
	// [condition 1]: check if increase in price hits the threshold
	if pair.priceIncreaseTimes.Cmp(big.NewFloat(dt.config.buyCondition1Params.minPriceIncreaseTimes)) >= 0 {
		return true
	}

	// [condition 2]: check if buy count hits the thresholds
	if pair.priceIncreaseTimes.Cmp(big.NewFloat(dt.config.buyCondition2Params.minPriceIncreaseTimes)) >= 0 && pair.buyCount >= dt.config.buyCondition2Params.minBuyCount {
		return true
	}

	// [condition 3]: check if buyer count and buy count hit the thresholds
	// if pair.priceIncreaseTimes.Cmp(big.NewFloat(dt.config.buyCondition3Params.minPriceIncreaseTimes)) >= 0 && pair.buyerCount >= dt.config.buyCondition3Params.minBuyerCount && pair.buyCount >= dt.config.buyCondition3Params.minBuyCount && pair.buyCount-pair.buyerCount >= dt.config.buyCondition3Params.minDiff {
	// 	return true
	// }

	if pair.priceIncreaseTimes.Cmp(big.NewFloat(dt.config.buyCondition4Params.minPriceIncreaseTimes)) >= 0 && len(pair.swapTxs.txs) >= dt.config.buyCondition4Params.minSwapCount && len(pair.swapTxs.txs) <= dt.config.buyCondition4Params.maxSwapCount && pair.buyCount >= dt.config.buyCondition4Params.minBuyCount {
		return true
	}

	return false
	// return true
}

func (dt *DarkTrader) parsePendingSwapTx(tx *types.Transaction) bool {
	result := dt.swapparser.ProcessRouterPendingTx(tx)
	if result == nil {
		return false
	}
	var pair *DTPair = nil
	if len(result.path) == 2 && result.path[0].Hex() == WETH_ADDRESS.Hex() {
		token := result.path[1].Hex()
		pair, _ = dt.pairs[token]
	} else {
		return false
	}

	if pair != nil {
		return dt.processPendingSwap(tx, pair, result)
	}

	go dt.darkSniper.ProcessSwapTx(&result.path[len(result.path)-1], tx, result)

	return false
}

func (dt *DarkTrader) parseInternalPendingSwapTx(tx *types.Transaction) bool {
	data := tx.Data()

	if len(data) < 4 {
		return false
	}

	methodSig := data[:4]
	if bytes.Equal(methodSig, dt.abiErc20.Methods["approve"].ID) || bytes.Equal(methodSig, dt.abiErc20.Methods["transferFrom"].ID) || bytes.Equal(methodSig, dt.abiErc20.Methods["transfer"].ID) || bytes.Equal(methodSig, dt.abiErc20.Methods["transferOwnership"].ID) {
		return false
	}

	var botContract *common.Address = nil
	var pair *DTPair = nil
	currentHead := dt.bc.CurrentHeader()
	hexInput := strings.ToLower(hexutils.BytesToHex(data))
	dt.tokensByPairMutex.RLock()
	for address, token := range dt.tokensByPair {
		addr := strings.ToLower(address[2:])
		if strings.Contains(hexInput, addr) {
			botContract = tx.To()
			pair = dt.pairs[token]
			break
		}
		addr = strings.ToLower(token[2:])
		if strings.Contains(hexInput, addr) {
			botContract = tx.To()
			pair = dt.pairs[token]
			break
		}
	}
	dt.tokensByPairMutex.RUnlock()

	swapInfo := &SwapParserResult{
		address:         botContract,
		router:          "Contract",
		routerAddr:      nil,
		path:            nil,
		amtIn:           nil,
		amtOut:          nil,
		maxAmountIn:     nil,
		priceMax:        nil,
		exactAmountType: -1,
		swapParams:      nil,
	}

	if pair == nil {
		go dt.darkSniper.ProcessSwapTx(nil, tx, swapInfo)
		return false
	}
	if botContract.Hex() == v3RouterAddr.Hex() {
		// we don't handle univ3 router for now
		pair.firstSwapBlkNo = currentHead.Number
		return true
	}
	return dt.processPendingSwap(tx, pair, swapInfo)
}

func (dt *DarkTrader) processPendingTxForPair(tx *types.Transaction) bool {

	data := tx.Data()
	if len(data) < 4 {
		return false
	}

	methodSig := data[:4]
	if bytes.Equal(methodSig, dt.abiErc20.Methods["approve"].ID) || bytes.Equal(methodSig, dt.abiErc20.Methods["transferFrom"].ID) || bytes.Equal(methodSig, dt.abiErc20.Methods["transfer"].ID) || bytes.Equal(methodSig, dt.abiErc20.Methods["transferOwnership"].ID) {
		return false
	}

	pair, exists := dt.pairs[tx.To().Hex()]
	if exists {
		pair.triggerTx = tx
	}

	// try tx
	// pair.triggerTxMutex.Lock()
	// _, err := dt.erc20.CallTx(tx, nil, rpc.PendingBlockNumber, true)
	// // pair.triggerTxMutex.Unlock()
	// if err != nil {
	// 	return false
	// }

	return false
}

func (dt *DarkTrader) isNewLiquidity(token common.Address) bool {
	pairAddr, _ := CalculatePoolAddressV2(WETH_ADDRESS.Hex(), token.Hex())

	input, err := dt.uniswapv2.abiPair.Pack("kLast")
	if err != nil {
		fmt.Println("[DS] - isNewLiquidity - packKLast", err, pairAddr.Hex())
		return false
	}

	hexInput := hexutil.Bytes(input)

	args := ethapi.TransactionArgs{
		To:    &pairAddr,
		Input: &hexInput,
	}
	blockNo := rpc.BlockNumberOrHashWithNumber(rpc.LatestBlockNumber)
	output, err := dt.bcApi.Call(context.Background(), args, &blockNo, nil, nil)
	if err != nil {
		fmt.Println("[DS] - isNewLiquidity - call", err, pairAddr.Hex())
		return false
	}

	res1, err := dt.uniswapv2.abiPair.Methods["kLast"].Outputs.Unpack(output)
	if err != nil {
		fmt.Println("[DS] - isNewLiquidity - unpackKLast", err, pairAddr.Hex())
		fmt.Println("[DS] - ", output)
		return false
	}

	if len(res1) == 0 {
		fmt.Println("[DS] - isNewLiquidity - res is empty", pairAddr.Hex())
		fmt.Println("[DS] - ", output)
		return false
	}

	kLast := res1[0].(*big.Int)

	if kLast == nil {
		fmt.Println("[DS] - isNewLiquidity - res is nil", pairAddr.Hex())
		fmt.Println("[DS] - ", output)
		return false
	}
	if kLast.Cmp(big.NewInt(0)) == 0 {
		return true
	}

	return false
}

// check if pending tx is to add liqudity
func (dt *DarkTrader) processPendingAddLiquidity(tx *types.Transaction) bool {
	var (
		pair   *DTPair
		exists bool = false
	)

	token, baseToken, tokenReserve, baseReserve, err := dt.uniswapv2.ParseAddLiquidity(tx, dt.erc20, rpc.LatestBlockNumber)

	if err != nil || token == nil {
		if err != nil {
			if token != nil {
				fmt.Println("[Pending LP] reserve error", token.Hex(), err)
			} else {
				fmt.Println("[Pending LP] reserve error", err)
			}
		}
		return false
	}
	if pair, exists = dt.pairs[token.Hex()]; !exists {
		return false
	}
	pair.token = token
	pair.baseToken = baseToken
	pair.tokenReserve = tokenReserve
	pair.baseReserve = baseReserve
	pair.initialTokenReserve = tokenReserve
	pair.initialBaseReserve = baseReserve

	pair.triggerTx = tx

	dt.checkPairTrigger(pair, nil)

	fmt.Println("[Pending LP]:", pair.token, pair.symbol, pair.tokenReserve, ViewableEthAmount(pair.baseReserve), tx.Hash().Hex())

	return true
}

func (dt *DarkTrader) processMinedAddLiquidity(pair *DTPair, tx *types.Transaction, blockNumber rpc.BlockNumber) bool {
	token, baseToken, tokenReserve, baseReserve, err := dt.uniswapv2.ParseAddLiquidity(tx, dt.erc20, blockNumber-1)
	if err != nil || token == nil || token.Hex() != pair.token.Hex() || baseToken == nil || baseToken.Hex() != pair.baseToken.Hex() {
		return false
	}

	pair.tokenReserve = tokenReserve
	pair.baseReserve = baseReserve
	pair.initialTokenReserve = tokenReserve
	pair.initialBaseReserve = baseReserve
	fmt.Println("[Mined LP]:", pair.token, pair.symbol, pair.tokenReserve, ViewableEthAmount(pair.baseReserve), tx.Hash().Hex())

	return true
}

// check if pending tx is to open trading
func (dt *DarkTrader) processPendingOpenTrading(tx *types.Transaction) bool {
	if !dt.uniswapv2.IsOpenTrading(tx) {
		return false
	}

	pair, exists := dt.pairs[tx.To().Hex()]
	if !exists {
		if dt.onDetectNewToken(*tx.To(), nil, "") {
			pair, exists = dt.pairs[tx.To().Hex()]
			if !exists {
				return false
			}
		} else {
			return false
		}
	}

	txs := []*types.Transaction{tx}
	baseReserve, tokenReserve, err, _ := dt.erc20.GetPairReservesAfterTxs(
		pair.address,
		pair.baseToken,
		pair.token,
		txs,
		rpc.LatestBlockNumber,
		false,
	)
	if err != nil {
		return false
	}
	pair.tokenReserve = tokenReserve
	pair.baseReserve = baseReserve
	pair.initialTokenReserve = tokenReserve
	pair.initialBaseReserve = baseReserve

	pair.triggerTx = tx

	dt.checkPairTrigger(pair, nil)

	fmt.Println("[Pending OpenT]", pair.token, pair.symbol, pair.tokenReserve, ViewableEthAmount(pair.baseReserve))

	return true
}

func (dt *DarkTrader) processMinedOpenTrading(pair *DTPair, tx *types.Transaction, blockNumber rpc.BlockNumber) bool {
	// if !dt.uniswapv2.IsOpenTrading(tx) {
	// 	return false
	// }

	txs := []*types.Transaction{tx}
	baseReserve, tokenReserve, err, _ := dt.erc20.GetPairReservesAfterTxs(
		pair.address,
		pair.baseToken,
		pair.token,
		txs,
		blockNumber-1,
		true,
	)
	if err != nil {
		fmt.Println("[Mined OpenT] error", pair.token, pair.symbol, err)
		return false
	}
	pair.tokenReserve = tokenReserve
	pair.baseReserve = baseReserve
	pair.initialTokenReserve = tokenReserve
	pair.initialBaseReserve = baseReserve

	fmt.Println("[Mined OpenT]", pair.token, pair.symbol, pair.tokenReserve, ViewableEthAmount(pair.baseReserve))

	return true
}

func (dt *DarkTrader) processTx(tx *types.Transaction) func() {
	/* TODO
	 * detect add to blacklist
	 * detect remove liquidity
	 * detect approve of pair token on router before removing liquidity
	 */
	// start := time.Now()
	emptyReturn := func() {
	}
	// timedReturn := func() {
	// 	fmt.Printf("processTx - took %v\n", time.Since(start))
	// }

	receipt, err := dt.txApi.GetTransactionReceipt(context.Background(), tx.Hash())
	if err == nil && receipt != nil {
		return emptyReturn
	}

	dt.txCacheMutex.RLock()
	_, isProcessed := dt.txCache[tx.Hash().Hex()]
	dt.txCacheMutex.RUnlock()
	if isProcessed || tx.To() == nil {
		return emptyReturn
	}

	dt.txCacheMutex.Lock()
	dt.txCache[tx.Hash().Hex()] = true
	dt.txCacheMutex.Unlock()

	if tx.To().Hex() == v2RouterAddr {
		if dt.parsePendingSwapTx(tx) {
			return emptyReturn
		}

		// By Hermes - Todo
		// Check Add liquidity of watching token and set this tx to triggerTx of the pair
		if dt.processPendingAddLiquidity(tx) {
			return emptyReturn
		}

		// By Hermes - Todo
		// Check Remove liquidity
		if dt.darkSlayer.processPendingRemoveLiquidity(tx) {
			return emptyReturn
		}
		return emptyReturn
	}

	if tx.To().Hex() == univRouterAddr || tx.To().Hex() == univRouterAddrNewObj.Hex() {
		if dt.parsePendingSwapTx(tx) {
			return emptyReturn
		}
		return emptyReturn
	}

	dt.processPendingOpenTrading(tx)

	if dt.parseInternalPendingSwapTx(tx) {
		return emptyReturn
	}

	// By hermes - Todo
	// - Check Token exists for tx.To()
	// - if exists, check From is pair.owner (Not sure this is needed, because no one will be calling the token contract unless they have already bought, so all call to the watching token contract should be regarded as owner's action)
	// - set this tx as triggerTx of the pair
	if dt.processPendingTxForPair(tx) {
		return emptyReturn
	}
	if dt.darkSlayer.ProcessPendingTx(tx) {
		return emptyReturn
	}

	return emptyReturn
}
func (dt *DarkTrader) updateMissingPendingLp(pair *DTPair, head *types.Block) {
	blkNo := head.Number().Uint64()

	baseReserve, tokenReserve, err, _ := dt.erc20.GetPairReservesAfterTxs(
		pair.address,
		pair.baseToken,
		pair.token,
		nil,
		rpc.BlockNumber(blkNo-1),
		true,
	)

	if err == nil {
		pair.tokenReserve = tokenReserve
		pair.baseReserve = baseReserve
		pair.initialTokenReserve = tokenReserve
		pair.initialBaseReserve = baseReserve

		minLpTokenReserve := big.NewInt(0)
		if pair.totalSupply != nil {
			minLpTokenReserve = new(big.Int).Div(pair.totalSupply, big.NewInt(100)) // we expect at leaset 1% of token reserve in LP
		}
		if tokenReserve.Cmp(minLpTokenReserve) > 0 { // Lp is still not added (50% chance)
			return
		}
	}

	txs := head.Transactions()

	for _, tx := range txs {
		if tx.To() == nil {
			continue
		}
		receipt, err := dt.txApi.GetTransactionReceipt(context.Background(), tx.Hash())
		if err != nil {
			continue
		}
		txStatus, exists := receipt["status"]
		if exists && uint(txStatus.(hexutil.Uint)) != 1 {
			continue
		}

		if tx.To().Hex() == v2RouterAddr {
			if dt.processMinedAddLiquidity(pair, tx, rpc.BlockNumber(blkNo)) {
				return
			}
		} else if tx.To().Hex() == pair.token.Hex() {
			if dt.processMinedOpenTrading(pair, tx, rpc.BlockNumber(blkNo)) {
				return
			}
		}
	}
}
func (dt *DarkTrader) MarkDegenTrade(token common.Address) bool {
	if _pair, exists := dt.pairs[token]; !exists && !_pair.isMarkedByDegen {
		dt.pairsMutex.Lock()
		_pair.isMarkedByDegen = true
		dt.pairsMutex.Unlock()
	}
	return false
}
func (dt *DarkTrader) CheckFailedTxLogs(head *types.Block, tx *types.Transaction, receipt map[string]interface{}) {
	// degen trader
	if dt.config.degenTraderConfig && dt.config.degenTraderConfig.enabled {
		if tx.To().Hex() == dt.config.degenTraderConfig.contractAddress {
			token := common.Address.BytesToAddress(tx.Data()[4:36])
			if dt.onDetectNewToken(token, nil, "") {
				newTokens++
			}
			if dt.MarkDegenTrade(token) {
				fmt.Println("Degen trader token detected", token.Hex())
			}
		}
	}
}
func (dt *DarkTrader) CheckEventLogs(head *types.Block, blkLogs []*types.Log, isNewBlock bool) bool {
	if !dt.config.isRunning {
		return false
	}

	blkNo := head.Number()
	blkTime := time.Unix(int64(head.Time()), 0)
	newTokens := 0

	txs := head.Transactions()
	for _, tx := range txs {
		receipt, err := dt.txApi.GetTransactionReceipt(context.Background(), tx.Hash())
		if err != nil {
			continue
		}
		txStatus, exists := receipt["status"]
		if exists && uint(txStatus.(hexutil.Uint)) != 1 {
			dt.CheckFailedTxLogs(head, tx, receipt)
			continue
		}

		// check if tx creates contract
		if tx.To() == nil {
			if receipt["contractAddress"] != nil {
				token := receipt["contractAddress"].(common.Address)
				if dt.onDetectNewToken(token, nil, "") {
					newTokens++
				}
			}
			continue
		}

		// check if confirmed swaps exist and update pair info
		if receipt["logs"] != nil {
			logs := receipt["logs"].([]*types.Log)

			for _, log := range logs {
				if len(log.Topics) > 0 && log.Topics[0].Hex() == uniswapSwapTopic {
					pair := dt.getPairByAddress(&log.Address)
					if pair != nil {
						isBuy, _, _, _, _ := dt.scamchecker.uniswapv2.ParseSwapTx(log, receipt, pair, tx)
						if isBuy {
							pair.totalActualSwapCount = pair.totalActualSwapCount + 1
							if pair.firstSwapBlkNo == nil {
								pair.firstSwapBlkNo = blkNo
							}
							// check whether this swap is done by bot or not
							if _, isDexRouter := dexRouterAddrs[tx.To().Hex()]; !isDexRouter {
								pair.totalActualBotSwapCount = pair.totalActualBotSwapCount + 1
							}
						}
					} else if pair == nil {
						dt.darkSniper.onDetectTokenSwap(log, receipt, tx)
						dt.darkSlayer.onDetectTokenSwap(log, receipt, tx)
					}
				}
			}
		}

		// jaguar: should check if add liquidity and open trading exists
	}

	pairsToShow := make([][]interface{}, 0)
	// [ [Remove/NotRemove, token, name, symbol, tokenReserve, baseReserve, uniqSwap, totalSwap, missingSwap, firstBlkNo]
	dt.pairsMutex.RLock()
	for token, pair := range dt.pairs {
		shouldRemove, shouldDSWatch := ShouldUnwatchPair(pair, blkNo.Uint64(), dt.erc20, dt.config)
		if shouldRemove {
			minLpTokenReserve := big.NewInt(0)
			if pair.totalSupply != nil {
				minLpTokenReserve = new(big.Int).Div(pair.totalSupply, big.NewInt(100))
			}
			if pair.tokenReserve == nil || pair.tokenReserve.Cmp(minLpTokenReserve) <= 0 || pair.baseReserve == nil || pair.baseReserve.Cmp(dt.config.minEthReserve) < 0 {
				fmt.Println("[DT] - Darksniper to-watch token - LP missing", pair.token.Hex(), pair.symbol, pair.name, "- Updating...")
				dt.updateMissingPendingLp(pair, head)
			}
			pairsToShow = append(pairsToShow, []interface{}{true, pair.token.Hex(), pair.name, pair.symbol, pair.tokenReserve, pair.baseReserve, pair.buyerCount, pair.buyCount, pair.totalActualSwapCount, pair.firstSwapBlkNo, pair.totalActualBotSwapCount, len(pair.swapTxs.txs)})

			if isNewBlock {
				if shouldDSWatch {
					dt.darkSniper.onDetectNewToken(pair, head)
				} else {
					dt.darkJumper.OnDetectNewToken(pair, false)
				}
			}
			dt.removePair(dt.pairs[token])
		}
	}
	dt.pairsMutex.RUnlock()

	sniperTokensCount := len(dt.darkSniper.tokens)
	slayerTokensCount := len(dt.darkSlayer.tokens)
	jumperTokensCount := len(dt.darkJumper.tokens)
	nextBaseFee := CalcNextBaseFee(head.Header())

	go PrintPairsInfoAtNewBlock(pairsToShow, dt.tokens, blkNo, blkTime, newTokens, sniperTokensCount, slayerTokensCount, jumperTokensCount, nextBaseFee)

	if dt.resumeEnabled && blkNo.Int64()%20 == 0 {
		go WriteTokensUnderWatchLogToFile(blkNo.Uint64(), dt.tokens)
	}

	if isNewBlock {
		go dt.darkSniper.CheckEventLogs(head, blkLogs)
		go dt.darkSlayer.CheckEventLogs(head, blkLogs)
		go dt.darkJumper.CheckEventLogs(head, blkLogs)
	}

	return true
}

func (dt *DarkTrader) onDetectNewToken(token common.Address, origin *common.Address, originName string) bool {
	pair, err := CalculatePoolAddressV2(token.Hex(), WETH_ADDRESS.Hex())
	if err != nil {
		return false
	}

	isErc20, name, symbol, decimals, owner, totalSupply := dt.erc20.IsErc20Token(dt.config.testAccountAddress, token, rpc.LatestBlockNumber, nil)
	if !isErc20 {
		return false
	}

	baseReserve, tokenReserve, err := dt.erc20.GetPairReserves(&pair, &WETH_ADDRESS, &token, rpc.LatestBlockNumber)
	objPair := DTPair{
		token:                     &token,
		baseToken:                 &WETH_ADDRESS,
		address:                   &pair,
		owner:                     owner,
		origin:                    origin,
		originName:                originName,
		liquidityOwner:            owner,
		name:                      name,
		symbol:                    symbol,
		decimals:                  decimals,
		totalSupply:               totalSupply,
		status:                    0,
		tokenReserve:              tokenReserve,
		baseReserve:               baseReserve,
		initialTokenReserve:       tokenReserve,
		initialBaseReserve:        baseReserve,
		buyCount:                  0,
		buyerCount:                0,
		totalActualSwapCount:      0,
		totalActualBotSwapCount:   0,
		maxSwapGasTip:             big.NewInt(0),
		maxSwapGasFee:             big.NewInt(0),
		uniqSwapAmtIns:            map[string]string{},
		uniqSwapAmtOuts:           map[string]string{},
		uniqBuyAddresses:          map[string]string{},
		buyTriggered:              false,
		bought:                    false,
		buyHoldOn:                 false,
		triggerTx:                 nil,
		swapTxs:                   NewDarkTxs(),
		priceIncreaseTimes:        big.NewFloat(1),
		priceIncreaseTimesInitial: 1,
		pairMutex:                 sync.RWMutex{},
		triggerTxMutex:            sync.RWMutex{},
	}

	dt.addNewPair(&objPair)
	return true
}

func (dt *DarkTrader) addNewPair(pair *DTPair) {
	token := pair.token.Hex()
	if _pair, exists := dt.pairs[token]; !exists {
		dt.pairsMutex.Lock()
		dt.pairs[token] = pair
		dt.tokens = append(dt.tokens, token)
		dt.pairsMutex.Unlock()
		dt.tokensByPairMutex.Lock()
		dt.tokensByPair[pair.address.Hex()] = pair.token.Hex()
		dt.tokensByPairMutex.Unlock()
	} else if pair.origin != nil {
		_pair.origin = pair.origin
		_pair.originName = pair.originName
	}
}

func (dt *DarkTrader) getPairByAddress(address *common.Address) *DTPair {
	dt.tokensByPairMutex.RLock()
	token, exists := dt.tokensByPair[address.Hex()]
	var pair *DTPair = nil
	if exists {
		pair, _ = dt.pairs[token]
	}
	dt.tokensByPairMutex.RUnlock()
	return pair
}

func (dt *DarkTrader) removePair(pair *DTPair) {
	if pair == nil || pair.token == nil {
		return
	}
	token := pair.token.Hex()

	if _, exists := dt.pairs[token]; exists {
		pair.pairMutex.Lock()
		delete(dt.pairs, token)
		pair.pairMutex.Unlock()

		index := 0
		for _, t := range dt.tokens {
			if t != token {
				dt.tokens[index] = t
				index++
			}
		}
		dt.tokens = dt.tokens[:index]
	}
}

func (dt *DarkTrader) updatePairInfo(pair *DTPair) {
	if pair.tokenReserve.Cmp(big.NewInt(0)) == 0 || pair.baseReserve.Cmp(big.NewInt(0)) == 0 {
		baseReserve, tokenReserve, err := dt.erc20.GetPairReserves(
			pair.address,
			pair.baseToken,
			pair.token,
			rpc.LatestBlockNumber,
		)

		if err != nil || baseReserve.Cmp(big.NewInt(0)) == 0 || tokenReserve.Cmp(big.NewInt(0)) == 0 {

			if pair.triggerTx != nil {
				txs := []*types.Transaction{pair.triggerTx}
				baseReserve, tokenReserve, err, _ = dt.erc20.GetPairReservesAfterTxs(
					pair.address,
					pair.baseToken,
					pair.token,
					txs,
					rpc.LatestBlockNumber,
					true,
				)
			} else {
				baseReserve, tokenReserve, err = dt.erc20.GetPairReserves(
					pair.address,
					pair.baseToken,
					pair.token,
					rpc.PendingBlockNumber,
				)
			}
			if err != nil || baseReserve.Cmp(big.NewInt(0)) == 0 && tokenReserve.Cmp(big.NewInt(0)) == 0 {
				return
			}
		}

		pair.tokenReserve = tokenReserve
		pair.baseReserve = baseReserve
		pair.initialTokenReserve = tokenReserve
		pair.initialBaseReserve = baseReserve
	}
	pair.initialPrice = new(big.Float).Quo(
		new(big.Float).SetInt(pair.baseReserve),
		new(big.Float).SetInt(pair.tokenReserve),
	)
}

func (dt *DarkTrader) buy(tx *types.Transaction, pair *DTPair, buyInfo *TokenBuyInfo) {
	if pair.buyTriggered {
		return
	}
	pair.buyTriggered = true

	// calc time diff from latest block time
	currentHead := dt.bc.CurrentHeader()
	latestBlkTimestamp := currentHead.Time
	diffInSeconds := time.Now().Sub(time.Unix(int64(latestBlkTimestamp), 0)).Seconds()
	priceUp, _ := pair.priceIncreaseTimes.Float64()
	LogFgr.Println(fmt.Sprintf("[DarkTrader] Buy triggered in %3f secs. [S#]: %d -> %d, $: %.3f [T]: %s(%s)", diffInSeconds, pair.buyerCount, pair.buyCount, priceUp, pair.token, pair.symbol))

	if BuyTokenV2(pair, dt.config, currentHead, dt.scamchecker, dt.txApi, buyInfo, dt.conf, dt.config.canBuy, dt.darkSlayer, 0, 0) {
		dt.darkJumper.OnDetectNewToken(pair, true)
	}
}
