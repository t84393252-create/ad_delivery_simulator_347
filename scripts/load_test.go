package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ad-delivery-simulator/internal/models"
	"github.com/google/uuid"
)

const (
	baseURL            = "http://localhost:8080"
	numWorkers         = 100
	requestsPerWorker  = 100
	testDuration       = 30 * time.Second
)

type LoadTestStats struct {
	TotalRequests      int64
	SuccessfulRequests int64
	FailedRequests     int64
	TotalLatency       int64
	MinLatency         int64
	MaxLatency         int64
}

func main() {
	fmt.Println("Starting Ad Delivery Simulator Load Test")
	fmt.Printf("Workers: %d\n", numWorkers)
	fmt.Printf("Duration: %v\n", testDuration)
	fmt.Println("========================================")

	stats := &LoadTestStats{
		MinLatency: int64(^uint64(0) >> 1),
	}

	var wg sync.WaitGroup
	stopChan := make(chan bool)

	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go worker(i, stats, stopChan, &wg)
	}

	// Run for specified duration
	time.Sleep(testDuration)
	close(stopChan)

	// Wait for all workers to finish
	wg.Wait()

	// Print results
	printResults(stats)
}

func worker(id int, stats *LoadTestStats, stop chan bool, wg *sync.WaitGroup) {
	defer wg.Done()

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	for {
		select {
		case <-stop:
			return
		default:
			// Randomly choose between bid request and tracking events
			switch rand.Intn(10) {
			case 0, 1, 2, 3, 4, 5: // 60% bid requests
				sendBidRequest(client, stats)
			case 6, 7, 8: // 30% impressions
				sendImpressionEvent(client, stats)
			case 9: // 10% clicks
				sendClickEvent(client, stats)
			}
			
			// Small delay to prevent overwhelming the server
			time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
		}
	}
}

func sendBidRequest(client *http.Client, stats *LoadTestStats) {
	bidRequest := generateBidRequest()
	
	body, err := json.Marshal(bidRequest)
	if err != nil {
		atomic.AddInt64(&stats.FailedRequests, 1)
		return
	}

	start := time.Now()
	resp, err := client.Post(baseURL+"/api/v1/bid-request", "application/json", bytes.NewBuffer(body))
	latency := time.Since(start).Milliseconds()

	atomic.AddInt64(&stats.TotalRequests, 1)
	atomic.AddInt64(&stats.TotalLatency, latency)

	if err != nil || resp.StatusCode != http.StatusOK {
		atomic.AddInt64(&stats.FailedRequests, 1)
		if err != nil {
			fmt.Printf("Error sending bid request: %v\n", err)
		}
	} else {
		atomic.AddInt64(&stats.SuccessfulRequests, 1)
		updateLatencyStats(stats, latency)
	}

	if resp != nil {
		resp.Body.Close()
	}
}

func sendImpressionEvent(client *http.Client, stats *LoadTestStats) {
	impression := map[string]interface{}{
		"campaign_id": uuid.New().String(),
		"user_id":     fmt.Sprintf("user-%d", rand.Intn(10000)),
		"session_id":  uuid.New().String(),
	}

	body, err := json.Marshal(impression)
	if err != nil {
		atomic.AddInt64(&stats.FailedRequests, 1)
		return
	}

	start := time.Now()
	resp, err := client.Post(baseURL+"/api/v1/track/impression", "application/json", bytes.NewBuffer(body))
	latency := time.Since(start).Milliseconds()

	atomic.AddInt64(&stats.TotalRequests, 1)
	atomic.AddInt64(&stats.TotalLatency, latency)

	if err != nil || (resp != nil && resp.StatusCode != http.StatusOK) {
		atomic.AddInt64(&stats.FailedRequests, 1)
	} else {
		atomic.AddInt64(&stats.SuccessfulRequests, 1)
		updateLatencyStats(stats, latency)
	}

	if resp != nil {
		resp.Body.Close()
	}
}

func sendClickEvent(client *http.Client, stats *LoadTestStats) {
	click := map[string]interface{}{
		"campaign_id": uuid.New().String(),
		"user_id":     fmt.Sprintf("user-%d", rand.Intn(10000)),
		"session_id":  uuid.New().String(),
	}

	body, err := json.Marshal(click)
	if err != nil {
		atomic.AddInt64(&stats.FailedRequests, 1)
		return
	}

	start := time.Now()
	resp, err := client.Post(baseURL+"/api/v1/track/click", "application/json", bytes.NewBuffer(body))
	latency := time.Since(start).Milliseconds()

	atomic.AddInt64(&stats.TotalRequests, 1)
	atomic.AddInt64(&stats.TotalLatency, latency)

	if err != nil || (resp != nil && resp.StatusCode != http.StatusOK) {
		atomic.AddInt64(&stats.FailedRequests, 1)
	} else {
		atomic.AddInt64(&stats.SuccessfulRequests, 1)
		updateLatencyStats(stats, latency)
	}

	if resp != nil {
		resp.Body.Close()
	}
}

func generateBidRequest() *models.BidRequest {
	countries := []string{"US", "UK", "CA", "AU", "DE", "FR"}
	deviceTypes := []int{1, 2, 4, 5}
	
	return &models.BidRequest{
		ID: uuid.New().String(),
		Imp: []models.Impression{
			{
				ID: uuid.New().String(),
				Banner: &models.Banner{
					W: 300,
					H: 250,
					Format: []models.Format{
						{W: 300, H: 250},
						{W: 728, H: 90},
					},
				},
				BidFloor: rand.Float64() * 5,
			},
		},
		Site: &models.Site{
			ID:     fmt.Sprintf("site-%d", rand.Intn(100)),
			Domain: fmt.Sprintf("example%d.com", rand.Intn(100)),
			Cat:    []string{"IAB1", "IAB2"},
			Page:   fmt.Sprintf("https://example%d.com/page", rand.Intn(100)),
		},
		Device: models.Device{
			UA:         "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
			IP:         fmt.Sprintf("192.168.%d.%d", rand.Intn(255), rand.Intn(255)),
			DeviceType: deviceTypes[rand.Intn(len(deviceTypes))],
			Make:       "Apple",
			Model:      "iPhone",
			OS:         "iOS",
			OSV:        "14.0",
			Language:   "en",
			Geo: &models.Geo{
				Country: countries[rand.Intn(len(countries))],
				City:    "New York",
				ZIP:     "10001",
				Type:    2,
			},
		},
		User: models.User{
			ID:       fmt.Sprintf("user-%d", rand.Intn(10000)),
			BuyerUID: uuid.New().String(),
		},
		AT:   2,
		TMax: 100,
		Cur:  []string{"USD"},
	}
}

func updateLatencyStats(stats *LoadTestStats, latency int64) {
	for {
		oldMin := atomic.LoadInt64(&stats.MinLatency)
		if latency >= oldMin {
			break
		}
		if atomic.CompareAndSwapInt64(&stats.MinLatency, oldMin, latency) {
			break
		}
	}

	for {
		oldMax := atomic.LoadInt64(&stats.MaxLatency)
		if latency <= oldMax {
			break
		}
		if atomic.CompareAndSwapInt64(&stats.MaxLatency, oldMax, latency) {
			break
		}
	}
}

func printResults(stats *LoadTestStats) {
	total := atomic.LoadInt64(&stats.TotalRequests)
	successful := atomic.LoadInt64(&stats.SuccessfulRequests)
	failed := atomic.LoadInt64(&stats.FailedRequests)
	totalLatency := atomic.LoadInt64(&stats.TotalLatency)
	minLatency := atomic.LoadInt64(&stats.MinLatency)
	maxLatency := atomic.LoadInt64(&stats.MaxLatency)

	var avgLatency int64
	if successful > 0 {
		avgLatency = totalLatency / successful
	}

	successRate := float64(successful) / float64(total) * 100
	requestsPerSecond := float64(total) / testDuration.Seconds()

	fmt.Println("\n========================================")
	fmt.Println("Load Test Results")
	fmt.Println("========================================")
	fmt.Printf("Total Requests:       %d\n", total)
	fmt.Printf("Successful Requests:  %d\n", successful)
	fmt.Printf("Failed Requests:      %d\n", failed)
	fmt.Printf("Success Rate:         %.2f%%\n", successRate)
	fmt.Printf("Requests/Second:      %.2f\n", requestsPerSecond)
	fmt.Printf("Avg Latency:          %dms\n", avgLatency)
	fmt.Printf("Min Latency:          %dms\n", minLatency)
	fmt.Printf("Max Latency:          %dms\n", maxLatency)
	fmt.Println("========================================")

	if successRate < 95 {
		fmt.Println("⚠️  Warning: Success rate below 95%")
	} else {
		fmt.Println("✓ Test completed successfully")
	}

	if avgLatency > 100 {
		fmt.Println("⚠️  Warning: Average latency above 100ms")
	} else {
		fmt.Println("✓ Performance within acceptable range")
	}
}