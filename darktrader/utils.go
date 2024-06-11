package darktrader

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"math/rand"
	"sort"
	"time"

	ethUnit "github.com/DeOne4eg/eth-unit-converter"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/rpc"
)

var CHAINID *big.Int = big.NewInt(1)

const v2FactoryAddr string = "0x5C69bEe701ef814a2B6a3EDD4B1652CB9cc5aA6f"

var v2FactoryAddrObj = common.HexToAddress(v2FactoryAddr)

const v2FactoryTopic string = "0x0d3648bd0f6ba80134a33ba9275ac585d9d315f0ad8355cddefde31afa28d0e9"
const v2RouterAddr string = "0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D"

var v2RouterAddrObj = common.HexToAddress(v2RouterAddr)

var univRouterAddr string = "0xEf1c6E67703c7BD7107eed8303Fbe6EC2554BF6B"

var univRouterAddrObj = common.HexToAddress(univRouterAddr)
var univRouterAddrNewObj = common.HexToAddress("0x3fC91A3afd70395Cd496C647d5a6CC9D4B2b7FAD")

var v3RouterAddr = common.HexToAddress("0xE592427A0AEce92De3Edee1F18E0157C05861564")

var routerMetamaskSwap = "0x881D40237659C251811CEC9c364ef91dC08D300C"
var router1InchV2 = "0x111111125434b319222CdBf8C261674aDB56F3ae"
var router1InchV3 = "0x11111112542D85B3EF69AE05771c2dCCff4fAa26"
var router1InchV4 = "0x1111111254fb6c44bAC0beD2854e76F90643097d"
var router1InchV5 = "0x1111111254EEB25477B68fb85Ed929f73A960582"
var routerKyberswapMeta = "0x617Dee16B86534a5d792A4d7A62FB491B544111E"
var routerKyberswapV2 = "0xDF1A1b60f2D438842916C0aDc43748768353EC25"
var routerKyberswapV3 = "0x00555513Acf282B42882420E5e5bA87b44D8fA6E"
var routerKyberswapMetaV2 = "0x6131B5fae19EA4f9D964eAc0408E4408b66337b5"
var dexRouterAddrs = map[string]string{
	"0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D": "UniswapV2",           // uniswap v2
	"0xEf1c6E67703c7BD7107eed8303Fbe6EC2554BF6B": "UniswapUniversalOld", // uniswap universal old
	"0x3fC91A3afd70395Cd496C647d5a6CC9D4B2b7FAD": "UniswapUniversalNew", // uniswap universal new
	"0xE592427A0AEce92De3Edee1F18E0157C05861564": "UniswapV3",           // uniswap v3
	routerMetamaskSwap:                           "MetamaskSwap",        // metamask swap
	router1InchV2:                                "1InchV2",             // 1inch router v2
	router1InchV3:                                "1InchV3",             // 1inch router v3
	router1InchV4:                                "1InchV4",             // 1inch router v4
	router1InchV5:                                "1InchV5",             // 1inch router v5
	routerKyberswapMeta:                          "KyberswapMeta",       // kyberswap meta aggregation router
	routerKyberswapV2:                            "KyberswapV2",         // kyberswap aggregation router v2
	routerKyberswapV3:                            "KyberswapV3",         // kyberswap aggregation router v3
	routerKyberswapMetaV2:                        "KyberswapMetaV2",     // kyberswap meta aggregation router v2
}

var uniswapSwapTopic = "0xd78ad95fa46c994b6551d0da85fc275fe613ce37657fb8d5e3d130840159d822"

var flashbotBribeAddr = common.HexToAddress("0xC4595E3966e0Ce6E3c46854647611940A09448d3")

var PoolInitCodeV2, _ = hex.DecodeString("96e8ac4277198ff8b6f785478aa9a39f403cb768dd02cbee326c3e7da348845f")

var WETH_ADDRESS = common.HexToAddress("0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2")

var unicryptAddr = common.HexToAddress("0x663A5C229c09b049E36dCc11a9B0d4a8Eb9db214")
var teamFinanceAddr = common.HexToAddress("0xE2fE530C047f2d85298b07D9333C05737f1435fB")
var pinkLockAddr = common.HexToAddress("0x71B5759d73262FBb223956913ecF4ecC51057641")

var UINT256_MAX, _ = new(big.Int).SetString("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", 16)
var DECIMALS int64 = 1000000000000000000

var AMOUNT_100 *big.Int = big.NewInt(1).Mul(big.NewInt(DECIMALS), big.NewInt(100))
var AMOUNT_99 *big.Int = big.NewInt(1).Mul(big.NewInt(DECIMALS), big.NewInt(99))
var AMOUNT_10 *big.Int = big.NewInt(1).Mul(big.NewInt(DECIMALS), big.NewInt(10))
var AMOUNT_1 *big.Int = big.NewInt(1).Mul(big.NewInt(DECIMALS), big.NewInt(1))

// CalculatePoolAddressV2 calculate uniswapV2 pool address offline from pool tokens
func CalculatePoolAddressV2(token0 string, token1 string) (pairAddress common.Address, err error) {
	tkn0, tkn1 := sortAddressess(common.HexToAddress(token0), common.HexToAddress(token1))

	msg := []byte{255}
	msg = append(msg, v2FactoryAddrObj.Bytes()...)
	addrBytes := tkn0.Bytes()
	addrBytes = append(addrBytes, tkn1.Bytes()...)
	msg = append(msg, crypto.Keccak256(addrBytes)...)

	msg = append(msg, PoolInitCodeV2...)
	hash := crypto.Keccak256(msg)
	pairAddressBytes := big.NewInt(0).SetBytes(hash)
	pairAddressBytes = pairAddressBytes.Abs(pairAddressBytes)
	return common.BytesToAddress(pairAddressBytes.Bytes()), nil
}

func contains(piece uint64, array []uint64) bool {
	for _, a := range array {
		if a == piece {
			return true
		}
	}
	return false
}
func sortAddressess(tkn0, tkn1 common.Address) (common.Address, common.Address) {
	token0Rep := new(big.Int).SetBytes(tkn0.Bytes())
	token1Rep := new(big.Int).SetBytes(tkn1.Bytes())

	if token0Rep.Cmp(token1Rep) > 0 {
		tkn0, tkn1 = tkn1, tkn0
	}

	return tkn0, tkn1
}

func isBaseToken(token string) bool {
	switch token {
	case WETH_ADDRESS.Hex():
		// "0xdAC17F958D2ee523a2206206994597C13D831ec7",
		// "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48":
		return true
	}
	return false
}

func GetFrom(tx *types.Transaction) (common.Address, error) {
	from, err := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx)
	return from, err
}

func BuildSwapPath(pair *DTPair) ([]common.Address, []common.Address) {
	var (
		buyPath  []common.Address
		sellPath []common.Address
	)

	if pair.baseToken.Hex() == WETH_ADDRESS.Hex() {
		buyPath = make([]common.Address, 2)
		buyPath[0] = WETH_ADDRESS
		buyPath[1] = *pair.token
		sellPath = make([]common.Address, 2)
		sellPath[0] = *pair.token
		sellPath[1] = WETH_ADDRESS
	} else {
		buyPath = make([]common.Address, 3)
		buyPath[0] = WETH_ADDRESS
		buyPath[1] = *pair.baseToken
		buyPath[2] = *pair.token
		sellPath = make([]common.Address, 3)
		sellPath[0] = *pair.token
		sellPath[1] = *pair.baseToken
		sellPath[2] = WETH_ADDRESS
	}
	return buyPath, sellPath
}

func CalcBuyAmountAndCount(pair *DTPair, config *DTConfig) (*big.Int, uint) {
	buyAmount := new(big.Int).Mul(big.NewInt(195), big.NewInt(DECIMALS/10000))
	buyCount := uint(len(config.wallets))
	if pair.tokenReserve.Cmp(big.NewInt(0)) == 0 {
		return buyAmount, buyCount
	}

	buyAmountInEth := new(big.Int).Div(
		new(big.Int).Mul(
			pair.baseReserve,
			big.NewInt(config.maxBuyTokenAmountInPercent),
		),
		big.NewInt(100),
	)
	if buyAmountInEth.Cmp(config.maxBuyAmount) > 0 {
		return config.maxBuyAmount, buyCount
	}

	return buyAmountInEth, buyCount
}

func CalcTip(pair *DTPair, param1 BuyCondition1Params, param2 BuyCondition2Params, param3 BuyCondition3Params, config *DTConfig) float64 {
	// defaultTip := float64(10)

	// if param2.minPriceIncreaseTimes == 0 || param2.minBuyCount == 0 {
	// 	return defaultTip
	// }

	// // timesCount, _ := new(big.Float).SetFloat64(float64(pair.buyCount) / float64(param2.minBuyCount)).SetPrec(9).Float64()
	// // timesPrice, _ := new(big.Float).Quo(pair.priceIncreaseTimes, new(big.Float).SetFloat64(param2.minPriceIncreaseTimes)).SetPrec(9).Float64()
	// timesCount := float64(0)
	// totalCountLimit := 25
	// timesTotalCount := len(pair.swapTxs.txs)
	// if timesTotalCount < totalCountLimit {
	// 	timesCount = 10
	// } else {
	// 	timesCount = float64(pair.buyCount)
	// }

	// timesPrice, _ := new(big.Float).Quo(pair.priceIncreaseTimes, new(big.Float).SetFloat64(float64(6))).SetPrec(9).Float64()
	// if timesCount >= 20 {
	// 	timesPrice = timesPrice * 5
	// }

	// // if timesPrice
	// // tip := defaultTip * (timesCount*7 + timesPrice*3) / 10
	// // tip := defaultTip * timesCount / 2
	// tip := defaultTip * (float64(timesCount) + timesPrice)

	// if tip < 100 {
	// 	tip = 101
	// } else if tip > 401 {
	// 	tip = 401
	// }

	// return tip
	defaultTip := config.defaultBaseTipMax
	totalCountLimit := 20
	timesCount := float64(0)
	timesTotalCount := len(pair.swapTxs.txs)
	failedCount := 0

	if timesTotalCount < totalCountLimit {
		timesCount = 10
	} else {
		defaultTip = defaultTip * 1.25
		failedCount = timesTotalCount - pair.buyCount
		timesCount = float64(pair.buyCount) + float64(failedCount)/4
	}

	timesPrice, _ := pair.priceIncreaseTimes.Float64()
	tip := defaultTip * timesCount
	deductionByPriceInc := timesPrice / float64(pair.buyCount) * 6
	tip = tip * deductionByPriceInc

	// buy & sell fee consideration
	if pair.buyInfo != nil {
		totalInExclusiveFee := float64(100-pair.buyInfo.buyFeeInPercent) / 100 * float64(100-pair.buyInfo.sellFeeInPercent) / 100
		tip = tip * totalInExclusiveFee
	}
	// check tip and buy amount comparison
	// generic gas amount
	gasAmount := big.NewInt(200000)
	maxGasFeeMultiplier := int64(5)
	if pair.buyInfo.buyAmount.Cmp(new(big.Int).Mul(big.NewInt(int64(tip*float64(1000000000/maxGasFeeMultiplier))), gasAmount)) < 0 {
		fmt.Println("Too much tip for buying amount. Tip: ", tip, "Buy amount", ViewableEthAmount(pair.buyInfo.buyAmount))
		tip, _ = new(big.Float).Quo(new(big.Float).SetInt(new(big.Int).Div(new(big.Int).Mul(pair.buyInfo.buyAmount, big.NewInt(5)), gasAmount)), big.NewFloat(1000000000)).Float64()
		fmt.Println("   Reducing tip to ", tip)
	}
	return tip
}
func CalculateTxTips(baseTip float64, buyCount uint) []*big.Int {
	var (
		tips         []*big.Int = make([]*big.Int, buyCount)
		tipsGwei     []int64    = make([]int64, buyCount)
		totalTips    float64    = baseTip * float64(buyCount)
		randSource              = rand.NewSource(time.Now().UnixNano())
		randProducer            = rand.New(randSource)
		randDists               = make([]float64, buyCount)
		randSum                 = 0.0
	)
	for i := 0; i < int(buyCount); i++ {
		randDists[i] = randProducer.Float64()*300 + 100
		randSum += randDists[i]
	}
	for i := 0; i < int(buyCount); i++ {
		tipsGwei[i] = int64(totalTips*randDists[i]/randSum + 0.5)
	}
	sort.Slice(tipsGwei, func(i int, j int) bool {
		return tipsGwei[i] > tipsGwei[j]
	})

	for i := 0; i < int(buyCount); i++ {
		tips[i] = new(big.Int).Mul(big.NewInt(tipsGwei[i]), big.NewInt(DECIMALS/1000000000))
	}
	// ,
	return tips
}

func CalculateTokenAmountsToBuy(tokenAmount *big.Int, buyCount uint) []*big.Int {
	var (
		tokenAmountsToBuy       = make([]*big.Int, int(buyCount))
		randSource              = rand.NewSource(time.Now().UnixNano())
		randProducer            = rand.New(randSource)
		randDists               = make([]float64, buyCount)
		multiplier        int64 = 100
	)
	for i := 0; i < int(buyCount); i++ {
		randDists[i] = (randProducer.Float64()*4 + 96) * float64(multiplier)
	}
	for i := 0; i < int(buyCount); i++ {
		tokenAmountsToBuy[i] = new(big.Int).Mul(tokenAmount, big.NewInt(int64(randDists[i])))
		tokenAmountsToBuy[i].Div(tokenAmountsToBuy[i], big.NewInt(multiplier*100))
	}
	return tokenAmountsToBuy
}

func PadOrTrim(bb []byte, size int) []byte {
	l := len(bb)
	if l == size {
		return bb
	}
	if l > size {
		return bb[l-size:]
	}
	tmp := make([]byte, size)
	copy(tmp[size-l:], bb)
	return tmp
}

func CalcNextBaseFee(header *types.Header) *big.Int {
	gasTarget := uint64(30000000 / 2)
	nextBaseFee := uint64(0)
	BASE_FEE_MAX_CHANGE_DENOMINATOR := uint64(8)
	parent_base_fee_per_gas := header.BaseFee.Uint64()
	if header.GasUsed == gasTarget {
		nextBaseFee = header.BaseFee.Uint64()
	} else if header.GasUsed > gasTarget {
		gas_used_delta := header.GasUsed - gasTarget
		base_fee_per_gas_delta := parent_base_fee_per_gas * gas_used_delta / gasTarget / BASE_FEE_MAX_CHANGE_DENOMINATOR
		nextBaseFee = parent_base_fee_per_gas + base_fee_per_gas_delta
	} else {
		gas_used_delta := gasTarget - header.GasUsed
		base_fee_per_gas_delta := parent_base_fee_per_gas * gas_used_delta / gasTarget / BASE_FEE_MAX_CHANGE_DENOMINATOR
		nextBaseFee = parent_base_fee_per_gas - base_fee_per_gas_delta
	}
	return new(big.Int).SetUint64(nextBaseFee + 1)
}

func ShouldUnwatchPair(pair *DTPair, blockNumber uint64, erc20 *Erc20, config *DTConfig) (bool, bool) {
	if pair == nil {
		return true, false
	}
	if pair.bought == true {
		return true, false
	}
	if pair.firstSwapBlkNo == nil {
		baseReserve, tokenReserve, err := erc20.GetPairReserves(pair.address, pair.baseToken, pair.token, rpc.LatestBlockNumber)
		if err != nil {
			// for some reason, it's not possible to get the reserve, watch until next block
			return false, false
		}

		minLpTokenReserve := big.NewInt(0)
		if pair.totalSupply != nil {
			minLpTokenReserve = new(big.Int).Div(pair.totalSupply, big.NewInt(100)) // we expect at leaset 1% of token reserve in LP
		}
		if tokenReserve.Cmp(minLpTokenReserve) <= 0 { // Lp is still not added (50% chance)
			return false, false
		}
		if baseReserve.Cmp(config.minEthReserve) < 0 {
			fmt.Println("[DT] - LP removed/too small", ViewableEthAmount(baseReserve), "[T]", pair.token.Hex(), "(", pair.symbol, ")")
			return true, false
		}
		if pair.baseReserve.Cmp(config.maxEthReserve) > 0 {
			fmt.Println("[DT] - LP too large", ViewableEthAmount(baseReserve), "[T]", pair.token.Hex(), "(", pair.symbol, ")")
			return true, false
		}
		return false, false
	}
	return true, true
}

func IsLiquidityInRange(pair *DTPair, config *DTConfig) bool {
	if pair.baseToken.Hex() == "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2" && pair.initialBaseReserve.Cmp(config.minEthReserve) < 0 {
		return false // Remove cause too small
	} else if pair.baseToken.Hex() == "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2" && pair.initialBaseReserve.Cmp(config.maxEthReserve) > 0 {
		return false // Remove cause too large
	}
	return true
}

func ViewableEthAmount(amount *big.Int) float64 {
	kk := big.NewInt(1).Div(big.NewInt(1).Mul(amount, big.NewInt(100)), big.NewInt(DECIMALS)).Int64()
	return float64(kk) / 100
	// return ethUnit.NewWei(amount).Ether()
}
func ShouldForceAdvancedTip(pair *DTPair, config *DTConfig) bool {
	fPriceUp, _ := pair.priceIncreaseTimes.Float64()
	iPriceUp := int(fPriceUp + 0.5)
	if iPriceUp >= 6 && pair.buyCount >= config.buyCondition3Params.minBuyCount*2 {
		return true
	}
	return false
}

func CheckTokenTxs(currentHead *types.Header, pair *DTPair, tx *types.Transaction, bcApi *ethapi.BlockChainAPI, erc20 *Erc20, scamchecker *ScamChecker, txApi *ethapi.TransactionAPI, config *DTConfig, considerConfigFee bool) *TokenBuyInfo {
	if pair.tokenReserve.Cmp(big.NewInt(0)) == 0 {
		LogFwBrOb.Println("[Error]: Reserves not detected!")
		return nil
	}
	fmt.Println("Initial reserve", ViewableEthAmount(pair.initialBaseReserve), pair.initialTokenReserve)
	if !IsLiquidityInRange(pair, config) {
		fmt.Println("[Error]: LP not in the range!", pair.initialBaseReserve)
		return nil
	}

	nextBaseFee := CalcNextBaseFee(currentHead)

	if pair.triggerTx != nil {
		// check whether the trigger tx has already been mined
		receipt, err := txApi.GetTransactionReceipt(context.Background(), pair.triggerTx.Hash())
		if err == nil && receipt != nil {
			fmt.Println("TriggerTx already mined", pair.triggerTx.Hash().Hex())
			pair.triggerTx = nil
		}
	}

	var (
		buyInfo TokenBuyInfo
		err     error
	)
	buyInfo.buyAmount, buyInfo.buyCount, buyInfo.tokenAmountExpected, buyInfo.tokenAmountActual, err = scamchecker.CalcBuyAmount(pair, config, rpc.LatestBlockNumber, pair.triggerTx, nil)
	if err != nil {
		LogFwBrOb.Println("[Error]: Calc buy amount failed!", err)
		return nil
	} else if config.minBuyAmount.Cmp(buyInfo.buyAmount) > 0 {
		LogFwBrOb.Println("[Error]: Too low buy amount!", buyInfo.buyAmount)
		return nil
	}

	// check buy fee
	buyInfo.buyFeeInPercent = 100 - new(big.Int).Div(big.NewInt(0).Mul(buyInfo.tokenAmountActual, big.NewInt(100)), buyInfo.tokenAmountExpected).Uint64()

	// try buy and sell
	sellAmountExpected, sellAmountActual, err := scamchecker.CheckBuySell(pair, buyInfo.buyAmount, buyInfo.tokenAmountActual, currentHead.Number.Int64(), currentHead.Time, config.buySellCheckCount, pair.triggerTx)
	if err != nil {
		LogFwBrOb.Println("[Error]: Try buy and sell failed!", err)
		return nil
	}

	// check sell fee
	buyInfo.sellFeeInPercent = 100 - big.NewInt(0).Div(big.NewInt(0).Mul(sellAmountActual, big.NewInt(100)), sellAmountExpected).Uint64()

	var (
		maxBuyFeeInPercent   uint64 = 0
		maxSellFeeInPercent  uint64 = 0
		maxTotalFeeInPercent uint64 = 0
	)

	for _, tipMode := range config.multiTipModes {
		if maxBuyFeeInPercent < tipMode.maxBuyFee {
			maxBuyFeeInPercent = tipMode.maxBuyFee
		}
		if maxSellFeeInPercent < tipMode.maxSellFee {
			maxSellFeeInPercent = tipMode.maxSellFee
		}
		if maxTotalFeeInPercent < tipMode.maxTotalFee {
			maxTotalFeeInPercent = tipMode.maxTotalFee
		}
	}
	if considerConfigFee && buyInfo.buyFeeInPercent > maxBuyFeeInPercent {
		LogFwBrOb.Println("[Error]: Buy fee too high!", buyInfo.buyFeeInPercent, ">", maxBuyFeeInPercent)
		return nil
	}
	if considerConfigFee && buyInfo.sellFeeInPercent > maxSellFeeInPercent {
		LogFwBrOb.Println("[Error]: Sell fee too high!", buyInfo.sellFeeInPercent, ">", maxSellFeeInPercent)
		return nil
	}

	buyInfo.totalFeeInPercent = 100 - (100-buyInfo.buyFeeInPercent)*(100-buyInfo.sellFeeInPercent)/100

	if considerConfigFee && buyInfo.totalFeeInPercent > maxTotalFeeInPercent {
		LogFwBrOb.Println("[Error]: total fee too high!", buyInfo.totalFeeInPercent, ">", maxTotalFeeInPercent)
		return nil
	}

	// buyInfo.buyCount = uint(len(config.wallets)) // to be removed

	// dynamically set the buy count based on the buy, sell fee
	// totalExclFee := (100 - buyInfo.buyFeeInPercent) * (100 - buyInfo.sellFeeInPercent) / 100
	totalCount := uint64(len(config.wallets))
	// buyInfo.buyCount = uint(totalCount * totalExclFee / 100)
	buyInfo.buyCount = uint(totalCount)

	if buyInfo.buyCount == 0 {
		if len(config.wallets) == 0 {
			LogFwBrOb.Println("[Error]: No wallets available!", len(config.wallets))
			return nil
		} else {
			buyInfo.buyCount = 1
		}
	}

	txs := pair.swapTxs.BuildMaxProfitableSwapTxs(txApi, nextBaseFee, currentHead)

	// path, _ := BuildSwapPath(pair)
	// if int(buyInfo.buyCount) > len(config.wallets) {
	// 	buyInfo.buyCount = uint(len(config.wallets))
	// }

	var stateOverrides ethapi.StateOverride = make(ethapi.StateOverride)
	balanceTest := hexutil.Big(*AMOUNT_10)
	balanceTest1 := &balanceTest
	for i := 0; i < int(buyInfo.buyCount); i++ {
		stateOverrides[scamchecker.testWallets[i].address] = ethapi.OverrideAccount{
			Balance: &balanceTest1,
		}
	}

	var batchCallConfig = ethapi.BatchCallConfig{
		Block:          rpc.BlockNumberOrHashWithNumber(rpc.LatestBlockNumber),
		StateOverrides: &stateOverrides,
	}
	callIndex := 0
	if pair.triggerTx != nil {
		// trigger tx, my buy txs, other buy txs, balance of weth, balance of token
		batchCallConfig.Calls = make([]ethapi.BatchCallArgs, 1+len(txs)+2)

		txFrom, _ := GetFrom(pair.triggerTx)
		txValue := hexutil.Big(*pair.triggerTx.Value())
		txInput := hexutil.Bytes(pair.triggerTx.Data())
		gas := hexutil.Uint64(pair.triggerTx.Gas())
		nonce := hexutil.Uint64(pair.triggerTx.Nonce())
		batchCallConfig.Calls[0] = ethapi.BatchCallArgs{
			TransactionArgs: ethapi.TransactionArgs{
				From:    &txFrom,
				To:      pair.triggerTx.To(),
				Value:   &txValue,
				Input:   &txInput,
				ChainID: &scamchecker.chainId,
				Gas:     &gas,
				Nonce:   &nonce,
			},
		}
		callIndex++
	} else {
		batchCallConfig.Calls = make([]ethapi.BatchCallArgs, len(txs)+2)
	}

	// slippagePercent := config.maxBuySlippageInPercent
	// buyInfo.maxAmountIn = new(big.Int).Div(
	// 	new(big.Int).Mul(
	// 		buyInfo.buyAmount,
	// 		big.NewInt(100),
	// 	),
	// 	new(big.Int).SetUint64(slippagePercent),
	// )

	buyInfo.tokenAmountsToBuy = CalculateTokenAmountsToBuy(buyInfo.tokenAmountExpected, buyInfo.buyCount)

	// noSlippageAmountIn := big.NewInt(0).Add(buyInfo.maxAmountIn, buyInfo.maxAmountIn)
	// for i := 0; i < int(buyInfo.buyCount); i++ {
	// 	account := scamchecker.testWallets[i]
	// 	swapInput, err := scamchecker.uniswapv2.abi.Pack("swapETHForExactTokens", buyInfo.tokenAmountsToBuy[i], path, account.address, big.NewInt(time.Now().Unix()+200))
	// 	swapPayload := hexutil.Bytes(swapInput)
	// 	if err != nil {
	// 		fmt.Println("Error in pack buy input", err)
	// 		return nil
	// 	}
	// 	amountToSend := noSlippageAmountIn
	// 	if i == 0 {
	// 		amountToSend = buyInfo.maxAmountIn
	// 	}
	// 	swapTxValue := hexutil.Big(*amountToSend)
	// 	batchCallConfig.Calls[callIndex] = ethapi.BatchCallArgs{
	// 		TransactionArgs: ethapi.TransactionArgs{
	// 			From:    &account.address,
	// 			To:      &v2RouterAddrObj,
	// 			Value:   &swapTxValue,
	// 			Input:   &swapPayload,
	// 			ChainID: &scamchecker.chainId,
	// 		},
	// 	}
	// 	callIndex++
	// }
	callIndexTxs := callIndex
	for i := 0; i < len(txs); i++ {
		txFrom, _ := GetFrom(txs[i].tx)
		txValue := hexutil.Big(*txs[i].tx.Value())
		txInput := hexutil.Bytes(txs[i].tx.Data())
		gas := hexutil.Uint64(txs[i].tx.Gas())
		Nonce := hexutil.Uint64(txs[i].tx.Nonce())
		batchCallConfig.Calls[callIndexTxs+i] = ethapi.BatchCallArgs{
			TransactionArgs: ethapi.TransactionArgs{
				From:    &txFrom,
				To:      txs[i].tx.To(),
				Value:   &txValue,
				Input:   &txInput,
				ChainID: &scamchecker.chainId,
				Gas:     &gas,
				Nonce:   &Nonce,
			},
		}
		callIndex++
	}
	callIndexWeth := callIndex
	txBalanceOfBaseTokenInput, err := erc20.BuildBalanceOfInput(pair.address)
	if err != nil {
		fmt.Println("Error in pack buy input", err)
		return nil
	}
	batchCallConfig.Calls[callIndex] = ethapi.BatchCallArgs{
		TransactionArgs: ethapi.TransactionArgs{
			To:    pair.baseToken,
			Input: txBalanceOfBaseTokenInput,
		},
	}
	callIndex++
	callIndexToken := callIndex
	txBalanceOfTokenInput, err := erc20.BuildBalanceOfInput(pair.address)
	if err != nil {
		fmt.Println("Error in pack buy input", err)
		return nil
	}
	batchCallConfig.Calls[callIndex] = ethapi.BatchCallArgs{
		TransactionArgs: ethapi.TransactionArgs{
			To:    pair.token,
			Input: txBalanceOfTokenInput,
		},
	}
	callIndex++

	results, err := bcApi.BatchCall(context.Background(), batchCallConfig)

	if err != nil {
		fmt.Println("[CheckTokenTxs]-BatchCall", err)
		return nil
	}

	if results[callIndexWeth].Error != nil || results[callIndexToken].Error != nil {
		fmt.Println("[CheckTokenTxs]-BatchCall", results[callIndexWeth].Error)
		fmt.Println("[CheckTokenTxs]-BatchCall", results[callIndexToken].Error)
		return nil
	}

	baseReserve, err := erc20.ParseBalanceOfOutput(results[callIndexWeth].Return)
	if err != nil {
		fmt.Println("[CheckTokenTxs]-ParserBaseReserve", err)
		return nil
	}
	tokenReserve, err := erc20.ParseBalanceOfOutput(results[callIndexToken].Return)
	if err != nil {
		fmt.Println("[CheckTokenTxs]-ParserTokenReserve", err)
		return nil
	}
	if tokenReserve.Cmp(big.NewInt(0)) == 0 {
		fmt.Println("[CheckTokenTxs]-Token reserve zero", ViewableEthAmount(baseReserve), tokenReserve)
		return nil
	}
	pair.pairMutex.Lock()
	blockInitialPrice := new(big.Float).Quo(
		new(big.Float).SetInt(pair.baseReserve),
		new(big.Float).SetInt(pair.tokenReserve),
	)
	if pair.initialPrice == nil || pair.initialPrice.Cmp(big.NewFloat(0)) == 0 {
		pair.initialPrice = blockInitialPrice
	}
	curPrice := new(big.Float).Quo(
		new(big.Float).SetInt(baseReserve),
		new(big.Float).SetInt(tokenReserve),
	)
	pair.priceIncreaseTimes = new(big.Float).Quo(curPrice, blockInitialPrice).SetPrec(5)
	pair.priceIncreaseTimesInitial, _ = new(big.Float).Quo(blockInitialPrice, pair.initialPrice).Float64()
	pair.uniqBuyAddresses = make(map[string]string)
	pair.uniqSwapAmtIns = make(map[string]string)
	pair.uniqSwapAmtOuts = make(map[string]string)
	pair.buyerCount = 0
	pair.buyCount = 0
	buyInfo.txsToBundle = make([]*types.Transaction, 0)
	for i := 0; i < len(txs); i++ {
		if results[callIndexTxs+i].Error != nil {
			LogFbBlr.Println(fmt.Sprintf("[Call swap error] tx(%s - %s)", txs[i].swapInfo.GetDexRouter(), txs[i].tx.Hash()), err)
		}
		if results[callIndexTxs+i].Error == nil {
			pair.buyCount++
			address := txs[i].swapInfo.address.Hex()
			swapType := txs[i].swapInfo.router
			amtIn := txs[i].swapInfo.amtIn
			amtOut := txs[i].swapInfo.amtOut
			if _, exists := pair.uniqBuyAddresses[address]; exists == false {
				uniq := false
				// check if swap is from uniq account
				if swapType == "Contract" {
					if _, exists = pair.uniqBuyAddresses[address]; exists == false {
						uniq = true
						pair.uniqBuyAddresses[address] = txs[i].tx.Hash().Hex()
					}
				} else if amtIn != nil && amtIn.Cmp(big.NewInt(0)) > 0 {
					if _, exists := pair.uniqSwapAmtIns[amtIn.String()]; exists == false {
						pair.uniqSwapAmtIns[amtIn.String()] = txs[i].tx.Hash().Hex()
						uniq = true
					}
				} else if amtOut != nil && amtOut.Cmp(big.NewInt(0)) > 0 { // check if the same amtOut exists
					if _, exists := pair.uniqSwapAmtOuts[amtOut.String()]; exists == false {
						pair.uniqSwapAmtOuts[amtOut.String()] = txs[i].tx.Hash().Hex()
						uniq = true
					}
				}

				if uniq {
					pair.buyerCount++
				}

				pair.uniqBuyAddresses[address] = txs[i].tx.Hash().Hex()
			}
		}
		if results[callIndexTxs+i].Error == nil || txs[i].tx.GasTipCap().Cmp(big.NewInt(5*1000000000)) >= 0 {
			buyInfo.txsToBundle = append(buyInfo.txsToBundle, txs[i].tx)
		}
	}
	pair.pairMutex.Unlock()

	pair.buyInfo = &buyInfo

	return &buyInfo
}

func CheckTokenTxsForJumper(currentHead *types.Header, pair *DTPair, bcApi *ethapi.BlockChainAPI, erc20 *Erc20, scamchecker *ScamChecker, txApi *ethapi.TransactionAPI, config *DTConfig) *TokenBuyInfo {
	if pair.tokenReserve.Cmp(big.NewInt(0)) == 0 {
		LogFwBrOb.Println("[Error]: Reserves not detected!")
		return nil
	}
	var (
		buyInfo TokenBuyInfo
		err     error
	)
	buyAmounts := make([]*big.Int, 0)
	curBuyAmounts := ethUnit.NewEther(big.NewFloat(config.darkJumper.maxBuyAmountInEth)).Wei()
	for {
		if curBuyAmounts.Cmp(config.minBuyAmount) < 0 {
			break
		}
		buyAmounts = append(buyAmounts, curBuyAmounts)
		curBuyAmounts = new(big.Int).Sub(curBuyAmounts, ethUnit.NewEther(big.NewFloat(0.005)).Wei())
	}

	buyInfo.buyAmount, buyInfo.buyCount, buyInfo.tokenAmountExpected, buyInfo.tokenAmountActual, err = scamchecker.CalcBuyAmount(pair, config, rpc.LatestBlockNumber, nil, buyAmounts)
	if err != nil {
		LogFwBrOb.Println("[Error]: Calc buy amount failed!", err)
		return nil
	} else if config.minBuyAmount.Cmp(buyInfo.buyAmount) > 0 {
		LogFwBrOb.Println("[Error]: Too low buy amount!", buyInfo.buyAmount)
		return nil
	}

	// check buy fee
	buyInfo.buyFeeInPercent = 100 - new(big.Int).Div(big.NewInt(0).Mul(buyInfo.tokenAmountActual, big.NewInt(100)), buyInfo.tokenAmountExpected).Uint64()

	// try buy and sell
	sellAmountExpected, sellAmountActual, err := scamchecker.CheckBuySell(pair, buyInfo.buyAmount, buyInfo.tokenAmountActual, currentHead.Number.Int64(), currentHead.Time, config.buySellCheckCount, pair.triggerTx)
	if err != nil {
		if err.Error() == "TransferHelper: TRANSFER_FROM_FAILED" {
			buyInfo.sellFeeInPercent = 50
			LogFwBrOb.Println("[Error]: Can't calc sell fee. Force to ", buyInfo.sellFeeInPercent, "%")
		} else {
			LogFwBrOb.Println("[Error]: Try buy and sell failed!", err)
			return nil
		}
	} else {
		// check sell fee
		buyInfo.sellFeeInPercent = 100 - big.NewInt(0).Div(big.NewInt(0).Mul(sellAmountActual, big.NewInt(100)), sellAmountExpected).Uint64()
	}

	// buyInfo.buyCount = uint(len(config.wallets)) // to be removed

	// dynamically set the buy count based on the buy, sell fee
	totalExclFee := (100 - buyInfo.buyFeeInPercent) * (100 - buyInfo.sellFeeInPercent) / 100
	totalCount := uint64(config.darkJumper.maxBuyCount)
	buyInfo.buyCount = uint(totalCount * totalExclFee / 100)

	if buyInfo.buyCount == 0 {
		if len(config.wallets) == 0 {
		} else {
			buyInfo.buyCount = 1
		}
	}

	pair.buyInfo = &buyInfo

	return &buyInfo
}

func CalculateSlippage(pair *DTPair, tip float64, buyInfo *TokenBuyInfo, initialSlippage uint64) uint64 {
	if initialSlippage >= 90 {
		return initialSlippage
	}
	defaultTip := float64(100)
	if tip <= defaultTip {
		return initialSlippage
	}
	slippage := uint64(tip / defaultTip * float64(initialSlippage))

	if slippage >= 90 {
		return 90
	}
	return slippage
}

func BuyTokenV2(pair *DTPair, config *DTConfig, currentHead *types.Header, scamchecker *ScamChecker, txApi *ethapi.TransactionAPI, buyInfo *TokenBuyInfo, conf *Configuration, canBuy bool, darkslayer *DarkSlayer, customSlippage uint64, buyMode uint64) bool {
	blockNumber := currentHead.Number.Int64()

	nextBaseFee := CalcNextBaseFee(currentHead)

	// go pair.swapTxs.WriteToLog(pair)

	PrintBuyInfo(pair, buyInfo.buyAmount, buyInfo.buyCount, buyInfo.tokenAmountExpected, buyInfo.tokenAmountActual, buyInfo.buyFeeInPercent, buyInfo.sellFeeInPercent)

	path, _ := BuildSwapPath(pair)
	if int(buyInfo.buyCount) > len(config.wallets) {
		buyInfo.buyCount = uint(len(config.wallets))
	}

	pair.wallets = make([]*DTAccount, buyInfo.buyCount)

	var tip float64 = 0
	var appliedTipMode = 0
	if config.tipMode == 0 {
		appliedTipMode = 0
	} else if config.tipMode == 1 {
		appliedTipMode = 1
	} else if config.tipMode == 2 {
		if pair.priceIncreaseTimes.Cmp(big.NewFloat(config.buyCondition1Params.minPriceIncreaseTimes)) >= 0 {
			appliedTipMode = 1
		} else {
			appliedTipMode = 3
		}
	} else if config.tipMode == 3 {
		appliedTipMode = 3
	} else if config.tipMode == 4 {
		appliedTipMode = 4
	}
	if appliedTipMode == 0 {
		tip = config.defaultBaseTipMax * 10
	} else if appliedTipMode == 1 {
		tip = CalcTip(pair, config.buyCondition1Params, config.buyCondition2Params, config.buyCondition3Params, config) - float64(nextBaseFee.Int64())/1000000000
	} else if appliedTipMode == 3 {
		tip = config.defaultBaseTipMax*10 - float64(nextBaseFee.Int64())/1000000000
		if tip <= 50 {
			tip = 50
		}
	}

	fmt.Println("[Blk #]:", blockNumber, "[Tip]:", tip, "Gwei (Applied mode - ", appliedTipMode, ")")

	initialTip := tip
	slippageTipReduce := float64(config.defaultBaseTipMax) / float64(config.defaultBaseTipMin)

	initialSlippagePercent := customSlippage
	usingCustomSlippage := false
	if initialSlippagePercent <= 0 {
		initialSlippagePercent = config.minBuySlippageInPercent
	} else {
		fmt.Println("Custom slippage", initialSlippagePercent)
		usingCustomSlippage = true
	}

	// initialSlippagePercent = CalculateSlippage(pair, tip, buyInfo, initialSlippagePercent)

	fmt.Println("Slippage - ", initialSlippagePercent)
	fmt.Println("Current base fee:", currentHead.BaseFee)
	fmt.Println("Base gas fee in next block:", nextBaseFee)
	fmt.Println("Buying amounts", buyInfo.tokenAmountsToBuy)
	if pair.origin != nil {
		fmt.Println("Origin", pair.origin.Hex(), "(", pair.originName, ")")
	}

	slippages := []uint64{config.maxBuySlippageInPercent, initialSlippagePercent}
	slippageTips := []float64{initialTip, initialTip / slippageTipReduce}
	buyCounts := []uint{buyInfo.buyCount, buyInfo.buyCount}
	if appliedTipMode == 0 {
		slippages = []uint64{initialSlippagePercent}
		slippageTips = []float64{initialTip}
		buyCounts = []uint{buyInfo.buyCount}
		if initialSlippagePercent < 95 {
			slippages = append(slippages, 95)
			slippageTips = append(slippageTips, initialTip+5)
			buyCounts = append(buyCounts, buyInfo.buyCount)
		}
	} else if appliedTipMode == 1 {
		if config.maxBuySlippageInPercent < initialSlippagePercent {
			slippages = []uint64{initialSlippagePercent}
			slippageTips = []float64{initialTip / slippageTipReduce}
			buyCounts = []uint{buyInfo.buyCount}
		}
	} else if appliedTipMode == 3 {
		if usingCustomSlippage {
			slippages = []uint64{initialSlippagePercent}
			slippageTips = []float64{initialTip}
			buyCounts = []uint{buyInfo.buyCount}
			slp := initialSlippagePercent / 2
			idx := 0
			for {
				if slp < 25 {
					break
				}
				slippages = append(slippages, slp)
				slippageTips = append(slippageTips, slippageTips[idx]*float64(slippages[idx+1])/float64(slippages[idx]))
				buyCounts = append(buyCounts, uint(float64(buyCounts[idx])*float64(slippages[idx+1])/float64(slippages[idx])))
				idx = idx + 1
				slp = slp / 2
			}
		} else {
			slippages = []uint64{98, 50, 30, 25, 20}
			slippageTips = []float64{initialTip + 1, initialTip, initialTip * 3 / 5, initialTip / 2, initialTip * 2 / 5}
			buyCounts = []uint{buyInfo.buyCount, buyInfo.buyCount, uint(float64(buyInfo.buyCount) * float64(slippages[2]) / float64(slippages[1])), uint(float64(buyInfo.buyCount) * float64(slippages[3]) / float64(slippages[1])), uint(float64(buyInfo.buyCount) * float64(slippages[4]) / float64(slippages[1]))}
		}
	} else if appliedTipMode == 4 {
		if usingCustomSlippage {
			slippages = make([]uint64, 0)
			slippageTips = make([]float64, 0)
			buyCounts = make([]uint, 0)
			for _, tipMode := range config.multiTipModes {
				if !contains(buyMode, tipMode.modes) {
					continue
				}
				_slippage := uint64(float64(tipMode.slippage)*float64(initialSlippagePercent)/50 + 0.5)
				if _slippage > 95 {
					continue
				}
				if buyInfo.buyFeeInPercent > tipMode.maxBuyFee || buyInfo.sellFeeInPercent > tipMode.maxSellFee || buyInfo.totalFeeInPercent > tipMode.maxTotalFee {
					if buyInfo.buyFeeInPercent > tipMode.maxBuyFee {
						fmt.Println("   Ignore ", tipMode.slippage, "tipMode - buy fee", buyInfo.buyFeeInPercent, ">", tipMode.maxBuyFee)
					}
					if buyInfo.sellFeeInPercent > tipMode.maxSellFee {
						fmt.Println("   Ignore ", tipMode.slippage, "tipMode - sell fee", buyInfo.sellFeeInPercent, ">", tipMode.maxSellFee)
					}
					if buyInfo.totalFeeInPercent > tipMode.maxTotalFee {
						fmt.Println("   Ignore ", tipMode.slippage, "tipMode - total fee", buyInfo.totalFeeInPercent, ">", tipMode.maxTotalFee)
					}
					continue
				}
				slippages = append(slippages, _slippage)
				slippageTips = append(slippageTips, tipMode.tip)
				_buyCount := uint(float64(buyInfo.buyCount)*float64(tipMode.buyCountPercent)/100 + 0.5)
				if _buyCount <= 0 {
					_buyCount = 1
				}
				buyCounts = append(buyCounts, _buyCount)
			}
		} else {
			slippages = make([]uint64, 0)
			slippageTips = make([]float64, 0)
			buyCounts = make([]uint, 0)
			for _, tipMode := range config.multiTipModes {
				if !contains(buyMode, tipMode.modes) {
					continue
				}
				if buyInfo.buyFeeInPercent > tipMode.maxBuyFee || buyInfo.sellFeeInPercent > tipMode.maxSellFee || buyInfo.totalFeeInPercent > tipMode.maxTotalFee {
					if buyInfo.buyFeeInPercent > tipMode.maxBuyFee {
						fmt.Println("   Ignore ", tipMode.slippage, "tipMode - buy fee", buyInfo.buyFeeInPercent, ">", tipMode.maxBuyFee)
					}
					if buyInfo.sellFeeInPercent > tipMode.maxSellFee {
						fmt.Println("   Ignore ", tipMode.slippage, "tipMode - sell fee", buyInfo.sellFeeInPercent, ">", tipMode.maxSellFee)
					}
					if buyInfo.totalFeeInPercent > tipMode.maxTotalFee {
						fmt.Println("   Ignore ", tipMode.slippage, "tipMode - total fee", buyInfo.totalFeeInPercent, ">", tipMode.maxTotalFee)
					}
					continue
				}
				slippages = append(slippages, tipMode.slippage)
				slippageTips = append(slippageTips, tipMode.tip)
				_buyCount := uint(float64(buyInfo.buyCount)*float64(tipMode.buyCountPercent)/100 + 0.5)
				if _buyCount <= 0 {
					_buyCount = 1
				}
				buyCounts = append(buyCounts, _buyCount)
			}
		}
	}

	if isBlockedOrigin(pair, config) {
		LogFwBrOb.Println("Origin blocked!")
		return false
	}

	broadcastCount := int(0)
	for slippageIdx, slippagePercent := range slippages {
		tip = slippageTips[slippageIdx]

		var (
			boughtAccounts    []string
			txBundles         []*TxBundle
			txBundleBroadcast *TxBundle  = nil
			tips              []*big.Int = CalculateTxTips(tip, buyInfo.buyCount)
		)

		bundleCount := 1
		if pair.triggerTx != nil {
			targetTxPayload, _ := pair.triggerTx.MarshalBinary()

			txBundles = make([]*TxBundle, bundleCount)
			for i := 0; i < bundleCount; i++ {
				txBundles[i] = &TxBundle{
					txs:           [][]byte{targetTxPayload},
					blkNumber:     blockNumber + 1,
					blkCount:      2,
					revertableTxs: []common.Hash{pair.triggerTx.Hash()},
				}
			}

			txBundleBroadcast = &TxBundle{
				txs:           [][]byte{targetTxPayload},
				blkNumber:     blockNumber + 1,
				blkCount:      2,
				revertableTxs: []common.Hash{pair.triggerTx.Hash()},
			}
		} else {
			txBundles = make([]*TxBundle, bundleCount)
			for i := 0; i < bundleCount; i++ {
				txBundles[i] = &TxBundle{
					txs:           [][]byte{},
					blkNumber:     blockNumber + 1,
					blkCount:      2,
					revertableTxs: []common.Hash{},
				}
			}
			txBundleBroadcast = &TxBundle{
				txs:           [][]byte{},
				blkNumber:     blockNumber + 1,
				blkCount:      2,
				revertableTxs: []common.Hash{},
			}
		}

		if !usingCustomSlippage && config.broadcastCount > 0 {
		} else {
			txBundleBroadcast = nil
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

		maxSlippageAmountIn := big.NewInt(0).Add(
			maxAmountIn,
			big.NewInt(0).Div(
				maxAmountIn,
				big.NewInt(2),
			),
		)

		for i := 0; i < int(buyCounts[slippageIdx]); i++ {
			account := config.wallets[i]

			nonce, err := txApi.GetTransactionCount(context.Background(), account.address, rpc.BlockNumberOrHashWithNumber(rpc.PendingBlockNumber))
			if err != nil {
				fmt.Println("Error in get nonce", err)
				continue
			}

			buyTxPayloadInput, err := scamchecker.uniswapv2.abi.Pack("swapETHForExactTokens", buyInfo.tokenAmountsToBuy[i], path, account.address, big.NewInt(time.Now().Unix()+200))
			if err != nil {
				fmt.Println("Error in pack buy input", err)
				return false
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

				if txBundleBroadcast != nil && broadcastCount == 0 { //< config.broadcastCount {
					LogFgr.Println("   [! - Broadcast individually]", account.szAddress)
					// txApi.SendRawTransaction(context.Background(), buyTxPayload)
					txBundleBroadcast.txs = append(txBundleBroadcast.txs, buyTxPayload)
					broadcastCount = broadcastCount + 1
				}
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

		if !canBuy {
			fmt.Println("Buy not enabled")
			return false
		} else {
			go SendRawTransaction(txBundles)
			if txBundleBroadcast != nil {
				go SendRawTransaction([]*TxBundle{txBundleBroadcast})
			}
		}
	}

	if canBuy && len(slippages) > 0 {
		var initial_price float64 = 0
		if pair.initialPrice != nil {
			initial_price, _ = pair.initialPrice.Float64()
		}
		price_up, _ := pair.priceIncreaseTimes.Float64()
		msg := PairBoughtEvent{
			Token0:         pair.token.Hex(),
			Token1:         pair.baseToken.Hex(),
			Pair:           pair.address.Hex(),
			Owner:          "",
			LiquidityOwner: "",
			TokenName:      pair.name,
			TokenSymbol:    pair.symbol,
			TokenDecimals:  pair.decimals,
			AdditionalInfo: PairBoughtEventAdditionalInfo{
				Accounts:     []string{},
				BlockNumber:  int(blockNumber),
				TotalBuyers:  pair.buyerCount,
				TotalSwaps:   pair.buyCount,
				InitialPrice: initial_price,
				PriceUp:      price_up,
				BuyFee:       buyInfo.buyFeeInPercent,
				SellFee:      buyInfo.sellFeeInPercent,
				AmountIn:     buyInfo.buyAmount,
				Tip:          slippageTips,
				Slippage:     slippages,
			},
		}
		if pair.owner != nil {
			msg.Owner = pair.owner.Hex()
		}
		if pair.liquidityOwner != nil {
			msg.LiquidityOwner = pair.liquidityOwner.Hex()
		}
		if conf != nil {
			szMsg, err := json.Marshal(msg)
			if err == nil {
				go conf.Publish("channel:token-bought", string(szMsg))
			}
			fmt.Println()
		}
	}

	pair.bought = true
	pair.status = 1

	pair.boughtBlkNo = uint64(blockNumber + 1) // next block number is possibly the target no

	// dark slayer, start watching bought token
	darkslayer.AddPair(pair)

	return true
}

func GetTimeDurationLeftToNextBlockMined(currentHead *types.Header, buyTriggerInSeconds int) time.Duration {
	latestBlkTimestamp := currentHead.Time
	now := time.Now()
	diffIn := now.Sub(time.Unix(int64(latestBlkTimestamp), 0))

	if buyTriggerInSeconds == 0 {
		buyTriggerInSeconds = 10
	}
	var holdOnDuration time.Duration = 0
	if diffIn.Milliseconds() < int64(buyTriggerInSeconds)*1000 {
		holdOnDuration = time.Duration(buyTriggerInSeconds)*time.Second - time.Duration(diffIn.Nanoseconds())
	}

	fmt.Println(fmt.Sprintf("Current time: %s, Latest Block: %s(%d), Hold on: %.3f", now, time.Unix(int64(latestBlkTimestamp), 0), currentHead.Number, holdOnDuration.Seconds()))
	return holdOnDuration
}

func isAddrInArray(addr *common.Address, list []*common.Address) bool {
	for _, blockedOrigin := range list {
		if blockedOrigin != nil && blockedOrigin.Hex() == addr.Hex() {
			return true
		}
	}
	return false
}
func isBlockedOrigin(pair *DTPair, config *DTConfig) bool {
	if pair.origin == nil || len(config.blockedOrigins) == 0 {
		return false
	}
	return isAddrInArray(pair.origin, config.blockedOrigins)
}

func CheckSpecialBuyMode(pair *DTPair, config *DTConfig, isFromTrader bool, conf *Configuration, currentHead *types.Header, bcApi *ethapi.BlockChainAPI, erc20 *Erc20, scamchecker *ScamChecker, txApi *ethapi.TransactionAPI, darkslayer *DarkSlayer) bool {
	/*
		enabled                bool
		minBuyFee              uint64
		maxBuyFee              uint64
		minSellFee             uint64
		maxSellFee             uint64
		minTotalFee            uint64
		maxTotalFee            uint64
		origins                []string
		maxPriceIncreasedTimes float64
		tip                    float64
		minEthReserve          float64
		maxEthReserve          float64
		minEthIn               float64
		maxEthIn               float64
	*/
	// for modeIdx, specialMode := range config.specialModes {
	// 	if pair.origin != nil && !isAddrInArray(pair.origin, specialMode.origins) {
	// 		// fmt.Println("[SpecialBuyMode -", (modeIdx + 1), "] - Ignore. Origin not in the range", pair.origin, "(", pair.originName, ")")
	// 		continue
	// 	}
	// 	if specialMode.minEthReserve.Cmp(pair.initialBaseReserve) > 0 || specialMode.maxEthReserve.Cmp(pair.initialBaseReserve) < 0 {
	// 		fmt.Println("[SpecialBuyMode -", (modeIdx + 1), "] - LP out of range!", ViewableEthAmount(pair.initialBaseReserve), "(", ViewableEthAmount(specialMode.minEthReserve), "~", ViewableEthAmount(specialMode.maxEthReserve), ")")
	// 		continue
	// 	}
	// 	if !isFromTrader && specialMode.maxPriceIncreasedTimes < pair.sniperDetectionPriceUp {
	// 		fmt.Println("[SpecialBuyMode -", (modeIdx + 1), "] - Price up too high!", pair.sniperDetectionPriceUp, ">", specialMode.maxPriceIncreasedTimes)
	// 		continue
	// 	}

	// 	maxBuyAmount := big.NewInt(0).Add(big.NewInt(0), config.maxBuyAmount)
	// 	minBuyAmount := big.NewInt(0).Add(big.NewInt(0), config.minBuyAmount)
	// 	if !isFromTrader && pair.sniperDetectionPriceUp > 0 {
	// 		big.NewFloat(0).Mul(new(big.Float).SetInt(maxBuyAmount), new(big.Float).SetFloat64(pair.sniperDetectionPriceUp)).Int(config.maxBuyAmount)
	// 		big.NewFloat(0).Mul(new(big.Float).SetInt(minBuyAmount), new(big.Float).SetFloat64(pair.sniperDetectionPriceUp)).Int(config.minBuyAmount)
	// 	}
	// 	buyInfo := CheckTokenTxs(currentHead, pair, nil, bcApi, erc20, scamchecker, txApi, config, false)
	// 	config.maxBuyAmount = maxBuyAmount
	// 	config.minBuyAmount = minBuyAmount
	// 	if buyInfo == nil {
	// 		fmt.Println("[SpecialBuyMode -", (modeIdx + 1), "] - Cant decide the buyInfo")
	// 		continue
	// 	}
	// 	if buyInfo.buyFeeInPercent > specialMode.maxBuyFee || buyInfo.buyFeeInPercent < specialMode.minBuyFee {
	// 		fmt.Println("[SpecialBuyMode -", (modeIdx + 1), "] - Buy fee out of range!", buyInfo.buyFeeInPercent, "(", specialMode.minBuyFee, "~", specialMode.maxBuyFee, ")")
	// 		continue
	// 	}
	// 	if buyInfo.sellFeeInPercent > specialMode.maxSellFee || buyInfo.sellFeeInPercent < specialMode.minSellFee {
	// 		fmt.Println("[SpecialBuyMode -", (modeIdx + 1), "] - Sell fee out of range!", buyInfo.sellFeeInPercent, "(", specialMode.minSellFee, "~", specialMode.maxSellFee, ")")
	// 		continue
	// 	}
	// 	if buyInfo.totalFeeInPercent > specialMode.maxTotalFee || buyInfo.totalFeeInPercent < specialMode.minTotalFee {
	// 		fmt.Println("[SpecialBuyMode -", (modeIdx + 1), "] - Total fee out of range!", buyInfo.totalFeeInPercent, "(", specialMode.minTotalFee, "~", specialMode.maxTotalFee, ")")
	// 		continue
	// 	}

	// 	originAddr := ""
	// 	if pair.origin != nil {
	// 		originAddr = pair.origin.Hex()
	// 	}
	// 	LogFgr.Println(fmt.Sprintf("[DarkTrader] - [Found Special Buy] %s(%s), Origin %s(%s) - %.3f, # Fee - %d%%+%d%%=%d%%", pair.symbol, pair.token.Hex(), originAddr, pair.originName, pair.sniperDetectionPriceUp, buyInfo.buyFeeInPercent, buyInfo.sellFeeInPercent, buyInfo.totalFeeInPercent))

	// 	maxAmountIn := big.NewInt(0)
	// 	if isFromTrader || pair.sniperDetectionPriceUp == 0 {
	// 		maxAmountIn = new(big.Int).Div(
	// 			new(big.Int).Mul(
	// 				buyInfo.buyAmount,
	// 				new(big.Int).SetInt64(int64(specialMode.maxPriceIncreasedTimes*100)),
	// 			),
	// 			big.NewInt(100),
	// 		)
	// 	} else {
	// 		maxAmountIn = new(big.Int).Div(
	// 			new(big.Int).Mul(
	// 				buyInfo.buyAmount,
	// 				new(big.Int).SetInt64(int64(specialMode.maxPriceIncreasedTimes*10000/pair.sniperDetectionPriceUp)),
	// 			),
	// 			big.NewInt(10000),
	// 		)
	// 	}
	// 	blockNumber := currentHead.Number.Int64()

	// 	nextBaseFee := CalcNextBaseFee(currentHead)

	// 	// go pair.swapTxs.WriteToLog(pair)

	// 	PrintBuyInfo(pair, buyInfo.buyAmount, buyInfo.buyCount, buyInfo.tokenAmountExpected, buyInfo.tokenAmountActual, buyInfo.buyFeeInPercent, buyInfo.sellFeeInPercent)

	// 	path, _ := BuildSwapPath(pair)

	// 	fmt.Println("Buy amount in", buyInfo.buyAmount, "Max: ", maxAmountIn)
	// 	buyCount := uint64(uint64(buyInfo.buyCount) * specialMode.maxBuyCount / 100)
	// 	if buyCount <= 0 {
	// 		buyCount = 1
	// 	}
	// 	fmt.Println("Buy count", buyCount)
	// 	fmt.Println("Tip: ", specialMode.tip)

	// 	if !specialMode.enabled {
	// 		fmt.Println("[SpecialBuyMode -", (modeIdx + 1), "] Mode not enabled")
	// 		continue
	// 	}

	// 	// buy
	// 	var (
	// 		txBundles      []*TxBundle
	// 		boughtAccounts []string
	// 	)
	// 	bundleCount := 1
	// 	if pair.triggerTx != nil {
	// 		targetTxPayload, _ := pair.triggerTx.MarshalBinary()

	// 		txBundles = make([]*TxBundle, bundleCount)
	// 		for i := 0; i < bundleCount; i++ {
	// 			txBundles[i] = &TxBundle{
	// 				txs:           [][]byte{targetTxPayload},
	// 				blkNumber:     blockNumber + 1,
	// 				blkCount:      2,
	// 				revertableTxs: []common.Hash{pair.triggerTx.Hash()},
	// 			}
	// 		}

	// 	} else {
	// 		txBundles = make([]*TxBundle, bundleCount)
	// 		for i := 0; i < bundleCount; i++ {
	// 			txBundles[i] = &TxBundle{
	// 				txs:           [][]byte{},
	// 				blkNumber:     blockNumber + 1,
	// 				blkCount:      2,
	// 				revertableTxs: []common.Hash{},
	// 			}
	// 		}
	// 	}
	// 	var maxGas uint64 = 500000

	// 	maxSlippageAmountIn := big.NewInt(0).Add(
	// 		maxAmountIn,
	// 		big.NewInt(0).Div(
	// 			maxAmountIn,
	// 			big.NewInt(2),
	// 		),
	// 	)
	// 	for i := 0; i < int(buyCount); i++ {
	// 		account := config.wallets[i]

	// 		nonce, err := txApi.GetTransactionCount(context.Background(), account.address, rpc.BlockNumberOrHashWithNumber(rpc.PendingBlockNumber))
	// 		if err != nil {
	// 			fmt.Println("Error in get nonce", err)
	// 			continue
	// 		}

	// 		buyTxPayloadInput, err := scamchecker.uniswapv2.abi.Pack("swapETHForExactTokens", buyInfo.tokenAmountsToBuy[i], path, account.address, big.NewInt(time.Now().Unix()+200))
	// 		if err != nil {
	// 			fmt.Println("Error in pack buy input", err)
	// 			return false
	// 		}

	// 		amountToSend := maxSlippageAmountIn
	// 		if i == 0 {
	// 			amountToSend = maxAmountIn
	// 		}
	// 		gasTip := new(big.Int).Mul(
	// 			big.NewInt(int64(specialMode.tip*100)),
	// 			big.NewInt(10000000),
	// 		)
	// 		// include buy tx to the bundle
	// 		for j := 0; j < len(txBundles); j++ {
	// 			dfBuyTx := types.DynamicFeeTx{
	// 				ChainID:   CHAINID,
	// 				Nonce:     uint64(*nonce),
	// 				GasTipCap: gasTip,
	// 				GasFeeCap: big.NewInt(1).Add(nextBaseFee, gasTip),
	// 				Gas:       maxGas,
	// 				To:        &v2RouterAddrObj,
	// 				Value:     amountToSend,
	// 				Data:      buyTxPayloadInput,
	// 			}
	// 			buyTx := types.NewTx(types.TxData(&dfBuyTx))
	// 			signedBuyTx, err := types.SignTx(buyTx, types.LatestSignerForChainID(buyTx.ChainId()), account.key)
	// 			if err != nil {
	// 				fmt.Println("Error in signing tx", err)
	// 				break
	// 			}

	// 			buyTxPayload, _ := signedBuyTx.MarshalBinary()
	// 			txBundles[j].txs = append(txBundles[j].txs, buyTxPayload)
	// 		}

	// 		boughtAccounts = append(boughtAccounts, account.szAddress)
	// 	}

	// 	// bundling other txs
	// 	// for i := 0; i < len(txBundles); i++ {
	// 	// 	for _, bundleTx := range buyInfo.txsToBundle {
	// 	// 		otherTxPayload, _ := bundleTx.MarshalBinary()
	// 	// 		txBundles[i].txs = append(txBundles[i].txs, otherTxPayload)
	// 	// 		txBundles[i].revertableTxs = append(txBundles[i].revertableTxs, bundleTx.Hash())
	// 	// 	}
	// 	// }

	// 	go SendRawTransaction(txBundles)

	// 	var initial_price float64 = 0
	// 	if pair.initialPrice != nil {
	// 		initial_price, _ = pair.initialPrice.Float64()
	// 	}
	// 	price_up, _ := pair.priceIncreaseTimes.Float64()
	// 	msg := PairBoughtEvent{
	// 		Token0:         pair.token.Hex(),
	// 		Token1:         pair.baseToken.Hex(),
	// 		Pair:           pair.address.Hex(),
	// 		Owner:          "",
	// 		LiquidityOwner: "",
	// 		TokenName:      pair.name,
	// 		TokenSymbol:    pair.symbol,
	// 		TokenDecimals:  pair.decimals,
	// 		AdditionalInfo: PairBoughtEventAdditionalInfo{
	// 			Accounts:     []string{},
	// 			BlockNumber:  int(blockNumber),
	// 			TotalBuyers:  pair.buyerCount,
	// 			TotalSwaps:   pair.buyCount,
	// 			InitialPrice: initial_price,
	// 			PriceUp:      price_up,
	// 			BuyFee:       buyInfo.buyFeeInPercent,
	// 			SellFee:      buyInfo.sellFeeInPercent,
	// 			AmountIn:     buyInfo.buyAmount,
	// 			Tip:          []float64{specialMode.tip},
	// 			Slippage:     []uint64{uint64(100 / specialMode.maxPriceIncreasedTimes)},
	// 		},
	// 	}
	// 	if pair.owner != nil {
	// 		msg.Owner = pair.owner.Hex()
	// 	}
	// 	if pair.liquidityOwner != nil {
	// 		msg.LiquidityOwner = pair.liquidityOwner.Hex()
	// 	}
	// 	if conf != nil {
	// 		szMsg, err := json.Marshal(msg)
	// 		if err == nil {
	// 			go conf.Publish("channel:token-bought", string(szMsg))
	// 		}
	// 		fmt.Println()
	// 	}

	// 	pair.bought = true
	// 	pair.status = 1

	// 	pair.boughtBlkNo = uint64(blockNumber + 1) // next block number is possibly the target no

	// 	// dark slayer, start watching bought token
	// 	darkslayer.AddPair(pair)

	// 	return true
	// }
	return false
}
