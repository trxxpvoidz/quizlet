package api

import (
	"embed"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
)

// -------------------------------------------------
// Embed index.html directly into the binary
// -------------------------------------------------

//go:embed page.html
var indexHTML []byte

// -------------------------------------------------

func internalServerError(w http.ResponseWriter, err error) {
	if err != nil {
		log.Printf("Internal server error: %v", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func Handler(w http.ResponseWriter, r *http.Request) {

	defer func() {
		if err := recover(); err != nil {
			log.Printf("WithHandler panic: %v", err)
			http.Error(w, fmt.Sprintf("internal server error: %v", err), http.StatusInternalServerError)
		}
	}()

	htmlProxy := os.Getenv("HTTP_PROXY_ENABLE") == "true"

	// CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, X-PROXY-HOST, X-PROXY-SCHEME")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// -------------------------------------------------
	// Serve embedded index.html at /
	// -------------------------------------------------
	if r.URL.Path == "/" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write(indexHTML)
		return
	}

	// -------------------------------------------------
	// Proxy logic
	// -------------------------------------------------

	re := regexp.MustCompile(`^/*(https?:)/*`)
	u := re.ReplaceAllString(r.URL.Path, "$1//")
	if r.URL.RawQuery != "" {
		u += "?" + r.URL.RawQuery
	}
	if !strings.HasPrefix(u, "http") {
		http.Error(w, "invalid url: "+u, http.StatusBadRequest)
		return
	}

	req, err := http.NewRequest(r.Method, u, r.Body)
	if err != nil {
		internalServerError(w, err)
		return
	}

	for k, v := range r.Header {
		for _, vv := range v {
			req.Header.Add(k, vv)
		}
	}

	if htmlProxy && strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		req.Header.Set("Accept-Encoding", "gzip")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		internalServerError(w, err)
		return
	}
	defer resp.Body.Close()

	for k, v := range resp.Header {
		for _, vv := range v {
			w.Header().Add(k, vv)
		}
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
