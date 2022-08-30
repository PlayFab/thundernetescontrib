package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	totalSessions     int    = 350
	workerCount       int    = 20
	buildID           string = "85ffe8da-c82f-4035-86c5-9d2b5f42d6f6"
	IP                string = ""
	invalidStatusCode string = "invalid status code"
)

var (
	results = sync.Map{}
	client  = &http.Client{
		Timeout: time.Second * 10,
	}
)

type result struct {
	sessionID  uuid.UUID
	timeInMs   int
	statusCode int
}

func main() {
	if IP == "" {
		panic("IP not set")
	}

	if buildID == "" {
		panic("buildID not set")
	}

	sessionIDs := make([]uuid.UUID, totalSessions)

	var wg sync.WaitGroup
	wg.Add(totalSessions)
	for i := 0; i < totalSessions; i++ {
		sessionIDs[i] = uuid.New()
	}

	processingSessionIDs := make(chan uuid.UUID, workerCount)

	// spin up goroutines to do the work
	for i := 0; i < workerCount; i++ {
		go func() {
			for sessionID := range processingSessionIDs {
				start := time.Now()
				statusCode, err := allocate(sessionID)
				results.Store(sessionID, result{sessionID, int(time.Since(start).Milliseconds()), statusCode})
				if err != nil {
					fmt.Printf("Error for sessionID %s: %v\n", sessionID.String(), err)
				}
				wg.Done()
			}
		}()
	}

	go func() {
		for i := 0; i < totalSessions; i++ {
			processingSessionIDs <- sessionIDs[i]
		}
	}()

	wg.Wait()
	close(processingSessionIDs)

	fmt.Println("---------------------------------")
	fmt.Println("Results:")

	totalTime := 0
	totalErrors := 0
	results.Range(func(k, v interface{}) bool {
		fmt.Printf("%s %d %d\n", k.(uuid.UUID).String(), v.(result).statusCode, v.(result).timeInMs)
		value := v.(result)
		if value.statusCode != http.StatusOK {
			totalErrors++
		} else {
			totalTime += value.timeInMs
		}
		return true
	})

	fmt.Printf("Total allocation attempts: %d\n", totalSessions)
	fmt.Println("Total errors:", totalErrors)
	fmt.Printf("Total successful allocations: %d\n", totalSessions-totalErrors)
	fmt.Println("Average time for successful allocations:", totalTime/(totalSessions-totalErrors))

}

func allocate(sessionID uuid.UUID) (int, error) {
	postBody, _ := json.Marshal(map[string]interface{}{
		"buildID":        buildID,
		"sessionID":      sessionID.String(),
		"sessionCookie":  "randomCookie",
		"initialPlayers": []string{"player1", "player2"},
	})
	postBodyBytes := bytes.NewBuffer(postBody)
	resp, err := client.Post(fmt.Sprintf("http://%s:5000/api/v1/allocate", IP), "application/json", postBodyBytes)
	defer resp.Body.Close()
	//Handle Error
	if err != nil {
		return -1, err
	}
	
	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, fmt.Errorf("%s %d", invalidStatusCode, resp.StatusCode)
	}
	//Read the response body
	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, err
	}
	return resp.StatusCode, nil
}
