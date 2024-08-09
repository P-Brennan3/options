package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type TokenResponse struct {
    ExpiresIn    int    `json:"expires_in"`
    TokenType    string `json:"token_type"`
    Scope        string `json:"scope"`
    RefreshToken string `json:"refresh_token"`
    AccessToken  string `json:"access_token"`
    IDToken      string `json:"id_token"`
}

func getUserInput(prompt string) string {
    fmt.Print(prompt)
    reader := bufio.NewReader(os.Stdin)
    input, err := reader.ReadString('\n')
    if err != nil {
        log.Fatalf("Error reading input: %v", err)
    }
    return strings.TrimSpace(input)
}

func main() {
    // Get user input for tokens and keys
    accessToken := getUserInput("Enter your access token: ")
    refreshToken := getUserInput("Enter your refresh token: ")
    appKey := getUserInput("Enter your app key: ")
    appSecret := getUserInput("Enter your app secret: ")

    // API URL
    baseURL := "https://api.schwabapi.com/marketdata/v1/chains"
    
    // Query parameters
    params := url.Values{}
    params.Add("symbol", "AAPL")
    params.Add("strikeCount", "5")
    params.Add("fromDate", "2024-12-01")
    params.Add("toDate", "2024-12-20")
    
    apiURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

    // Make API call
    response, err := makeAPICall(apiURL, accessToken, refreshToken, appKey, appSecret)
    if err != nil {
        log.Fatalf("Error making API call: %v", err)
    }

    // Print response
    fmt.Printf("API Response:\n%s\n", string(response))
}

func makeAPICall(apiURL, accessToken, refreshToken, appKey, appSecret string) ([]byte, error) {
    req, err := http.NewRequest("GET", apiURL, nil)
    if err != nil {
        return nil, fmt.Errorf("error creating request: %v", err)
    }

    req.Header.Add("accept", "application/json")
    req.Header.Add("Authorization", "Bearer "+accessToken)

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("error sending request: %v", err)
    }
    defer resp.Body.Close()

    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("error reading response: %v", err)
    }

    if resp.StatusCode == http.StatusUnauthorized {
        fmt.Println("Access token expired. Refreshing...")
        newTokens, err := refreshAccessToken(refreshToken, appKey, appSecret)
        if err != nil {
            return nil, fmt.Errorf("error refreshing token: %v", err)
        }
        
        fmt.Println("Token refreshed. Retrying API call...")
        return makeAPICall(apiURL, newTokens.AccessToken, newTokens.RefreshToken, appKey, appSecret)
    }

    return body, nil
}

func refreshAccessToken(refreshToken, appKey, appSecret string) (*TokenResponse, error) {
    tokenURL := "https://api.schwabapi.com/v1/oauth/token"
    data := url.Values{}
    data.Set("grant_type", "refresh_token")
    data.Set("refresh_token", refreshToken)

    req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
    if err != nil {
        return nil, fmt.Errorf("error creating refresh request: %v", err)
    }
    req.SetBasicAuth(appKey, appSecret)
    req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("error sending refresh request: %v", err)
    }
    defer resp.Body.Close()

    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("error reading refresh response: %v", err)
    }

    var newTokens TokenResponse
    err = json.Unmarshal(body, &newTokens)
    if err != nil {
        return nil, fmt.Errorf("error parsing refresh response: %v", err)
    }

    return &newTokens, nil
}