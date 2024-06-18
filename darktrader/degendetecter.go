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

func (dd *DegenDetector) updatePairInfo(pair *DTPair) {
	if pair.tokenReserve.Cmp(big.NewInt(0)) == 0 || pair.baseReserve.Cmp(big.NewInt(0)) == 0 {
		baseReserve, tokenReserve, err := dd.erc20.GetPairReserves(
			pair.address,
			pair.baseToken,
			pair.token,
			rpc.LatestBlockNumber,
		)

		if err != nil || baseReserve.Cmp(big.NewInt(0)) == 0 || tokenReserve.Cmp(big.NewInt(0)) == 0 {

			if pair.triggerTx != nil {
				txs := []*types.Transaction{pair.triggerTx}
				baseReserve, tokenReserve, err, _ = dd.erc20.GetPairReservesAfterTxs(
					pair.address,
					pair.baseToken,
					pair.token,
					txs,
					rpc.LatestBlockNumber,
					true,
				)
			} else {
				baseReserve, tokenReserve, err = dd.erc20.GetPairReserves(
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

func (dd *DegenDetector) onDetectPendingInitTrade(token *common.Address, baseToken *common.Address, tokenReserve *big.Int, baseReserve *big.Int, tx *types.Transaction) {
	_pair, exists := dd.pairs[token.Hex()]
	if !exists {
		return
	}

	if tx != nil {
		_pair.triggerTx = tx
	}
	if baseToken != nil {
		_pair.baseToken = baseToken
	}
	if tokenReserve != nil {
		_pair.tokenReserve = tokenReserve
	}
	if baseReserve != nil {
		_pair.baseReserve = baseReserve
	}

	dd.updatePairInfo(_pair)
}

func (dd *DegenDetector) onDetectPendingTxForToken(token *common.Address, tx *types.Transaction) {
	_pair, exists := dd.pairs[token.Hex()]
	if !exists {
		return
	}
	_pair.triggerTx = tx
	dd.updatePairInfo(_pair)

	dd.onDetectPendingInitTrade(token, nil, nil, nil, tx)
}
