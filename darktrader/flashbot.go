package darktrader

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/lmittmann/flashbots"
)

type TxBundle struct {
	txs           [][]byte
	blkNumber     int64
	blkCount      int64
	revertableTxs []common.Hash
}

// 'https://api.blocknative.com/v1/auction',

var builders = map[string]string{
	"builder0x69":       "https://builder0x69.io",
	"beaverbuild":       "https://rpc.beaverbuild.org",
	"rsync-builder":     "https://rsync-builder.xyz",
	"titanbuilder":      "https://rpc.titanbuilder.xyz",
	"flashbots":         "https://relay.flashbots.net",
	"eth-builder":       "https://eth-builder.com",
	"edennetwork":       "https://api.edennetwork.io/v1/bundle",
	"lightspeedbuilder": "https://rpc.lightspeedbuilder.info/",
	"nfactorial":        "https://rpc.nfactorial.xyz/",
	"buildai":           "https://BuildAI.net",
	"f1b":               "https://rpc.f1b.io",
	"payload":           "https://rpc.payload.de",
	"gmbiit":            "https://builder.gmbit.co/rpc",
	"ethbuilder":        "https://eth-builder.com",
	"builderai":         "https://buildai.net",
}

const FLASHBOTS_PRIVATE_KEY_PATH string = "/root/flashbots-private-key.pem"

var flashbotPrivateKey, _ = crypto.LoadECDSA(FLASHBOTS_PRIVATE_KEY_PATH)

// var (
// 	flashbotTxId int = 1
// )

// func flashbotHeader(signature []byte, flashbotPrivateKey *ecdsa.PrivateKey) string {
// 	return crypto.PubkeyToAddress(flashbotPrivateKey.PublicKey).Hex() +
// 		":" + hexutil.Encode(signature)
// }

// func sendPrvTx(builder string, tx []byte, blkNumber int64, blkCount int64) (common.Hash, error) {
// 	fbClient := flashbots.MustDial(builder, flashbotPrivateKey)

// 	var txHash common.Hash

// 	err := fbClient.Call(flashbots.SendPrivateTx(&flashbots.SendPrivateTxRequest{
// 		RawTx:          tx,
// 		MaxBlockNumber: big.NewInt(blkNumber + blkCount),
// 		Fast:           true,
// 	}).Returns(&txHash))

// 	if err != nil {
// 		fmt.Println(builder, "error", err)
// 	}
// 	fmt.Println(builder, txHash.Hex())
// 	return txHash, err
// }

// func sendBundleTx(builder string, txs [][]byte, blkNumber int64, blkCount int64) {
// 	szTx := ""

// 	for i, tx := range txs {
// 		szTx += `"0x` + hex.EncodeToString(tx) + `"`
// 		if i != 0 {
// 			szTx = "," + szTx
// 		}
// 	}

// 	var payload = `{
// 		"jsonrpc": "2.0",
// 		"id": 1,
// 		"method": "eth_sendBundle",
// 		"params": [
// 			{
// 				"txs": [` + szTx + `],
// 				"blockNumber": "` + fmt.Sprintf("0x%x", blkNumber+1) + `"
// 			}
// 		]
// 	}`
// 	fmt.Println("payload", payload)
// 	var jsonStr = []byte(payload)
// 	req, err := http.NewRequest("POST", builder, bytes.NewBuffer(jsonStr))
// 	req.Header.Set("Content-Type", "application/json")

// 	client := &http.Client{}
// 	resp, err := client.Do(req)
// 	if err != nil {
// 		fmt.Println(builder, "error", err)
// 	} else {
// 		defer resp.Body.Close()

//			body, _ := io.ReadAll(resp.Body)
//			fmt.Println(builder, "response", string(body))
//		}
//	}
func callBundleTx(builder string, bundle *TxBundle) {
	fbClient := flashbots.MustDial(builder, flashbotPrivateKey)

	var callBundleResponse flashbots.CallBundleResponse
	err := fbClient.Call(flashbots.CallBundle(&flashbots.CallBundleRequest{
		RawTransactions: bundle.txs,
		BlockNumber:     big.NewInt(bundle.blkNumber),
	}).Returns(&callBundleResponse))

	if err != nil {
		fmt.Println("call error", err)
	} else {
	}
}

func sendBundleTx(builder string, bundle *TxBundle) {
	fbClient := flashbots.MustDial(builder, flashbotPrivateKey)

	for i := int64(0); i < bundle.blkCount; i++ {
		// fmt.Println("Sending bundle to blk #", bundle.blkNumber+i)
		var bundleHash common.Hash
		err := fbClient.Call(flashbots.SendBundle(&flashbots.SendBundleRequest{
			RawTransactions:   bundle.txs,
			BlockNumber:       big.NewInt(bundle.blkNumber + i),
			RevertingTxHashes: bundle.revertableTxs,
		}).Returns(&bundleHash))

		if err != nil {
			fmt.Println(builder, "error")
		} else {
			// fmt.Println(builder, bundleHash.Hex())
			// time.AfterFunc(10*time.Second, func() {
			// 	fmt.Println(builder, bundleHash.Hex())

			// 	var bundleStatRes flashbots.BundleStatsV2Response
			// 	fbClient.Call(flashbots.BundleStatsV2(bundleHash, big.NewInt(bundle.blkNumber)).Returns(&bundleStatRes))

			// 	fmt.Println(builder, "stat", bundleStatRes)
			// })
		}
	}
}

// func sendBundleTx1(builder string, tx []byte, blkNumber int64, blkCount int64) {
// 	rpc := flashbotsrpc.New(builder)
// 	rpc.Debug = true

// 	// simulate block
// 	client, err := ethclient.Dial("http://127.0.0.1:8545")
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	blk, err := client.BlockByNumber(context.Background(), big.NewInt(blkNumber))

// 	simulationResult, err := rpc.FlashbotsSimulateBlock(flashbotPrivateKey, blk, 0)
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	fmt.Println("Simulation result:", simulationResult)

// 	// send bundle
// 	sendBundleArgs := flashbotsrpc.FlashbotsSendBundleRequest{
// 		Txs:         []string{hexutil.Encode(tx)},
// 		BlockNumber: fmt.Sprintf("0x%x", blkNumber+2),
// 	}

// 	result, err := rpc.FlashbotsSendBundle(flashbotPrivateKey, sendBundleArgs)
// 	if err != nil {
// 		if errors.Is(err, flashbotsrpc.ErrRelayErrorResponse) {
// 			// ErrRelayErrorResponse means it's a standard Flashbots relay error response, so probably a user error, rather than JSON or network error
// 			fmt.Println(err.Error())
// 		} else {
// 			fmt.Printf("error: %+v\n", err)
// 		}
// 	}

// 	// Print result
// 	fmt.Printf("%+v\n", result)
// }

// rpc endpoints which require auth
// "https://mev.api.blxrbdn.com", "https://api.securerpc.com/v1"
func SendRawTransaction(bundles []*TxBundle) {
	bundleBuilders := []string{"https://builder0x69.io", "https://rsync-builder.xyz", "https://rpc.beaverbuild.org", "https://rpc.titanbuilder.xyz", "https://relay.flashbots.net", "https://api.edennetwork.io/v1/bundle", "https://api.blocknative.com/v1/auction", "https://rpc.nfactorial.xyz/private", "https://rpc.payload.de", "https://eth-builder.com", "https://rpc.f1b.io", "https://builder.gmbit.co/rpc", "https://eth-builder.com", "https://buildai.net", "https://rpc.lightspeedbuilder.info/"}

	for _, bundle := range bundles {
		// fmt.Println("Bundle #", idx)
		for _, builder := range bundleBuilders {
			go sendBundleTx(builder, bundle)
		}
		callBundleTx("https://relay.flashbots.net", bundle)
	}
}
