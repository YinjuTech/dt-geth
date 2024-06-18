package darktrader

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/status-im/keycard-go/hexutils"
)

type DarkSlayerConfig struct {
	isAddBotEnabled    bool
	isRemoveLpEnabled  bool
	isOtherEnabled     bool
	scamTxMinFeeInGwei float64
	watchBlockCount    uint64
}

type PairTrackInfo struct {
	buyCount  int
	sellCount int
	price     *big.Float
}

type DarkSlayer struct {
	pairs      map[string]*DTPair
	tokens     []string
	owners     map[string]string
	pairsMutex sync.RWMutex

	tokensByPair      map[string]string
	tokensByPairMutex sync.RWMutex

	pairsTrackInfo map[string](map[uint64]*PairTrackInfo)

	config *DTConfig

	conf        *Configuration
	erc20       *Erc20
	uniswapv2   *UniswapV2
	scamchecker *ScamChecker

	bc    blockChain
	bcApi *ethapi.BlockChainAPI
	txApi *ethapi.TransactionAPI

	subWalletIdx map[string]int
}

func NewDarkSlayer(conf *Configuration, erc20 *Erc20, uniswapv2 *UniswapV2, scamchecker *ScamChecker, bc blockChain, bcApi *ethapi.BlockChainAPI, txApi *ethapi.TransactionAPI) *DarkSlayer {
	slayer := DarkSlayer{}

	slayer.Init(conf, erc20, uniswapv2, scamchecker, bc, bcApi, txApi)

	return &slayer
}

func (this *DarkSlayer) SetConf(conf *Configuration) {
	this.conf = conf
}

func (this *DarkSlayer) SetConfig(config *DTConfig) {
	this.config = config
}

func (this *DarkSlayer) Init(conf *Configuration, erc20 *Erc20, uniswapv2 *UniswapV2, scamchecker *ScamChecker, bc blockChain, bcApi *ethapi.BlockChainAPI, txApi *ethapi.TransactionAPI) {
	this.conf = conf

	this.erc20 = erc20
	this.uniswapv2 = uniswapv2
	this.scamchecker = scamchecker

	this.bc = bc
	this.bcApi = bcApi
	this.txApi = txApi

	this.pairs = make(map[string]*DTPair)
	this.tokens = []string{}
	this.owners = make(map[string]string)
	this.pairsMutex = sync.RWMutex{}
	this.pairsTrackInfo = make(map[string]map[uint64]*PairTrackInfo)

	this.tokensByPair = make(map[string]string)
	this.tokensByPairMutex = sync.RWMutex{}

	this.subWalletIdx = make(map[string]int)
}

func (this *DarkSlayer) shouldUnWatchPairForAccount(pair *DTPair, account *DTAccount) bool {
	amount, err := this.erc20.callBalanceOf(&account.address, pair.token, rpc.LatestBlockNumber, nil)
	if err != nil {
		// for some reason, it's possible to get the balance of the token, wait until next block
		return false
	}
	if amount.Cmp(big.NewInt(0)) > 0 {
		return false
	}
	return true
}
func (this *DarkSlayer) ShouldUnWatchPair(pair *DTPair, blockNumber uint64) bool {
	if !pair.bought {
		return true
	}
	if blockNumber > pair.boughtBlkNo+3 {
		reserveBase, _, _ := this.erc20.GetPairReserves(pair.address, pair.baseToken, pair.token, rpc.LatestBlockNumber)
		if reserveBase != nil && reserveBase.Cmp(this.config.minEthReserve) < 0 {
			fmt.Println("[DarkSlayer] - LP removed/too small", pair.token.Hex(), "("+pair.symbol+")")
			return true
		}
	}
	if blockNumber > pair.boughtBlkNo+this.config.darkSlayer.watchBlockCount*100 {
		// if passed more than darkSlayer.watchBlockCount * 100, force unwatch regardless of bought
		return true
	}
	if blockNumber > pair.boughtBlkNo+this.config.darkSlayer.watchBlockCount {
		for _, account := range this.config.wallets {
			if !this.shouldUnWatchPairForAccount(pair, account) {
				return false
			}
		}
		for _, account := range this.config.subWallets {
			if !this.shouldUnWatchPairForAccount(pair, account) {
				return false
			}
		}
		// this means, we were not able to buy or have already sold out, so all account balance of this token is zero
		// ? - should darksniper start watching this pair for later raising?(even after we bought and sold)
		return true
	}
	// still need to check further whether we will buy
	return false
}

func (this *DarkSlayer) CheckBoughtStatus(pair *DTPair, blkNo uint64, head *types.Block) {
	if pair.pairBoughtInfo != nil && len(pair.pairBoughtInfo) > 0 {
		return
	}
	pair.pairBoughtInfo = make([]*DTPairBoughtInfo, 0)
	// txs := head.Transactions()
	for _, account := range this.config.wallets {
		amount, err := this.erc20.callBalanceOf(&account.address, pair.token, rpc.LatestBlockNumber, nil)
		if err != nil {
			continue
		}
		if amount.Cmp(big.NewInt(0)) > 0 {
			var buyTx *types.Transaction = nil
			// for _, tx := range txs {
			// 	txFrom, err := GetFrom(tx)
			// 	if err != nil || txFrom.Hex() != account.szAddress {
			// 		continue
			// 	}
			// 	buyTx = tx
			// 	break
			// }

			boughtInfo := DTPairBoughtInfo{
				wallet:         account,
				tx:             buyTx,
				amountIn:       nil,
				amountOut:      nil,
				tokenAmount:    amount,
				fee:            nil,
				blockNumber:    blkNo,
				routerApproved: false,
			}
			if buyTx != nil {
				receipt, err := this.txApi.GetTransactionReceipt(context.Background(), buyTx.Hash())
				if err == nil {
					effectiveGasPrice := receipt["effectiveGasPrice"].(*hexutil.Big).ToInt()
					gasUsed := uint64(receipt["gasUsed"].(hexutil.Uint64))
					boughtInfo.fee = new(big.Int).Mul(effectiveGasPrice, new(big.Int).SetUint64(gasUsed))
					var inputs map[string]interface{} = make(map[string]interface{})
					for _, log := range receipt["logs"].([]*types.Log) {
						if len(log.Topics) != 3 || log.Topics[0].Hex() != uniswapSwapTopic {
							continue
						}
						this.uniswapv2.abiPair.Events["Swap"].Inputs.UnpackIntoMap(inputs, log.Data)
						var (
							amount0In  *big.Int
							amount1In  *big.Int
							amount0Out *big.Int
							amount1Out *big.Int
						)
						amount0In = inputs["amount0In"].(*big.Int)
						amount1In = inputs["amount1In"].(*big.Int)
						amount0Out = inputs["amount0Out"].(*big.Int)
						amount1Out = inputs["amount1Out"].(*big.Int)
						if amount0In.Cmp(big.NewInt(0)) == 0 {
							boughtInfo.amountIn = amount1In
							boughtInfo.amountOut = amount0Out
						} else {
							boughtInfo.amountIn = amount0In
							boughtInfo.amountOut = amount1Out
						}
						break
					}
				}
			}
			pair.pairMutex.Lock()
			pair.pairBoughtInfo = append(pair.pairBoughtInfo, &boughtInfo)
			pair.pairMutex.Unlock()
		}
	}
}

func (this *DarkSlayer) trackPairInfo(head *types.Block, pairCheck *DTPair) {
	txs := head.Transactions()
	blockNumber := head.NumberU64()
	if pairCheck == nil {
		this.pairsMutex.Lock()
		for _, pair := range this.pairs {
			if _, exists := this.pairsTrackInfo[pair.token.Hex()]; !exists {
				this.pairsTrackInfo[pair.token.Hex()] = make(map[uint64]*PairTrackInfo)
			}
			this.pairsTrackInfo[pair.token.Hex()][blockNumber] = &PairTrackInfo{
				buyCount:  0,
				sellCount: 0,
				price:     big.NewFloat(0),
			}
			baseReserve, tokenReserve, err := this.erc20.GetPairReserves(pair.address, pair.baseToken, pair.token, rpc.LatestBlockNumber)
			if err != nil || tokenReserve.Cmp(big.NewInt(0)) == 0 {
			}
			pair.baseReserve = baseReserve
			pair.tokenReserve = tokenReserve
			if err != nil || tokenReserve.Cmp(big.NewInt(0)) == 0 {
			} else {
				this.pairsTrackInfo[pair.token.Hex()][blockNumber].price = new(big.Float).Quo(
					new(big.Float).SetInt(baseReserve),
					new(big.Float).SetInt(tokenReserve),
				)
			}
		}
		this.pairsMutex.Unlock()
	} else {
		if _, exists := this.pairsTrackInfo[pairCheck.token.Hex()]; !exists {
			this.pairsTrackInfo[pairCheck.token.Hex()] = make(map[uint64]*PairTrackInfo)
		}
		this.pairsTrackInfo[pairCheck.token.Hex()][blockNumber] = &PairTrackInfo{
			buyCount:  0,
			sellCount: 0,
			price:     big.NewFloat(0),
		}
		baseReserve, tokenReserve, err := this.erc20.GetPairReserves(pairCheck.address, pairCheck.baseToken, pairCheck.token, rpc.LatestBlockNumber)
		if err != nil || tokenReserve.Cmp(big.NewInt(0)) == 0 {
		} else {
			this.pairsTrackInfo[pairCheck.token.Hex()][blockNumber].price = new(big.Float).Quo(
				new(big.Float).SetInt(baseReserve),
				new(big.Float).SetInt(tokenReserve),
			)
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
					pair := this.getPairByAddress(&log.Address)
					if pair != nil && (pairCheck == nil || pairCheck.token.Hex() == pair.token.Hex()) {
						isBuy, _, _, _, _ := this.scamchecker.uniswapv2.ParseSwapTx(log, receipt, pair, tx)
						if _, exists := this.pairsTrackInfo[pair.token.Hex()][blockNumber]; !exists {
							baseReserve, tokenReserve, err := this.erc20.GetPairReserves(pair.address, pair.baseToken, pair.token, rpc.LatestBlockNumber)
							if err != nil || tokenReserve.Cmp(big.NewInt(0)) == 0 {
							} else {
								price := new(big.Float).Quo(
									new(big.Float).SetInt(baseReserve),
									new(big.Float).SetInt(tokenReserve),
								)
								this.pairsTrackInfo[pair.token.Hex()][blockNumber] = &PairTrackInfo{
									buyCount:  0,
									sellCount: 0,
									price:     price,
								}
							}
						}
						if isBuy {
							this.pairsTrackInfo[pair.token.Hex()][blockNumber].buyCount++
						} else {
							this.pairsTrackInfo[pair.token.Hex()][blockNumber].sellCount++
						}
						break
					}
				}
			}
		}
	}
}

func (this *DarkSlayer) CheckEventLogs(head *types.Block, blkLogs []*types.Log) {
	if !this.config.isRunning {
		return
	}
	blkNo := head.Number().Uint64()

	// track info
	this.trackPairInfo(head, nil)

	tip1 := big.NewInt(1).Mul(big.NewInt(1), big.NewInt(DECIMALS/1000000000)) // 0.1 GWEI
	for _, pair := range this.pairs {
		if pair == nil {
			continue
		}
		// print out current price up times
		var priceUp int = 0

		curPrice, _ := this.pairsTrackInfo[pair.token.Hex()][blkNo].price.Float64()
		initialPrice, _ := pair.initialPrice.Float64()
		priceUp = int(curPrice*10000/initialPrice)/100 - 100
		aggBuyCount := 0
		aggSellCount := 0
		for i := uint64(0); i < uint64(this.config.darkJumper.txCountConsiderBlockCount); i++ {
			trackInfo, exists := this.pairsTrackInfo[pair.token.Hex()][blkNo-i]
			if !exists {
				continue
			}
			aggBuyCount = aggBuyCount + trackInfo.buyCount
			aggSellCount = aggSellCount + trackInfo.sellCount
		}
		fmt.Println(LogFbOb.Sprintf("[DarkSlayer] - %s Up++:", pair.symbol) + " " + LogFgr.Sprintf("%d%%  (LP %.2f), #: %d/%d(%d/%d)", priceUp, ViewableEthAmount(pair.baseReserve), this.pairsTrackInfo[pair.token.Hex()][blkNo].buyCount, this.pairsTrackInfo[pair.token.Hex()][blkNo].sellCount, aggBuyCount, aggSellCount))
		//
		this.CheckBoughtStatus(pair, blkNo, head)
		shouldRemove := this.ShouldUnWatchPair(pair, blkNo)
		if shouldRemove {
			this.RemovePair(pair)

			fmt.Println("[DarkSlayer] - Stopped watching token", pair.token.Hex(), pair.symbol)
			this.conf.SendMessage("", pair.token.Hex(), MSG_TYPE_DANGER, "", "STOP_WATCH_TOKEN", "")
		} else {
			// If bug, comment following
			if pair.pairBoughtInfo != nil && len(pair.pairBoughtInfo) > 0 && pair.pairBoughtInfo[0].blockNumber+3 >= blkNo {
				var bundles [][]byte = make([][]byte, 0)
				maxSellCount := int(float64(priceUp)/1000 + 0.15)
				if priceUp < 1000 {
					maxSellCount = 0
				}
				tip10 := big.NewInt(1).Mul(big.NewInt(int64(priceUp/1000*10+1)), big.NewInt(DECIMALS/1000000000)) // 11 GWEI
				if maxSellCount > 3 {
					maxSellCount = 3
				}
				for i, boughtInfo := range pair.pairBoughtInfo {
					// if boughtInfo.tokenAmount == nil {
					boughtInfo.tokenAmount, _ = this.erc20.callBalanceOf(&boughtInfo.wallet.address, pair.token, rpc.LatestBlockNumber, nil)
					// }
					if boughtInfo.tokenAmount == nil || boughtInfo.tokenAmount.Cmp(big.NewInt(0)) == 0 {
						continue
					}

					if boughtInfo.routerApproved == false {
						fmt.Println(i, boughtInfo.wallet.address.Hex(), boughtInfo.tokenAmount, boughtInfo.routerApproved)
						allowance, err := this.erc20.callAllowance(&boughtInfo.wallet.address, pair.token, &boughtInfo.wallet.address, &v2RouterAddrObj, rpc.LatestBlockNumber, nil)
						if err == nil && allowance.Cmp(boughtInfo.tokenAmount) >= 0 {
							boughtInfo.routerApproved = true
						}
					}
					var justApproved = false
					if boughtInfo.routerApproved == false {
						payload, err := BuildApproveTx(head.Header(), this.txApi, boughtInfo.wallet, &v2RouterAddrObj, pair, tip1, 100000, this.erc20)
						if err == nil {
							bundles = append(bundles, payload)
							justApproved = true
							boughtInfo.routerApproved = true
						} else {
							fmt.Println("[DarkSlayer] - build approve tx", err)
						}
					}
					if priceUp/100 >= 10 && i < maxSellCount && pair.autoSellEnabled {
						nonceOffset := uint64(0)
						if justApproved {
							nonceOffset = 1
						}
						_, sellAmountActual, err := this.scamchecker.CalcSellAmount(pair, &boughtInfo.wallet.address, boughtInfo.tokenAmount, rpc.LatestBlockNumber, nil)
						sellAmountActual.Mul(sellAmountActual, big.NewInt(90))
						sellAmountActual.Div(sellAmountActual, big.NewInt(100)) // min amount out 90%

						payload, err := BuildSellTx(head.Header(), this.erc20, this.scamchecker.uniswapv2, this.txApi, pair, boughtInfo.wallet, &boughtInfo.wallet.address, sellAmountActual, nil, tip10, 0, nonceOffset)
						if err != nil {
							fmt.Println("[DarkSlayer] - Quick selling", err)
						} else {
							LogFwBgOb.Println("[DarkSlayer] - (", pair.token, pair.symbol, ") Enough pricing up. Selling account", boughtInfo.wallet.address.Hex(), "Expecting", ViewableEthAmount(sellAmountActual))
							bundles = append(bundles, payload)
						}
					}
				}

				if len(bundles) > 0 {
					this.sendBundle(bundles, nil, 1)
				}
			}
			// end of buggable code
		}
	}
}

func (this *DarkSlayer) onDetectTokenSwap(log *types.Log, receipt map[string]interface{}, tx *types.Transaction) {
	// currentHead := this.bc.CurrentHeader()
	// this.pairsMutex.Lock()
	// for _, pair := range this.pairs {
	// 	if pair == nil || pair.address.Hex() != log.Address.Hex() {
	// 		continue
	// 	}
	// 	isBuy, amountIn, amountOut, fee, to := this.scamchecker.uniswapv2.ParseSwapTx(log, receipt, pair, tx)
	// 	if isBuy {
	// 		fmt.Println("Bought detected", pair.symbol, to.Hex())
	// 		for _, wallet := range this.config.wallets {
	// 			if wallet.szAddress == to.Hex() {
	// 				tokenAmount, _ := this.erc20.callBalanceOf(&wallet.address, pair.token, rpc.LatestBlockNumber, nil)
	// 				pair.pairBoughtInfo = append(pair.pairBoughtInfo, &DTPairBoughtInfo{
	// 					wallet:         &wallet,
	// 					tx:             tx,
	// 					amountIn:       amountIn,
	// 					amountOut:      amountOut,
	// 					tokenAmount:    tokenAmount,
	// 					fee:            fee,
	// 					blockNumber:    currentHead.Number.Uint64(),
	// 					routerApproved: false,
	// 				})
	// 				fmt.Println("Bought detected for ", wallet.szAddress)
	// 				break
	// 			}
	// 		}
	// 	}
	// 	break
	// }
	// this.pairsMutex.Unlock()
}

func (this *DarkSlayer) processPendingRemoveLiquidity(tx *types.Transaction) bool {
	data := tx.Data()
	if len(data) < 4 {
		return false
	}

	methodSig := data[:4]

	var token *common.Address
	var liquidityToRemove *big.Int
	if bytes.Equal(methodSig, this.uniswapv2.abi.Methods["removeLiquidity"].ID) {
		args, err := this.uniswapv2.abi.Methods["removeLiquidity"].Inputs.Unpack(data[4:])
		if err != nil {
			return false
		}

		if args[0] == WETH_ADDRESS.Hex() {
			token1 := args[1].(common.Address)
			token = &token1
		} else if args[1] == WETH_ADDRESS.Hex() {
			token1 := args[0].(common.Address)
			token = &token1
		} else {
			return false
		}

		liquidityToRemove = args[2].(*big.Int)
	} else if bytes.Equal(methodSig, this.uniswapv2.abi.Methods["removeLiquidityETH"].ID) {
		args, err := this.uniswapv2.abi.Methods["removeLiquidityETH"].Inputs.Unpack(data[4:])
		if err != nil {
			return false
		}

		token1 := args[0].(common.Address)
		token = &token1
		liquidityToRemove = args[1].(*big.Int)
	} else if bytes.Equal(methodSig, this.uniswapv2.abi.Methods["removeLiquidityWithPermit"].ID) {
		args, err := this.uniswapv2.abi.Methods["removeLiquidityWithPermit"].Inputs.Unpack(data[4:])
		if err != nil {
			return false
		}

		if args[0] == WETH_ADDRESS.Hex() {
			token1 := args[1].(common.Address)
			token = &token1
		} else if args[1] == WETH_ADDRESS.Hex() {
			token1 := args[0].(common.Address)
			token = &token1
		} else {
			return false
		}

		liquidityToRemove = args[2].(*big.Int)
	} else if bytes.Equal(methodSig, this.uniswapv2.abi.Methods["removeLiquidityETHWithPermit"].ID) {
		args, err := this.uniswapv2.abi.Methods["removeLiquidityETHWithPermit"].Inputs.Unpack(data[4:])
		if err != nil {
			return false
		}

		token1 := args[0].(common.Address)
		token = &token1
		liquidityToRemove = args[1].(*big.Int)
	} else if bytes.Equal(methodSig, this.uniswapv2.abi.Methods["removeLiquidityETHSupportingFeeOnTransferTokens"].ID) {
		args, err := this.uniswapv2.abi.Methods["removeLiquidityETHSupportingFeeOnTransferTokens"].Inputs.Unpack(data[4:])
		if err != nil {
			return false
		}

		token1 := args[0].(common.Address)
		token = &token1
		liquidityToRemove = args[1].(*big.Int)
	} else if bytes.Equal(methodSig, this.uniswapv2.abi.Methods["removeLiquidityETHWithPermitSupportingFeeOnTransferTokens"].ID) {
		args, err := this.uniswapv2.abi.Methods["removeLiquidityETHWithPermitSupportingFeeOnTransferTokens"].Inputs.Unpack(data[4:])
		if err != nil {
			return false
		}

		token1 := args[0].(common.Address)
		token = &token1
		liquidityToRemove = args[1].(*big.Int)
	}

	if token == nil {
		return false
	}

	pair, exists := this.pairs[token.Hex()]

	if !exists || !pair.bought {
		return false
	}

	_, err := this.erc20.CallTx(tx, nil, rpc.LatestBlockNumber, true)
	if err != nil {
		return false
	}

	// from, _ := GetFrom(tx)

	totalLpToken, _ := this.erc20.CallTotalSupply(pair.address, rpc.LatestBlockNumber, nil)

	if totalLpToken == nil || totalLpToken.Cmp(big.NewInt(0)) == 0 {
		fmt.Println("[TotalLpToken is zero]", pair.token, pair.name, pair.symbol, liquidityToRemove)
		return false
	}

	percentLpRemove := big.NewInt(1).Div(big.NewInt(1).Mul(liquidityToRemove, big.NewInt(100)), totalLpToken).Uint64()

	if percentLpRemove > 50 {
		LogFwBrOb.Println("[Detect LP removal]", "(", pair.token, pair.symbol, "), LP=", liquidityToRemove, "/", totalLpToken, "=", percentLpRemove, "%")
		this.conf.SendMessage(tx.Hash().Hex(), token.Hex(), MSG_TYPE_DANGER, "", "Detect LP removal", map[string]interface{}{
			"liquidityToRemove": liquidityToRemove,
			"totalToken":        totalLpToken,
			"percentToRemove":   percentLpRemove,
		})

		bundleTxs := this.buildSellTxs(pair, nil, this.bc.CurrentHeader())

		if len(bundleTxs) > 0 && this.config.darkSlayer.isRemoveLpEnabled {
			this.sendBundle(bundleTxs, tx, 2)
		}
	}
	return true
}

func (this *DarkSlayer) ProcessPendingTx(tx *types.Transaction) bool {
	txFrom, _ := GetFrom(tx)

	this.pairsMutex.RLock()
	szToken, exists := this.owners[txFrom.Hex()]
	this.pairsMutex.RUnlock()
	if !exists {
		return false
	}
	fmt.Println("[DarkSlayer] - Checking owner action", szToken, tx.Hash().Hex())

	data := tx.Data()

	this.pairsMutex.RLock()
	pair, exists := this.pairs[szToken]
	this.pairsMutex.RUnlock()
	if exists == false {
		return false
	}
	pair.triggerTx = tx

	// try tx
	// pair.triggerTxMutex.Lock()
	_, err := this.erc20.CallTx(tx, nil, rpc.LatestBlockNumber, true)
	// pair.triggerTxMutex.Unlock()
	if err != nil {
		return false
	}
	// TODO - Check set Fee high or something similar
	if len(data) == 0 && tx.Value().Cmp(big.NewInt(0)) > 0 && tx.To() != nil {
		LogFwBgOb.Println("[Owner action]-[Transfer] ", pair.symbol, "(", pair.token.Hex(), ")", tx.To().Hex(), ViewableEthAmount(tx.Value()), tx.Hash().Hex())
		this.conf.SendMessage(tx.Hash().Hex(), pair.token.Hex(), MSG_TYPE_GENERAL, "", fmt.Sprintf("Owner Transfer %.2f eth to %s", ViewableEthAmount(tx.Value()), tx.To().Hex()), tx.Hash().Hex())

		return true
	}

	if len(data) < 4 {
		return false
	}

	methodSig := data[:4]
	// if bytes.Equal(methodSig, this.erc20.abi.Methods["approve"].ID) || bytes.Equal(methodSig, this.erc20.abi.Methods["transferFrom"].ID) || bytes.Equal(methodSig, this.erc20.abi.Methods["transfer"].ID) {
	// 	return false
	// }

	bundleTxs := make([][]byte, 0)
	bytesHex := strings.ToLower(hexutils.BytesToHex(data))
	if tx.To().Hex() == pair.address.Hex() { // action on the pair
		LogFwBrOb.Println("[Owner action]-[Pair] ", pair.token, pair.symbol, tx.Hash().Hex())
		this.conf.SendMessage(tx.Hash().Hex(), pair.token.Hex(), MSG_TYPE_GENERAL, bytesHex, "Owner's Action on Pair", tx.Hash().Hex())
		method, err := this.scamchecker.abiUniswapV2Pair.MethodById(methodSig)
		if method == nil || err != nil {
			fmt.Println("error or method not found", err)
			return false
		}
		var out map[string]interface{} = make(map[string]interface{})
		err = method.Inputs.UnpackIntoMap(out, data[4:])
		if err != nil {
			fmt.Println("error on unpack", err)
		}
		if method.Name == "approve" {
			if out["spender"] != nil && out["spender"].(common.Address).Hex() == v2RouterAddr {
				LogFwBrOb.Println("     Approve uniswapV2Router", out)
				this.conf.SendMessage(tx.Hash().Hex(), pair.token.Hex(), MSG_TYPE_WARNING, bytesHex, "Approve on UniswapRouterV2", out)
			} else if out["spender"] != nil && out["spender"].(common.Address).Hex() == unicryptAddr.Hex() {
				LogFwBrOb.Println("     Approve Unicrypt", out)
				this.conf.SendMessage(tx.Hash().Hex(), pair.token.Hex(), MSG_TYPE_GOOD, bytesHex, "Approve on Unicrypt", out)
			} else if out["spender"] != nil && out["spender"].(common.Address).Hex() == teamFinanceAddr.Hex() {
				LogFwBrOb.Println("     Approve TeamFinance", out)
				this.conf.SendMessage(tx.Hash().Hex(), pair.token.Hex(), MSG_TYPE_GOOD, bytesHex, "Approve on TeamFinance", out)
			} else if out["spender"] != nil && out["spender"].(common.Address).Hex() == pinkLockAddr.Hex() {
				LogFwBrOb.Println("     Approve PinkLock", out)
				this.conf.SendMessage(tx.Hash().Hex(), pair.token.Hex(), MSG_TYPE_GOOD, bytesHex, "Approve on PinkLock", out)
			} else {
				LogFwBrOb.Println("     Approve ", out)
			}
		} else if method.Name == "transfer" {
			LogFwBrOb.Println("     Transfer LpToken", out)
			this.conf.SendMessage(tx.Hash().Hex(), pair.token.Hex(), MSG_TYPE_SUCCESS, bytesHex, "Transfer LpToken", out)
		} else {
			LogFwBrOb.Println("     ", method.Name, out)
			this.conf.SendMessage(tx.Hash().Hex(), pair.token.Hex(), MSG_TYPE_GENERAL, bytesHex, "LpToken->"+method.Name, out)
		}
	} else if tx.To().Hex() == unicryptAddr.Hex() { // action on the unicrypt
		LogFwBrOb.Println("[Owner action]-[Unicrypt] ", pair.token, pair.symbol, tx.Hash().Hex())
		this.conf.SendMessage(tx.Hash().Hex(), pair.token.Hex(), MSG_TYPE_GENERAL, bytesHex, "Owner's Action on Unicrypt", tx.Hash().Hex())
		method, err := this.scamchecker.abiUnicrypt.MethodById(methodSig)
		if method == nil || err != nil {
			fmt.Println("error or method not found", err)
			return false
		}
		var out map[string]interface{} = make(map[string]interface{})
		err = method.Inputs.UnpackIntoMap(out, data[4:])
		if err != nil {
			fmt.Println("error on unpack", err)
			return false
		}
		LogFwBrOb.Println("     ", method.Name, out)
		if method.Name == "lockLPToken" {
			totalLpToken, err := this.erc20.CallTotalSupply(pair.address, rpc.LatestBlockNumber, nil)
			if err != nil || totalLpToken == nil {
				fmt.Println("error on pair.totalSupply", err)
				return false
			}
			amountToLock := out["_amount"].(*big.Int)
			percentToLock := float64(new(big.Int).Div(new(big.Int).Mul(amountToLock, big.NewInt(10000)), totalLpToken).Int64()) / 100
			dateToUnlock := time.Unix(out["_unlock_date"].(*big.Int).Int64(), 0)
			LogFwBrOb.Println("     ", "amountToLock:", percentToLock, "%", "till: ", dateToUnlock)
			this.conf.SendMessage(tx.Hash().Hex(), pair.token.Hex(), MSG_TYPE_SUCCESS, bytesHex, "Lock LpToken on Unicrypt", map[string]interface{}{
				"amount": percentToLock,
				"till":   dateToUnlock,
			})
		}
	} else if tx.To().Hex() == teamFinanceAddr.Hex() { // action on the teamfinance
		LogFwBrOb.Println("[Owner action]-[TeamFinance] ", pair.token, pair.symbol, tx.Hash().Hex())
		this.conf.SendMessage(tx.Hash().Hex(), pair.token.Hex(), MSG_TYPE_GENERAL, bytesHex, "Owner's Action on TeamFinance", tx.Hash().Hex())
		method, err := this.scamchecker.abiTeamFinance.MethodById(methodSig)
		if method == nil || err != nil {
			fmt.Println("error or method not found", err)
			return false
		}
		var out map[string]interface{} = make(map[string]interface{})
		err = method.Inputs.UnpackIntoMap(out, data[4:])
		if err != nil {
			fmt.Println("error on unpack", err)
			return false
		}
		LogFwBrOb.Println("     ", method.Name, out)
		if method.Name == "lockToken" {
			totalLpToken, err := this.erc20.CallTotalSupply(pair.address, rpc.LatestBlockNumber, nil)
			if err != nil || totalLpToken == nil {
				fmt.Println("error on pair.totalSupply", err)
				return false
			}
			amountToLock := out["_amount"].(*big.Int)
			percentToLock := float64(new(big.Int).Div(new(big.Int).Mul(amountToLock, big.NewInt(10000)), totalLpToken).Int64()) / 100
			dateToUnlock := time.Unix(out["_unlockTime"].(*big.Int).Int64(), 0)
			LogFwBrOb.Println("     ", "amountToLock:", percentToLock, "%", "till: ", dateToUnlock)
			this.conf.SendMessage(tx.Hash().Hex(), pair.token.Hex(), MSG_TYPE_SUCCESS, bytesHex, "Lock LpToken on TeamFinance", map[string]interface{}{
				"amount": percentToLock,
				"till":   dateToUnlock,
			})
		}
	} else if tx.To().Hex() == pinkLockAddr.Hex() { //action on picklock
		LogFwBrOb.Println("[Owner action]-[PinkLock] ", pair.token, pair.symbol, tx.Hash().Hex())
		this.conf.SendMessage(tx.Hash().Hex(), pair.token.Hex(), MSG_TYPE_GENERAL, bytesHex, "Owner's Action on PinkLock", tx.Hash().Hex())
		method, err := this.scamchecker.abiPinkLock.MethodById(methodSig)
		if method == nil || err != nil {
			fmt.Println("error or method not found", err)
			return false
		}
		var out map[string]interface{} = make(map[string]interface{})
		err = method.Inputs.UnpackIntoMap(out, data[4:])
		if err != nil {
			fmt.Println("error on unpack", err)
			return false
		}
		LogFwBrOb.Println("     ", method.Name, out)
		if method.Name == "lock" {
			lpToken := out["token"].(common.Address)
			if lpToken.Hex() == pair.address.Hex() {
				totalLpToken, err := this.erc20.CallTotalSupply(pair.address, rpc.LatestBlockNumber, nil)
				if err != nil || totalLpToken == nil {
					fmt.Println("error on pair.totalSupply", err)
					return false
				}
				amountToLock := out["amount"].(*big.Int)
				percentToLock := float64(new(big.Int).Div(new(big.Int).Mul(amountToLock, big.NewInt(10000)), totalLpToken).Int64()) / 100
				dateToUnlock := time.Unix(out["unlockDate"].(*big.Int).Int64(), 0)
				LogFwBrOb.Println("     ", "amountToLock:", percentToLock, "%", "till: ", dateToUnlock)
				this.conf.SendMessage(tx.Hash().Hex(), pair.token.Hex(), MSG_TYPE_SUCCESS, bytesHex, "Lock LpToken on PinkLock", map[string]interface{}{
					"amount": percentToLock,
					"till":   dateToUnlock,
				})
			}
		}
	} else if szToken == tx.To().Hex() || (pair.owner != nil && txFrom.Hex() == pair.owner.Hex()) { // action on the token
		LogFwBrOb.Println("[Owner action]-[Token] ", pair.token, pair.symbol, tx.Hash().Hex())
		this.conf.SendMessage(tx.Hash().Hex(), pair.token.Hex(), MSG_TYPE_GENERAL, bytesHex, "Owner's Action on Token", tx.Hash().Hex())
		LogFwBbOb.Println("    ", bytesHex)

		if bytes.Equal(methodSig, this.erc20.abi.Methods["renounceOwnership"].ID) {
			LogFwBbOb.Println("    RenounceOwnership Detected! New Owner: ", bytesHex)
			this.conf.SendMessage(tx.Hash().Hex(), pair.token.Hex(), MSG_TYPE_GOOD, bytesHex, "Renounce Ownership", "")
		} else if bytes.Equal(methodSig, this.erc20.abi.Methods["transferOwnership"].ID) {
			LogFwBbOb.Println("    TransferOwnership Detected! New Owner: ", bytesHex)
			this.conf.SendMessage(tx.Hash().Hex(), pair.token.Hex(), MSG_TYPE_GOOD, bytesHex, "Transfer Ownership", "")
		} else if bytesHex == "715018a6" {
			LogFwBbOb.Println("    RenounceOwnership Detected!")
			this.conf.SendMessage(tx.Hash().Hex(), pair.token.Hex(), MSG_TYPE_SUCCESS, bytesHex, "Renounce Ownership", "")
		} else if bytesHex == "751039fc" {
			LogFwBbOb.Println("    RemoveLimits Detected!")
			this.conf.SendMessage(tx.Hash().Hex(), pair.token.Hex(), MSG_TYPE_GOOD, bytesHex, "Remove Limits", "")
		}
		var walletsToSell []*DTAccount
		var walletsToRescue []*DTAccount

		minGasTipForScam := new(big.Int).SetInt64(int64(this.config.darkSlayer.scamTxMinFeeInGwei * 1000000000))
		gasTip := tx.GasTipCap()
		highFee := false
		if gasTip != nil && tx.GasTipCap().Cmp(minGasTipForScam) >= 0 {
			highFee = true
		}

		totalWallets := this.config.wallets

		for _, account := range totalWallets {
			tokenBalance, err := this.erc20.callBalanceOf(&account.address, pair.token, rpc.LatestBlockNumber, nil)
			if err != nil || tokenBalance.Cmp(big.NewInt(0)) <= 0 {
				// fmt.Println("[Slayer]-[CheckTx]-error or balance zero", account.szAddress, err, tokenBalance)
				continue
			}
			if this.checkAddBot(pair, account, tx) {
				walletsToRescue = append(walletsToRescue, account)
				LogFwBrOb.Println("[DarkSlayer] - Add bot detected", account.address.Hex())
				this.conf.SendMessage(tx.Hash().Hex(), pair.token.Hex(), MSG_TYPE_DANGER, bytesHex, "Add bot", account.address.Hex())
			} else if this.checkScamAction(pair, account, tx, tokenBalance, bytesHex) {
				walletsToSell = append(walletsToSell, account)
			}
		}
		if len(walletsToSell) > 0 {
			if highFee == false {
				countToSell := len(walletsToSell) / 2
				if len(walletsToSell)%2 == 1 {
					countToSell = countToSell + 1
				}
				walletsToSell = walletsToSell[:countToSell]
			}
			subTxs := this.buildSellTxs(pair, walletsToSell, this.bc.CurrentHeader())
			if this.config.darkSlayer.isOtherEnabled {
				bundleTxs = append(bundleTxs, subTxs...)
			}
		}
		if len(walletsToRescue) > 0 {
			subTxs := this.buildRescueTxs(pair, walletsToRescue)
			if this.config.darkSlayer.isAddBotEnabled {
				bundleTxs = append(bundleTxs, subTxs...)
			}
		}
	}

	if len(bundleTxs) > 0 {
		this.sendBundle(bundleTxs, tx, 2)
	}
	return true
}

func (this *DarkSlayer) RemovePair(pair *DTPair) {
	token := pair.token.Hex()
	owner := pair.owner

	if _, exists := this.pairs[token]; exists {
		pair.pairMutex.Lock()
		delete(this.pairs, token)
		delete(this.pairsTrackInfo, token)

		index := 0
		for _, t := range this.tokens {
			if t != token {
				this.tokens[index] = t
				index++
			}
		}
		this.tokens = this.tokens[:index]
		if owner != nil {
			delete(this.owners, owner.Hex())
		}
		pair.pairMutex.Unlock()
	}
}
func (this *DarkSlayer) AddPair(pair *DTPair) {
	this.pairsMutex.Lock()
	this.tokens = append(this.tokens, pair.token.Hex())
	this.pairs[pair.token.Hex()] = pair
	if pair.owner != nil {
		this.owners[pair.owner.Hex()] = pair.token.Hex()
	}
	// safe check, add lp owner as well
	// if pair.liquidityOwner != nil && (pair.owner == nil || pair.owner.Hex() != pair.liquidityOwner.Hex()) {
	// 	this.owners[pair.liquidityOwner.Hex()] = pair.token.Hex()
	// }
	this.pairsTrackInfo[pair.token.Hex()] = make(map[uint64]*PairTrackInfo)
	this.pairsMutex.Unlock()

	pair.pairMutex.Lock()
	pair.buyHoldOn = false
	pair.triggerTx = nil
	pair.pairMutex.Unlock()

	this.tokensByPairMutex.Lock()
	this.tokensByPair[pair.address.Hex()] = pair.token.Hex()
	this.tokensByPairMutex.Unlock()

	head := this.bc.CurrentHeader()
	blockNumber := head.Number.Uint64()
	for i := uint64(0); i < uint64(this.config.darkJumper.txCountConsiderBlockCount); i++ {
		block := this.bc.GetBlockByNumber(blockNumber - i)
		this.trackPairInfo(block, pair)
	}

	fmt.Println("[DarkSlayer] - [Watching T]: ", pair.token, pair.symbol)
}

func (this *DarkSlayer) checkAddBot(pair *DTPair, account *DTAccount, tx *types.Transaction) bool {
	inputHex := strings.ToLower(hexutils.BytesToHex(tx.Data()))
	address := strings.ToLower(account.szAddress[2:])

	// fmt.Println("[Slayer]-[CheckAddBot]", inputHex, address)
	if strings.Contains(inputHex, address) {
		return true
	}
	return false
}
func (this *DarkSlayer) checkScamAction(pair *DTPair, account *DTAccount, tx *types.Transaction, tokenBalance *big.Int, bytesHex string) bool {
	sellAmountExpected, sellAmountActual, err := this.scamchecker.CalcSellAmount(pair, &account.address, tokenBalance, rpc.LatestBlockNumber, tx)

	if err == nil {
		sellFee := 100 - big.NewInt(0).Div(big.NewInt(0).Mul(sellAmountActual, big.NewInt(100)), sellAmountExpected).Uint64()
		fmt.Println(fmt.Sprintf("[DarkSlayer] - [ScamAction], fee = %d = %.2f/%.2f", sellFee, ViewableEthAmount(sellAmountActual), ViewableEthAmount(sellAmountExpected)))

		if sellFee <= this.config.maxSellFeeInPercent {
			this.conf.SendMessage(tx.Hash().Hex(), pair.token.Hex(), MSG_TYPE_GENERAL, bytesHex, "Sell Fee", map[string]interface{}{
				"account":        account.szAddress,
				"fee":            sellFee,
				"amountActual":   ViewableEthAmount(sellAmountActual),
				"amountExpected": ViewableEthAmount(sellAmountExpected),
			})
			return false
		}
		this.conf.SendMessage(tx.Hash().Hex(), pair.token.Hex(), MSG_TYPE_DANGER, bytesHex, "Sell Fee", map[string]interface{}{
			"account":        account.szAddress,
			"fee":            sellFee,
			"amountActual":   ViewableEthAmount(sellAmountActual),
			"amountExpected": ViewableEthAmount(sellAmountExpected),
		})
	} else {
		fmt.Println("[DarkSlayer] - [checkScamAction] - err", err)
		this.conf.SendMessage(tx.Hash().Hex(), pair.token.Hex(), MSG_TYPE_DANGER, bytesHex, "Scam action", map[string]interface{}{
			"account": account.szAddress,
			"err":     err.Error(),
		})
	}
	// Danger!, Instantly Sell
	return true
}

func (this *DarkSlayer) buildRescueTxs(pair *DTPair, accounts []*DTAccount) [][]byte {
	LogFwBrOb.Println("[DarkSlayer] Add bot detected! Transfer now!", pair.token, pair.name)
	this.conf.SendMessage("", pair.token.Hex(), MSG_TYPE_DANGER, "", "Rescue wallets", "")
	if accounts == nil {
		fmt.Println("  All accounts")
	} else {
		fmt.Println("  Selected accounts")
	}

	currentHead := this.bc.CurrentHeader()
	txs := make([][]byte, 0)
	tip := big.NewInt(1).Mul(big.NewInt(101), big.NewInt(DECIMALS/1000000000)) // 100 GWEI
	idx, exists := this.subWalletIdx[pair.token.Hex()]
	if !exists {
		idx = 0
	}
	for _, account := range accounts {
		if idx >= len(this.config.subWallets) {
			// Should not be here
			LogFwBrOb.Println("[DarkSlayer] lack of sub wallets. wants", idx, "count", len(this.config.subWallets))
			this.conf.SendMessage("", pair.token.Hex(), MSG_TYPE_DANGER, "", "Lack of sub wallets", "")
			continue
		}
		tx, err := BuildErc20TransferTx(currentHead, this.erc20, this.txApi, pair.token, account, &this.config.subWallets[idx].address, nil, tip, 0)
		LogFbBlr.Println("   ", account.address.Hex(), "->", this.config.subWallets[idx].address.Hex())
		this.conf.SendMessage("", pair.token.Hex(), MSG_TYPE_GOOD, "", "  Rescue", map[string]string{
			"from": account.address.Hex(),
			"to":   this.config.subWallets[idx].address.Hex(),
		})
		if err != nil {
			fmt.Println(err)
			continue
		}
		txs = append(txs, tx)
		idx++
	}
	this.subWalletIdx[pair.token.Hex()] = idx
	return txs
}
func (this *DarkSlayer) buildSellTxs(pair *DTPair, accounts []*DTAccount, header *types.Header) [][]byte {
	LogFwBrOb.Println("[DarkSlayer] Danger action detected, Sell now!", pair.token, pair.name)
	this.conf.SendMessage("", pair.token.Hex(), MSG_TYPE_DANGER, "", "Sell token for wallets", "")

	if accounts == nil {
		fmt.Println("  All accounts")
	} else {
		fmt.Println("  Selected accounts")
		for _, account := range accounts {
			fmt.Println("   ", account.address)
		}
	}

	if accounts == nil {
		accounts = append(this.config.wallets, this.config.subWallets...)
	}

	currentHead := this.bc.CurrentHeader()
	txs := make([][]byte, 0)
	maxTip := big.NewInt(1).Mul(big.NewInt(21), big.NewInt(DECIMALS/1000000000)) // 20 GWEI
	nextBaseFee := CalcNextBaseFee(header)
	_, sellPath := BuildSwapPath(pair)
	for _, account := range accounts {
		balance, err := this.erc20.callBalanceOf(&account.address, pair.token, rpc.LatestBlockNumber, nil)
		if err != nil {
			continue
		}
		if balance.Cmp(big.NewInt(0)) <= 0 {
			continue
		}

		// try sell to get output
		swapInput, err := this.uniswapv2.BuildSwapExactTokensForTokensSupportingFeeOnTransferTokensInput(balance, big.NewInt(0), sellPath, account.address, big.NewInt(time.Now().Unix()+200))
		if err != nil {
			continue
		}

		arg := ethapi.TransactionArgs{
			From:  &account.address,
			To:    &v2RouterAddrObj,
			Input: swapInput,
		}

		// calc tip and others
		blkNo := rpc.BlockNumberOrHashWithNumber(rpc.LatestBlockNumber)
		estimatedSellGasAmount, err := this.bcApi.EstimateGas(context.Background(), arg, &blkNo, nil)
		if err != nil || estimatedSellGasAmount == 0 {
			estimatedSellGasAmount = 200000
		}

		_, sellAmountActual, err := this.scamchecker.CalcSellAmount(pair, &account.address, balance, rpc.LatestBlockNumber, nil)
		tip := new(big.Int).Div(sellAmountActual, big.NewInt(int64(estimatedSellGasAmount)))
		if tip.Cmp(nextBaseFee) <= 0 || err != nil {
			fmt.Println(account.szAddress, "Skip sell, Can't cover the fee, amountOut", ViewableEthAmount(sellAmountActual))
			this.conf.SendMessage("", pair.token.Hex(), MSG_TYPE_DANGER, "", "  Skip sell", map[string]interface{}{
				"account":          account.szAddress,
				"sellAmountActual": ViewableEthAmount(sellAmountActual),
				"error":            errors.New("Skip sell"),
			})
			continue
		}
		tip = tip.Sub(tip, nextBaseFee)
		tip = tip.Div(tip, big.NewInt(2))
		if tip.Cmp(maxTip) > 0 {
			tip = maxTip
		}

		// _, err = this.bcApi.Call(context.Background(), arg, rpc.BlockNumberOrHashWithNumber(rpc.LatestBlockNumber), nil)
		// if err != nil {
		// 	fmt.Println(account.szAddress, "Skip sell, minAmoutOut", ViewableEthAmount(amountOutMin), err)
		// 	this.conf.SendMessage("", pair.token.Hex(), MSG_TYPE_DANGER, "", "  Skip sell", map[string]interface{}{
		// 		"account":      account.szAddress,
		// 		"minAmountOut": ViewableEthAmount(amountOutMin),
		// 		"error":        err.Error(),
		// 	})
		// 	continue
		// } else {
		this.conf.SendMessage("", pair.token.Hex(), MSG_TYPE_DANGER, "", "  Sell", map[string]interface{}{
			"account": account.szAddress,
			"balance": balance,
		})
		// }

		tx, err := BuildSellTx(currentHead, this.erc20, this.uniswapv2, this.txApi, pair, account, &account.address, nil, nil, tip, 0, 0)
		fmt.Println("   ", account.address)
		if err != nil {
			fmt.Println(err)
			continue
		}
		txs = append(txs, tx)
	}
	return txs
}

func (this *DarkSlayer) sendBundle(bundles [][]byte, tx *types.Transaction, blkCount int64) {
	if tx != nil {
		txPayload, _ := tx.MarshalBinary()
		bundles = append(bundles, txPayload)
	}
	blkNumber := this.bc.CurrentHeader().Number.Int64() + 1

	txBundles := make([]*TxBundle, 1)
	txBundles[0] = &TxBundle{
		txs:           bundles,
		blkNumber:     blkNumber,
		blkCount:      blkCount,
		revertableTxs: []common.Hash{},
	}

	SendRawTransaction(txBundles)
}

func (this *DarkSlayer) getPairByAddress(address *common.Address) *DTPair {
	this.tokensByPairMutex.RLock()
	token, exists := this.tokensByPair[address.Hex()]
	this.tokensByPairMutex.RUnlock()
	var pair *DTPair = nil
	if exists {
		pair, _ = this.pairs[token]
	}
	return pair
}
