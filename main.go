package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"golang.org/x/time/rate"
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

	appKey := os.Getenv("APP_KEY")
	secretKey := os.Getenv("SECRET_KEY")
	if appKey == "" || secretKey == "" {
		log.Fatal("APP_KEY and SECRET_KEY must be set in the environment")
	}

	// Get the initial access token
	accessToken, refreshToken, err := getInitialToken(appKey, secretKey)
	if err != nil {
		log.Fatalf("Error getting initial token: %v", err)
	}

	stockTickers, err := readStocksFile("tickers.stocks")
	if err != nil {
		log.Fatalf("Error reading stocks file: %v", err)
	}

	fmt.Printf("Fetching options data for %d stocks\n", len(stockTickers))

	limiter := rate.NewLimiter(rate.Limit(120), 120)

	jobs := make(chan string, len(stockTickers))
	results := make(chan []Option, len(stockTickers))

	numWorkers := 10
	for i := 0; i < numWorkers; i++ {
		go worker(jobs, results, limiter, accessToken, refreshToken, appKey, secretKey)
	}

	for _, stock := range stockTickers {
		jobs <- stock
	}
	close(jobs)

	var options []Option
	for i := 0; i < len(stockTickers); i++ {
		options = append(options, <-results...)
	}

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

func getInitialToken(appKey, secretKey string) (string, string, error) {
    authURL := "https://api.schwabapi.com/v1/oauth/authorize"
    tokenURL := "https://api.schwabapi.com/v1/oauth/token"
    redirectURL := "https://127.0.0.1"

    // Step 1: Get authorization code
    authCodeURL := fmt.Sprintf("%s?client_id=%s&redirect_uri=%s", 
        authURL, appKey, redirectURL)

    fmt.Printf("Visit this URL to authorize the application: %v\n", authCodeURL)
    fmt.Println("After authorization, you will be redirected. Copy and paste the ENTIRE redirected URL here:")

    var redirectURIWithCode string
    fmt.Scanln(&redirectURIWithCode)

    parsedURL, err := url.Parse(redirectURIWithCode)
    if err != nil {
        return "", "", fmt.Errorf("couldn't parse redirect URI: %v", err)
    }
    code := parsedURL.Query().Get("code")
    if code == "" {
        return "", "", fmt.Errorf("no code found in redirect URI")
    }

    // Step 2: Exchange authorization code for tokens
    data := url.Values{}
    data.Set("grant_type", "authorization_code")
    data.Set("code", code)
    data.Set("redirect_uri", redirectURL)

    req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
    if err != nil {
        return "", "", fmt.Errorf("error creating token request: %v", err)
    }

    // Set headers
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    authHeader := base64.StdEncoding.EncodeToString([]byte(appKey + ":" + secretKey))
    req.Header.Set("Authorization", "Basic "+authHeader)

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return "", "", fmt.Errorf("error exchanging code for token: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return "", "", fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, string(body))
    }

    var result struct {
        AccessToken  string `json:"access_token"`
        RefreshToken string `json:"refresh_token"`
    }

    err = json.NewDecoder(resp.Body).Decode(&result)
    if err != nil {
        return "", "", fmt.Errorf("error decoding token response: %v", err)
    }

    return result.AccessToken, result.RefreshToken, nil
}

func worker(jobs <-chan string, results chan<- []Option, limiter *rate.Limiter, accessToken, refreshToken, appKey, secretKey string) {
	for stock := range jobs {
		err := limiter.Wait(context.Background())
		if err != nil {
			log.Printf("Rate limiter error: %v", err)
			continue
		}

		options, newAccessToken, newRefreshToken, err := getOptionsData(stock, accessToken, refreshToken, appKey, secretKey)
		if err != nil {
			log.Printf("Error getting options data for %s: %v", stock, err)
			continue
		}

		// Update tokens if they've changed
		if newAccessToken != "" {
			accessToken = newAccessToken
		}
		if newRefreshToken != "" {
			refreshToken = newRefreshToken
		}

		results <- options
	}
}

func getOptionsData(stock, accessToken, refreshToken, appKey, secretKey string) ([]Option, string, string, error) {
	now := time.Now()
	start := now.AddDate(0, 3, 0)
	dateFormat := "2006-01-02"
	startDate := start.Format(dateFormat)

	end := now.AddDate(0, 9, 0)
	endDate := end.Format(dateFormat)

	optionsChainURL := fmt.Sprintf("https://api.schwabapi.com/marketdata/v1/chains?symbol=%s&includeUnderlyingQuote=true&range=NTM&strikeCount=10&fromDate=%s&toDate=%s", stock, startDate, endDate)

	req, err := http.NewRequest("GET", optionsChainURL, nil)
	if err != nil {
		return nil, "", "", fmt.Errorf("error creating request for %s: %v", stock, err)
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, "", "", fmt.Errorf("error making API call for %s: %v", stock, err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusUnauthorized {
		// Token might be expired, try to refresh
		newAccessToken, newRefreshToken, err := refreshTokens(refreshToken, appKey, secretKey)
		if err != nil {
			return nil, "", "", fmt.Errorf("error refreshing token: %v", err)
		}

		// Retry the request with the new access token
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", newAccessToken))
		res, err = client.Do(req)
		if err != nil {
			return nil, "", "", fmt.Errorf("error making API call with refreshed token for %s: %v", stock, err)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			return nil, "", "", fmt.Errorf("API call failed with status code %d after token refresh", res.StatusCode)
		}

		accessToken = newAccessToken
		refreshToken = newRefreshToken
	} else if res.StatusCode != http.StatusOK {
		return nil, "", "", fmt.Errorf("API call failed with status code %d", res.StatusCode)
	}

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, "", "", fmt.Errorf("could not read response body for %s: %v", stock, err)
	}

	var optionsChain OptionsChain
	err = json.Unmarshal(resBody, &optionsChain)
	if err != nil {
		return nil, "", "", fmt.Errorf("error unmarshaling JSON for %s: %v", stock, err)
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
	return options, accessToken, refreshToken, nil
}

func refreshTokens(refreshToken, appKey, secretKey string) (string, string, error) {
    tokenURL := "https://api.schwabapi.com/oauth2/v1/token"

    data := url.Values{}
    data.Set("grant_type", "refresh_token")
    data.Set("refresh_token", refreshToken)

    req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
    if err != nil {
        return "", "", fmt.Errorf("error creating refresh token request: %v", err)
    }

    // Set headers
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    authHeader := base64.StdEncoding.EncodeToString([]byte(appKey + ":" + secretKey))
    req.Header.Set("Authorization", "Basic "+authHeader)

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return "", "", fmt.Errorf("error refreshing token: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return "", "", fmt.Errorf("token refresh failed with status %d: %s", resp.StatusCode, string(body))
    }

    var result struct {
        AccessToken  string `json:"access_token"`
        RefreshToken string `json:"refresh_token"`
    }

    err = json.NewDecoder(resp.Body).Decode(&result)
    if err != nil {
        return "", "", fmt.Errorf("error decoding refresh response: %v", err)
    }

    return result.AccessToken, result.RefreshToken, nil
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
