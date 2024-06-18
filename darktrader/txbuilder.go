package darktrader

import (
	"context"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/rpc"
)

type TokenBuyInfo struct {
	buyAmount           *big.Int
	buyCount            uint
	tokenAmountExpected *big.Int
	tokenAmountActual   *big.Int
	buyFeeInPercent     uint64
	sellFeeInPercent    uint64
	totalFeeInPercent uint64
	maxAmountIn         *big.Int
	tokenAmountsToBuy   []*big.Int
	txsToBundle []*types.Transaction
}

func BuildErc20TransferTx(currentHead *types.Header, erc20 *Erc20, txApi *ethapi.TransactionAPI, token *common.Address, account *DTAccount, to *common.Address, amount *big.Int, tip *big.Int, maxGas uint64) ([]byte, error) {
	nextBaseFee := CalcNextBaseFee(currentHead)

	nonce, err := txApi.GetTransactionCount(context.Background(), account.address, rpc.BlockNumberOrHashWithNumber(rpc.PendingBlockNumber))
	if err != nil {
		return nil, err
	}

	if amount == nil {
		amount, err = erc20.callBalanceOf(&account.address, token, rpc.LatestBlockNumber, nil)
		if err != nil {
			return nil, err
		}
	}
	txPayloadInput, err := erc20.abi.Pack("transfer", *to, amount)
	if err != nil {
		return nil, err
	}

	if tip == nil {
		tip = big.NewInt(1).Mul(big.NewInt(1), big.NewInt(DECIMALS/10000000000)) // 0.1 Gwei
	}
	if maxGas == 0 {
		maxGas = 300000
	}

	dfTx := types.DynamicFeeTx{
		ChainID:   CHAINID,
		Nonce:     uint64(*nonce),
		GasTipCap: tip,
		GasFeeCap: big.NewInt(1).Add(nextBaseFee, tip),
		Gas:       maxGas,
		To:        token,
		Data:      txPayloadInput,
	}
	transferTx := types.NewTx(types.TxData(&dfTx))
	signedTx, err := types.SignTx(transferTx, types.LatestSignerForChainID(CHAINID), account.key)
	if err != nil {
		return nil, err
	}

	txPayload, _ := signedTx.MarshalBinary()

	return txPayload, nil
}
func BuildApproveTx(currentHead *types.Header, txApi *ethapi.TransactionAPI, account *DTAccount, spender *common.Address, pair *DTPair, tip *big.Int, maxGas uint64, erc20 *Erc20) ([]byte, error) {
	nextBaseFee := CalcNextBaseFee(currentHead)

	nonce, err := txApi.GetTransactionCount(context.Background(), account.address, rpc.BlockNumberOrHashWithNumber(rpc.PendingBlockNumber))
	if err != nil {
		return nil, err
	}

	txPayloadInput, err := erc20.abi.Pack("approve", spender, UINT256_MAX)
	if err != nil {
		return nil, err
	}

	dfTx := types.DynamicFeeTx{
		ChainID:   CHAINID,
		Nonce:     uint64(*nonce),
		GasTipCap: tip,
		GasFeeCap: big.NewInt(1).Add(nextBaseFee, tip),
		Gas:       maxGas,
		To:        pair.token,
		Data:      txPayloadInput,
	}
	sellTx := types.NewTx(types.TxData(&dfTx))
	signedTx, err := types.SignTx(sellTx, types.LatestSignerForChainID(CHAINID), account.key)
	if err != nil {
		return nil, err
	}

	txPayload, _ := signedTx.MarshalBinary()

	return txPayload, nil
}
func BuildSellTx(currentHead *types.Header, erc20 *Erc20, uniswapv2 *UniswapV2, txApi *ethapi.TransactionAPI, pair *DTPair, account *DTAccount, to *common.Address, minAmountOut *big.Int, amount *big.Int, tip *big.Int, maxGas uint64, nonceOffset uint64) ([]byte, error) {
	nextBaseFee := CalcNextBaseFee(currentHead)

	nonce, err := txApi.GetTransactionCount(context.Background(), account.address, rpc.BlockNumberOrHashWithNumber(rpc.PendingBlockNumber))
	if err != nil {
		return nil, err
	}

	if amount == nil {
		amount, err = erc20.callBalanceOf(&account.address, pair.token, rpc.LatestBlockNumber, nil)
		if err != nil {
			return nil, err
		}
	}
	_, path := BuildSwapPath(pair)

	if minAmountOut == nil {
		minAmountOut = big.NewInt(0)
	}
	txPayloadInput, err := uniswapv2.abi.Pack("swapExactTokensForETHSupportingFeeOnTransferTokens", amount, minAmountOut, path, *to, big.NewInt(time.Now().Unix()+200))
	if err != nil {
		return nil, err
	}

	if tip == nil {
		tip = big.NewInt(1).Mul(big.NewInt(1), big.NewInt(DECIMALS/10000000000)) // 0.1 Gwei
	}
	if maxGas == 0 {
		maxGas = 500000
	}

	dfTx := types.DynamicFeeTx{
		ChainID:   CHAINID,
		Nonce:     uint64(*nonce) + nonceOffset,
		GasTipCap: tip,
		GasFeeCap: big.NewInt(1).Add(nextBaseFee, tip),
		Gas:       maxGas,
		To:        &v2RouterAddrObj,
		Data:      txPayloadInput,
	}
	sellTx := types.NewTx(types.TxData(&dfTx))
	signedTx, err := types.SignTx(sellTx, types.LatestSignerForChainID(CHAINID), account.key)
	if err != nil {
		return nil, err
	}

	txPayload, _ := signedTx.MarshalBinary()

	return txPayload, nil
}
