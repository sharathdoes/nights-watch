package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/stealth"
	"mime/multipart"
)

type WatchRequest struct {
	URL string `json:"url"`
	Texts []string `json:"texts"`
	IntervalSecs int `json:"interval_secs"`
	StrictURL bool `json:"strict_url"`
	NotifyMethod string `json:"notify_method"`
	NotifyTarget string `json:"notify_target"`
}

type Job struct {
	ID string
	Request WatchRequest
	Stop chan struct{}
}

var (
	jobs = make(map[string]*Job)
	jobsMu sync.Mutex
	browser *rod.Browser
	browserMu sync.Mutex
)

func main() {
	initBrowser()
	defer browser.MustClose()
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w,r, "ui/index.html")
	})
	http.HandleFunc("/watch", addWatchHandler)
	http.HandleFunc("/jobs", listJobsHandler)
	http.HandleFunc("/stop", stopWatchHandler)

	port := ":7693"
	fmt.Printf("Generic Stealth Watcher running on http://localhost%s\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}

func initBrowser() {
	fmt.Println("Launching a headless browser... ")
	path, _ := launcher.LookPath()
	u := launcher.New().Bin(path).
		Headless(true). // Set to false if you want to visually watch Chrome open tabs
		MustLaunch()
	browser=rod.New().ControlURL(u).MustConnect()
	fmt.Println("Browser ready!")
}

func addWatchHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method!=http.MethodPost {
		http.Error(w, "ONlY POST ALLOWED", http.StatusMethodNotAllowed)
		return
	}
	var req WatchRequest
	if err:=json.NewDecoder(r.Body).Decode(&req); err!=nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if req.URL == "" || len(req.Texts)==0 || req.IntervalSecs < 15 {
		http.Error(w, "Provide url, text, and interval_secs (minimum 15)", http.StatusBadRequest)
		return
	}
	jobId:=fmt.Sprintf("job_%d", time.Now().UnixNano())
	job:=&Job{
		ID: jobId,
		Request: req,
		Stop:    make(chan struct{}),
	}
	jobsMu.Lock()
	jobs[jobId]=job
	jobsMu.Unlock()

	go startWatching(job)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "watching",
		"job_id": jobId,
	})
}


func startWatching( job *Job) {
	log.Printf("[%s] Started watching %s for '%s' every %ds\n",	job.ID, job.Request.URL, job.Request.Texts, job.Request.IntervalSecs)
	
	if runCheck(job) {
		return
	}

	ticker:= time.NewTicker(time.Duration(job.Request.IntervalSecs)*time.Second)
	defer ticker.Stop()

	for {
		select { //Think of select as a switch statement specifically built for channels (pipes).

		case <-ticker.C :
			time.Sleep(time.Duration(rand.Intn(5))*time.Second)
			if runCheck(job) {
				return
			}
		case <-job.Stop :
			log.Printf("[%s] Stopped by user.\n", job.ID)
			return
		}
	}


}

func runCheck(job *Job) bool {
	found, matchedWord, screenshot, err:=checkStealth(job.Request.URL, job.Request.Texts, job.Request.StrictURL)
	if err!=nil {
		log.Printf("[%s] Check error: %v\n", job.ID, err)
		return false
	}

	if found {
		notifyUser(job.Request, matchedWord, screenshot)
		jobsMu.Lock()
		delete(jobs, job.ID)
		jobsMu.Unlock()
		return true
	} 
	log.Printf("[%s] Checked page — %v not found yet.\n", job.ID, job.Request.Texts)
	return false

}

func checkStealth(targetURL string, targetTexts []string, strictURL bool) (bool,string, []byte,  error) {
	browserMu.Lock()
	page,err:=stealth.Page(browser)
	browserMu.Unlock()
	if err!=nil {
		return false, "", nil, fmt.Errorf("failed to open tab: %w", err)
	}
	defer page.MustClose()

	// Set a crisp 1280x800 desktop window size so the screenshot looks clean
	page.MustSetViewport(1280, 800, 1, false)

	err = page.Timeout(25*time.Second).Navigate(targetURL)
	if err != nil {
		return false,"", nil,fmt.Errorf("navigation timeout: %w", err)
	}
	_= page.Timeout(15*time.Second).WaitStable(1*time.Second)
	_=strictURL
	body,err:=page.Timeout(10*time.Second).Element("body")

	if err != nil {
		return false, "", nil,fmt.Errorf("could not find body element: %w", err)
	}

	bodyText,err:=body.Text()

	if err != nil {
		return false,"", nil, fmt.Errorf("could not read text: %w", err)
	}

	for _,text :=range targetTexts {
		if strings.Contains(strings.ToLower(bodyText), strings.ToLower(text)) {
			imgBytes, _ := page.Screenshot(false, nil)
			return true, text, imgBytes, nil
		}
	}
	return false,"", nil, nil

}


func notifyUser(req WatchRequest, word string, screenshot []byte) {
	msg := fmt.Sprintf("🚨 **Match Found!**\nText: `%s`\nURL: %s", word, req.URL)
	switch strings.ToLower(req.NotifyMethod) {
	case "discord", "webhook":
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)

		// 1. Add the text message field
		_ = writer.WriteField("content", msg)

		// 2. Attach the screenshot bytes as "match.png" if we captured one
		if len(screenshot) > 0 {
			part, err := writer.CreateFormFile("file", "match.png")
			if err == nil {
				_, _ = part.Write(screenshot)
			}
		}
		writer.Close()

		res,err:=http.Post(req.NotifyTarget,  writer.FormDataContentType(), &body)
		if err != nil {
			log.Printf("Webhook notification error: %v\n", err)
			return
		}
		defer res.Body.Close() // else the network connection stays open in memory forever
		log.Printf("Webhook notification sent (HTTP %d)\n", res.StatusCode)
	default : 
		fmt.Println("\n" + msg + "\n")
	}	
}

func listJobsHandler(w http.ResponseWriter, r *http.Request) {
	jobsMu.Lock()
	defer jobsMu.Unlock()

	var active []map[string]interface{}
	for id, j := range jobs {
		active = append(active, map[string]interface{}{
			"id":            id,
			"url":           j.Request.URL,
			"text":          j.Request.Texts,
			"interval_secs": j.Request.IntervalSecs,
			"strict_url":    j.Request.StrictURL,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(active)
}

// POST /stop?id=job_123
func stopWatchHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	jobsMu.Lock()
	job, exists := jobs[id]
	if exists {
		close(job.Stop)
		delete(jobs, id)
	}
	jobsMu.Unlock()

	if !exists {
		http.Error(w, "Job ID not found", http.StatusNotFound)
		return
	}
	w.Write([]byte("Stopped " + id))
}


//GET /users/42?name=sharath&age=25&tag=go&tag=api HTTP/1.1

//PATH     fmt.Println(r.URL.Path)
// /users/42
// Identifies the resource or route.


// QUERY STRING
// query := r.URL.Query()
// 3. Get one value - name := query.Get("name")
// ?name=sharath&age=25&tag=go&tag=api
// Contains optional parameters sent with the URL.



//logs

/*

fmt.Println("usual shit")
fmt.Printf("Port: %d\n", 8080) print variables on screen
fmt.Sprintf() it returns a string variable
fmt.Fprintf(w, ...) Sends text into a specific destination w (like a file or an HTTP webpage response)
fmt.Fprintf(w, "Hello %s", name)
log.Printf(...) Your terminal screen + automatic Date & Time stamp
log.Fatal(...) Log + Kill Program

s*/
