package darktrader

import (
	"bytes"
	"errors"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
)

type UniswapV2 struct {
	abi     abi.ABI
	abiPair abi.ABI
}

func NewUniswapV2() *UniswapV2 {
	uniswapV2 := UniswapV2{}

	uniswapV2.Init()

	return &uniswapV2
}

func (this *UniswapV2) Init() {
	this.abi, _ = abi.JSON(strings.NewReader(ABI_UNISWAP_ROUTER_V2))
	this.abiPair, _ = abi.JSON(strings.NewReader(ABI_UNISWAP_PAIR_V2))
}

func (this *UniswapV2) BuildSwapExactEthForTokenInput(amountOutMin *big.Int, path []common.Address, to common.Address, deadline *big.Int) (*hexutil.Bytes, error) {
	input, err := this.abi.Pack("swapExactETHForTokens", amountOutMin, path, to, deadline)

	if err != nil {
		return nil, err
	}
	hexInput := hexutil.Bytes(input)

	return &hexInput, nil
}

func (this *UniswapV2) ParseSwapExactEthForTokenOutput(output []byte) ([]*big.Int, error) {
	res, err := this.abi.Methods["swapExactETHForTokens"].Outputs.Unpack(output)
	if err != nil {
		return nil, err
	}

	if len(res) == 0 {
		return nil, errors.New("Unexpected output")
	}

	amountOut := res[0].([]*big.Int)

	return amountOut, nil
}

func (this *UniswapV2) BuildSwapETHForExactTokensInput(amountOut *big.Int, path []common.Address, to common.Address, deadline *big.Int) (*hexutil.Bytes, error) {
	input, err := this.abi.Pack("swapETHForExactTokens", amountOut, path, to, deadline)

	if err != nil {
		return nil, err
	}
	hexInput := hexutil.Bytes(input)

	return &hexInput, nil
}

func (this *UniswapV2) ParseSwapETHForExactTokensOutput(output []byte) ([]*big.Int, error) {
	res, err := this.abi.Methods["swapETHForExactTokens"].Outputs.Unpack(output)
	if err != nil {
		return nil, err
	}

	if len(res) == 0 {
		return nil, errors.New("Unexpected output")
	}

	amountOut := res[0].([]*big.Int)

	return amountOut, nil
}

func (this *UniswapV2) BuildSwapExactTokensForTokensInput(amountToken *big.Int, amountOutMin *big.Int, path []common.Address, to common.Address, deadline *big.Int) (*hexutil.Bytes, error) {
	input, err := this.abi.Pack("swapExactTokensForTokens", amountToken, amountOutMin, path, to, deadline)

	if err != nil {
		return nil, err
	}
	hexInput := hexutil.Bytes(input)

	return &hexInput, nil
}

func (this *UniswapV2) ParseSwapExactTokensForTokensOutput(output []byte) ([]*big.Int, error) {
	res, err := this.abi.Methods["swapExactTokensForTokens"].Outputs.Unpack(output)
	if err != nil {
		return nil, err
	}

	if len(res) == 0 {
		return nil, errors.New("Unexpected output")
	}

	amountOut := res[0].([]*big.Int)

	return amountOut, nil
}

func (this *UniswapV2) BuildSwapExactTokensForETHInput(amountToken *big.Int, amountOutMin *big.Int, path []common.Address, to common.Address, deadline *big.Int) (*hexutil.Bytes, error) {
	input, err := this.abi.Pack("swapExactTokensForETH", amountToken, amountOutMin, path, to, deadline)

	if err != nil {
		return nil, err
	}
	hexInput := hexutil.Bytes(input)

	return &hexInput, nil
}

func (this *UniswapV2) ParseSwapExactTokensForETHOutput(output []byte) ([]*big.Int, error) {
	res, err := this.abi.Methods["swapExactTokensForETH"].Outputs.Unpack(output)
	if err != nil {
		return nil, err
	}

	if len(res) == 0 {
		return nil, errors.New("Unexpected output")
	}

	amountOut := res[0].([]*big.Int)

	return amountOut, nil
}

func (this *UniswapV2) BuildSwapExactTokensForTokensSupportingFeeOnTransferTokensInput(amountToken *big.Int, amountOutMin *big.Int, path []common.Address, to common.Address, deadline *big.Int) (*hexutil.Bytes, error) {
	input, err := this.abi.Pack("swapExactTokensForTokensSupportingFeeOnTransferTokens", amountToken, amountOutMin, path, to, deadline)

	if err != nil {
		return nil, err
	}
	hexInput := hexutil.Bytes(input)

	return &hexInput, nil
}

func (this *UniswapV2) BuildGetAmountsOutInput(amountToken *big.Int, path []common.Address) (*hexutil.Bytes, error) {
	input, err := this.abi.Pack("getAmountsOut", amountToken, path)

	if err != nil {
		return nil, err
	}
	hexInput := hexutil.Bytes(input)

	return &hexInput, nil
}

func (this *UniswapV2) ParseGetAmountsOutOutput(output []byte) ([]*big.Int, error) {
	res, err := this.abi.Methods["getAmountsOut"].Outputs.Unpack(output)
	if err != nil {
		return nil, err
	}

	if len(res) == 0 {
		return nil, errors.New("Unexpected output")
	}

	amountOut := res[0].([]*big.Int)

	return amountOut, nil
}

func (this *UniswapV2) BuildSwapExactETHForTokensSupportingFeeOnTransferTokensInput(amountOutMin *big.Int, path []common.Address, to common.Address, deadline *big.Int) (*hexutil.Bytes, error) {
	input, err := this.abi.Pack("swapExactETHForTokensSupportingFeeOnTransferTokens", amountOutMin, path, to, deadline)

	if err != nil {
		return nil, err
	}
	hexInput := hexutil.Bytes(input)

	return &hexInput, nil
}

func (this *UniswapV2) ParseSwapTx(log *types.Log, receipt map[string]interface{}, pair *DTPair, tx *types.Transaction) (bool, *big.Int, *big.Int, *big.Int, *common.Address) {
	effectiveGasPrice := big.NewInt(0)
	gasUsed := uint64(0)
	if receipt != nil {
		effectiveGasPrice = receipt["effectiveGasPrice"].(*hexutil.Big).ToInt()
		gasUsed = uint64(receipt["gasUsed"].(hexutil.Uint64))
	}
	var (
		isBuy     bool
		amountIn  *big.Int
		amountOut *big.Int
		to        common.Address
		fee       *big.Int = new(big.Int).Mul(effectiveGasPrice, new(big.Int).SetUint64(gasUsed))
	)

	var inputs map[string]interface{} = make(map[string]interface{})
	this.abiPair.Events["Swap"].Inputs.UnpackIntoMap(inputs, log.Data)
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
	if WETH_ADDRESS.Big().Cmp(pair.token.Big()) < 0 {
		if amount0In.Cmp(big.NewInt(0)) > 0 {
			isBuy = true
			amountIn = amount0In
			amountOut = amount1Out
		} else {
			isBuy = false
			amountIn = amount1In
			amountOut = amount0Out
		}
	} else {
		if amount1In.Cmp(big.NewInt(0)) > 0 {
			isBuy = true
			amountIn = amount1In
			amountOut = amount0Out
		} else {
			isBuy = false
			amountIn = amount0In
			amountOut = amount1Out
		}
	}
	to = common.HexToAddress(log.Topics[2].Hex())
	return isBuy, amountIn, amountOut, fee, &to
}

func (this *UniswapV2) ParseAddLiquidity(tx *types.Transaction, erc20 *Erc20, blockNumber rpc.BlockNumber) (*common.Address, *common.Address, *big.Int, *big.Int, error) {
	var (
		token        *common.Address = nil
		baseToken    *common.Address = nil
		baseReserve  *big.Int        = nil
		tokenReserve *big.Int        = nil
	)
	data := tx.Data()
	if len(data) < 4 {
		return nil, nil, nil, nil, nil
	}
	methodSig := data[:4]

	if bytes.Equal(methodSig, this.abi.Methods["addLiquidity"].ID) {
		args, err := this.abi.Methods["addLiquidity"].Inputs.Unpack(data[4:])
		if err != nil {
			return nil, nil, nil, nil, nil
		}

		token0 := args[0].(common.Address)
		token1 := args[1].(common.Address)
		reserve0 := args[2].(*big.Int)
		reserve1 := args[3].(*big.Int)
		if isBaseToken(token0.Hex()) {
			token = &token1
			baseToken = &token0
			tokenReserve = reserve1
			baseReserve = reserve0
		} else if isBaseToken(token1.Hex()) {
			baseToken = &token1
			token = &token0
			tokenReserve = reserve0
			baseReserve = reserve1
		} else {
			return nil, nil, nil, nil, nil
		}
	} else if bytes.Equal(methodSig, this.abi.Methods["addLiquidityETH"].ID) {
		args, err := this.abi.Methods["addLiquidityETH"].Inputs.Unpack(data[4:])
		if err != nil {
			return nil, nil, nil, nil, nil
		}

		token0 := args[0].(common.Address)
		baseToken = &WETH_ADDRESS
		tokenReserve = args[1].(*big.Int)
		baseReserve = tx.Value()
		token = &token0
	} else {
		return nil, nil, nil, nil, nil
	}

	return token, baseToken, tokenReserve, baseReserve, nil
}

func (this *UniswapV2) IsOpenTrading(tx *types.Transaction) bool {
	data := tx.Data()
	if len(data) < 4 {
		return false
	}

	methodSig := data[:4]
	if !bytes.Equal(methodSig, []byte{201, 86, 123, 249}) {
		return false
	}
	return true
}
