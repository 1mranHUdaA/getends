package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"golang.org/x/net/html"
)

// The customResolver will be a public DNS resolver (Cloudflare)
// We will use this in a custom http.Transport.
var customResolver = &net.Resolver{
	PreferGo: true,
	Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
		// Try Cloudflare DNS (1.1.1.1) first
		d := net.Dialer{
			Timeout: 10 * time.Second,
		}
		conn, err := d.DialContext(ctx, "udp", "1.1.1.1:53")
		if err == nil {
			return conn, nil
		}
		// If Cloudflare fails, fall back to Google DNS (8.8.8.8)
		return d.DialContext(ctx, "udp", "8.8.8.8:53")
	},
}

func main() {
	fmt.Println(`
       __  ____      __
 ___ ____ / /_/ __/__  ___/ /__
 / _  / -_) __/ _// _  / _  (_-<
 \_, /\__/\__/___/_//_/\_,_/___/
 /___/   - Links Extractor      
    `)
	var (
		singleURL   string
		listFile    string
		outputFile  string
		sameDomain  bool
		jsOnly      bool
		noAccept    bool
	)

	flag.StringVar(&singleURL, "u", "", "Single URL to fetch")
	flag.StringVar(&listFile, "l", "", "Text file containing a list of URLs")
	flag.StringVar(&outputFile, "o", "extracted.txt", "Output file to write extracted URLs")
	flag.BoolVar(&sameDomain, "d", false, "Extract only links on the same domain as the target")
	flag.BoolVar(&jsOnly, "j", false, "Extract only .js files")
	flag.BoolVar(&noAccept, "no-accept", false, "Do not send the Accept header")
	flag.Parse()

	if singleURL == "" && listFile == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	var urlsToProcess []string

	if singleURL != "" {
		urlsToProcess = append(urlsToProcess, singleURL)
	}

	if listFile != "" {
		urlsFromFile, err := readURLsFromFile(listFile)
		if err != nil {
			fmt.Println(color.RedString("Error reading URLs from file:"), err)
			os.Exit(1)
		}
		urlsToProcess = append(urlsToProcess, urlsFromFile...)
	}

	allExtractedURLs := make(map[string]struct{})

	userAgent := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/97.0.4692.99 Safari/537.36"
	acceptHeader := "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"

	// Create a custom HTTP client with the custom resolver and DNS timeout
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		DialContext: (&net.Dialer{
			Timeout:   15 * time.Second,
			KeepAlive: 15 * time.Second,
			Resolver:  customResolver,
		}).DialContext,
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   30 * time.Second,
	}

	for _, targetURL := range urlsToProcess {
		// Check and add scheme if missing
		if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
			targetURL = "http://" + targetURL
		}

		req, err := http.NewRequest("GET", targetURL, nil)
		if err != nil {
			fmt.Println(color.RedString("Error creating request for"), color.YellowString(targetURL), ":", err)
			continue
		}
		req.Header.Set("User-Agent", userAgent)
		if !noAccept {
			req.Header.Set("Accept", acceptHeader)
		}

		resp, err := client.Do(req)
		if err != nil {
			// Check if the error is due to a TLS handshake failure or a DNS issue
			if urlErr, ok := err.(*url.Error); ok {
				if strings.Contains(urlErr.Error(), "x509: certificate") || strings.Contains(urlErr.Error(), "tls:") {
					fmt.Println(color.YellowString("Warning: Skipping SSL error for"), color.YellowString(targetURL))
					continue
				} else if urlErr.Timeout() {
					fmt.Println(color.YellowString("Warning: Timeout during connection for"), color.YellowString(targetURL))
					continue
				} else if strings.Contains(urlErr.Error(), "lookup") || strings.Contains(urlErr.Error(), "connect") {
					fmt.Println(color.YellowString("Warning: DNS or connection error for"), color.YellowString(targetURL), "-", urlErr)
					continue
				}
			}
			fmt.Println(color.RedString("Error fetching"), color.YellowString(targetURL), ":", err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			fmt.Println(color.RedString("Error response for"), color.YellowString(targetURL), ":", resp.Status)
			continue
		}

		fmt.Println(color.CyanString("--- [INFO] Processing"), color.YellowString(targetURL), "---")
		links := extractLinks(resp.Body, targetURL)

		targetHostname := getHostname(targetURL)

		for _, link := range links {
			parsedLink, err := url.Parse(link)
			if err != nil {
				continue
			}

			resolvedLink := ""
			if !parsedLink.IsAbs() {
				baseURL, err := url.Parse(targetURL)
				if err != nil {
					continue
				}
				resolvedLink = baseURL.ResolveReference(parsedLink).String()
			} else {
				resolvedLink = parsedLink.String()
			}

			resolvedLinkHostname := getHostname(resolvedLink)

			// In-scope check
			if !strings.HasSuffix(resolvedLinkHostname, "."+targetHostname) && resolvedLinkHostname != targetHostname {
				continue
			}

			// Skip if the link is a mailto, tel, or similar
			if strings.HasPrefix(parsedLink.Scheme, "mail") || strings.HasPrefix(parsedLink.Scheme, "tel") {
				continue
			}

			// Junk file check
			if isJunkFile(parsedLink.Path) {
				continue
			}

			if jsOnly && !strings.HasSuffix(parsedLink.Path, ".js") {
				continue
			} else if !jsOnly && strings.HasSuffix(parsedLink.Path, ".js") {
				continue
			}

			// Make sure the link isn't just the base URL itself
			if resolvedLink == targetURL {
				continue
			}

			// Check for duplicates before storing
			if _, loaded := allExtractedURLs[resolvedLink]; !loaded {
				allExtractedURLs[resolvedLink] = struct{}{}
				fmt.Println(color.GreenString("[EXTRACTED] " + resolvedLink))
			}
		}
	}

	var finalURLs []string
	for u := range allExtractedURLs {
		finalURLs = append(finalURLs, u)
	}

	if len(finalURLs) > 0 {
		err := writeURLsToFile(outputFile, finalURLs)
		if err != nil {
			fmt.Println(color.RedString("Error writing extracted URLs to file:"), err)
		} else {
			fmt.Println(color.MagentaString("--- [OUTPUT] Extracted URLs written to"), color.YellowString(outputFile), "---")
		}
	} else {
		fmt.Println(color.YellowString("No URLs extracted. Either no links were found or the filters were too restrictive."))
	}
}

// isJunkFile checks if a file path ends with a common media or junk file extension.
func isJunkFile(path string) bool {
	junkExtensions := []string{
		".css", ".jpeg", ".jpg", ".png", ".gif", ".svg", ".ico", ".webp",
		".mp4", ".mov", ".avi", ".webm", ".mkv",
		".woff", ".woff2", ".ttf", ".eot", ".otf",
		".pdf", ".docx", ".xlsx", ".pptx", ".zip", ".rar", ".7z",
		".xml",
	}

	for _, ext := range junkExtensions {
		if strings.HasSuffix(strings.ToLower(path), ext) {
			return true
		}
	}
	return false
}

// extractLinks parses HTML from an io.Reader and returns a list of links.
func extractLinks(body io.Reader, baseURL string) []string {
	links := make([]string, 0)
	z := html.NewTokenizer(body)

	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			if z.Err() == io.EOF {
				return links
			}
			return links
		case html.StartTagToken, html.SelfClosingTagToken:
			token := z.Token()
			if token.Data == "a" {
				for _, attr := range token.Attr {
					if attr.Key == "href" {
						links = append(links, attr.Val)
					}
				}
			} else if token.Data == "script" || token.Data == "link" {
				for _, attr := range token.Attr {
					if attr.Key == "src" || attr.Key == "href" {
						links = append(links, attr.Val)
					}
				}
			}
		}
	}
}

// readURLsFromFile reads a list of URLs from a file.
func readURLsFromFile(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var urls []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		urls = append(urls, strings.TrimSpace(scanner.Text()))
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return urls, nil
}

// getHostname extracts the hostname from a URL.
func getHostname(u string) string {
	parsedURL, err := url.Parse(u)
	if err != nil {
		return ""
	}
	return parsedURL.Hostname()
}

// writeURLsToFile writes a slice of URLs to a file, one per line, in append mode.
func writeURLsToFile(filename string, urls []string) error {
	// Open the file with append, create, and write permissions
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, u := range urls {
		_, err := writer.WriteString(u + "\n")
		if err != nil {
			return err
		}
	}
	return writer.Flush()
}
