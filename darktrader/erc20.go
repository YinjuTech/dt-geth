package darktrader

import (
	"context"
	"errors"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/rpc"
)

type Erc20 struct {
	abi abi.ABI

	bcApi *ethapi.BlockChainAPI
}

func NewErc20(bcApi *ethapi.BlockChainAPI) *Erc20 {
	erc20 := Erc20{}

	erc20.Init(bcApi)

	return &erc20
}

func (this *Erc20) Init(bcApi *ethapi.BlockChainAPI) {
	this.abi, _ = abi.JSON(strings.NewReader(ABI_ERC20))

	this.bcApi = bcApi
}

func (this *Erc20) IsErc20Token(from common.Address, token common.Address, blockNumber rpc.BlockNumber, overrides *ethapi.StateOverride) (bool, string, string, uint8, *common.Address, *big.Int) {
	name, err := this.callName(&from, &token, blockNumber, overrides)
	if err != nil {
		return false, "", "", 0, nil, nil
	}
	_, err = this.callBalanceOf(&from, &token, blockNumber, overrides)
	if err != nil {
		return false, "", "", 0, nil, nil
	}
	symbol, err := this.callSymbol(&from, &token, blockNumber, overrides)
	if err != nil {
		return false, "", "", 0, nil, nil
	}
	decimals, err := this.callDecimals(&from, &token, blockNumber, overrides)
	if err != nil {
		return false, "", "", 0, nil, nil
	}
	_, err = this.callAllowance(&from, &token, &from, &token, blockNumber, overrides)
	if err != nil {
		return false, "", "", 0, nil, nil
	}
	owner, _ := this.callOwner(&from, &token, blockNumber, overrides)
	if err != nil {
		return false, "", "", 0, nil, nil
	}

	totalSupply, _ := this.CallTotalSupply(&token, blockNumber, overrides)
	if err != nil {
		return false, "", "", 0, nil, nil
	}

	if name == "" || symbol == "" {
		return false, "", "", 0, nil, nil
	}

	return true, name, symbol, decimals, owner, totalSupply
}

func (this *Erc20) BuildNameInput() (*hexutil.Bytes, error) {
	input, err := this.abi.Pack("name")
	if err != nil {
		return nil, err
	}

	hexInput := hexutil.Bytes(input)

	return &hexInput, nil
}

func (this *Erc20) ParseNameOutput(output []byte) (string, error) {
	res, err := this.abi.Methods["name"].Outputs.Unpack(output)
	if err != nil {
		return "", err
	}

	if len(res) == 0 {
		return "", errors.New("Unexpected output")
	}

	name := res[0].(string)

	return name, nil
}

func (this *Erc20) callName(from *common.Address, token *common.Address, blockNumber rpc.BlockNumber, overrides *ethapi.StateOverride) (string, error) {
	input, err := this.BuildNameInput()
	if err != nil {
		return "", err
	}

	args := ethapi.TransactionArgs{
		From:  from,
		To:    token,
		Input: input,
	}
	blockNo := rpc.BlockNumberOrHashWithNumber(blockNumber)
	res, err := this.bcApi.Call(context.Background(), args, &blockNo, overrides, nil)
	if err != nil {
		return "", err
	}

	name, err := this.ParseNameOutput(res)
	if err != nil {
		return "", err
	}

	return name, nil
}

func (this *Erc20) BuildSymbolInput() (*hexutil.Bytes, error) {
	input, err := this.abi.Pack("symbol")
	if err != nil {
		return nil, err
	}

	hexInput := hexutil.Bytes(input)

	return &hexInput, nil
}

func (this *Erc20) ParseSymbolOutput(output []byte) (string, error) {
	res, err := this.abi.Methods["symbol"].Outputs.Unpack(output)
	if err != nil {
		return "", err
	}

	if len(res) == 0 {
		return "", errors.New("Unexpected output")
	}

	symbol := res[0].(string)

	return symbol, nil
}

func (this *Erc20) callSymbol(from *common.Address, token *common.Address, blockNumber rpc.BlockNumber, overrides *ethapi.StateOverride) (string, error) {
	input, err := this.BuildSymbolInput()
	if err != nil {
		return "", err
	}

	arg := ethapi.TransactionArgs{
		From:  from,
		To:    token,
		Input: input,
	}
	blockNo := rpc.BlockNumberOrHashWithNumber(blockNumber)
	res, err := this.bcApi.Call(context.Background(), arg, &blockNo, overrides, nil)
	if err != nil {
		return "", err
	}

	symbol, err := this.ParseSymbolOutput(res)
	if err != nil {
		return "", err
	}

	return symbol, nil
}

func (this *Erc20) BuildBalanceOfInput(address *common.Address) (*hexutil.Bytes, error) {
	input, err := this.abi.Pack("balanceOf", address)
	if err != nil {
		return nil, err
	}

	hexInput := hexutil.Bytes(input)

	return &hexInput, nil
}

func (this *Erc20) ParseBalanceOfOutput(output []byte) (*big.Int, error) {
	res, err := this.abi.Methods["balanceOf"].Outputs.Unpack(output)
	if err != nil {
		return big.NewInt(0), err
	}

	if len(res) == 0 {
		return big.NewInt(0), errors.New("Unexpected output")
	}

	balance := res[0].(*big.Int)

	return balance, nil
}

func (this *Erc20) callBalanceOf(address *common.Address, token *common.Address, blockNumber rpc.BlockNumber, overrides *ethapi.StateOverride) (*big.Int, error) {
	input, err := this.BuildBalanceOfInput(address)
	if err != nil {
		return nil, err
	}

	arg := ethapi.TransactionArgs{
		From:  address,
		To:    token,
		Input: input,
	}
	blockNo := rpc.BlockNumberOrHashWithNumber(blockNumber)
	res, err := this.bcApi.Call(context.Background(), arg, &blockNo, overrides, nil)
	if err != nil {
		return nil, err
	}

	balance, err := this.ParseBalanceOfOutput(res)

	if err != nil {
		return nil, err
	}

	return balance, nil
}

func (this *Erc20) BuildApproveInput(spender *common.Address, amount *big.Int) (*hexutil.Bytes, error) {
	input, err := this.abi.Pack("approve", spender, amount)
	if err != nil {
		return nil, err
	}

	hexInput := hexutil.Bytes(input)

	return &hexInput, nil
}

func (this *Erc20) ParseApproveOutput(output []byte) (bool, error) {
	res, err := this.abi.Methods["approve"].Outputs.Unpack(output)
	if err != nil {
		return false, err
	}

	if len(res) == 0 {
		return false, errors.New("Unexpected output")
	}

	result := res[0].(bool)

	return result, nil
}

func (this *Erc20) callApprove(from *common.Address, token *common.Address, spender *common.Address, amount *big.Int, blockNumber rpc.BlockNumber, overrides *ethapi.StateOverride) (bool, error) {
	input, err := this.BuildApproveInput(spender, amount)
	if err != nil {
		return false, err
	}

	arg := ethapi.TransactionArgs{
		From:  from,
		To:    token,
		Input: input,
	}
	blockNo := rpc.BlockNumberOrHashWithNumber(blockNumber)
	res, err := this.bcApi.Call(context.Background(), arg, &blockNo, overrides, nil)
	if err != nil {
		return false, err
	}

	result, err := this.ParseApproveOutput(res)

	if err != nil {
		return false, err
	}

	return result, nil
}

func (this *Erc20) BuildAllowanceInput(owner *common.Address, spender *common.Address) (*hexutil.Bytes, error) {
	input, err := this.abi.Pack("allowance", owner, spender)
	if err != nil {
		return nil, err
	}

	hexInput := hexutil.Bytes(input)

	return &hexInput, nil
}

func (this *Erc20) ParseAllowanceOutput(output []byte) (*big.Int, error) {
	res, err := this.abi.Methods["allowance"].Outputs.Unpack(output)
	if err != nil {
		return big.NewInt(0), err
	}

	if len(res) == 0 {
		return big.NewInt(0), errors.New("Unexpected output")
	}

	balance := res[0].(*big.Int)

	return balance, nil
}

func (this *Erc20) callAllowance(from *common.Address, token *common.Address, owner *common.Address, spender *common.Address, blockNumber rpc.BlockNumber, overrides *ethapi.StateOverride) (*big.Int, error) {
	input, err := this.BuildAllowanceInput(owner, spender)
	if err != nil {
		return big.NewInt(0), err
	}

	arg := ethapi.TransactionArgs{
		From:  from,
		To:    token,
		Input: input,
	}
	blockNo := rpc.BlockNumberOrHashWithNumber(blockNumber)
	res, err := this.bcApi.Call(context.Background(), arg, &blockNo, overrides, nil)
	if err != nil {
		return big.NewInt(0), err
	}

	balance, err := this.ParseAllowanceOutput(res)

	if err != nil {
		return big.NewInt(0), err
	}

	return balance, nil
}

func (this *Erc20) BuildDecimalsInput() (*hexutil.Bytes, error) {
	input, err := this.abi.Pack("decimals")
	if err != nil {
		return nil, err
	}

	hexInput := hexutil.Bytes(input)

	return &hexInput, nil
}

func (this *Erc20) ParseDecimalsOutput(output []byte) (uint8, error) {
	res, err := this.abi.Methods["decimals"].Outputs.Unpack(output)
	if err != nil {
		return 0, err
	}

	if len(res) == 0 {
		return 0, errors.New("Unexpected output")
	}

	balance := res[0].(uint8)

	return balance, nil
}

func (this *Erc20) callDecimals(from *common.Address, token *common.Address, blockNumber rpc.BlockNumber, overrides *ethapi.StateOverride) (uint8, error) {
	input, err := this.BuildDecimalsInput()
	if err != nil {
		return 0, err
	}

	arg := ethapi.TransactionArgs{
		From:  from,
		To:    token,
		Input: input,
	}
	blockNo := rpc.BlockNumberOrHashWithNumber(blockNumber)
	res, err := this.bcApi.Call(context.Background(), arg, &blockNo, overrides, nil)
	if err != nil {
		return 0, err
	}

	decimals, err := this.ParseDecimalsOutput(res)
	if err != nil {
		return 0, err
	}

	return decimals, nil
}

func (this *Erc20) BuildOwnerInput() (*hexutil.Bytes, error) {
	input, err := this.abi.Pack("owner")
	if err != nil {
		return nil, err
	}

	hexInput := hexutil.Bytes(input)

	return &hexInput, nil
}

func (this *Erc20) ParseOwnerOutput(output []byte) (*common.Address, error) {
	res, err := this.abi.Methods["owner"].Outputs.Unpack(output)
	if err != nil {
		return nil, err
	}

	if len(res) == 0 {
		return nil, errors.New("Unexpected output")
	}

	address := res[0].(common.Address)

	return &address, nil
}

func (this *Erc20) callOwner(from *common.Address, token *common.Address, blockNumber rpc.BlockNumber, overrides *ethapi.StateOverride) (*common.Address, error) {
	input, err := this.BuildOwnerInput()
	if err != nil {
		return nil, err
	}

	arg := ethapi.TransactionArgs{
		From:  from,
		To:    token,
		Input: input,
	}
	blockNo := rpc.BlockNumberOrHashWithNumber(blockNumber)
	res, err := this.bcApi.Call(context.Background(), arg, &blockNo, overrides, nil)
	if err != nil {
		return nil, err
	}

	owner, err := this.ParseOwnerOutput(res)
	if err != nil {
		return nil, err
	}

	return owner, nil
}

func (this *Erc20) getRpcBlockNumber(blockNumber *big.Int) rpc.BlockNumber {
	blockNr := rpc.PendingBlockNumber
	if blockNumber != nil {
		blockNr = rpc.BlockNumber(blockNumber.Int64())
	}

	return blockNr
}

func (this *Erc20) BuildBatchCallArgs(tx *types.Transaction, ignoreGasFee bool) ethapi.TransactionArgs {
	chainID := hexutil.Big(*tx.ChainId())
	from, _ := GetFrom(tx)
	value := hexutil.Big(*tx.Value())
	input := hexutil.Bytes(tx.Data())
	gas := tx.Gas()

	txArg := ethapi.TransactionArgs{
		ChainID: &chainID,
		From:    &from,
		To:      tx.To(),
		Value:   &value,
		Input:   &input,
		Gas:     (*hexutil.Uint64)(&gas),
	}

	if !ignoreGasFee {
		if tx.Type() == 2 {
			maxFeePerGas := hexutil.Big(*new(big.Int).Add(tx.GasFeeCap(), tx.GasTipCap()))
			maxPriorityFeePerGas := hexutil.Big(*tx.GasTipCap())
			txArg.MaxFeePerGas = &maxFeePerGas
			txArg.MaxPriorityFeePerGas = &maxPriorityFeePerGas
		} else {
			gasPrice := hexutil.Big(*tx.GasPrice())
			txArg.GasPrice = &gasPrice
		}
	}

	return txArg
}

func (this *Erc20) CallTx(tx *types.Transaction, triggerTx *types.Transaction, blockNumber rpc.BlockNumber, considerGasPrice bool) (hexutil.Bytes, error) {

	chainID := hexutil.Big(*tx.ChainId())
	if triggerTx == nil {
		// blockNr := this.getRpcBlockNumber(blockNumber)
		from, _ := GetFrom(tx)
		value := hexutil.Big(*tx.Value())
		input := hexutil.Bytes(tx.Data())
		gas := tx.Gas()

		txArgs := ethapi.TransactionArgs{
			ChainID: &chainID,
			From:    &from,
			To:      tx.To(),
			Value:   &value,
			Input:   &input,
			Gas:     (*hexutil.Uint64)(&gas),
		}
		if considerGasPrice {
			if tx.GasPrice() != nil {
				gasPrice := hexutil.Big(*tx.GasPrice())
				txArgs.GasPrice = &gasPrice
			} else {
				maxFeePerGas := hexutil.Big(*new(big.Int).Add(tx.GasFeeCap(), tx.GasTipCap()))
				maxPriorityFeePerGas := hexutil.Big(*tx.GasTipCap())
				txArgs.MaxFeePerGas = &maxFeePerGas
				txArgs.MaxPriorityFeePerGas = &maxPriorityFeePerGas
			}
		}
		blockNo := rpc.BlockNumberOrHashWithNumber(blockNumber)
		output, err := this.bcApi.Call(
			context.Background(),
			txArgs,
			&blockNo,
			nil,
			nil,
		)

		return output, err
	} else {
		var batchCallConfig = ethapi.SimOpts{
			BlockStateCalls:        make([]ethapi.SimBlock, 1),
			TraceTransfers:         false,
			Validation:             false,
			ReturnFullTransactions: true,
		}

		from, _ := GetFrom(triggerTx)
		triggerNonce := hexutil.Uint64(triggerTx.Nonce())
		value := hexutil.Big(*triggerTx.Value())
		input := hexutil.Bytes(triggerTx.Data())
		batchCallConfig.BlockStateCalls[0] = ethapi.SimBlock{
			BlockOverrides: nil,
			StateOverrides: nil,
			Calls:          make([]ethapi.TransactionArgs, 2),
		}
		batchCallConfig.BlockStateCalls[0].Calls[0] = ethapi.TransactionArgs{
			ChainID: &chainID,
			From:    &from,
			To:      triggerTx.To(),
			Value:   &value,
			Input:   &input,
			Nonce:   &triggerNonce,
		}

		// 1
		from, _ = GetFrom(tx)
		value = hexutil.Big(*tx.Value())
		input = hexutil.Bytes(tx.Data())
		gas := tx.Gas()

		batchCallConfig.BlockStateCalls[0].Calls[1] = ethapi.TransactionArgs{
			ChainID: &chainID,
			From:    &from,
			To:      tx.To(),
			Value:   &value,
			Input:   &input,
			Gas:     (*hexutil.Uint64)(&gas),
		}

		if considerGasPrice {
			if tx.GasPrice() != nil {
				gasPrice := hexutil.Big(*tx.GasPrice())
				batchCallConfig.BlockStateCalls[0].Calls[1].GasPrice = &gasPrice
			} else {
				maxFeePerGas := hexutil.Big(*new(big.Int).Add(tx.GasFeeCap(), tx.GasTipCap()))
				maxPriorityFeePerGas := hexutil.Big(*tx.GasTipCap())
				batchCallConfig.BlockStateCalls[0].Calls[1].MaxFeePerGas = &maxFeePerGas
				batchCallConfig.BlockStateCalls[0].Calls[1].MaxPriorityFeePerGas = &maxPriorityFeePerGas
			}
		}

		blockNoOrHash := rpc.BlockNumberOrHashWithNumber(blockNumber)
		results, err := this.bcApi.SimulateV1(context.Background(), batchCallConfig, &blockNoOrHash)
		if err != nil {
			return nil, err
		}
		if results[1] == nil {
			return nil, nil
		}

		callResult := (results[0]["calls"].([]ethapi.SimCallResult))[1]

		return callResult.ReturnValue, nil
	}
}

// func (this *Erc20) buildGetReservesBatchCallArgs(
// 	pair *common.Address,
// 	baseToken *common.Address,
// 	token *common.Address,
// ) (*ethapi.BatchCallArgs, *ethapi.BatchCallArgs, error) {
// 	var (
// 		baseTokenReserveBatchCallArgs ethapi.BatchCallArgs
// 		tokenReserveBatchCallArgs     ethapi.BatchCallArgs
// 	)

// 	txBalanceOfBaseTokenInput, err := this.BuildBalanceOfInput(pair)
// 	if err != nil {
// 		return nil, nil, err
// 	}
// 	baseTokenReserveBatchCallArgs = ethapi.BatchCallArgs{
// 		TransactionArgs: ethapi.TransactionArgs{
// 			To:    baseToken,
// 			Input: txBalanceOfBaseTokenInput,
// 		},
// 	}

// 	// 2. include balanceOf token tx in batch
// 	txBalanceOfTokenInput, err := this.BuildBalanceOfInput(pair)
// 	if err != nil {
// 		return nil, nil, err
// 	}
// 	tokenReserveBatchCallArgs = ethapi.BatchCallArgs{
// 		TransactionArgs: ethapi.TransactionArgs{
// 			To:    token,
// 			Input: txBalanceOfTokenInput,
// 		},
// 	}

// 	return &baseTokenReserveBatchCallArgs, &tokenReserveBatchCallArgs, nil
// }

func (this *Erc20) GetPairReserves(
	pair *common.Address,
	baseToken *common.Address,
	token *common.Address,
	blockNumber rpc.BlockNumber,
) (*big.Int, *big.Int, error) {
	baseReserve := big.NewInt(0)
	tokenReserve := big.NewInt(0)

	// blockNr := this.getRpcBlockNumber(blockNumber)
	// var batchCallConfig = ethapi.BatchCallConfig{
	// 	Block: rpc.BlockNumberOrHashWithNumber(blockNumber),
	// 	Calls: make([]ethapi.BatchCallArgs, 2),
	// }
	var batchCallConfig = ethapi.SimOpts{
		BlockStateCalls:        make([]ethapi.SimBlock, 1),
		TraceTransfers:         false,
		Validation:             false,
		ReturnFullTransactions: true,
	}
	batchCallConfig.BlockStateCalls[0] = ethapi.SimBlock{
		BlockOverrides: nil,
		StateOverrides: nil,
		Calls:          make([]ethapi.TransactionArgs, 2),
	}

	// 1. include balanceOf baseToken tx in batch
	txBalanceOfBaseTokenInput, err := this.BuildBalanceOfInput(pair)
	if err != nil {
		return baseReserve, tokenReserve, err
	}

	batchCallConfig.BlockStateCalls[0].Calls[0] = ethapi.TransactionArgs{
		To:    baseToken,
		Input: txBalanceOfBaseTokenInput,
	}

	// 2. include balanceOf token tx in batch
	txBalanceOfTokenInput, err := this.BuildBalanceOfInput(pair)
	if err != nil {
		return baseReserve, tokenReserve, err
	}

	batchCallConfig.BlockStateCalls[0].Calls[1] = ethapi.TransactionArgs{
		To:    token,
		Input: txBalanceOfTokenInput,
	}

	blockNoOrHash := rpc.BlockNumberOrHashWithNumber(blockNumber)
	outputs, err := this.bcApi.SimulateV1(context.Background(), batchCallConfig, &blockNoOrHash)

	if err != nil {
		return baseReserve, tokenReserve, err
	}

	callResults := outputs[0]["calls"].([]ethapi.SimCallResult)

	baseReserve, err = this.ParseBalanceOfOutput(callResults[0].ReturnValue)
	if err != nil {
		return baseReserve, tokenReserve, err
	}

	tokenReserve, err = this.ParseBalanceOfOutput(callResults[1].ReturnValue)
	if err != nil {
		return baseReserve, tokenReserve, err
	}

	return baseReserve, tokenReserve, nil
}

// func (this *Erc20) BathCallTxs(
// 	txs []*types.Transaction,
// 	blockNumber rpc.BlockNumber,
// ) ([]ethapi.CallResult, error) {
// 	txCount := 0
// 	if txs != nil {
// 		txCount = len(txs)
// 	}

// 	// blockNr := this.getRpcBlockNumber(blockNumber)
// 	var batchCallConfig = ethapi.BatchCallConfig{
// 		Block: rpc.BlockNumberOrHashWithNumber(blockNumber),
// 		Calls: make([]ethapi.BatchCallArgs, txCount),
// 	}

// 	if txs != nil {
// 		for idx, tx := range txs {
// 			batchCallConfig.Calls[idx] = this.BuildBatchCallArgs(tx, true)
// 		}
// 	}

// 	outputs, err := this.bcApi.BatchCall(context.Background(), batchCallConfig)

// 	return outputs, err
// }

func (this *Erc20) GetPairReservesAfterTxs(
	pair *common.Address,
	baseToken *common.Address,
	token *common.Address,
	txs []*types.Transaction,
	blockNumber rpc.BlockNumber,
	ignoreGasFee bool,
) (*big.Int, *big.Int, error, *common.Hash) {
	baseReserve := big.NewInt(0)
	tokenReserve := big.NewInt(0)
	txCount := 0
	if txs != nil {
		txCount = len(txs)
	}

	// batch call config
	// blockNr := this.getRpcBlockNumber(blockNumber)
	// var batchCallConfig = ethapi.BatchCallConfig{
	// 	Block: rpc.BlockNumberOrHashWithNumber(blockNumber),
	// 	Calls: make([]ethapi.BatchCallArgs, txCount+2),
	// }

	var batchCallConfig = ethapi.SimOpts{
		BlockStateCalls:        make([]ethapi.SimBlock, 1),
		TraceTransfers:         false,
		Validation:             false,
		ReturnFullTransactions: true,
	}
	batchCallConfig.BlockStateCalls[0] = ethapi.SimBlock{
		BlockOverrides: nil,
		StateOverrides: nil,
		Calls:          make([]ethapi.TransactionArgs, txCount+2),
	}

	// 1. include all txs in batch
	if txs != nil {
		for idx, tx := range txs {
			batchCallConfig.BlockStateCalls[0].Calls[idx] = this.BuildBatchCallArgs(tx, ignoreGasFee)
		}
	}

	// 2. include balanceOf baseToken tx in batch
	txBalanceOfBaseTokenInput, err := this.BuildBalanceOfInput(pair)
	if err != nil {
		return baseReserve, tokenReserve, err, nil
	}
	batchCallConfig.BlockStateCalls[0].Calls[txCount] = ethapi.TransactionArgs{
		To:    baseToken,
		Input: txBalanceOfBaseTokenInput,
	}

	// 4. include balanceOf token tx in batch
	txBalanceOfTokenInput, err := this.BuildBalanceOfInput(pair)
	if err != nil {
		return baseReserve, tokenReserve, err, nil
	}
	batchCallConfig.BlockStateCalls[0].Calls[txCount+1] = ethapi.TransactionArgs{
		To:    token,
		Input: txBalanceOfTokenInput,
	}

	// 4. batch call
	blockNoOrHash := rpc.BlockNumberOrHashWithNumber(blockNumber)
	outputs, err := this.bcApi.SimulateV1(context.Background(), batchCallConfig, &blockNoOrHash)
	if err != nil {
		return baseReserve, tokenReserve, err, nil
	}

	callResults := outputs[0]["calls"].([]ethapi.SimCallResult)

	for idx, output := range callResults {
		if idx < txCount {
			if output.Error != nil {
				txHash := txs[idx].Hash()
				return baseReserve, tokenReserve, errors.New(output.Error.Message), &txHash
			}
		}
	}

	baseReserve, err = this.ParseBalanceOfOutput(callResults[txCount].ReturnValue)
	if err != nil {
		return baseReserve, tokenReserve, err, nil
	}

	tokenReserve, err = this.ParseBalanceOfOutput(callResults[txCount+1].ReturnValue)
	if err != nil {
		return baseReserve, tokenReserve, err, nil
	}

	return baseReserve, tokenReserve, nil, nil
}

func (this *Erc20) CallTotalSupply(token *common.Address, blockNumber rpc.BlockNumber, overrides *ethapi.StateOverride) (*big.Int, error) {
	input, err := this.abi.Pack("totalSupply")
	if err != nil {
		return nil, err
	}

	hexInput := hexutil.Bytes(input)

	arg := ethapi.TransactionArgs{
		From:  token,
		To:    token,
		Input: &hexInput,
	}
	blockNo := rpc.BlockNumberOrHashWithNumber(blockNumber)
	output, err := this.bcApi.Call(context.Background(), arg, &blockNo, overrides, nil)
	if err != nil {
		return nil, err
	}

	res, err := this.abi.Methods["totalSupply"].Outputs.Unpack(output)
	if err != nil {
		return nil, err
	}

	if len(res) == 0 {
		return nil, errors.New("Unexpected output")
	}

	totalSupply := res[0].(*big.Int)

	return totalSupply, nil
}
