package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	languageToolURL = "http://localhost:8081/v2/check"
	chapterPattern  = "chapter_*.md"
)

type LanguageToolResponse struct {
	Matches []struct {
		Message      string `json:"message"`
		Offset       int    `json:"offset"`
		Length       int    `json:"length"`
		Replacements []struct {
			Value string `json:"value"`
		} `json:"replacements"`
		Rule struct {
			Category struct {
				ID string `json:"id"`
			} `json:"category"`
		} `json:"rule"`
	} `json:"matches"`
}

func main() {
	// Check if LanguageTool is running
	if err := checkLanguageTool(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintf(os.Stderr, "\nTo start LanguageTool:\n")
		fmt.Fprintf(os.Stderr, "  1. Install: brew install languagetool\n")
		fmt.Fprintf(os.Stderr, "  2. Run: languagetool --http --port 8081\n")
		return
	}

	// Find all chapter files
	files, err := filepath.Glob(chapterPattern)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding chapter files: %v\n", err)
		return
	}

	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "No chapter files found. Run ./crop-ocr first.\n")
		return
	}

	sort.Strings(files)

	fmt.Printf("Found %d chapter files\n\n", len(files))

	// Process each chapter
	for _, file := range files {
		fmt.Printf("Processing %s...\n", file)

		// Read chapter content
		content, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Error reading %s: %v\n", file, err)
			continue
		}

		// Correct with LanguageTool
		corrected, corrections, err := correctWithLanguageTool(string(content))
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Error correcting %s: %v\n", file, err)
			continue
		}

		fmt.Printf("  Found %d corrections\n", corrections)

		// Save corrected version
		outputFile := strings.TrimSuffix(file, ".md") + "_corrected.md"
		if err := os.WriteFile(outputFile, []byte(corrected), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "  Error writing %s: %v\n", outputFile, err)
			continue
		}

		fmt.Printf("  ✅ Saved to %s\n\n", outputFile)
	}

	fmt.Println("✅ All chapters corrected!")
}

func checkLanguageTool() error {
	resp, err := http.Get("http://localhost:8081/v2/languages")
	if err != nil {
		return fmt.Errorf("LanguageTool is not running at localhost:8081")
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("LanguageTool returned status %d", resp.StatusCode)
	}

	return nil
}

func correctWithLanguageTool(text string) (string, int, error) {
	// Prepare request
	data := url.Values{}
	data.Set("text", text)
	data.Set("language", "en-US")
	data.Set("enabledOnly", "false")

	// Send request
	resp, err := http.PostForm(languageToolURL, data)
	if err != nil {
		return "", 0, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", 0, fmt.Errorf("LanguageTool returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("failed to read response: %w", err)
	}

	var ltResp LanguageToolResponse
	if err := json.Unmarshal(body, &ltResp); err != nil {
		return "", 0, fmt.Errorf("failed to parse response: %w", err)
	}

	// Apply corrections in reverse order (to maintain offsets)
	corrected := text
	matches := ltResp.Matches

	// Sort by offset in descending order
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Offset > matches[j].Offset
	})

	correctionCount := 0
	for _, match := range matches {
		// Only apply if there's a replacement suggestion
		if len(match.Replacements) > 0 {
			offset := match.Offset
			length := match.Length
			replacement := match.Replacements[0].Value

			// Apply the correction
			if offset+length <= len(corrected) {
				corrected = corrected[:offset] + replacement + corrected[offset+length:]
				correctionCount++
			}
		}
	}

	return corrected, correctionCount, nil
}
