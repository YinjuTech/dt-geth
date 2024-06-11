package darktrader

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/redis/go-redis/v9"
)

type Configuration struct {
	dt  *DarkTrader
	rdb *redis.Client
}

type PairBoughtEventAdditionalInfo struct {
	BlockNumber  int       `json:"block_number"`
	TotalBuyers  int       `json:"total_buyers"`
	TotalSwaps   int       `json:"total_swaps"`
	InitialPrice float64   `json:"initial_price"`
	PriceUp      float64   `json:"price_up"`
	Accounts     []string  `json:"accounts"`
	BuyFee       uint64    `json:"buy_fee"`
	SellFee      uint64    `json:"sell_fee"`
	AmountIn     *big.Int  `json:"amount_in"`
	Tip          []float64 `json:"tip"`
	Slippage     []uint64  `json:"slippage"`
}
type PairBoughtEvent struct {
	Token0         string                        `json:"token0"`
	Token1         string                        `json:"token1"`
	Pair           string                        `json:"pair"`
	Owner          string                        `json:"owner"`
	LiquidityOwner string                        `json:"liquidity_owner"`
	TokenName      string                        `json:"token_name"`
	TokenSymbol    string                        `json:"token_symbol"`
	TokenDecimals  uint8                         `json:"token_decimals"`
	AdditionalInfo PairBoughtEventAdditionalInfo `json:"additional_info"`
}

var ctx = context.Background()

const CONFIG_PATH string = "/root/dt.config.json"

func NewConfiguration(dt *DarkTrader) *Configuration {
	conf := &Configuration{
		dt: dt,
	}

	return conf
}

func (this *Configuration) Init() {
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

	configFile, err := os.Open(CONFIG_PATH)
	if err != nil {
		LogFwBrOb.Println("[Read log error]: Errors while opening log file")
		return
	}

	bytesRead, err := io.ReadAll(configFile)
	if err != nil {
		LogFwBrOb.Println("[Read log error]: Errors while reading log file")
		return
	}

	this.setConfig(bytesRead)

	go this.subscribe()
}

func (this *Configuration) Publish(channel string, payload string) {
	this.rdb.Publish(ctx, channel, payload)
}

const MSG_TYPE_SUCCESS = 1
const MSG_TYPE_GOOD = 2
const MSG_TYPE_GENERAL = 3
const MSG_TYPE_WARNING = 4
const MSG_TYPE_DANGER = 5

func (this *Configuration) SendMessage(tx string, token string, msgType int, szInput string, title string, payload interface{}) {
	msg := map[string]interface{}{
		"tx":      tx,
		"token":   token,
		"type":    msgType,
		"title":   title,
		"input":   szInput,
		"payload": payload,
	}
	szMsg, _ := json.Marshal(msg)
	this.Publish("channel:message", string(szMsg))
}

func (this *Configuration) subscribe() {
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
			this.processToken([]byte(msg.Payload))
		}
	}
}

type RedisBuyCondition1Params struct {
	MinPriceIncreaseTimes float64 `json:"minPriceIncreaseTimes"`
}

type RedisBuyCondition2Params struct {
	MinPriceIncreaseTimes float64 `json:"minPriceIncreaseTimes"`
	MinBuyCount           int     `json:"minBuyCount"`
}

type RedisBuyCondition3Params struct {
	MinPriceIncreaseTimes float64 `json:"minPriceIncreaseTimes"`
	MinBuyCount           int     `json:"minBuyCount"`
	MinBuyerCount         int     `json:"minBuyerCount"`
	MinDiff               int     `json:"minDiff"`
}
type RedisBuyCondition4Params struct {
	MinPriceIncreaseTimes float64 `json:"minPriceIncreaseTimes"`
	MinSwapCount          int     `json:"minSwapCount"`
	MaxSwapCount          int     `json:"maxSwapCount"`
	MinBuyCount           int     `json:"minBuyCount"`
}

type RedisDSBuyCondition4Params struct {
	MaxDetectedPendingTxCount int     `json:"maxDetectedPendingTxCount"`
	MinMinedTxCount           int     `json:"minMinedTxCount"`
	MinPriceIncreasedTimes    float64 `json:"minPriceIncreasedTimes"`
	MaxPriceIncreasedTimes    float64 `json:"maxPriceIncreasedTimes"`
	MinSwapCount              int     `json:"minSwapCount"`
	MinBuyCount               int     `json:"minBuyCount"`
	WatchBlockCount           int     `json:"WatchBlockCount"`
}

type RedisDarkSniperConfig struct {
	CanBuy                bool                       `json:"canBuy"`
	MaxPriceIncreaseTimes float64                    `json:"maxPriceIncreaseTimes"`
	ValidBlockCount       int                        `json:"validBlockCount"`
	BuyCondition1Params   RedisBuyCondition1Params   `json:"buyCondition1Params"`
	BuyCondition2Params   RedisBuyCondition2Params   `json:"buyCondition2Params"`
	BuyCondition3Params   RedisBuyCondition3Params   `json:"buyCondition3Params"`
	BuyCondition4Params   RedisDSBuyCondition4Params `json:"buyCondition4Params"`
}

type RedisDarkSlayerConfig struct {
	IsAddBotEnabled    bool    `json:"isAddBotEnabled"`
	IsRemoveLpEnabled  bool    `json:"isRemoveLpEnabled"`
	IsOtherEnabled     bool    `json:"isOtherEnabled"`
	ScamTxMinFeeInGwei float64 `json:"scamTxMinFeeInGwei"`
	WatchBlockCount    uint64  `json:"watchBlockCount"`
}

type RedisDarkJumperConfig struct {
	IsRunning                 bool    `json:"isRunning"`
	CanBuy                    bool    `json:"canBuy"`
	MinPriceDown              float64 `json:"minPriceDown"`
	MinPickPriceUp            float64 `json:"minPickPriceUp"`
	SwapRatio                 float64 `json:"swapRatio"`
	TxCountConsiderBlockCount int     `json:"txCountConsiderBlockCount"`
	MinTxCount                int     `json:"minTxCount"`
	MaxWatchingBlockCount     int     `json:"maxWatchingBlockCount"`
	BuyHoldOnThres            int     `json:"buyHoldOnThres"`
	MaxBuyAmountInEth         float64 `json:"maxBuyAmountInEth"`
	MaxBuyCount               int     `json:"maxBuyCount"`
	BuyFee                    float64 `json:"buyFee"`
	Slippage                  float64 `json:"slippage"`
	TotalTokenFee             float64 `json:"totalTokenFee"`
}

type RedisMultiTipModeConfig struct {
	Slippage            uint64   `json:"slippage"`
	Tip                 float64  `json:"tip"`
	BuyCountPercent     uint64   `json:"buyCountPercent"`
	MaxBuyFee           uint64   `json:"maxBuyFee"`
	MaxSellFee          uint64   `json:"maxSellFee"`
	MaxTotalFee         uint64   `json:"maxTotalFee"`
	Modes               []uint64 `json:"modes"`
	MaxEthAmountPercent uint64   `json:"maxEthAmountPercent"`
}
type RedisSpecialModeConfig struct {
	Enabled                bool     `json:"enabled"`
	MinBuyFee              uint64   `json:"minBuyFee"`
	MaxBuyFee              uint64   `json:"maxBuyFee"`
	MinSellFee             uint64   `json:"minSellFee"`
	MaxSellFee             uint64   `json:"maxSellFee"`
	MinTotalFee            uint64   `json:"minTotalFee"`
	MaxTotalFee            uint64   `json:"maxTotalFee"`
	Origins                []string `json:"origins"`
	MaxPriceIncreasedTimes float64  `json:"maxPriceIncreasedTimes"`
	Tip                    float64  `json:"tip"`
	MinEthReserve          float64  `json:"minEthReserve"`
	MaxEthReserve          float64  `json:"maxEthReserve"`
	MinEthIn               float64  `json:"minEthIn"`
	MaxEthIn               float64  `json:"maxEthIn"`
	MaxBuyCount            uint64   `json:"maxBuyCount"`
}
type RedisDegenTraderConfig struct {
	Enabled                bool     `json:"enabled"`
	ContractAddress		   string	`json:"address"`
}
type RedisPayloadConfig struct {
	MinEthReserve              uint64                    `json:"minEthReserve"`
	MaxEthReserve              uint64                    `json:"maxEthReserve"`
	MinBuyAmount               uint64                    `json:"minBuyAmount"`
	MaxBuyAmount               uint64                    `json:"maxBuyAmount"`
	MaxBuyFeeInPercent         uint64                    `json:"maxBuyFeeInPercent"`
	MaxSellFeeInPercent        uint64                    `json:"maxSellFeeInPercent"`
	MaxTotalFeeInPercent       uint64                    `json:"maxTotalFeeInPercent"`
	DefaultBaseTipMin          float64                   `json:"defaultBaseTipMin"`
	DefaultBaseTipMax          float64                   `json:"defaultBaseTipMax"`
	MaxBuyTokenAmountInPercent int64                     `json:"maxBuyTokenAmountInPercent"`
	BuyCondition1Params        RedisBuyCondition1Params  `json:"buyCondition1Params"`
	BuyCondition2Params        RedisBuyCondition2Params  `json:"buyCondition2Params"`
	BuyCondition3Params        RedisBuyCondition3Params  `json:"buyCondition3Params"`
	BuyCondition4Params        RedisBuyCondition4Params  `json:"buyCondition4Params"`
	MaxBuySlippageInPercent    uint64                    `json:"maxBuySlippageInPercent"`
	MinBuySlippageInPercent    uint64                    `json:"minBuySlippageInPercent"`
	BuySellCheckCount          uint8                     `json:"buySellCheckCount"`
	TestAccountAddress         string                    `json:"testAccountAddress"`
	IsRunning                  bool                      `json:"isRunning"`
	CanBuy                     bool                      `json:"canBuy"`
	HideImportantLogs          bool                      `json:"hideImportantLogs"`
	Wallets                    []string                  `json:"wallets"`
	SubWallets                 []string                  `json:"subWallets"`
	BuyTriggerInSeconds        int                       `json:"buyTriggerInSeconds"`
	TipMode                    int                       `json:"tipMode"`
	MultiTipModes              []RedisMultiTipModeConfig `json:"multiTipModes"`
	SpecialModes               []RedisSpecialModeConfig  `json:"specialModes"`
	BroadcastCount             int                       `json:"broadcastCount"`
	DarkSniper                 RedisDarkSniperConfig     `json:"darkSniper"`
	DarkSlayer                 RedisDarkSlayerConfig     `json:"darkSlayer"`
	DarkJumper                 RedisDarkJumperConfig     `json:"darkJumper"`
	BlockedOrigins             []string                  `json:"blockedOrigins"`
	DegenTraderConfig 		   RedisDegenTraderConfig	 `json:"degenTraderConfig"`
}

func (this *Configuration) setConfig(payload []byte) {
	var config RedisPayloadConfig

	// Unmarshal or Decode the JSON to the interface.
	json.Unmarshal(payload, &config)

	var wallets []*DTAccount = []*DTAccount{}
	var subWallets []*DTAccount = []*DTAccount{}
	canBuy := false
	if config.Wallets != nil {
		canBuy = config.CanBuy
		wallets = make([]*DTAccount, len(config.Wallets))
		for idx, wallet := range config.Wallets {
			wallets[idx] = this.makeAccount(wallet)
		}
	}
	if config.SubWallets != nil {
		subWallets = make([]*DTAccount, len(config.SubWallets))
		for idx, wallet := range config.SubWallets {
			subWallets[idx] = this.makeAccount(wallet)
		}
	}

	var dsConfig = DarkSniperConfig{
		canBuy:                config.DarkSniper.CanBuy,
		maxPriceIncreaseTimes: config.DarkSniper.MaxPriceIncreaseTimes,
		validBlockCount:       config.DarkSniper.ValidBlockCount,
		buyCondition1Params: BuyCondition1Params{
			minPriceIncreaseTimes: config.DarkSniper.BuyCondition1Params.MinPriceIncreaseTimes,
		},
		buyCondition2Params: BuyCondition2Params{
			minPriceIncreaseTimes: config.DarkSniper.BuyCondition2Params.MinPriceIncreaseTimes,
			minBuyCount:           config.DarkSniper.BuyCondition2Params.MinBuyCount,
		},
		buyCondition3Params: BuyCondition3Params{
			minPriceIncreaseTimes: config.DarkSniper.BuyCondition3Params.MinPriceIncreaseTimes,
			minBuyCount:           config.DarkSniper.BuyCondition3Params.MinBuyCount,
			minBuyerCount:         config.DarkSniper.BuyCondition3Params.MinBuyerCount,
			minDiff:               config.DarkSniper.BuyCondition3Params.MinDiff,
		},
		buyCondition4Params: DSBuyCondition4Params{
			maxDetectedPendingTxCount: config.DarkSniper.BuyCondition4Params.MaxDetectedPendingTxCount,
			minMinedTxCount:           config.DarkSniper.BuyCondition4Params.MinMinedTxCount,
			minSwapCount:              config.DarkSniper.BuyCondition4Params.MinSwapCount,
			minBuyCount:               config.DarkSniper.BuyCondition4Params.MinBuyCount,
			minPriceIncreasedTimes:    config.DarkSniper.BuyCondition4Params.MinPriceIncreasedTimes,
			maxPriceIncreasedTimes:    config.DarkSniper.BuyCondition4Params.MaxPriceIncreasedTimes,
			watchBlockCount:           config.DarkSniper.BuyCondition4Params.WatchBlockCount,
		},
	}

	var darkSlayerConfig = DarkSlayerConfig{
		isAddBotEnabled:    config.DarkSlayer.IsAddBotEnabled,
		isRemoveLpEnabled:  config.DarkSlayer.IsRemoveLpEnabled,
		isOtherEnabled:     config.DarkSlayer.IsOtherEnabled,
		scamTxMinFeeInGwei: config.DarkSlayer.ScamTxMinFeeInGwei,
		watchBlockCount:    config.DarkSlayer.WatchBlockCount,
	}

	var darkJumperConfig = DarkJumperConfig{
		isRunning:                 config.DarkJumper.IsRunning,
		canBuy:                    config.DarkJumper.CanBuy,
		minPriceDown:              config.DarkJumper.MinPriceDown,
		minPickPriceUp:            config.DarkJumper.MinPickPriceUp,
		swapRatio:                 config.DarkJumper.SwapRatio,
		txCountConsiderBlockCount: config.DarkJumper.TxCountConsiderBlockCount,
		minTxCount:                config.DarkJumper.MinTxCount,
		maxWatchingBlockCount:     config.DarkJumper.MaxWatchingBlockCount,
		buyHoldOnThres:            config.DarkJumper.BuyHoldOnThres,
		maxBuyAmountInEth:         config.DarkJumper.MaxBuyAmountInEth,
		maxBuyCount:               config.DarkJumper.MaxBuyCount,
		buyFee:                    config.DarkJumper.BuyFee,
		slippage:                  config.DarkJumper.Slippage,
		totalTokenFee:             config.DarkJumper.TotalTokenFee,
	}

	var multiTipModes = make([]MultiTipModeConfig, len(config.MultiTipModes))
	for i := 0; i < len(config.MultiTipModes); i++ {
		multiTipModes[i] = MultiTipModeConfig{
			slippage:            config.MultiTipModes[i].Slippage,
			tip:                 config.MultiTipModes[i].Tip,
			buyCountPercent:     config.MultiTipModes[i].BuyCountPercent,
			maxBuyFee:           config.MultiTipModes[i].MaxBuyFee,
			maxSellFee:          config.MultiTipModes[i].MaxSellFee,
			maxTotalFee:         config.MultiTipModes[i].MaxTotalFee,
			modes:               config.MultiTipModes[i].Modes,
			maxEthAmountPercent: config.MultiTipModes[i].MaxEthAmountPercent,
		}
	}

	var specialModes = make([]SpecialModeConfig, len(config.SpecialModes))
	for i := 0; i < len(config.SpecialModes); i++ {
		origins := make([]*common.Address, len(config.SpecialModes[i].Origins))
		for j, originAddr := range config.SpecialModes[i].Origins {
			addr := common.HexToAddress(originAddr)
			origins[j] = &addr
		}
		specialModes[i] = SpecialModeConfig{
			enabled:                config.SpecialModes[i].Enabled,
			minBuyFee:              config.SpecialModes[i].MinBuyFee,
			maxBuyFee:              config.SpecialModes[i].MaxBuyFee,
			minSellFee:             config.SpecialModes[i].MinSellFee,
			maxSellFee:             config.SpecialModes[i].MaxSellFee,
			minTotalFee:            config.SpecialModes[i].MinTotalFee,
			maxTotalFee:            config.SpecialModes[i].MaxTotalFee,
			origins:                origins,
			maxPriceIncreasedTimes: config.SpecialModes[i].MaxPriceIncreasedTimes,
			tip:                    config.SpecialModes[i].Tip,
			minEthReserve: new(big.Int).Mul(
				new(big.Int).SetInt64(int64(config.SpecialModes[i].MinEthReserve*100)),
				big.NewInt(10000000000000000),
			),
			maxEthReserve: new(big.Int).Mul(
				new(big.Int).SetInt64(int64(config.SpecialModes[i].MaxEthReserve*100)),
				big.NewInt(10000000000000000),
			),
			minEthIn: new(big.Int).Mul(
				new(big.Int).SetInt64(int64(config.SpecialModes[i].MinEthIn*100)),
				big.NewInt(10000000000000000),
			),
			maxEthIn: new(big.Int).Mul(
				new(big.Int).SetInt64(int64(config.SpecialModes[i].MaxEthIn*100)),
				big.NewInt(10000000000000000),
			),
			maxBuyCount: config.SpecialModes[i].MaxBuyCount,
		}
	}

	var blockedOrigins = make([]*common.Address, 0)
	for _, origin := range config.BlockedOrigins {
		originAddr := common.HexToAddress(origin)
		blockedOrigins = append(blockedOrigins, &originAddr)
	}

	var degenTraderConfig = DegenTraderConfig {
		enabled: config.DegenTraderConfig.Enabled,
		contractAddress: config.DegenTraderConfig.ContractAddress,
	}

	var dtConfig = DTConfig{
		minEthReserve:              new(big.Int).SetUint64(config.MinEthReserve),
		maxEthReserve:              new(big.Int).SetUint64(config.MaxEthReserve),
		minBuyAmount:               new(big.Int).SetUint64(config.MinBuyAmount),
		maxBuyAmount:               new(big.Int).SetUint64(config.MaxBuyAmount),
		maxBuyTokenAmountInPercent: config.MaxBuyTokenAmountInPercent,
		maxBuyFeeInPercent:         config.MaxBuyFeeInPercent,
		maxSellFeeInPercent:        config.MaxSellFeeInPercent,
		maxTotalFeeInPercent:       config.MaxTotalFeeInPercent,
		defaultBaseTipMin:          config.DefaultBaseTipMin,
		defaultBaseTipMax:          config.DefaultBaseTipMax,
		broadcastCount:             config.BroadcastCount,
		buyCondition1Params: BuyCondition1Params{
			minPriceIncreaseTimes: config.BuyCondition1Params.MinPriceIncreaseTimes,
		},
		buyCondition2Params: BuyCondition2Params{
			minPriceIncreaseTimes: config.BuyCondition2Params.MinPriceIncreaseTimes,
			minBuyCount:           config.BuyCondition2Params.MinBuyCount,
		},
		buyCondition3Params: BuyCondition3Params{
			minPriceIncreaseTimes: config.BuyCondition3Params.MinPriceIncreaseTimes,
			minBuyCount:           config.BuyCondition3Params.MinBuyCount,
			minBuyerCount:         config.BuyCondition3Params.MinBuyerCount,
			minDiff:               config.BuyCondition3Params.MinDiff,
		},
		buyCondition4Params: BuyCondition4Params{
			minPriceIncreaseTimes: config.BuyCondition4Params.MinPriceIncreaseTimes,
			minSwapCount:          config.BuyCondition4Params.MinSwapCount,
			maxSwapCount:          config.BuyCondition4Params.MaxSwapCount,
			minBuyCount:           config.BuyCondition4Params.MinBuyCount,
		},
		maxBuySlippageInPercent: config.MaxBuySlippageInPercent,
		minBuySlippageInPercent: config.MinBuySlippageInPercent,
		buySellCheckCount:       config.BuySellCheckCount,
		testAccountAddress:      common.HexToAddress(config.TestAccountAddress),
		isRunning:               config.IsRunning,
		hideImportantLogs:       config.HideImportantLogs,
		canBuy:                  canBuy,
		wallets:                 wallets,
		subWallets:              subWallets,
		buyTriggerInSeconds:     config.BuyTriggerInSeconds,
		tipMode:                 config.TipMode,
		multiTipModes:           multiTipModes,
		specialModes:            specialModes,
		darkSniper:              &dsConfig,
		darkSlayer:              &darkSlayerConfig,
		darkJumper:              &darkJumperConfig,
		blockedOrigins:          blockedOrigins,
		degenTraderConfig:		 &degenTraderConfig,
	}

	this.dt.setConfig(&dtConfig)
}

func (this *Configuration) makeAccount(key string) *DTAccount {
	ecdsaPrivateKey, _ := crypto.HexToECDSA(key)
	address := crypto.PubkeyToAddress(ecdsaPrivateKey.PublicKey)

	dtAccount := DTAccount{
		key:       ecdsaPrivateKey,
		address:   address,
		szAddress: address.Hex(),
	}
	return &dtAccount
}

type RedisToken struct {
	Token      string `json:"token"`
	Origin     string `json:"origin"`
	OriginName string `json:"originName"`
}

func (this *Configuration) processToken(payload []byte) {
	var tokens []RedisToken
	json.Unmarshal(payload, &tokens)
	fmt.Println(len(tokens), "tokens read/updated")
	for _, token := range tokens {
		var (
			tokenAddr  common.Address
			originAddr *common.Address = nil
		)
		tokenAddr = common.HexToAddress(token.Token)
		if token.Origin != "" {
			_originAddr := common.HexToAddress(token.Origin)
			originAddr = &_originAddr
		}
		this.dt.onDetectNewToken(tokenAddr, originAddr, token.OriginName)
	}
}
