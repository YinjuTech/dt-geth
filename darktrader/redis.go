package darktrader

import (
	"encoding/json"
	"io"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/redis/go-redis/v9"
)

type redisBuyCondition1Params struct {
	MinPriceIncreaseTimes float64 `json:"minPriceIncreaseTimes"`
}

type redisBuyCondition2Params struct {
	MinPriceIncreaseTimes float64 `json:"minPriceIncreaseTimes"`
	MinBuyCount           int     `json:"minBuyCount"`
}

type redisBuyCondition3Params struct {
	MinPriceIncreaseTimes float64 `json:"minPriceIncreaseTimes"`
	MinBuyCount           int     `json:"minBuyCount"`
	MinBuyerCount         int     `json:"minBuyerCount"`
	MinDiff               int     `json:"minDiff"`
}

type redisConfigPayload struct {
	MinEthReserve              uint64                   `json:"minEthReserve"`
	MaxEthReserve              uint64                   `json:"maxEthReserve"`
	MinBuyAmount               uint64                   `json:"minBuyAmount"`
	MaxBuyAmount               uint64                   `json:"maxBuyAmount"`
	MaxBuyFeeInPercent         uint64                   `json:"maxBuyFeeInPercent"`
	MaxSellFeeInPercent        uint64                   `json:"maxSellFeeInPercent"`
	MaxBuyTokenAmountInPercent int64                    `json:"maxBuyTokenAmountInPercent"`
	BuyCondition1Params        redisBuyCondition1Params `json:"buyCondition1Params"`
	BuyCondition2Params        redisBuyCondition2Params `json:"buyCondition2Params"`
	BuyCondition3Params        redisBuyCondition3Params `json:"buyCondition3Params"`
	MaxBuySlippageInPercent    uint64                   `json:"maxBuySlippageInPercent"`
	BuySellCheckCount          uint8                    `json:"buySellCheckCount"`
	TestAccountAddress         string                   `json:"testAccountAddress"`
	IsRunning                  bool                     `json:"isRunning"`
	CanBuy                     bool                     `json:"canBuy"`
	Wallets                    []string                 `json:"wallets"`
}

type redisTokensPayload struct {
	Tokens []string `json:"tokens"`
}

func makeAccount(key string) *DTAccount {
	ecdsaPrivateKey, _ := crypto.HexToECDSA(key)
	address := crypto.PubkeyToAddress(ecdsaPrivateKey.PublicKey)

	dtAccount := DTAccount{
		key:       ecdsaPrivateKey,
		address:   address,
		szAddress: address.Hex(),
	}
	return &dtAccount
}

type Redis struct {
	dt  *DarkTrader
	rdb *redis.Client
}

func NewRedis(dt *DarkTrader) *Redis {
	r := &Redis{
		dt: dt,
	}

	return r
}

func (this *Redis) Init() {
	this.rdb = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	// read conf first
	_, err := os.Stat(CONFIG_PATH)
	if os.IsNotExist(err) {
		return
	}

	logFile, err := os.Open(CONFIG_PATH)
	if err != nil {
		LogFwBrOb.Println("[Read log error]: Errors while opening log file")
		return
	}

	bytesRead, err := io.ReadAll(logFile)
	if err != nil {
		LogFwBrOb.Println("[Read log error]: Errors while reading log file")
		return
	}

	this.setConfig(bytesRead)

	go this.subscribe()
}

func (this *Redis) Publish(channel string, payload interface{}) {
	this.rdb.Publish(ctx, channel, payload)
}

func (this *Redis) subscribe() {
	pubsub := this.rdb.Subscribe(ctx, "channel:config", "channel:tokens")

	for {
		msg, err := pubsub.ReceiveMessage(ctx)
		if err != nil {
			continue
		}

		switch msg.Channel {
		case "channel:config":
			LogFgr.Println("[Redis] Set config")
			this.setConfig([]byte(msg.Payload))
		case "channel:tokens":
			LogFgr.Println("[Redis] Remove tokens")
			this.removeTokens([]byte(msg.Payload))
		}
	}
}

func (this *Redis) setConfig(payload []byte) {
	var configPayload redisConfigPayload

	// Unmarshal or Decode the JSON to the interface.
	json.Unmarshal(payload, &configPayload)

	var wallets []*DTAccount = []*DTAccount{}
	canBuy := false
	if configPayload.Wallets != nil {
		canBuy = configPayload.CanBuy
		wallets = make([]*DTAccount, len(configPayload.Wallets))
		for idx, wallet := range configPayload.Wallets {
			wallets[idx] = makeAccount(wallet)
		}
	}

	var dtConfig = DTConfig{
		minEthReserve:              new(big.Int).SetUint64(configPayload.MinEthReserve),
		maxEthReserve:              new(big.Int).SetUint64(configPayload.MaxEthReserve),
		minBuyAmount:               new(big.Int).SetUint64(configPayload.MinBuyAmount),
		maxBuyAmount:               new(big.Int).SetUint64(configPayload.MaxBuyAmount),
		maxBuyTokenAmountInPercent: configPayload.MaxBuyTokenAmountInPercent,
		maxBuyFeeInPercent:         configPayload.MaxBuyFeeInPercent,
		maxSellFeeInPercent:        configPayload.MaxSellFeeInPercent,
		buyCondition1Params: BuyCondition1Params{
			minPriceIncreaseTimes: configPayload.BuyCondition1Params.MinPriceIncreaseTimes,
		},
		buyCondition2Params: BuyCondition2Params{
			minPriceIncreaseTimes: configPayload.BuyCondition2Params.MinPriceIncreaseTimes,
			minBuyCount:           configPayload.BuyCondition2Params.MinBuyCount,
		},
		buyCondition3Params: BuyCondition3Params{
			minPriceIncreaseTimes: configPayload.BuyCondition3Params.MinPriceIncreaseTimes,
			minBuyCount:           configPayload.BuyCondition3Params.MinBuyCount,
			minBuyerCount:         configPayload.BuyCondition3Params.MinBuyerCount,
			minDiff:               configPayload.BuyCondition3Params.MinDiff,
		},
		maxBuySlippageInPercent: configPayload.MaxBuySlippageInPercent,
		buySellCheckCount:       configPayload.BuySellCheckCount,
		testAccountAddress:      common.HexToAddress(configPayload.TestAccountAddress),
		isRunning:               configPayload.IsRunning,
		canBuy:                  canBuy,
		wallets:                 wallets,
		degenTraderConfig:			configPayload.degenTraderConfig,
	}

	this.dt.setConfig(&dtConfig)
}

func (this *Redis) removeTokens(payload []byte) {
	var tokensPayload redisTokensPayload

	// Unmarshal or Decode the JSON to the interface.
	json.Unmarshal(payload, &tokensPayload)

	// this.dt.removeTokens(tokensPayload.Tokens)
}
