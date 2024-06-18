package darktrader

import (
	"context"
	"fmt"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/rpc"
)

type DegenDetector struct {
	tokens     []string
	pairs      map[string]*DTPair
	pairsMutex sync.RWMutex
	config     *DTConfig

	erc20     *Erc20
	uniswapv2 *UniswapV2
	txApi     *ethapi.TransactionAPI
}

func NewDegenDetector(_erc20 *Erc20, _uniswapv2 *UniswapV2, _txApi *ethapi.TransactionAPI) *DegenDetector {
	dd := DegenDetector{}

	dd.Init(_erc20, _uniswapv2, _txApi)

	return &dd
}

func (dd *DegenDetector) SetConfig(config *DTConfig) {
	dd.config = config
}

func (dd *DegenDetector) Init(_erc20 *Erc20, _uniswapv2 *UniswapV2, _txApi *ethapi.TransactionAPI) {
	dd.pairs = make(map[string]*DTPair)
	dd.pairsMutex = sync.RWMutex{}

	dd.erc20 = _erc20
	dd.uniswapv2 = _uniswapv2
	dd.txApi = _txApi
}

func (dd *DegenDetector) CheckEventLogs(head *types.Block, blkLogs []*types.Log) bool {

	// blkNo := head.Number()
	// blkTime := time.Unix(int64(head.Time()), 0)
	// newTokens := 0

	txs := head.Transactions()
	for _, tx := range txs {
		receipt, err := dd.txApi.GetTransactionReceipt(context.Background(), tx.Hash())
		if err != nil {
			continue
		}

		dd.CheckFailedTxLogs(head, tx, receipt)
	}

	return true
}

func (dd *DegenDetector) CheckFailedTxLogs(head *types.Block, tx *types.Transaction, receipt map[string]interface{}) {
	// degen trader
	if dd.config.degenTraderConfig.enabled && tx.To() != nil {
		if tx.To().Hex() == dd.config.degenTraderConfig.contractAddress {
			token := common.BytesToAddress(tx.Data()[4:36])
			if dd.onDetectNewToken(token, nil, "") {
				fmt.Println("[DD] - Degen trader token detected", token.Hex())
			}
		}
	}
}

func (dd *DegenDetector) onDetectNewToken(token common.Address, origin *common.Address, originName string) bool {
	pair, err := CalculatePoolAddressV2(token.Hex(), WETH_ADDRESS.Hex())
	if err != nil {
		return false
	}

	isErc20, name, symbol, decimals, owner, totalSupply := dd.erc20.IsErc20Token(dd.config.testAccountAddress, token, rpc.LatestBlockNumber, nil)
	if !isErc20 {
		return false
	}

	baseReserve, tokenReserve, err := dd.erc20.GetPairReserves(&pair, &WETH_ADDRESS, &token, rpc.LatestBlockNumber)
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
		isMarkedByDegen:           true,
	}

	dd.addNewPair(&objPair)
	return true
}

func (dd *DegenDetector) addNewPair(pair *DTPair) {
	token := pair.token.Hex()
	if _pair, exists := dd.pairs[token]; !exists {
		dd.pairsMutex.Lock()
		dd.pairs[token] = pair
		dd.tokens = append(dd.tokens, token)
		dd.pairsMutex.Unlock()
	} else if pair.origin != nil {
		_pair.origin = pair.origin
		_pair.originName = pair.originName
	}
}
