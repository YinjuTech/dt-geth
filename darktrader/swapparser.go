package darktrader

import (
	"bytes"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type SwapParserResult struct {
	address         *common.Address
	router          string
	routerAddr      *common.Address
	path            []common.Address
	amtIn           *big.Int
	amtOut          *big.Int
	maxAmountIn     *big.Int
	priceMax        *big.Float
	exactAmountType ExactAmountType
	swapParams      []*big.Int
}

func (this *SwapParserResult) GetDexRouter() string {
	if this.router == "Contract" {
		router, exists := dexRouterAddrs[this.address.Hex()]
		if !exists {
			router = "Contract"
		}
		return router
	}
	return this.router
}

type SwapParser struct {
	erc20     *Erc20
	uniswapV2 *UniswapV2

	abiUniswapV2                      *abi.ABI
	abiUniswapUniversal               abi.ABI
	abiUniswapUniversalRouterInternal abi.ABI
	// abiMetaMaskSwap                   abi.ABI
	// abi1inchV3                        abi.ABI
	// abi1inchV4                        abi.ABI
	// abi1inchV5                        abi.ABI
	// abiKyberSwapMeta                  abi.ABI
	// abiKyberSwapV2                    abi.ABI
	// abiKyberSwapV3                    abi.ABI
	// abiKyberSwapMetaV2                abi.ABI
}

func NewSwapParser(erc20 *Erc20, uniswapV2 *UniswapV2) *SwapParser {
	swapParser := SwapParser{}

	swapParser.erc20 = erc20
	swapParser.uniswapV2 = uniswapV2

	swapParser.Init()
	return &swapParser
}

func (this *SwapParser) Init() {
	this.abiUniswapV2 = &this.uniswapV2.abi
	this.abiUniswapUniversal, _ = abi.JSON(strings.NewReader(ABI_UNISWAP_UNIVERSAL_ROUTER))
	this.abiUniswapUniversalRouterInternal, _ = abi.JSON(strings.NewReader(ABI_UNISWAP_UNIVERSAL_ROUTER_INTERNAL))
	// this.abiMetaMaskSwap, _ = abi.JSON(strings.NewReader(ABI_METAMASK_SWAP))
	// this.abi1inchV3, _ = abi.JSON(strings.NewReader(ABI_1INCH_V3))
	// this.abi1inchV4, _ = abi.JSON(strings.NewReader(ABI_1INCH_V4))
	// this.abi1inchV5, _ = abi.JSON(strings.NewReader(ABI_1INCH_V5))
	// this.abiKyberSwapMeta, _ = abi.JSON(strings.NewReader(ABI_KYBERSWAP_META))
	// this.abiKyberSwapV2, _ = abi.JSON(strings.NewReader(ABI_KYBERSWAP_V2))
	// this.abiKyberSwapV3, _ = abi.JSON(strings.NewReader(ABI_KYBERSWAP_V3))
	// this.abiKyberSwapMetaV2, _ = abi.JSON(strings.NewReader(ABI_KYBERSWAP_META_V2))
}

func (this *SwapParser) ProcessRouterPendingTx(tx *types.Transaction) *SwapParserResult {
	if tx.To() == nil {
		return nil
	}
	data := tx.Data()
	if len(data) < 4 {
		return nil
	}
	to := tx.To().Hex()
	if to == v2RouterAddr || to == univRouterAddr || to == univRouterAddrNewObj.Hex() || to == v3RouterAddr.Hex() {
		return this.ParseUniswap(tx)
	}
	return nil
}

func (this *SwapParser) ParseUniswap(tx *types.Transaction) *SwapParserResult {
	data := tx.Data()
	methodSig := data[:4]

	// extract tx info
	var result = SwapParserResult{}
	txFrom, _ := GetFrom(tx)
	result.address = &txFrom
	txTo := tx.To().Hex()
	result.maxAmountIn = new(big.Int)

	if txTo == v2RouterAddr {
		if bytes.Equal(methodSig, this.abiUniswapV2.Methods["swapExactTokensForTokens"].ID) {
			args, _ := this.abiUniswapV2.Methods["swapExactTokensForTokens"].Inputs.Unpack(data[4:])
			if len(args) < 5 {
				return nil
			}
			result.amtIn = args[0].(*big.Int)
			result.path = args[2].([]common.Address)
			amtOutMin := args[1].(*big.Int)

			result.maxAmountIn.Set(result.amtIn)
			if result.amtIn != nil && amtOutMin != nil && result.amtIn.Cmp(big.NewInt(0)) > 0 && amtOutMin.Cmp(big.NewInt(0)) > 0 {
				result.priceMax = new(big.Float).Quo(new(big.Float).SetInt(result.amtIn), new(big.Float).SetInt(amtOutMin))
			}

			result.swapParams = []*big.Int{result.amtIn, amtOutMin}
			result.exactAmountType = ExactAmountIn
		} else if bytes.Equal(methodSig, this.abiUniswapV2.Methods["swapTokensForExactTokens"].ID) {
			args, _ := this.abiUniswapV2.Methods["swapTokensForExactTokens"].Inputs.Unpack(data[4:])
			if len(args) < 5 {
				return nil
			}
			amtInMax := args[1].(*big.Int)
			result.amtOut = args[0].(*big.Int)
			result.path = args[2].([]common.Address)
			result.maxAmountIn.Set(amtInMax)
			if amtInMax != nil && result.amtOut != nil && amtInMax.Cmp(big.NewInt(0)) > 0 && result.amtOut.Cmp(big.NewInt(0)) > 0 {
				result.priceMax = new(big.Float).Quo(new(big.Float).SetInt(amtInMax), new(big.Float).SetInt(result.amtOut))
			}

			result.swapParams = []*big.Int{result.amtOut, amtInMax}
			result.exactAmountType = ExactAmountOut
		} else if bytes.Equal(methodSig, this.abiUniswapV2.Methods["swapExactETHForTokens"].ID) {
			args, _ := this.abiUniswapV2.Methods["swapExactETHForTokens"].Inputs.Unpack(data[4:])
			if len(args) < 4 {
				return nil
			}
			result.amtIn = tx.Value()
			result.path = args[1].([]common.Address)
			amtOutMin := args[0].(*big.Int)
			result.maxAmountIn.Set(result.amtIn)
			if result.amtIn != nil && amtOutMin != nil && result.amtIn.Cmp(big.NewInt(0)) > 0 && amtOutMin.Cmp(big.NewInt(0)) > 0 {
				result.priceMax = new(big.Float).Quo(new(big.Float).SetInt(result.amtIn), new(big.Float).SetInt(amtOutMin))
			}

			result.swapParams = []*big.Int{result.amtIn, amtOutMin}
			result.exactAmountType = ExactAmountIn
		} else if bytes.Equal(methodSig, this.abiUniswapV2.Methods["swapETHForExactTokens"].ID) {
			args, _ := this.abiUniswapV2.Methods["swapETHForExactTokens"].Inputs.Unpack(data[4:])
			if len(args) < 4 {
				return nil
			}
			result.amtOut = args[0].(*big.Int)
			result.path = args[1].([]common.Address)
			amtInMax := tx.Value()
			result.maxAmountIn.Set(amtInMax)
			if amtInMax != nil && result.amtOut != nil && amtInMax.Cmp(big.NewInt(0)) > 0 && result.amtOut.Cmp(big.NewInt(0)) > 0 {
				result.priceMax = new(big.Float).Quo(new(big.Float).SetInt(amtInMax), new(big.Float).SetInt(result.amtOut))
			}

			result.swapParams = []*big.Int{result.amtOut, amtInMax}
			result.exactAmountType = ExactAmountOut
		} else if bytes.Equal(methodSig, this.abiUniswapV2.Methods["swapExactTokensForTokensSupportingFeeOnTransferTokens"].ID) {
			args, _ := this.abiUniswapV2.Methods["swapExactTokensForTokensSupportingFeeOnTransferTokens"].Inputs.Unpack(data[4:])
			if len(args) < 5 {
				return nil
			}
			result.amtIn = args[0].(*big.Int)
			result.path = args[2].([]common.Address)
			amtOutMin := args[1].(*big.Int)
			result.maxAmountIn.Set(result.amtIn)
			if result.amtIn != nil && amtOutMin != nil && result.amtIn.Cmp(big.NewInt(0)) > 0 && amtOutMin.Cmp(big.NewInt(0)) > 0 {
				result.priceMax = new(big.Float).Quo(new(big.Float).SetInt(result.amtIn), new(big.Float).SetInt(amtOutMin))
			}

			result.swapParams = []*big.Int{result.amtIn, amtOutMin}
			result.exactAmountType = ExactAmountIn
		} else if bytes.Equal(methodSig, this.abiUniswapV2.Methods["swapExactETHForTokensSupportingFeeOnTransferTokens"].ID) {
			args, _ := this.abiUniswapV2.Methods["swapExactETHForTokensSupportingFeeOnTransferTokens"].Inputs.Unpack(data[4:])
			if len(args) < 4 {
				return nil
			}
			result.amtIn = tx.Value()
			result.path = args[1].([]common.Address)
			amtOutMin := args[0].(*big.Int)
			result.maxAmountIn.Set(result.amtIn)
			if result.amtIn != nil && amtOutMin != nil && result.amtIn.Cmp(big.NewInt(0)) > 0 && amtOutMin.Cmp(big.NewInt(0)) > 0 {
				result.priceMax = new(big.Float).Quo(new(big.Float).SetInt(result.amtIn), new(big.Float).SetInt(amtOutMin))
			}

			result.swapParams = []*big.Int{result.amtIn, amtOutMin}
			result.exactAmountType = ExactAmountIn
		} else {
			return nil
		}
		result.router = "UniswapV2"
	} else if txTo == univRouterAddr || txTo == univRouterAddrNewObj.Hex() {
		args, _ := this.abiUniswapUniversal.Methods["execute"].Inputs.Unpack(data[4:])
		if len(args) < 2 {
			return nil
		}
		var (
			inputs  []interface{}
			cmdType int
			err     error
		)
		args1 := args[1].([][]byte)

		for idx, cmd := range args[0].([]byte) {
			if idx >= len(args1) {
				return nil
			}
			if cmd == 8 || cmd == 88 { // V2_SWAP_EXACT_IN
				inputs, err = this.abiUniswapUniversalRouterInternal.Methods["v2SwapExactInput"].Inputs.Unpack(args1[idx])
				if err != nil {
					return nil
				}
				cmdType = 8
				break
			} else if cmd == 9 || cmd == 99 { // V2_SWAP_EXACT_OUT
				inputs, err = this.abiUniswapUniversalRouterInternal.Methods["v2SwapExactOutput"].Inputs.Unpack(args1[idx])
				if err != nil {
					return nil
				}
				cmdType = 9
				break
			}
		}
		if len(inputs) != 5 {
			return nil
		}
		if cmdType == 8 {
			result.amtIn = inputs[1].(*big.Int)
			amtOutMin := inputs[2].(*big.Int)

			result.maxAmountIn.Set(result.amtIn)
			if result.amtIn != nil && amtOutMin != nil && result.amtIn.Cmp(big.NewInt(0)) > 0 && amtOutMin.Cmp(big.NewInt(0)) > 0 {
				result.priceMax = new(big.Float).Quo(new(big.Float).SetInt(result.amtIn), new(big.Float).SetInt(amtOutMin))
			}

			result.swapParams = []*big.Int{result.amtIn, amtOutMin}
			result.exactAmountType = ExactAmountIn
		} else if cmdType == 9 {
			result.amtOut = inputs[1].(*big.Int)
			amtInMax := inputs[2].(*big.Int)
			result.maxAmountIn.Set(amtInMax)
			if amtInMax != nil && result.amtOut != nil && amtInMax.Cmp(big.NewInt(0)) > 0 && result.amtOut.Cmp(big.NewInt(0)) > 0 {
				result.priceMax = new(big.Float).Quo(new(big.Float).SetInt(amtInMax), new(big.Float).SetInt(result.amtOut))
			}

			result.swapParams = []*big.Int{result.amtOut, amtInMax}
			result.exactAmountType = ExactAmountOut
		}
		result.path = inputs[3].([]common.Address)
		result.router = "Universal"
	}

	return &result
}
