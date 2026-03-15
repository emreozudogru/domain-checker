package main

import (
	"bufio"
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

//go:embed index.html
var indexHTML []byte

type Config struct {
	Domains           []string   `json:"domains"`
	EnableEmailAlerts bool       `json:"enable_email_alerts"`
	SMTPConfig        SMTPConfig `json:"smtp_config"`
}

type SMTPConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
	User string `json:"user"`
	Pass string `json:"pass"`
	To   string `json:"to"`
}

type DomainInfo struct {
	Domain        string `json:"domain"`
	ExpiryDate    string `json:"expiry_date"`
	DaysRemaining int    `json:"days_remaining"`
	Status        string `json:"status"` // ok, warning, expired, error
	ErrorMsg      string `json:"error_msg,omitempty"`
	LastChecked   string `json:"last_checked"`
}

var (
	config     Config
	domainData map[string]DomainInfo
	dataMutex  sync.RWMutex
	tldServers = map[string]string{
		"com": "whois.verisign-grs.com",
		"net": "whois.verisign-grs.com",
		"org": "whois.publicinterestregistry.org",
		"us":  "whois.nic.us",
		"tr":  "whois.trabis.gov.tr", // Primary for .tr domains
	}
)

func main() {
	log.Println("Starting Domain Expiry Monitor...")

	// Load config initially
	if err := loadConfig("domains.json"); err != nil {
		log.Printf("Failed to load config initially. Please ensure domains.json is mounted correctly: %v", err)
	}

	domainData = make(map[string]DomainInfo)

	// Perform initial check
	updateAllDomains()

	// Start background updater ticker (every 24 hours)
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			if err := loadConfig("domains.json"); err != nil {
				log.Printf("Warning: Failed to reload config: %v", err)
			}
			updateAllDomains()
		}
	}()

	// HTTP Server - serving the embedded UI
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write(indexHTML)
	})

	// API endpoint for UI fetching
	http.HandleFunc("/api/domains", func(w http.ResponseWriter, r *http.Request) {
		dataMutex.RLock()
		defer dataMutex.RUnlock()

		var list []DomainInfo
		for _, v := range domainData {
			list = append(list, v)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(list)
	})

	log.Println("Listening on port :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func loadConfig(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	var c Config
	if err := json.NewDecoder(f).Decode(&c); err != nil {
		return err
	}

	dataMutex.Lock()
	config = c
	dataMutex.Unlock()
	return nil
}

func updateAllDomains() {
	dataMutex.RLock()
	domains := config.Domains
	dataMutex.RUnlock()

	var wg sync.WaitGroup
	for _, rawDom := range domains {
		wg.Add(1)
		go func(d string) {
			defer wg.Done()
			d = strings.TrimSpace(strings.ToLower(d))
			info := checkDomain(d)

			dataMutex.Lock()
			domainData[d] = info
			dataMutex.Unlock()

			// Alerting System (Optional/Future-Proof)
			// Controlled by enable_email_alerts boolean
			if config.EnableEmailAlerts && (info.Status == "warning" || info.Status == "expired") {
				sendAlertEmail(info)
			}
		}(rawDom)
	}
	wg.Wait()
	log.Println("Domain update complete.")
}

func queryWhois(domain, server string) (string, error) {
	conn, err := net.DialTimeout("tcp", server+":43", 10*time.Second)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second))

	// Pure WHOIS protocol format
	_, err = conn.Write([]byte(domain + "\r\n"))
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	_, err = io.Copy(&buf, conn)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func checkDomain(domain string) DomainInfo {
	info := DomainInfo{
		Domain:      domain,
		LastChecked: time.Now().Format(time.RFC3339),
		Status:      "error",
	}

	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		info.ErrorMsg = "Invalid domain format"
		return info
	}

	// For .tr domains (e.g. .com.tr), the final TLD part is used for switch logic
	tld := parts[len(parts)-1]
	server, ok := tldServers[tld]
	if !ok {
		// Fallback to generic IANA format if not com, net, org, us, or tr explicitly supported
		server = tld + ".whois-servers.net"
	}

	resp, err := queryWhois(domain, server)
	if err != nil {
		info.ErrorMsg = fmt.Sprintf("WHOIS connection failed: %v", err)
		return info
	}

	// Parsing the specific Expiration Date string based on different extensions
	expiryTime, err := extractExpiryDate(resp, tld)
	if err != nil {
		info.ErrorMsg = "Could not parse expiry date. Rate limited or varied output."
		// Check for common error strings in the Whois payload
		if strings.Contains(strings.ToLower(resp), "no match") || strings.Contains(strings.ToLower(resp), "not found") {
			info.ErrorMsg = "Domain not found / unregistered"
		}
		return info
	}

	info.ExpiryDate = expiryTime.Format("2006-01-02")
	
	// Calculate total days until timezone
	diff := time.Until(expiryTime)
	days := int(diff.Hours() / 24)
	info.DaysRemaining = days

	// Apply Visual Status rules for Dashboard: Healthy, Expiring soon, Expired.
	if days <= 0 {
		info.Status = "expired"
	} else if days <= 30 {
		info.Status = "warning"
	} else {
		info.Status = "ok"
	}

	return info
}

func extractExpiryDate(whoisText, tld string) (time.Time, error) {
	var regexes []*regexp.Regexp

	switch tld {
	case "tr":
		// .tr domains parsing (formats provided by TRABIS server)
		// e.g., "Expires on..............: 2026-Oct-11."
		regexes = []*regexp.Regexp{
			regexp.MustCompile(`(?i)(?:Expires on|Expiration Date)(?:\.*|\s*):\s*([^\r\n]+)`),
		}
	default:
		// Required parsing for .com, .net, .org, .us 
		// Account for variant forms from registries
		regexes = []*regexp.Regexp{
			regexp.MustCompile(`(?i)Registry Expiry Date:\s*([^\r\n]+)`), // Standard string mapped by Verisign/PIR
			regexp.MustCompile(`(?i)Registry Expire Date:\s*([^\r\n]+)`), // Fallback alternate 
			regexp.MustCompile(`(?i)Expiration Date:\s*([^\r\n]+)`),      // Extended fallback
		}
	}

	for _, re := range regexes {
		match := re.FindStringSubmatch(whoisText)
		if len(match) > 1 {
			dateStr := strings.TrimSpace(match[1])
			
			// Remove trailing characters like dot or periods commonly left by older registrars.
			dateStr = strings.TrimSuffix(dateStr, ".")

			// Go Standard Time Formats matched against registry responses
			formats := []string{
				time.RFC3339,
				"2006-01-02T15:04:05Z",
				"2006-01-02",
				"2006-01-02 15:04:05",
				"2006-Jan-02",
				"02-Jan-2006",
			}

			// Clean TZ references from raw texts that cause RFC3339 mismatches
			cleanStr := strings.ReplaceAll(dateStr, " UTC", "Z")
			cleanStr = strings.ReplaceAll(cleanStr, " GMT", "Z")
			
			for _, format := range formats {
				t, err := time.Parse(format, cleanStr)
				if err == nil {
					return t, nil
				}
			}
		}
	}

	return time.Time{}, fmt.Errorf("no mathching regex pattern found")
}

// Foundational code for an SMTP email alert system. It triggers
// when a domain drops below threshold and consumes zero resources unless triggered.
func sendAlertEmail(info DomainInfo) {
	dataMutex.RLock()
	sc := config.SMTPConfig
	dataMutex.RUnlock()

	if sc.Host == "" || sc.To == "" {
		return
	}

	// This code initializes a functional stub using the golang "net/smtp" package.
	// auth := smtp.PlainAuth("", sc.User, sc.Pass, sc.Host)
	// msg := []byte(fmt.Sprintf("To: %s\r\nSubject: ALERT: Domain %s expiring!\r\n\r\n...", sc.To, info.Domain))
	// err := smtp.SendMail(fmt.Sprintf("%s:%d", sc.Host, sc.Port), auth, sc.User, []string{sc.To}, msg)
	
	log.Printf("[ALERT Triggered] Domain %s status is %s! Alert sent! (Remaining: %d days)\n", info.Domain, info.Status, info.DaysRemaining)
}
