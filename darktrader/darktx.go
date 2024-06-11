package darktrader

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/internal/ethapi"
)

type ExactAmountType int

const (
	ExactAmountIn ExactAmountType = iota
	ExactAmountOut
)

type DarkTx struct {
	tx          *types.Transaction
	priorityFee *big.Int
	swapInfo    *SwapParserResult
	blockNo     uint64
}

func NewDarkTx(
	tx *types.Transaction,
	baseFee *big.Int,
	swapInfo *SwapParserResult,
	blockNo uint64,
) *DarkTx {
	darkTx := &DarkTx{
		tx:       tx,
		swapInfo: swapInfo,
		blockNo:  blockNo,
	}
	if tx.Type() == 2 {
		darkTx.priorityFee = tx.GasTipCap()
	} else if tx.Type() == 1 {
		darkTx.priorityFee = new(big.Int).Sub(tx.GasFeeCap(), baseFee)
	}
	return darkTx
}

type DarkTxs struct {
	txs []*DarkTx
}

func NewDarkTxs() *DarkTxs {
	return &DarkTxs{
		txs: make([]*DarkTx, 0),
	}
}

func (this *DarkTxs) append(tx *DarkTx) {
	this.txs = append(this.txs, tx)
}

var SWAP_FILE_PATH string = "/root/tokens_swap/"

func (this *DarkTxs) WriteToLog(pair *DTPair) {
	f, err := os.Create(SWAP_FILE_PATH + time.Now().Format(time.RFC3339) + "-" + pair.token.Hex() + ".log")
	if err != nil {
		fmt.Println("error in writing swap logs", err)
		return
	}

	var swaps []string
	for _, tx := range this.txs {
		swaps = append(swaps, fmt.Sprintf("{\"hash\": \"%s\", \"amountIn\": \"%d\", \"maxPrice\": \"%f\", \"tip\": \"%d\"}", tx.tx.Hash().Hex(), tx.swapInfo.maxAmountIn, tx.swapInfo.priceMax, tx.priorityFee))
	}

	bytes, err := json.Marshal(swaps)

	_, err = f.Write(bytes)

	if err != nil {
		fmt.Println("error in writing swap logs", err)
		return
	}

	if err := f.Close(); err != nil {
		fmt.Println("error in writing swap logs", err)
		return
	}
}

func (this *DarkTxs) removeTxByHash(hash *common.Hash) bool {
	for i, tx := range this.txs {
		if tx.tx.Hash().Hex() == hash.Hex() {
			return this.removeTxByIndex(i)
		}
	}
	return false
}

func (this *DarkTxs) removeTxByIndex(i int) bool {
	if i == len(this.txs)-1 {
		this.txs = this.txs[:i]
	} else if i == 0 {
		this.txs = this.txs[1:]
	} else {
		this.txs = append(this.txs[:i], this.txs[i+1:]...)
	}
	return true
}

func (this *DarkTxs) calcReservesForAmountOut(baseReserve *big.Int, tokenReserve *big.Int, swapParams []*big.Int) (*big.Int, *big.Int) {
	newTokenReserve := new(big.Int).Sub(tokenReserve, swapParams[0])
	newBaseReserve := new(big.Int).Div(new(big.Int).Mul(baseReserve, tokenReserve), newTokenReserve)
	return newBaseReserve, newTokenReserve
}

func (this *DarkTxs) calcReservesForAmountIn(baseReserve *big.Int, tokenReserve *big.Int, swapParams []*big.Int) (*big.Int, *big.Int) {
	amountInWithFee := new(big.Int).Mul(swapParams[0], big.NewInt(997))
	newBaseReserve := new(big.Int).Div(new(big.Int).Add(new(big.Int).Mul(baseReserve, big.NewInt(1000)), amountInWithFee), big.NewInt(1000))
	newTokenReserve := new(big.Int).Div(new(big.Int).Mul(baseReserve, tokenReserve), newBaseReserve)
	return newBaseReserve, newTokenReserve
}

func (this *DarkTxs) checkRunnableForAmountOut(baseReserve *big.Int, tokenReserve *big.Int, swapParams []*big.Int, buyFeeInPercent uint64) bool {
	numerator := new(big.Int).Mul(new(big.Int).Mul(baseReserve, swapParams[0]), big.NewInt(1000))
	denominator := new(big.Int).Mul(new(big.Int).Sub(tokenReserve, swapParams[0]), big.NewInt(997))
	amountIn := new(big.Int).Add(new(big.Int).Div(numerator, denominator), big.NewInt(1))
	return amountIn.Cmp(swapParams[1]) <= 0
}

func (this *DarkTxs) checkRunnableForAmountIn(baseReserve *big.Int, tokenReserve *big.Int, swapParams []*big.Int, buyFeeInPercent uint64) bool {
	amountInWithBuyFee := new(big.Int).Div(new(big.Int).Mul(swapParams[0], new(big.Int).SetUint64(100-buyFeeInPercent)), big.NewInt(100))
	amountInWithFee := new(big.Int).Mul(amountInWithBuyFee, big.NewInt(997))
	numerator := new(big.Int).Mul(amountInWithFee, tokenReserve)
	denominator := new(big.Int).Add(new(big.Int).Mul(baseReserve, big.NewInt(1000)), amountInWithFee)
	amountOut := new(big.Int).Div(numerator, denominator)
	return amountOut.Cmp(swapParams[1]) >= 0
}

func (this *DarkTxs) RemoveMinedAndOldTxs(txApi *ethapi.TransactionAPI) {
	newTxs := make([]*DarkTx, 0)
	for _, tx := range this.txs {
		receipt, err := txApi.GetTransactionReceipt(context.Background(), tx.tx.Hash())
		if err == nil && receipt != nil {
		} else {
			newTxs = append(newTxs, tx)
		}
	}
	this.txs = newTxs
}
func (this *DarkTxs) BuildMaxProfitableSwapTxs(txApi *ethapi.TransactionAPI, nextBaseFee *big.Int, currentHead *types.Header) []*DarkTx {
	this.RemoveMinedAndOldTxs(txApi)
	// sort.Slice(this.txs, func(i, j int) bool {
	// 	if this.txs[i].priorityFee == nil {
	// 		return false
	// 	}
	// 	if this.txs[j].priorityFee == nil {
	// 		return true
	// 	}
	// 	if this.txs[i].swapInfo.priceMax != nil && this.txs[i].swapInfo.priceMax.Cmp(big.NewFloat(0)) == 0 {
	// 		return false
	// 	}
	// 	if this.txs[j].swapInfo.priceMax != nil && this.txs[j].swapInfo.priceMax.Cmp(big.NewFloat(0)) == 0 {
	// 		return true
	// 	}
	// 	cmpFee := this.txs[i].priorityFee.Cmp(this.txs[j].priorityFee)
	// 	if cmpFee == 0 {
	// 		if this.txs[i].swapInfo.priceMax == nil {
	// 			return false
	// 		}
	// 		if this.txs[j].swapInfo.priceMax == nil {
	// 			return true
	// 		}
	// 		cmpPrice := this.txs[i].swapInfo.priceMax.Cmp(this.txs[j].swapInfo.priceMax)
	// 		if cmpPrice == 0 {
	// 			if this.txs[i].swapInfo.maxAmountIn == nil {
	// 				return false
	// 			}
	// 			if this.txs[j].swapInfo.maxAmountIn == nil {
	// 				return true
	// 			}
	// 			cmpAmountIn := this.txs[i].swapInfo.maxAmountIn.Cmp(this.txs[j].swapInfo.maxAmountIn)
	// 			return cmpAmountIn >= 0
	// 		} else {
	// 			return cmpPrice < 0
	// 		}
	// 	} else {
	// 		return cmpFee > 0
	// 	}
	// })

	txs := make([]*DarkTx, 0)
	for _, tx := range this.txs {
		if tx.tx.GasFeeCap().Cmp(nextBaseFee) >= 0 {
			txs = append(txs, tx)
		}
	}
	return txs
}

func (this *DarkTxs) HasAtLeastSwap(count int) bool {
	cnt := 0
	for _, tx := range this.txs {
		if tx.swapInfo.router == "UniswapV2" || tx.swapInfo.router == "Universal" {
			cnt++
		}
	}
	return cnt >= count
}
