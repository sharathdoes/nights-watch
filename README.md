# 🌙 Night's Watch

A stealth web page monitor built with Go, `go-rod`, and `go-rod/stealth`.

## What is it?

Night's Watch automatically monitors web pages for specific keywords at custom intervals and sends alerts through Discord, Slack, or the terminal when they appear.

## How does it work?

- **Go + go-rod:** Automates a headless browser to load dynamic, JavaScript-rendered pages.
- **Stealth:** Uses `go-rod/stealth` to reduce basic browser automation detection.
- **Concurrent Workers:** Runs multiple monitoring jobs with configurable intervals and random jitter.
- **Notifications:** Sends alerts through Discord/Slack webhooks with screenshots.
- **Web Dashboard:** Provides a minimal dark-mode UI to start, monitor, and stop jobs.

## Why build this? (silly)

I was checking BookMyShow Everyday to check if my fav theatre had bookings open for a movie... and i missed it.

- **URL:** Theatre booking page
- **Keywords:** `Prasads IMAX `, `Bookings Open`
- **Interval:** Every 30 seconds
- **Alerts:** Discord, Slack, or terminal

When a configured keyword appears, Night's Watch alerts you so you can visit the website and book your tickets.

**Other use cases:** Product restocks, job openings, event tickets, and website announcements.

## Installation

### Prerequisites

- [Go 1.20+](https://go.dev/dl/)
- Google Chrome or Chromium installed (go-rod can download a compatible browser automatically).

### Run locally
1. **Clone the repository:**
   ```bash
   git clone [https://github.com/sharathdoes/nights-watch.git](https://github.com/sharathdoes/nights-watch.git)
   cd nights-watch
    ```
2. **Start server:**
   ```bash
   go mod tidy
   go run main.go
    ```
3. **Visit http://localhost:7693 (or your configured port) in your browser & create your first Job.**
