package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

type AuthClient struct {
	TokenURL    string
	AccessToken string
	Mutex       *sync.Mutex
	Cond        *sync.Cond
	Refreshing  bool
}

func NewAuthClient(tokenURL string) *AuthClient {
	m := sync.Mutex{}
	c := sync.NewCond(&m)
	return &AuthClient{
		TokenURL: tokenURL,
		Mutex:    &m,
		Cond:     c,
	}
}

type SalesforceRequest struct {
	Token   string `json:"token"`
	Message string `json:"message"`
}

type TokenResponse struct {
	Token string `json:"token"`
}

type SalesforceResponse struct {
	ResponseMessage string `json:"message"`
}

func sleep() {
	source := rand.NewSource(time.Now().UnixNano())
	rng := rand.New(source)
	someTime := time.Duration(1+rng.Intn(5)) * time.Second
	time.Sleep(someTime)
}

func main() {
	authClient := NewAuthClient("http://localhost:8081/token/new")
	for i := 0; i < 20; i++ {
		sleep()
		msg, err := MakeRequest(authClient, 0)
		if err != nil {
			log.Println("Error: ", err)
			continue
		}
		fmt.Println("Salesforce says: ", msg)
	}
}

func (ac *AuthClient) fetchNewToken() (string, error) {
	log.Println("Getting new token...")

	// Make request to get token
	httpClient := &http.Client{}
	req, err := http.NewRequest("GET", ac.TokenURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}

	// Parse response
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var tokenResp TokenResponse
	err = json.Unmarshal(body, &tokenResp)
	if err != nil {
		return "", err
	}

	return tokenResp.Token, nil
}

func (ac *AuthClient) GetAccessToken() (string, error) {
	ac.Mutex.Lock()
	defer ac.Mutex.Unlock()

	if ac.AccessToken != "" && !ac.Refreshing {
		// If the token is valid and no refresh is in progress, return the token
		return ac.AccessToken, nil
	}

	if ac.Refreshing {
		// If a refresh is in progress, wait
		ac.Cond.Wait()
	} else {
		// If the token is empty and no refresh is in progress, refresh the token
		ac.Refreshing = true
		ac.Mutex.Unlock()

		token, err := ac.fetchNewToken()
		if err != nil {
			return "", err
		}

		ac.AccessToken = token

		ac.Mutex.Lock()
		ac.Refreshing = false
		ac.Cond.Broadcast()
	}

	return ac.AccessToken, nil
}

func (ac *AuthClient) RefreshToken() (string, error) {
	ac.Mutex.Lock()
	if ac.Refreshing {
		// If another goroutine is already refreshing the token, wait
		ac.Cond.Wait()
	} else {
		// If no other goroutine is refreshing the token, refresh it
		ac.Refreshing = true
		ac.Mutex.Unlock()

		log.Println("Token expired, refreshing...")
		ac.AccessToken = ""
		token, err := ac.fetchNewToken()
		if err != nil {
			return "", err
		}

		ac.AccessToken = token

		ac.Mutex.Lock()
		ac.Refreshing = false
		ac.Cond.Broadcast()
	}
	ac.Mutex.Unlock()

	return ac.AccessToken, nil
}
func MakeRequest(ac *AuthClient, retryCount int) (string, error) {
	if retryCount >= 3 {
		return "", fmt.Errorf("Maximum retry attempts exceeded")
	}

	accessToken, err := ac.GetAccessToken()
	if err != nil {
		return "", err
	}

	httpClient := &http.Client{}

	reqBody, err := json.Marshal(SalesforceRequest{
		Token:   accessToken,
		Message: "Salesforce",
	})

	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", "http://localhost:8081/salesforce", bytes.NewBuffer(reqBody))
	if err != nil {
		return "", err
	}

	log.Println("Making request to Salesforce...")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		_, err := ac.RefreshToken()
		if err != nil {
			return "", err
		}

		return MakeRequest(ac, retryCount+1)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var salesforceResp SalesforceResponse
	err = json.Unmarshal(data, &salesforceResp)
	if err != nil {
		return "", err
	}

	return salesforceResp.ResponseMessage, nil
}
