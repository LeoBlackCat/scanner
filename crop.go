package main

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/joho/godotenv"
	"github.com/otiai10/gosseract/v2"
	"github.com/sashabaranov/go-openai"
)

const (
	inputDir     = "/Users/leo/dev/work/scanner/screenshots"
	outputDir    = "/Users/leo/dev/work/scanner/cropped"
	outputMDFile = "/Users/leo/dev/work/scanner/output.md"
	topMargin    = 0.08 // 5% from top
	bottomMargin = 0.05 // 5% from bottom
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Error loading .env file: %v\n", err)
	}

	// Get OpenAI API key (disabled for now - saving raw chapters)
	// apiKey := os.Getenv("OPENAI_API_KEY")
	// if apiKey == "" {
	// 	fmt.Fprintf(os.Stderr, "Error: OPENAI_API_KEY not set in environment\n")
	// 	return
	// }

	// Initialize OpenAI client (disabled for now)
	// openaiClient := openai.NewClient(apiKey)

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
		return
	}

	// Read all files from input directory
	files, err := os.ReadDir(inputDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input directory: %v\n", err)
		return
	}

	// Filter and sort PNG files
	var pngFiles []string
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(file.Name()), ".png") {
			pngFiles = append(pngFiles, file.Name())
		}
	}
	sort.Strings(pngFiles)

	// Initialize Tesseract client
	client := gosseract.NewClient()
	defer client.Close()

	// Process chapter by chapter
	fmt.Println("=== Processing pages and correcting chapters ===\n")

	var currentChapter strings.Builder
	var allCorrectedText strings.Builder
	chapterCount := 0

	// Helper function to process current chapter (no OpenAI, just save raw)
	processChapter := func() error {
		if currentChapter.Len() == 0 {
			return nil
		}

		chapterCount++
		chapterText := currentChapter.String()

		// Ensure chapter starts with proper heading
		chapterText = ensureChapterHeading(chapterText, chapterCount)

		fmt.Printf("💾 Saving Chapter %d (length: %d chars)...\n", chapterCount, len(chapterText))

		// Save individual chapter file
		chapterFile := fmt.Sprintf("/Users/leo/dev/work/scanner/chapter_%02d.md", chapterCount)
		if err := os.WriteFile(chapterFile, []byte(chapterText), 0644); err != nil {
			return fmt.Errorf("failed to save chapter %d: %w", chapterCount, err)
		}

		allCorrectedText.WriteString(chapterText)
		allCorrectedText.WriteString("\n\n---\n\n")

		fmt.Printf("✅ Chapter %d saved to %s\n", chapterCount, chapterFile)

		// Reset for next chapter
		currentChapter.Reset()
		return nil
	}

	// Process each file
	for _, fileName := range pngFiles {
		inputPath := filepath.Join(inputDir, fileName)

		// Create output paths for left and right pages
		baseName := strings.TrimSuffix(fileName, ".png")
		leftPath := filepath.Join(outputDir, baseName+"_left.png")
		rightPath := filepath.Join(outputDir, baseName+"_right.png")

		// Crop and split the image into left and right halves
		if err := cropAndSplitImage(inputPath, leftPath, rightPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error processing %s: %v\n", fileName, err)
			continue
		}

		// Process left page
		client.SetImage(leftPath)
		leftText, err := client.Text()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error performing OCR on %s (left): %v\n", fileName, err)
		} else {
			leftText = cleanText(leftText)

			// Check if this is a new chapter start
			if isChapterStart(leftText) {
				fmt.Printf("\n📖 Chapter start detected: %s (left page) - %s\n", fileName, getFirstLine(leftText))

				// Process previous chapter if exists
				if err := processChapter(); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					return
				}
			}

			// Add to current chapter
			currentChapter.WriteString(leftText)
			if !strings.HasSuffix(leftText, "\n") {
				currentChapter.WriteString("\n")
			}
		}

		// Process right page
		client.SetImage(rightPath)
		rightText, err := client.Text()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error performing OCR on %s (right): %v\n", fileName, err)
		} else {
			rightText = cleanText(rightText)

			// Check if this is a new chapter start
			if isChapterStart(rightText) {
				fmt.Printf("\n📖 Chapter start detected: %s (right page) - %s\n", fileName, getFirstLine(rightText))

				// Process previous chapter if exists
				if err := processChapter(); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					return
				}
			}

			// Add to current chapter
			currentChapter.WriteString(rightText)
			if !strings.HasSuffix(rightText, "\n") {
				currentChapter.WriteString("\n")
			}
		}
	}

	// Process the last chapter
	if err := processChapter(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	// Save to markdown file
	fmt.Println("\n💾 Saving to file...")
	if err := os.WriteFile(outputMDFile, []byte(allCorrectedText.String()), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing output file: %v\n", err)
		return
	}

	fmt.Printf("✅ Complete! Output saved to: %s\n", outputMDFile)
	fmt.Printf("   Total chapters processed: %d\n", chapterCount)
	fmt.Printf("   Individual chapters saved as: chapter_01.md, chapter_02.md, etc.\n")
}

func cropAndSplitImage(inputPath, leftOutputPath, rightOutputPath string) error {
	// Open the image
	file, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open image: %w", err)
	}
	defer file.Close()

	// Decode the image
	img, err := png.Decode(file)
	if err != nil {
		return fmt.Errorf("failed to decode image: %w", err)
	}

	// Get original bounds
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Calculate crop coordinates
	topCrop := int(float64(height) * topMargin)
	bottomCrop := int(float64(height) * bottomMargin)
	croppedHeight := height - topCrop - bottomCrop

	// Calculate middle point for splitting
	middleX := width / 2

	// Create left half image
	leftImg := image.NewRGBA(image.Rect(0, 0, middleX, croppedHeight))
	for y := topCrop; y < height-bottomCrop; y++ {
		for x := 0; x < middleX; x++ {
			leftImg.Set(x, y-topCrop, img.At(x, y))
		}
	}

	// Create right half image
	rightImg := image.NewRGBA(image.Rect(0, 0, width-middleX, croppedHeight))
	for y := topCrop; y < height-bottomCrop; y++ {
		for x := middleX; x < width; x++ {
			rightImg.Set(x-middleX, y-topCrop, img.At(x, y))
		}
	}

	// Save left image
	leftFile, err := os.Create(leftOutputPath)
	if err != nil {
		return fmt.Errorf("failed to create left output file: %w", err)
	}
	defer leftFile.Close()

	if err := png.Encode(leftFile, leftImg); err != nil {
		return fmt.Errorf("failed to encode left image: %w", err)
	}

	// Save right image
	rightFile, err := os.Create(rightOutputPath)
	if err != nil {
		return fmt.Errorf("failed to create right output file: %w", err)
	}
	defer rightFile.Close()

	if err := png.Encode(rightFile, rightImg); err != nil {
		return fmt.Errorf("failed to encode right image: %w", err)
	}

	return nil
}

func correctWithOpenAI(client *openai.Client, text string) (string, error) {
	ctx := context.Background()

	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: openai.GPT4Turbo,
		Messages: []openai.ChatCompletionMessage{
			{
				Role: "system",
				Content: "You are an OCR text correction assistant. Your task is to fix OCR errors in the provided text while preserving the original content and meaning. " +
					"Do NOT summarize, edit, or modify the content in any way except to correct obvious OCR errors (character misrecognitions, spacing issues, etc.). " +
					"Format the output as clean markdown. Preserve all original paragraph breaks and structure.",
			},
			{
				Role:    "user",
				Content: text,
			},
		},
	})

	if err != nil {
		return "", fmt.Errorf("OpenAI API error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI")
	}

	return resp.Choices[0].Message.Content, nil
}

func ensureChapterHeading(text string, chapterNum int) string {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return text
	}

	// Find first non-empty line
	firstLineIdx := -1
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			firstLineIdx = i
			break
		}
	}

	if firstLineIdx == -1 {
		return text
	}

	firstLine := strings.TrimSpace(lines[firstLineIdx])

	// Remove any existing heading markers from first line
	firstLine = strings.TrimPrefix(firstLine, "###")
	firstLine = strings.TrimPrefix(firstLine, "##")
	firstLine = strings.TrimPrefix(firstLine, "#")
	firstLine = strings.TrimSpace(firstLine)

	// Create proper heading
	lines[firstLineIdx] = fmt.Sprintf("# %s", firstLine)

	return strings.Join(lines, "\n")
}

func isChapterStart(text string) bool {
	// Get the first non-empty line
	firstLine := getFirstLine(text)
	if firstLine == "" {
		return false
	}

	// Check if it matches "Chapter N" pattern (no punctuation, just Chapter and a number)
	// Trim whitespace and check
	trimmed := strings.TrimSpace(firstLine)

	// Pattern: "Chapter" followed by whitespace and one or more digits, nothing else
	if strings.HasPrefix(trimmed, "Chapter ") {
		// Extract what comes after "Chapter "
		after := strings.TrimPrefix(trimmed, "Chapter ")
		after = strings.TrimSpace(after)

		// Check if it's just a number (no punctuation, no other text)
		if len(after) > 0 && len(after) <= 3 { // Chapter numbers typically 1-3 digits
			for _, ch := range after {
				if ch < '0' || ch > '9' {
					return false
				}
			}
			return true
		}
	}

	return false
}

func getFirstLine(text string) string {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func cleanText(text string) string {
	// Replace 3+ consecutive newlines with just 2
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}
	return text
}
