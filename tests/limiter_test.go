package tests

import (
	"bytes"
	"net/http"
	"sync"
	"testing"
	"time"
)

// this method assumes you have a server running on localhost:9090
func doRequest() (int, error) {
	client := &http.Client{}
	request, err := http.NewRequest("POST", "http://localhost:9090/ping", bytes.NewReader([]byte{}))
	if err != nil {
		return 0, err
	}

	response, err := client.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()

	return response.StatusCode, nil
}

func doNRequests(n int) []int {
	var wg sync.WaitGroup
	statusChan := make(chan int, n)
	for range n {
		wg.Go(func() {
			status, _ := doRequest()
			statusChan <- status
		})
	}
	wg.Wait()
	close(statusChan)
	statusCodes := []int{}
	for status := range statusChan {
		statusCodes = append(statusCodes, status)
	}
	return statusCodes
}
func Test_OnLimitExceeded_ServerRespondsWithTooManyRequests(t *testing.T) {
	statusCodes := doNRequests(4)
	for _, code := range statusCodes {
		if code != 200 {
			t.Errorf("Expected 200, got %d", code)
		}
	}
	code, err := doRequest()
	if err != nil {
		t.Fatal(err.Error())
	}
	if code != http.StatusTooManyRequests {
		t.Errorf("Expected 429, got %d", code)
	}
}

func Test_OnLimitNotExceeded_ServerRespondsWith200(t *testing.T) {
	statusCodes := doNRequests(2)
	for _, code := range statusCodes {
		if code != 200 {
			t.Errorf("Expected 200, got %d", code)
		}
	}
}

func Test_OnLimitExceededAndCoolDownReached_ServerAllowsRequests(t *testing.T) {
	statusCodes := doNRequests(4)
	for _, code := range statusCodes {
		if code != 200 {
			t.Errorf("Expected 200, got %d", code)
		}
	}
	code, err := doRequest()
	if err != nil {
		t.Fatal(err.Error())
	}
	if code != http.StatusTooManyRequests {
		t.Errorf("Expected 429, got %d", code)
	}

	time.Sleep(time.Second * 1)
	code, err = doRequest()
	if err != nil {
		t.Fatal(err.Error())
	}
	if code != http.StatusOK {
		t.Errorf("Expected 200, got %d", code)
	}
}
