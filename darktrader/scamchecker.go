package darktrader

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/rpc"
)

type ScamChecker struct {
	config *DTConfig

	erc20     *Erc20
	uniswapv2 *UniswapV2
	bcApi     *ethapi.BlockChainAPI

	abiUnicrypt      abi.ABI
	abiTeamFinance   abi.ABI
	abiUniswapV2Pair abi.ABI
	abiPinkLock      abi.ABI

	// constants
	balance0 hexutil.Big
	chainId  hexutil.Big

	testWallets []*DTAccount
}

func NewScamChecker(config *DTConfig, erc20 *Erc20, uniswapv2 *UniswapV2, bcApi *ethapi.BlockChainAPI) *ScamChecker {
	checker := ScamChecker{}

	checker.Init(config, erc20, uniswapv2, bcApi)

	return &checker
}
func (this *ScamChecker) getBuyAmounts(initialBaseReserve *big.Int, initialTokenReserve *big.Int, currentBaseReserve *big.Int, currentTokenReserve *big.Int, minBuyAmount *big.Int, maxBuyAmount *big.Int, percent int64) []*big.Int {
	amounts := make([]*big.Int, 0)

	curPercent := int64(percent * 100)
	percentStep := int64(curPercent / 100)
	divider := big.NewInt(10000)

	// priceUp := new(big.Float).Quo(
	// 	new(big.Float).Quo(
	// 		new(big.Float).SetInt(currentBaseReserve),
	// 		new(big.Float).SetInt(currentTokenReserve),
	// 	),
	// 	new(big.Float).Quo(
	// 		new(big.Float).SetInt(initialBaseReserve),
	// 		new(big.Float).SetInt(initialTokenReserve),
	// 	),
	// )
	// _maxBuyAmount := big.NewInt(0)
	// new(big.Float).Mul(
	// 	new(big.Float).SetInt(maxBuyAmount),
	// 	priceUp,
	// ).Int(_maxBuyAmount)
	for {
		amountOut := new(big.Int).Div(new(big.Int).Mul(initialTokenReserve, big.NewInt(curPercent)), divider)
		if curPercent <= 0 {
			break
		}
		amount := new(big.Int).Div(
			new(big.Int).Mul(currentBaseReserve, amountOut),
			new(big.Int).Sub(currentTokenReserve, amountOut),
		)
		if amount.Cmp(maxBuyAmount) <= 0 {
			if amount.Cmp(minBuyAmount) < 0 {
				break
			}
			amounts = append(amounts, amount)
		}
		curPercent -= percentStep
	}
	return amounts
}
func (this *ScamChecker) Init(config *DTConfig, erc20 *Erc20, uniswapv2 *UniswapV2, bcApi *ethapi.BlockChainAPI) {
	this.bcApi = bcApi
	this.erc20 = erc20
	this.uniswapv2 = uniswapv2
	this.config = config

	this.abiUnicrypt, _ = abi.JSON(strings.NewReader(ABI_UNICRYPT))
	this.abiTeamFinance, _ = abi.JSON(strings.NewReader(ABI_TEAMFINANCE))
	this.abiUniswapV2Pair, _ = abi.JSON(strings.NewReader(ABI_UNISWAP_PAIR_V2))
	this.abiPinkLock, _ = abi.JSON(strings.NewReader(ABI_PINKLOCK))

	// constants
	this.balance0 = hexutil.Big(*big.NewInt(0))
	this.chainId = hexutil.Big(*big.NewInt(1))

	// create test wallets
	this.testWallets = make([]*DTAccount, 100)
	for i := 0; i < int(100); i++ {
		privateKey, _ := crypto.GenerateKey()
		this.testWallets[i] = &DTAccount{
			key:       privateKey,
			address:   crypto.PubkeyToAddress(privateKey.PublicKey),
			szAddress: crypto.PubkeyToAddress(privateKey.PublicKey).Hex(),
		}
	}
}

func (this *ScamChecker) CalcBuyAmount(pair *DTPair, config *DTConfig, blockNumber rpc.BlockNumber, pendingTx *types.Transaction, buyAmounts []*big.Int) (*big.Int, uint, *big.Int, *big.Int, error) {
	/* call order
	 * - pending Tx, if
	 	 - get amount out
		 - call buy - uniswap v2
		 - balanceOf token - from
	*/
	var stateOverrides ethapi.StateOverride = make(ethapi.StateOverride)
	balanceTest := hexutil.Big(*AMOUNT_10)
	balanceTest1 := &balanceTest
	stateOverrides[config.testAccountAddress] = ethapi.OverrideAccount{
		Balance: &balanceTest1,
	}

	var batchCallConfig = ethapi.BatchCallConfig{
		Block:          rpc.BlockNumberOrHashWithNumber(blockNumber),
		StateOverrides: &stateOverrides,
	}

	tryBuyAmount := buyAmounts
	if buyAmounts == nil {
		tryBuyAmount = this.getBuyAmounts(pair.initialBaseReserve, pair.initialTokenReserve, pair.baseReserve, pair.tokenReserve, config.minBuyAmount, config.maxBuyAmount, config.maxBuyTokenAmountInPercent)
	}
	if len(tryBuyAmount) == 0 {
		return big.NewInt(0), uint(0), big.NewInt(0), big.NewInt(0), errors.New("Can't decide the buy amount")
	}
	amount := tryBuyAmount[0]

	callIndex := 0
	swapCallIdx := 0

	// Pending Tx
	if pendingTx != nil {
		// triggerTxFrom, err := GetFrom(pendingTx)
		batchCallConfig.Calls = make([]ethapi.BatchCallArgs, 4)

		txFrom, _ := GetFrom(pendingTx)
		txValue := hexutil.Big(*pendingTx.Value())
		txInput := hexutil.Bytes(pendingTx.Data())
		gas := hexutil.Uint64(pendingTx.Gas())
		batchCallConfig.Calls[0] = ethapi.BatchCallArgs{
			TransactionArgs: ethapi.TransactionArgs{
				From:    &txFrom,
				To:      pendingTx.To(),
				Value:   &txValue,
				Input:   &txInput,
				ChainID: &this.chainId,
				Gas:     &gas,
			},
		}
		callIndex++
	} else {
		batchCallConfig.Calls = make([]ethapi.BatchCallArgs, 3)
	}

	path, _ := BuildSwapPath(pair)

	// get amounts out
	amountsOutInput, _ := this.uniswapv2.BuildGetAmountsOutInput(amount, path)
	batchCallConfig.Calls[callIndex] = ethapi.BatchCallArgs{
		TransactionArgs: ethapi.TransactionArgs{
			From:    &config.testAccountAddress,
			To:      &v2RouterAddrObj,
			Input:   amountsOutInput,
			ChainID: &this.chainId,
		},
	}
	callIndex++

	// swap
	swapInput, _ := this.uniswapv2.BuildSwapExactEthForTokenInput(big.NewInt(0), path, config.testAccountAddress, big.NewInt(time.Now().Unix()+200))
	swapTxValue := hexutil.Big(*amount)
	batchCallConfig.Calls[callIndex] = ethapi.BatchCallArgs{
		TransactionArgs: ethapi.TransactionArgs{
			From:    &config.testAccountAddress,
			To:      &v2RouterAddrObj,
			Value:   &swapTxValue,
			Input:   swapInput,
			ChainID: &this.chainId,
		},
	}
	swapCallIdx = callIndex
	callIndex++

	// balance of
	balanceOfInput, _ := this.erc20.BuildBalanceOfInput(&config.testAccountAddress)
	batchCallConfig.Calls[callIndex] = ethapi.BatchCallArgs{
		TransactionArgs: ethapi.TransactionArgs{
			From:    &config.testAccountAddress,
			To:      pair.token,
			Input:   balanceOfInput,
			ChainID: &this.chainId,
		},
	}

	tryIdx := 1
	for {
		results, err := this.bcApi.BatchCall(context.Background(), batchCallConfig)

		if err != nil {
			return big.NewInt(0), uint(0), big.NewInt(0), big.NewInt(0), err
		}

		if results[swapCallIdx].Error == nil {
			amountExpecteds, _ := this.uniswapv2.ParseGetAmountsOutOutput(results[swapCallIdx-1].Return)
			amountBought, _ := this.erc20.ParseBalanceOfOutput(results[swapCallIdx+1].Return)
			return amount, uint(len(config.wallets)), amountExpecteds[len(amountExpecteds)-1], amountBought, nil
		} else {
			// fmt.Println("Try buy amount", amount, "error", results[swapCallIdx].Error)
		}

		if tryIdx >= len(tryBuyAmount) {
			break
		}

		amount = tryBuyAmount[tryIdx]

		amountsOutInput, _ := this.uniswapv2.BuildGetAmountsOutInput(amount, path)
		batchCallConfig.Calls[swapCallIdx-1] = ethapi.BatchCallArgs{
			TransactionArgs: ethapi.TransactionArgs{
				From:    &config.testAccountAddress,
				To:      &v2RouterAddrObj,
				Input:   amountsOutInput,
				ChainID: &this.chainId,
			},
		}

		swapTxValue := hexutil.Big(*amount)
		batchCallConfig.Calls[swapCallIdx] = ethapi.BatchCallArgs{
			TransactionArgs: ethapi.TransactionArgs{
				From:    &config.testAccountAddress,
				To:      &v2RouterAddrObj,
				Value:   &swapTxValue,
				Input:   swapInput,
				ChainID: &this.chainId,
			},
		}

		tryIdx = tryIdx + 1
	}
	return big.NewInt(0), uint(0), big.NewInt(0), big.NewInt(0), errors.New("Couldn't decide the buy amount")
}

func (this *ScamChecker) CalcSellAmount(pair *DTPair, from *common.Address, amountToken *big.Int, blockNumber rpc.BlockNumber, pendingTx *types.Transaction) (*big.Int, *big.Int, error) {
	/* call order
	 * - pending Tx, if any
	   - call get amounts out
		 - call approve
		 - call sell - uniswap v2
		 - call balance of weth
	*/
	var batchCallConfig = ethapi.BatchCallConfig{
		Block: rpc.BlockNumberOrHashWithNumber(blockNumber),
	}

	callIndex := 0
	callSellIndex := 0

	// Pending Tx
	if pendingTx != nil {
		batchCallConfig.Calls = make([]ethapi.BatchCallArgs, 5)

		txFrom, _ := GetFrom(pendingTx)
		txValue := hexutil.Big(*pendingTx.Value())
		txInput := hexutil.Bytes(pendingTx.Data())
		batchCallConfig.Calls[0] = ethapi.BatchCallArgs{
			TransactionArgs: ethapi.TransactionArgs{
				From:    &txFrom,
				To:      pendingTx.To(),
				Value:   &txValue,
				Input:   &txInput,
				ChainID: &this.chainId,
			},
		}
		callIndex++
	} else {
		batchCallConfig.Calls = make([]ethapi.BatchCallArgs, 4)
	}

	_, sellPath := BuildSwapPath(pair)

	var nextBlockOverrides *ethapi.BlockOverrides = nil
	if blockNumber != rpc.PendingBlockNumber {
		nextBlockOverrides = &ethapi.BlockOverrides{}
		nextBlockNumber := hexutil.Big(*big.NewInt(blockNumber.Int64() + 5))
		nextBlockOverrides.Number = &nextBlockNumber
		newTime := hexutil.Uint64(big.NewInt(time.Now().Unix() + 5*12).Uint64())
		nextBlockOverrides.Time = &newTime
	}
	// get amount out
	getAmountOutInput, _ := this.uniswapv2.BuildGetAmountsOutInput(amountToken, sellPath)
	amountOutCallIndex := callIndex
	batchCallConfig.Calls[callIndex] = ethapi.BatchCallArgs{
		TransactionArgs: ethapi.TransactionArgs{
			From:    from,
			To:      &v2RouterAddrObj,
			Input:   getAmountOutInput,
			ChainID: &this.chainId,
		},
	}
	callIndex++

	//approve
	approveInput, _ := this.erc20.BuildApproveInput(&v2RouterAddrObj, amountToken)
	batchCallConfig.Calls[callIndex] = ethapi.BatchCallArgs{
		TransactionArgs: ethapi.TransactionArgs{
			From:    from,
			To:      pair.token,
			Input:   approveInput,
			ChainID: &this.chainId,
		},
	}
	callIndex++

	//sell

	swapSellInput, _ := this.uniswapv2.BuildSwapExactTokensForTokensSupportingFeeOnTransferTokensInput(amountToken, big.NewInt(0), sellPath, *from, big.NewInt(time.Now().Unix()+200))
	batchCallConfig.Calls[callIndex] = ethapi.BatchCallArgs{
		TransactionArgs: ethapi.TransactionArgs{
			From:    from,
			To:      &v2RouterAddrObj,
			Input:   swapSellInput,
			ChainID: &this.chainId,
		},
	}
	if nextBlockOverrides != nil {
		batchCallConfig.Calls[callIndex].BlockOverrides = nextBlockOverrides
	}
	callSellIndex = callIndex
	callIndex++

	balanceOfInput, _ := this.erc20.BuildBalanceOfInput(from)
	batchCallConfig.Calls[callIndex] = ethapi.BatchCallArgs{
		TransactionArgs: ethapi.TransactionArgs{
			From:    from,
			To:      pair.baseToken,
			Input:   balanceOfInput,
			ChainID: &this.chainId,
		},
	}
	if nextBlockOverrides != nil {
		batchCallConfig.Calls[callIndex].BlockOverrides = nextBlockOverrides
	}
	results, err := this.bcApi.BatchCall(context.Background(), batchCallConfig)

	if err != nil {
		return big.NewInt(0), big.NewInt(0), err
	}

	if results[callSellIndex].Error == nil {
		amountExpecteds, _ := this.uniswapv2.ParseSwapExactTokensForTokensOutput(results[amountOutCallIndex].Return)
		amountSold, _ := this.erc20.ParseBalanceOfOutput(results[callSellIndex+1].Return)
		return amountExpecteds[len(amountExpecteds)-1], amountSold, nil
	}

	return big.NewInt(0), big.NewInt(0), results[callSellIndex].Error
}

func (this *ScamChecker) CheckBuySell(pair *DTPair, amount *big.Int, amountToken *big.Int, blockNumber int64, timestamp uint64, buySellCheckCount uint8, pendingTx *types.Transaction) (*big.Int, *big.Int, error) {
	/* call order
	 * - pending Tx, if any
		 - call buy * n times for n different blocks
		 - call approve for 1st account
		 - call sell for 1st account
		 - get amount out
		 - call balance of weth
	*/
	var stateOverrides ethapi.StateOverride = make(ethapi.StateOverride)
	balanceTest := hexutil.Big(*AMOUNT_10)
	balanceTest1 := &balanceTest
	for i := 0; i < int(buySellCheckCount); i++ {
		stateOverrides[this.testWallets[i].address] = ethapi.OverrideAccount{
			Balance: &balanceTest1,
		}
	}
	var batchCallConfig = ethapi.BatchCallConfig{
		Block:          rpc.BlockNumberOrHashWithNumber(rpc.BlockNumber(blockNumber)),
		StateOverrides: &stateOverrides,
	}

	callIndex := 0
	callSellIndex := 0
	totalCallCount := buySellCheckCount + 1 + 1 + 1 + 1

	// Pending Tx
	if pendingTx != nil {
		totalCallCount++
		batchCallConfig.Calls = make([]ethapi.BatchCallArgs, totalCallCount)

		txFrom, _ := GetFrom(pendingTx)
		txValue := hexutil.Big(*pendingTx.Value())
		txInput := hexutil.Bytes(pendingTx.Data())
		batchCallConfig.Calls[0] = ethapi.BatchCallArgs{
			TransactionArgs: ethapi.TransactionArgs{
				From:    &txFrom,
				To:      pendingTx.To(),
				Value:   &txValue,
				Input:   &txInput,
				ChainID: &this.chainId,
			},
		}
		callIndex++
	} else {
		batchCallConfig.Calls = make([]ethapi.BatchCallArgs, totalCallCount)
	}

	buyPath, sellPath := BuildSwapPath(pair)

	for i := int64(0); i < int64(buySellCheckCount); i++ {
		swapInput, _ := this.uniswapv2.BuildSwapExactEthForTokenInput(big.NewInt(0), buyPath, this.testWallets[i].address, big.NewInt(time.Now().Unix()+200))
		swapTxValue := hexutil.Big(*amount)
		blkNumber1 := big.NewInt(blockNumber + i)
		blkNumber := hexutil.Big(*blkNumber1)
		blkTime := hexutil.Uint64(timestamp + uint64(i*12))
		batchCallConfig.Calls[callIndex] = ethapi.BatchCallArgs{
			TransactionArgs: ethapi.TransactionArgs{
				From:    &this.testWallets[i].address,
				To:      &v2RouterAddrObj,
				Value:   &swapTxValue,
				Input:   swapInput,
				ChainID: &this.chainId,
			},
			BlockOverrides: &ethapi.BlockOverrides{
				Number: &blkNumber,
				Time:   &blkTime,
			},
		}
		callIndex++
	}

	//approve
	blkNumber1 := big.NewInt(blockNumber + int64(buySellCheckCount))
	blkNumber := hexutil.Big(*blkNumber1)
	blkTime := hexutil.Uint64(timestamp + uint64(buySellCheckCount*12))

	approveInput, _ := this.erc20.BuildApproveInput(&v2RouterAddrObj, UINT256_MAX)
	batchCallConfig.Calls[callIndex] = ethapi.BatchCallArgs{
		TransactionArgs: ethapi.TransactionArgs{
			From:    &this.testWallets[0].address,
			To:      pair.token,
			Input:   approveInput,
			ChainID: &this.chainId,
		},
		BlockOverrides: &ethapi.BlockOverrides{
			Number: &blkNumber,
			Time:   &blkTime,
		},
	}
	callIndex++

	// get amount out
	getAmountOutInput, _ := this.uniswapv2.BuildGetAmountsOutInput(amountToken, sellPath)
	batchCallConfig.Calls[callIndex] = ethapi.BatchCallArgs{
		TransactionArgs: ethapi.TransactionArgs{
			From:    &this.testWallets[0].address,
			To:      &v2RouterAddrObj,
			Input:   getAmountOutInput,
			ChainID: &this.chainId,
		},
		BlockOverrides: &ethapi.BlockOverrides{
			Number: &blkNumber,
			Time:   &blkTime,
		},
	}
	callIndex++

	//sell
	swapSellInput, _ := this.uniswapv2.BuildSwapExactTokensForTokensSupportingFeeOnTransferTokensInput(amountToken, big.NewInt(0), sellPath, this.testWallets[0].address, big.NewInt(time.Now().Unix()+200))
	batchCallConfig.Calls[callIndex] = ethapi.BatchCallArgs{
		TransactionArgs: ethapi.TransactionArgs{
			From:    &this.testWallets[0].address,
			To:      &v2RouterAddrObj,
			Input:   swapSellInput,
			ChainID: &this.chainId,
		},
		BlockOverrides: &ethapi.BlockOverrides{
			Number: &blkNumber,
			Time:   &blkTime,
		},
	}
	callSellIndex = callIndex
	callIndex++

	// balance of
	balanceOfInput, _ := this.erc20.BuildBalanceOfInput(&this.testWallets[0].address)
	batchCallConfig.Calls[callIndex] = ethapi.BatchCallArgs{
		TransactionArgs: ethapi.TransactionArgs{
			From:    &this.testWallets[0].address,
			To:      pair.baseToken,
			Input:   balanceOfInput,
			ChainID: &this.chainId,
		},
		BlockOverrides: &ethapi.BlockOverrides{
			Number: &blkNumber,
			Time:   &blkTime,
		},
	}

	results, err := this.bcApi.BatchCall(context.Background(), batchCallConfig)

	if err != nil {
		return big.NewInt(0), big.NewInt(0), err
	}

	if results[callSellIndex].Error == nil {
		// amountExpecteds, _ := this.uniswapv2.ParseSwapExactTokensForTokensOutput(results[callSellIndex].Return)
		amountExpecteds, _ := this.uniswapv2.ParseGetAmountsOutOutput(results[callSellIndex-1].Return)
		amountSold, _ := this.erc20.ParseBalanceOfOutput(results[callSellIndex+1].Return)
		return amountExpecteds[len(amountExpecteds)-1], amountSold, nil
	}

	return big.NewInt(0), big.NewInt(0), results[callSellIndex].Error
}

// func (this *ScamChecker) CanBuy(pair *DTPair, from *common.Address, blockNumber rpc.BlockNumber, pendingTx *types.Transaction) (*big.Int, *big.Int, error) {
// 	amountOut := big.NewInt(0)
// 	amountIn := big.NewInt(0)

// 	to := this.config.wallets[0].address

// 	// now call
// 	batchCallConfig, err := this.BuildCanBuyCallConfig(pair, to, blockNumber, pendingTx)
// 	if err != nil {
// 		return amountIn, amountOut, err
// 	}
// 	results, err := this.bcApi.BatchCall(context.Background(), *batchCallConfig)

// 	for _, result := range results {
// 		if result.Error != nil {
// 			return amountIn, amountOut, result.Error
// 		}
// 	}
// 	if pendingTx == nil {
// 		balanceWeth0, err := this.erc20.ParseBalanceOfOutput(results[0].Return)
// 		if err != nil {
// 			return amountIn, amountOut, err
// 		}
// 		balanceWeth1, err := this.erc20.ParseBalanceOfOutput(results[2].Return)
// 		if err != nil {
// 			return amountIn, amountOut, err
// 		}
// 		amountOut, err = this.erc20.ParseBalanceOfOutput(results[3].Return)
// 		if err != nil {
// 			return amountIn, amountOut, err
// 		}
// 		amountIn.Sub(balanceWeth0, balanceWeth1)
// 	} else {
// 		balanceWeth0, err := this.erc20.ParseBalanceOfOutput(results[1].Return)
// 		if err != nil {
// 			return amountIn, amountOut, err
// 		}
// 		balanceWeth1, err := this.erc20.ParseBalanceOfOutput(results[3].Return)
// 		if err != nil {
// 			return amountIn, amountOut, err
// 		}
// 		amountOut, err = this.erc20.ParseBalanceOfOutput(results[4].Return)
// 		if err != nil {
// 			return amountIn, amountOut, err
// 		}
// 		amountIn.Sub(balanceWeth0, balanceWeth1)
// 	}

// 	return amountIn, amountOut, nil
// }

// func (this *ScamChecker) CanBuyAndSell(pair *DTPair, from *common.Address, blockNumber rpc.BlockNumber, pendingTx *types.Transaction) (*big.Int, error) {
// 	amountOut := big.NewInt(0)

// 	to := this.config.wallets[0].address

// 	// now call
// 	batchCallConfig, err := this.BuildCanBuyAndSellCallConfig(pair, to, blockNumber, pendingTx)
// 	if err != nil {
// 		return amountOut, err
// 	}
// 	results, err := this.bcApi.BatchCall(context.Background(), *batchCallConfig)

// 	for _, result := range results {
// 		if result.Error != nil {
// 			return amountOut, result.Error
// 		}
// 	}
// 	if pendingTx == nil {
// 		balanceWeth0, err := this.erc20.ParseBalanceOfOutput(results[0].Return)
// 		if err != nil {
// 			return amountOut, err
// 		}
// 		balanceWeth1, err := this.erc20.ParseBalanceOfOutput(results[2].Return)
// 		if err != nil {
// 			return amountOut, err
// 		}
// 		amountOut, err = this.erc20.ParseBalanceOfOutput(results[3].Return)
// 		if err != nil {
// 			return amountOut, err
// 		}
// 		amountIn.Sub(balanceWeth0, balanceWeth1)
// 	} else {
// 		balanceWeth0, err := this.erc20.ParseBalanceOfOutput(results[1].Return)
// 		if err != nil {
// 			return amountIn, amountOut, err
// 		}
// 		balanceWeth1, err := this.erc20.ParseBalanceOfOutput(results[3].Return)
// 		if err != nil {
// 			return amountIn, amountOut, err
// 		}
// 		amountOut, err = this.erc20.ParseBalanceOfOutput(results[4].Return)
// 		if err != nil {
// 			return amountIn, amountOut, err
// 		}
// 		amountIn.Sub(balanceWeth0, balanceWeth1)
// 	}

// 	return amountIn, amountOut, nil
// }

// func (this *ScamChecker)EstimateBuyAmount(pair *DTPair, to common.Address, amount *big.Int, blockNumber rpc.BlockNumber, pendingTx *types.Transaction) (*big.Int, error) {

// 	var batchCallConfig = ethapi.BatchCallConfig{
// 		Block: rpc.BlockNumberOrHashWithNumber(blockNumber),
// 	}

// 	callIndex := 0

// 	// Pending Tx
// 	if pendingTx != nil {
// 		batchCallConfig.Calls = make([]ethapi.BatchCallArgs, 3)

// 		txFrom, _ := GetFrom(pendingTx)
// 		txValue := hexutil.Big(*pendingTx.Value())
// 		txInput := hexutil.Bytes(pendingTx.Data())
// 		batchCallConfig.Calls[0] = ethapi.BatchCallArgs{
// 			TransactionArgs: ethapi.TransactionArgs{
// 				From:    &txFrom,
// 				To:      pendingTx.To(),
// 				Value:   &txValue,
// 				Input:   &txInput,
// 				ChainID: &this.chainId,
// 			},
// 		}
// 		callIndex ++
// 	} else {
// 		batchCallConfig.Calls = make([]ethapi.BatchCallArgs, 2)
// 		callIndex = 0
// 	}

// 	inputBuy, err := this.BuildBuyInput(pair,  amount, this.config.maxBuyAmount, to, big.NewInt(time.Now().Unix()+200))
// 	if err != nil {
// 		return nil, err
// 	}
// 	batchCallConfig.Calls[callIndex] = ethapi.BatchCallArgs{
// 		TransactionArgs: ethapi.TransactionArgs{
// 			From:    &to,
// 			To:      &this.config.contractAddress,
// 			Input:   inputBuy,
// 			ChainID: &this.chainId,
// 		},
// 	}
// 	callIndex ++

// 	// Balance Of token
// 	inputBalanceOfToken, err := this.erc20.BuildBalanceOfInput(pair.token, &to)
// 	if err != nil {
// 		return nil, err
// 	}
// 	batchCallConfig.Calls[callIndex] = ethapi.BatchCallArgs{
// 		TransactionArgs: ethapi.TransactionArgs{
// 			From:    &to,
// 			To:      pair.token,
// 			Input:   inputBalanceOfToken,
// 			ChainID: &this.chainId,
// 		},
// 	}

// 	// call
// 	results, err := this.bcApi.BatchCall(context.Background(), batchCallConfig)

// 	for _, result := range results {
// 		if result.Error != nil {
// 			return big.NewInt(0), result.Error
// 		}
// 	}

// 	// parse
// 	if pendingTx == nil {
// 		amountOut, err := this.erc20.ParseBalanceOfOutput(results[1].Return)
// 		if err != nil {
// 			return big.NewInt(0), err
// 		}
// 		return amountOut, nil
// 	}

// 	amountOut, err := this.erc20.ParseBalanceOfOutput(results[2].Return)
// 	if err != nil {
// 		return big.NewInt(0), err
// 	}

// 	return amountOut, nil
// }

// func (this *ScamChecker)BuildBuyInput(pair *DTPair, amount *big.Int, maxAmountIn *big.Int, account common.Address, deadline *big.Int) (*hexutil.Bytes, error) {
// 	path, _ := BuildSwapPath(pair)
// 	accounts := []common.Address{account}
// 	input, err := this.abiDarkTrader.Pack("buy", path, amount, maxAmountIn, accounts, deadline)
// 	if err != nil {
// 		return nil, err
// 	}
// 	hexInput := hexutil.Bytes(input)

// 	return &hexInput, nil
// }

// func (this *ScamChecker)BuildCanBuyCallConfig(pair *DTPair, to common.Address, blockNumber rpc.BlockNumber, pendingTx *types.Transaction) (*ethapi.BatchCallConfig, error) {

// 	/* call order
// 	 * - pending Tx, if any
// 	   - balanceOf WETH - darktrader contract
// 		 - call buy - darktrader contract
// 		 - balanceOf WETH - darktrader contract
// 		 - balanceOf token - from
// 	*/
// 	var batchCallConfig = ethapi.BatchCallConfig{
// 		Block: rpc.BlockNumberOrHashWithNumber(blockNumber),
// 	}

// 	callIndex := 0

// 	// Pending Tx
// 	if pendingTx != nil {
// 		batchCallConfig.Calls = make([]ethapi.BatchCallArgs, 5)

// 		txFrom, _ := GetFrom(pendingTx)
// 		txValue := hexutil.Big(*pendingTx.Value())
// 		txInput := hexutil.Bytes(pendingTx.Data())
// 		batchCallConfig.Calls[0] = ethapi.BatchCallArgs{
// 			TransactionArgs: ethapi.TransactionArgs{
// 				From:    &txFrom,
// 				To:      pendingTx.To(),
// 				Value:   &txValue,
// 				Input:   &txInput,
// 				ChainID: &this.chainId,
// 			},
// 		}
// 		callIndex ++
// 	} else {
// 		batchCallConfig.Calls = make([]ethapi.BatchCallArgs, 4)
// 		callIndex = 0
// 	}

// 	// Balance Of WETH
// 	inputBalanceOfWeth, err := this.erc20.BuildBalanceOfInput(&WETH_ADDRESS, &this.config.contractAddress)
// 	if err != nil {
// 		return nil, err
// 	}
// 	batchCallConfig.Calls[callIndex] = ethapi.BatchCallArgs{
// 		TransactionArgs: ethapi.TransactionArgs{
// 			From:    &to,
// 			To:      &WETH_ADDRESS,
// 			Input:   inputBalanceOfWeth,
// 			ChainID: &this.chainId,
// 		},
// 	}
// 	callIndex ++

// 	// call buy
// 	amount, _ := CalcBuyAmountAndCount(*pair, this.config)
// 	inputBuy, err := this.BuildBuyInput(pair,  amount, this.config.maxBuyAmount, to, big.NewInt(time.Now().Unix()+200))
// 	if err != nil {
// 		return nil, err
// 	}
// 	batchCallConfig.Calls[callIndex] = ethapi.BatchCallArgs{
// 		TransactionArgs: ethapi.TransactionArgs{
// 			From:    &to,
// 			To:      &this.config.contractAddress,
// 			Input:   inputBuy,
// 			ChainID: &this.chainId,
// 		},
// 	}
// 	callIndex ++

// 	// balanceOf Weth
// 	batchCallConfig.Calls[callIndex] = ethapi.BatchCallArgs{
// 		TransactionArgs: ethapi.TransactionArgs{
// 			From:    &to,
// 			To:      &WETH_ADDRESS,
// 			Input:   inputBalanceOfWeth,
// 			ChainID: &this.chainId,
// 		},
// 	}
// 	callIndex ++
// 	// Balance Of token
// 	inputBalanceOfToken, err := this.erc20.BuildBalanceOfInput(pair.token, &to)
// 	if err != nil {
// 		return nil, err
// 	}
// 	batchCallConfig.Calls[callIndex] = ethapi.BatchCallArgs{
// 		TransactionArgs: ethapi.TransactionArgs{
// 			From:    &to,
// 			To:      pair.token,
// 			Input:   inputBalanceOfToken,
// 			ChainID: &this.chainId,
// 		},
// 	}
// 	callIndex ++

// 	return &batchCallConfig, nil
// }

// func (this *ScamChecker)BuildCanBuyAndSellCallConfig(pair *DTPair, to common.Address, blockNumber rpc.BlockNumber, pendingTx *types.Transaction) (*ethapi.BatchCallConfig, error) {
// 	/* call order
// 	 * - pending Tx, if any
// 	   - balanceOf WETH - darktrader contract
// 		 - call buy - darktrader contract
// 		 - call sell - uniswap router
// 		 - balanceOf WETH - darktrader contract
// 	*/
// 	var batchCallConfig = ethapi.BatchCallConfig{
// 		Block: rpc.BlockNumberOrHashWithNumber(blockNumber),
// 	}

// 	callIndex := 0

// 	// Pending Tx
// 	if pendingTx != nil {
// 		batchCallConfig.Calls = make([]ethapi.BatchCallArgs, 5)

// 		txFrom, _ := GetFrom(pendingTx)
// 		txValue := hexutil.Big(*pendingTx.Value())
// 		txInput := hexutil.Bytes(pendingTx.Data())
// 		batchCallConfig.Calls[0] = ethapi.BatchCallArgs{
// 			TransactionArgs: ethapi.TransactionArgs{
// 				From:    &txFrom,
// 				To:      pendingTx.To(),
// 				Value:   &txValue,
// 				Input:   &txInput,
// 				ChainID: &this.chainId,
// 			},
// 		}
// 		callIndex ++
// 	} else {
// 		batchCallConfig.Calls = make([]ethapi.BatchCallArgs, 4)
// 		callIndex = 0
// 	}

// 	// Balance Of WETH
// 	inputBalanceOfWeth, err := this.erc20.BuildBalanceOfInput(&WETH_ADDRESS, &this.config.contractAddress)
// 	if err != nil {
// 		return nil, err
// 	}
// 	batchCallConfig.Calls[callIndex] = ethapi.BatchCallArgs{
// 		TransactionArgs: ethapi.TransactionArgs{
// 			From:    &to,
// 			To:      &WETH_ADDRESS,
// 			Input:   inputBalanceOfWeth,
// 			ChainID: &this.chainId,
// 		},
// 	}
// 	callIndex ++

// 	// call buy
// 	amount, _ := CalcBuyAmountAndCount(*pair, this.config)
// 	inputBuy, err := this.BuildBuyInput(pair,  amount, this.config.maxBuyAmount, to, big.NewInt(time.Now().Unix()+200))
// 	if err != nil {
// 		return nil, err
// 	}
// 	batchCallConfig.Calls[callIndex] = ethapi.BatchCallArgs{
// 		TransactionArgs: ethapi.TransactionArgs{
// 			From:    &to,
// 			To:      &this.config.contractAddress,
// 			Input:   inputBuy,
// 			ChainID: &this.chainId,
// 		},
// 	}
// 	callIndex ++
// 	// call sell
// 	amountBought, err := this.EstimateBuyAmount(pair, to, amount, blockNumber, pendingTx)
// 	if err != nil {
// 		return nil, err
// 	}
// 	inputSellToken, err := this.BuildSellOnUniswapV2Input(pair.token, &to, amountBought, big.NewInt(0), big.NewInt(time.Now().Unix()+200))
// 	if err != nil {
// 		return nil, err
// 	}
// 	batchCallConfig.Calls[callIndex] = ethapi.BatchCallArgs{
// 		TransactionArgs: ethapi.TransactionArgs{
// 			From:    &to,
// 			To:      &v2RouterAddrObj,
// 			Input:   inputSellToken,
// 			ChainID: &this.chainId,
// 		},
// 	}
// 	callIndex ++

// 	// balanceOf Weth
// 	batchCallConfig.Calls[callIndex] = ethapi.BatchCallArgs{
// 		TransactionArgs: ethapi.TransactionArgs{
// 			From:    &to,
// 			To:      &WETH_ADDRESS,
// 			Input:   inputBalanceOfWeth,
// 			ChainID: &this.chainId,
// 		},
// 	}
// 	callIndex ++

// 	return &batchCallConfig, nil
// }

// // func (this *ScamChecker) BuildSellOnUniswapV2Input(pair.token, &to, amountBought, big.NewInt(0), big.NewInt(time.Now().Unix()+200)) (hexutil.Bytes, error) {

// // }
