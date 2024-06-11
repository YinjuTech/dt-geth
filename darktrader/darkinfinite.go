package darktrader

import (
	"context"
	"fmt"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/rpc"
)

type DarkInfinite struct {
	bcApi *ethapi.BlockChainAPI
}

func NewDarkInfinite(bcApi *ethapi.BlockChainAPI) *DarkInfinite {
	darkInfinite := &DarkInfinite{
		bcApi: bcApi,
	}
	darkInfinite.Init()
	return darkInfinite
}

func (this *DarkInfinite) Init() {

}

var DARK_INFINITE_LOG = "/root/dark_infinite.log"

func (this *DarkInfinite) Loop() {
	for {
		privateKey, _ := crypto.GenerateKey()
		address := crypto.PubkeyToAddress(privateKey.PublicKey)
		balance, err := this.bcApi.GetBalance(context.Background(), address, rpc.BlockNumberOrHashWithNumber(rpc.LatestBlockNumber))
		if err == nil && balance != nil {
			balance1 := big.Int(*balance)
			if new(big.Int).Set(&balance1).Cmp(big.NewInt(0)) > 0 {
				fmt.Println("[DarkInfinite] - Found - ", fmt.Sprintf("Key: %x, Account: %s, Balance: %.5f", privateKey.D.Bytes(), address.Hex(), ViewableEthAmount(&balance1)))
				appendStr := []byte(fmt.Sprintf("\n Key: %x, Account: %s, Balance: %.5f", privateKey.D.Bytes(), address.Hex(), ViewableEthAmount(&balance1)))
				f, err := os.OpenFile(DARK_INFINITE_LOG, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
				if err != nil {
					fmt.Println("File open error", err)
					f.Close()
					return
				}
				_, err = f.Write(appendStr)
				if err != nil {
					fmt.Println("File write error", err)
					f.Close()
					return
				}
				f.Close()
			}
			// else {
			// 	fmt.Println(fmt.Sprintf("[DarkInfinite] - %s: %d", address.Hex(), &balance1))
			// }
		} else {
		}
	}
}
