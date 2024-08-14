package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/joho/godotenv"
	"golang.org/x/time/rate" // Import the rate limiting package
)

type Underlying struct {
	PercentChange    float64 `json:"percentChange"`
	Last             float64 `json:"last"`
	FiftyTwoWeekHigh float64 `json:"fiftyTwoWeekHigh"`
	FiftyTwoWeekLow  float64 `json:"fiftyTwoWeekLow"`
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

	// Create a rate limiter: 120 requests per minute
	// The second argument '120' is the burst size
	// This allows our program to potentially process all 120 stocks very quickly if the API can handle it
	limiter := rate.NewLimiter(rate.Limit(120), 120)

	// This limiter ensures that, on average, we don't exceed 120 requests per minute.
	// It does this by distributing the allowed requests evenly across time,
	// rather than allowing all 120 requests to be made at the start of each minute.

	// Create channels for jobs and results
	// These channels facilitate communication between the main goroutine and worker goroutines
	jobs := make(chan string, len(stockTickers))      // Channel to send stock tickers to workers
	results := make(chan []Option, len(stockTickers)) // Channel to receive processed options from workers

	// Create worker pool
	numWorkers := 10 // Adjust this number based on your needs and system capabilities
	// Launch multiple worker goroutines to process jobs concurrently
	for i := 0; i < numWorkers; i++ {
		go worker(jobs, results, limiter, apiKey)
	}

	// Send jobs (stock tickers) to the worker pool
	for _, stock := range stockTickers {
		jobs <- stock
	}
	close(jobs) // Close the jobs channel to signal that no more jobs will be sent

	// Collect results from all workers
	var options []Option
	for i := 0; i < len(stockTickers); i++ {
		options = append(options, <-results...)
	}

	// Sort and print results
	sort.Slice(options, func(i, j int) bool {
		return options[i].Volatility > options[j].Volatility
	})

	fmt.Printf("Options with the greatest IV\n")
	for _, option := range options[:40] {
		fmt.Printf("%-5s ($%4.2f) %10s %4s@$%.2f IV:%5.2f%% Trading at $%4.2f [%s]\n",
			option.Symbol,
			option.LastStockPrice,
			option.ExpirationDate[:10],
			option.optionType,
			option.StrikePrice,
			option.Volatility,
			option.Ask,
			option.OptionSymbol,
		)
	}

	fmt.Printf("\nOptions with the lowest IV\n")
	for i := 0; i < 40; i++ {
		option := options[len(options)-1-i]
		fmt.Printf("%-5s ($%4.2f) %10s %4s@$%.2f IV:%5.2f%% Trading at $%4.2f [%s]\n",
			option.Symbol,
			option.LastStockPrice,
			option.ExpirationDate[:10],
			option.optionType,
			option.StrikePrice,
			option.Volatility,
			option.Last,
			option.OptionSymbol,
		)
	}
}

// worker function: handles processing for individual stocks
// It receives jobs from the jobs channel, processes them, and sends results to the results channel
// The rate limiter is passed to each worker to ensure that all workers collectively adhere to the rate limit
func worker(jobs <-chan string, results chan<- []Option, limiter *rate.Limiter, apiKey string) {
	// The for range loop automatically terminates when the jobs channel is both closed and empty.
	// Once this loop exits, the worker function ends, and the goroutine terminates.
	for stock := range jobs {
		// Wait for rate limit
		// limiter.Wait() blocks until a token is available
		// This ensures that we don't exceed our rate limit across all workers
		err := limiter.Wait(context.Background())
		if err != nil {
			// If the context is canceled or times out, we'll get an error here
			log.Printf("Rate limiter error: %v", err)
			continue
		}

		// Once we've acquired a token, we can proceed with the API call
		options := getOptionsData(stock, apiKey)
		results <- options
	}
}

func getOptionsData(stock, apiKey string) []Option {
	now := time.Now()
	start := now.AddDate(0, 3, 0)
	dateFormat := "2006-01-02"
	startDate := start.Format(dateFormat)

	end := now.AddDate(0, 9, 0)
	endDate := end.Format(dateFormat)

	optionsChainURL := fmt.Sprintf("https://api.schwabapi.com/marketdata/v1/chains?symbol=%s&includeUnderlyingQuote=true&range=NTM&strikeCount=10&fromDate=%s&toDate=%s", stock, startDate, endDate)

	// Note: We don't need explicit rate limiting here because the worker function
	// already ensures that this function is called at the appropriate rate
	req, err := http.NewRequest("GET", optionsChainURL, nil)
	if err != nil {
		log.Printf("Error creating request for %s: %v", stock, err)
		return nil
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		log.Printf("Error making API call for %s: %v", stock, err)
		return nil
	}
	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		log.Printf("Could not read response body for %s: %v", stock, err)
		return nil
	}
	if res.StatusCode != 200 {
		fmt.Println(string(resBody))
		log.Printf("Call for %s failed with status code %d", stock, res.StatusCode)
		return nil
	}

	var optionsChain OptionsChain
	err = json.Unmarshal(resBody, &optionsChain)
	if err != nil {
		log.Printf("Error unmarshaling JSON for %s: %v", stock, err)
		return nil
	}

	options := []Option{}

	optionTypeMaps := []map[string]map[string][]OptionContract{optionsChain.CallExpDateMap, optionsChain.PutExpDateMap}
	for _, optionTypeMap := range optionTypeMaps {
		for _, strikes := range optionTypeMap {
			for _, contracts := range strikes {
				if len(contracts) > 0 {
					option := Option{
						Symbol:                 stock,
						Description:            contracts[0].Description,
						ExchangeName:           contracts[0].ExchangeName,
						LastStockPrice:         optionsChain.Underlying.Last,
						stockPercentChange:     optionsChain.Underlying.PercentChange,
						lastPrice:              contracts[0].Last,
						fiftyTwoWeekHigh:       optionsChain.Underlying.FiftyTwoWeekHigh,
						fiftyTwoWeekLow:        optionsChain.Underlying.FiftyTwoWeekLow,
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
						IntrinsicValue:         contracts[0].IntrinsicValue,
						ExtrinsicValue:         contracts[0].ExtrinsicValue,
						InTheMoney:             contracts[0].InTheMoney,
					}
					if option.Volatility > 0 {
						options = append(options, option)
					}
				}
			}
		}
	}
	return options
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
