package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/joho/godotenv"
)

type Underlying struct {
	percentChange    float64 `json:"percentChange"`
	lastPrice        float64 `json:"last"`
	fiftyTwoWeekHigh float64 `json:"fiftyTwoWeekHigh"`
	fiftyTwoWeekLow  float64 `json:"fiftyTwoWeekLow"`
}

type OptionContract struct {
	PutCall                string  `json:"putCall"`
	Symbol                 string  `json:"symbol"`
	Description            string  `json:"description"`
	ExchangeName           string  `json:"exchangeName"`
	Bid                    float64 `json:"bid"`
	Ask                    float64 `json:"ask"`
	Last                   float64 `json:"last"`
	Mark                   float64 `json:"mark"`
	BidSize                int     `json:"bidSize"`
	AskSize                int     `json:"askSize"`
	BidAskSize             string  `json:"bidAskSize"`
	LastSize               int     `json:"lastSize"`
	HighPrice              float64 `json:"highPrice"`
	LowPrice               float64 `json:"lowPrice"`
	OpenPrice              float64 `json:"openPrice"`
	ClosePrice             float64 `json:"closePrice"`
	TotalVolume            int     `json:"totalVolume"`
	TradeTimeInLong        int64   `json:"tradeTimeInLong"`
	QuoteTimeInLong        int64   `json:"quoteTimeInLong"`
	NetChange              float64 `json:"netChange"`
	Volatility             float64 `json:"volatility"`
	Delta                  float64 `json:"delta"`
	Gamma                  float64 `json:"gamma"`
	Theta                  float64 `json:"theta"`
	Vega                   float64 `json:"vega"`
	Rho                    float64 `json:"rho"`
	OpenInterest           int     `json:"openInterest"`
	TimeValue              float64 `json:"timeValue"`
	TheoreticalOptionValue float64 `json:"theoreticalOptionValue"`
	TheoreticalVolatility  float64 `json:"theoreticalVolatility"`
	StrikePrice            float64 `json:"strikePrice"`
	ExpirationDate         string  `json:"expirationDate"`
	DaysToExpiration       int     `json:"daysToExpiration"`
	ExpirationType         string  `json:"expirationType"`
	LastTradingDay         int64   `json:"lastTradingDay"`
	Multiplier             float64 `json:"multiplier"`
	SettlementType         string  `json:"settlementType"`
	DeliverableNote        string  `json:"deliverableNote"`
	PercentChange          float64 `json:"percentChange"`
	MarkChange             float64 `json:"markChange"`
	MarkPercentChange      float64 `json:"markPercentChange"`
	IntrinsicValue         float64 `json:"intrinsicValue"`
	ExtrinsicValue         float64 `json:"extrinsicValue"`
	InTheMoney             bool    `json:"inTheMoney"`
}

type OptionsChain struct {
	Symbol         string                                 `json:"symbol"`
	Underlying     Underlying                             `json:"underlying"`
	CallExpDateMap map[string]map[string][]OptionContract `json:"callExpDateMap"`
	PutExpDateMap  map[string]map[string][]OptionContract `json:"putExpDateMap"`
}

type Option struct {
	Symbol                 string
	Description            string
	ExchangeName           string
	LastStockPrice         float64
	stockPercentChange     float64
	lastPrice              float64
	fiftyTwoWeekHigh       float64
	fiftyTwoWeekLow        float64
	optionType             string
	OptionSymbol           string  `json:"symbol"`
	Bid                    float64 `json:"bid"`
	Ask                    float64 `json:"ask"`
	Last                   float64 `json:"last"`
	Mark                   float64 `json:"mark"`
	BidSize                int     `json:"bidSize"`
	AskSize                int     `json:"askSize"`
	BidAskSize             string  `json:"bidAskSize"`
	LastSize               int     `json:"lastSize"`
	HighPrice              float64 `json:"highPrice"`
	LowPrice               float64 `json:"lowPrice"`
	OpenPrice              float64 `json:"openPrice"`
	ClosePrice             float64 `json:"closePrice"`
	TotalVolume            int     `json:"totalVolume"`
	NetChange              float64 `json:"netChange"`
	Volatility             float64 `json:"volatility"`
	Delta                  float64 `json:"delta"`
	Gamma                  float64 `json:"gamma"`
	Theta                  float64 `json:"theta"`
	Vega                   float64 `json:"vega"`
	Rho                    float64 `json:"rho"`
	OpenInterest           int     `json:"openInterest"`
	TimeValue              float64 `json:"timeValue"`
	TheoreticalOptionValue float64 `json:"theoreticalOptionValue"`
	TheoreticalVolatility  float64 `json:"theoreticalVolatility"`
	StrikePrice            float64 `json:"strikePrice"`
	ExpirationDate         string  `json:"expirationDate"`
	DaysToExpiration       int     `json:"daysToExpiration"`
	LastTradingDay         int64   `json:"lastTradingDay"`
	PercentChange          float64 `json:"percentChange"`
	MarkChange             float64 `json:"markChange"`
	MarkPercentChange      float64 `json:"markPercentChange"`
	IntrinsicValue         float64 `json:"intrinsicValue"`
	ExtrinsicValue         float64 `json:"extrinsicValue"`
	InTheMoney             bool    `json:"inTheMoney"`
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	apiKey := os.Args[1]

	stockTickers, err := readStocksFile("tickers.stocks")
	if err != nil {
		log.Fatalf("Error reading stocks file: %v", err)
	}

	fmt.Printf("Fetching options data for %d stocks\n", len(stockTickers))

	// This is where we will store all the options we fetch
	options := []Option{}

	// This is a channel that we will use to send the options data back to the main goroutine
	c := make(chan []Option)

	// For each stock make a call to get its options
	for _, stock := range stockTickers {
		go getOptionsData(stock, apiKey, c)
	}

	// Wait for all the options to be fetched
	for i := 0; i < len(stockTickers); i++ {
		contracts := <-c
		options = append(options, contracts...)
	}

	// Sort the options data by IV Descending
	sort.Slice(options, func(i, j int) bool {
		return options[i].Volatility > options[j].Volatility
	})

	fmt.Printf("Option with the greatest IV %+v", options[0])

}

func getOptionsData(stock string, apiKey string, c chan []Option) {

	now := time.Now()
	start := now.AddDate(0, 3, 0)
	startDate := start.Format("2006-01-02")

	end := now.AddDate(0, 6, 0)
	endDate := end.Format("2006-01-02")

	optionsChainURL := fmt.Sprintf("https://api.schwabapi.com/marketdata/v1/chains?symbol=%s&includeUnderlyingQuote=true&range=NTM&fromDate=%s&toDate=%s", stock, startDate, endDate)

	req, err := http.NewRequest("GET", optionsChainURL, nil)
	if err != nil {
		log.Fatalf("Error creating request: %v", err)
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		log.Fatalf("Error making API call: %v", err)
	}
	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		log.Fatalf("Could not read response body: %v", err)
	}
	if res.StatusCode != 200 {
		fmt.Println(string(resBody))
		log.Fatalf("API Key expired")
	}

	var optionsChain OptionsChain
	err = json.Unmarshal(resBody, &optionsChain)
	if err != nil {
		log.Fatalf("Error unmarshaling JSON: %v", err)
	}

	options := []Option{}

	for _, strikes := range optionsChain.CallExpDateMap {
		for _, contracts := range strikes {
			if len(contracts) > 0 {
				option := Option{
					Symbol:                 stock,
					Description:            contracts[0].Description,
					ExchangeName:           contracts[0].ExchangeName,
					LastStockPrice:         optionsChain.Underlying.lastPrice,
					stockPercentChange:     optionsChain.Underlying.percentChange,
					lastPrice:              contracts[0].Last,
					fiftyTwoWeekHigh:       optionsChain.Underlying.fiftyTwoWeekHigh,
					fiftyTwoWeekLow:        optionsChain.Underlying.fiftyTwoWeekLow,
					optionType:             contracts[0].PutCall,
					OptionSymbol:           contracts[0].Symbol,
					Bid:                    contracts[0].Bid,
					Ask:                    contracts[0].Ask,
					Last:                   contracts[0].Last,
					Mark:                   contracts[0].Mark,
					BidSize:                contracts[0].BidSize,
					AskSize:                contracts[0].AskSize,
					BidAskSize:             contracts[0].BidAskSize,
					LastSize:               contracts[0].LastSize,
					HighPrice:              contracts[0].HighPrice,
					LowPrice:               contracts[0].LowPrice,
					OpenPrice:              contracts[0].OpenPrice,
					ClosePrice:             contracts[0].ClosePrice,
					TotalVolume:            contracts[0].TotalVolume,
					NetChange:              contracts[0].NetChange,
					Volatility:             contracts[0].Volatility,
					Delta:                  contracts[0].Delta,
					Gamma:                  contracts[0].Gamma,
					Theta:                  contracts[0].Theta,
					Vega:                   contracts[0].Vega,
					Rho:                    contracts[0].Rho,
					OpenInterest:           contracts[0].OpenInterest,
					TimeValue:              contracts[0].TimeValue,
					TheoreticalOptionValue: contracts[0].TheoreticalOptionValue,
					TheoreticalVolatility:  contracts[0].TheoreticalVolatility,
					StrikePrice:            contracts[0].StrikePrice,
					ExpirationDate:         contracts[0].ExpirationDate,
					DaysToExpiration:       contracts[0].DaysToExpiration,
					LastTradingDay:         contracts[0].LastTradingDay,
					PercentChange:          contracts[0].PercentChange,
					MarkChange:             contracts[0].MarkChange,
					MarkPercentChange:      contracts[0].MarkPercentChange,
				}
				options = append(options, option)
			}
		}
	}
	c <- options
	return
}

func readStocksFile(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	return lines, nil
}
