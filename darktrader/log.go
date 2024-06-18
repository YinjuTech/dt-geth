package darktrader

import (
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"
	"strings"
	"time"

	ethUnit "github.com/DeOne4eg/eth-unit-converter"
	"github.com/gookit/color"
)

var (
	LogFwBgOb  = color.New(color.FgWhite, color.BgGreen, color.OpBold)
	LogFbBlr   = color.New(color.FgBlack, color.BgMagenta)
	LogFwBrOb  = color.New(color.FgWhite, color.BgRed, color.OpBold)
	LogFwBbOb  = color.New(color.FgWhite, color.BgBlue, color.OpBold)
	LogFwBdgOb = color.New(color.FgWhite, color.BgDarkGray, color.OpBold)
	LogFbByOb  = color.New(color.FgBlack, color.BgYellow, color.OpBold)
	LogFbBhyOb = color.New(color.FgBlack, color.BgHiYellow, color.OpBold)
	LogFbOb    = color.New(color.FgBlue, color.OpBold)
	LogFbBwOb  = color.New(color.FgBlue, color.BgWhite, color.OpBold)
	LogFcBwOb  = color.New(color.FgCyan, color.BgWhite, color.OpBold)
	LogFcOb    = color.New(color.FgCyan, color.OpBold)
	LogFmOb    = color.New(color.FgMagenta, color.OpBold)
	LogFg      = color.New(color.FgGray)
	LogFgr     = color.New(color.FgGreen)
	LogFw      = color.New(color.FgWhite)
	LogFy      = color.New(color.FgYellow)
	LogFrOst   = color.New(color.FgRed, color.OpStrikethrough)
	LogFlrOst  = color.New(color.FgLightRed, color.OpStrikethrough)
)

func sprintSwapCounts(uniqPendingCount int, totalPendingCount int, totalActualSwapCount int, botSwapCount int, totalSwapCountDetectedInclusiveFails int) string {
	return LogFbBwOb.Sprintf("[PS]: %d / %d / %d - [AT(BOT)]: %d(%d)", totalSwapCountDetectedInclusiveFails, uniqPendingCount, totalPendingCount, totalActualSwapCount, botSwapCount)
}

func sprintTokenInfo(pair *DTPair) string {
	return LogFg.Sprintf("%s (%s)", pair.token, pair.symbol)
}

func PrintDarkSniperPairsToRemove(tokens [][]interface{}) {
	//   0			1				2					3					4					5							6								7 							8
	// [token, name, symbol, baseReserve, uniqSwap, totalSwap, missingSwap, increasedInPrice, firstSwapBlkNumber, totalBotSwap]
	logs := []string{}
	for i, token := range tokens {
		logs = append(logs, LogFw.Sprintf("[DS] %2d. ", i+1)+LogFlrOst.Sprintf("%s (%s)", token[0].(string), token[2].(string))+" "+sprintSwapCounts(token[4].(int), token[5].(int), token[6].(int), token[9].(int), token[5].(int))+" "+LogFcBwOb.Sprintf("[Res]: %.2f, [!++]: %.2f, [SwpBlk#]: %d", ViewableEthAmount(token[3].(*big.Int)), token[7].(float64), token[8].(*big.Int)))
	}

	fmt.Println(strings.Join(logs, "\n"))
}

func PrintDarkJumperPairsToRemove(tokens [][]interface{}) {
	//   0      1      2         3             4
	// [token, name, symbol, baseReserve, firstBlkNumber]
	logs := []string{}
	for i, token := range tokens {
		logs = append(
			logs,
			LogFw.Sprintf("[DJ] %2d. ", i+1)+
				LogFlrOst.Sprintf("%s (%s)", token[0].(string), token[2].(string))+" "+
				LogFcBwOb.Sprintf("[Res]: %.2f, [SwpBlk#]: %d", ViewableEthAmount(token[3].(*big.Int)), token[4].(*big.Int)))
	}
	fmt.Println(strings.Join(logs, "\n"))
}

func sprintPairInfo(token []interface{}) string {
	//				0								1			2			3				4					5						6					7						8					9						10						11
	// [ [Remove/NotRemove, token, name, symbol, reserve0, reserve1, uniqSwap, totalSwap, missingSwap, firstBlkNo, totalBotSwap	totalSwapCountDetectedInclusiveFails]

	var tokenInfo string

	if token[0].(bool) {
		tokenInfo = LogFrOst.Sprintf("%s (%s)", token[1].(string), token[3].(string))
	} else {
		tokenInfo = LogFgr.Sprintf("%s (%s)", token[1].(string), token[3].(string))
	}
	blkNo := LogFcBwOb.Sprintf("[SwpBlk #]: %d", token[9].(*big.Int))
	totalSwapCountDetectedInclusiveFails := token[7].(int)
	if len(token) == 12 {
		totalSwapCountDetectedInclusiveFails = token[11].(int)
	}
	return tokenInfo + " " + sprintSwapCounts(token[6].(int), token[7].(int), token[8].(int), token[10].(int), totalSwapCountDetectedInclusiveFails) + " " + LogFcBwOb.Sprintf("[Res]: %d, %.2f", token[4].(*big.Int), ViewableEthAmount(token[5].(*big.Int))) + " " + blkNo
}

func PrintPairsInfoAtNewBlock(tokens [][]interface{}, totalTokens []string, blkNo *big.Int, blkTime time.Time, newTokens int, sniperTokensCount int, slayerTokenCount int, jumperTokensCount int, nextBaseFee *big.Int) {
	nextBaseFeeFloat, _ := ethUnit.NewWei(nextBaseFee).GWei().Float64()
	logs := []string{
		LogFwBbOb.Sprintf("New Blk #(%d) at %s, nextBaseFee: %.9f", blkNo, blkTime, nextBaseFeeFloat) + "  " + LogFbBwOb.Sprintf("[Trader#]: %d (+%d), [Sniper#]: %d, [Jumper#]: %d, [Slayer#]: %d", len(totalTokens), newTokens, sniperTokensCount, jumperTokensCount, slayerTokenCount),
	}

	if len(tokens) > 0 {
		for i, token := range tokens {
			no := LogFw.Sprintf("[DT] %2d. ", i+1)
			logs = append(logs, no+sprintPairInfo(token))
		}
	}

	fmt.Println(strings.Join(logs, "\n"))
}

func PrintBuyInfo(pair *DTPair, buyAmount *big.Int, buyCount uint, tokenExpected *big.Int, tokenActual *big.Int, buyFee uint64, sellFee uint64) {
	logs := []string{
		LogFwBgOb.Sprint("Buy triggered"),
		" " + sprintTokenInfo(pair),
		" " + LogFcOb.Sprintf("[LP]: %d | %.2f eth", pair.tokenReserve, ViewableEthAmount(pair.baseReserve)),
		" " + LogFmOb.Sprintf("[Amounts]: %.2f * %d", ViewableEthAmount(buyAmount), buyCount),
		" " + LogFbBwOb.Sprintf("[Info]: Expected - %d, Actual - %d, Buy fee - %d, Sell fee - %d", tokenExpected, tokenActual, buyFee, sellFee),
		" " + sprintSwapCounts(pair.buyerCount, pair.buyCount, pair.totalActualSwapCount, pair.totalActualBotSwapCount, len(pair.swapTxs.txs)),
	}

	fmt.Println(strings.Join(logs, "\n"))
}

type TokensUnderWatchLog struct {
	BlockNumber uint64   `json:"blockNumber"`
	Tokens      []string `json:"tokens"`
}

const TOKENS_LOG_PATH string = "/home/ubuntu/dt.tokens.log.json"

func WriteTokensUnderWatchLogToFile(blockNumber uint64, tokens []string) {
	log := TokensUnderWatchLog{
		BlockNumber: blockNumber,
		Tokens:      tokens,
	}

	bytesToWrite, err := json.MarshalIndent(log, "", "  ")
	if err != nil {
		LogFwBrOb.Println("[Write log error]: Errors while marshalling")
	}

	err = os.WriteFile(TOKENS_LOG_PATH, bytesToWrite, 0644)
	if err != nil {
		LogFwBrOb.Println("[Write log error]: Errors while writing to log file")
	}
}

func ReadTokensUnderWatchLogFromFile() (uint64, []string) {
	_, err := os.Stat(TOKENS_LOG_PATH)
	if os.IsNotExist(err) {
		return 0, nil
	}

	logFile, err := os.Open(TOKENS_LOG_PATH)
	if err != nil {
		LogFwBrOb.Println("[Read log error]: Errors while opening log file")
		return 0, nil
	}

	bytesRead, err := io.ReadAll(logFile)
	if err != nil {
		LogFwBrOb.Println("[Read log error]: Errors while reading log file")
		return 0, nil
	}

	log := TokensUnderWatchLog{}
	err = json.Unmarshal(bytesRead, &log)
	if err != nil {
		LogFwBrOb.Println("[Read log error]: Errors while unmarshalling")
		return 0, nil
	}

	return log.BlockNumber, log.Tokens
}
