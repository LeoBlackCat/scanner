package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/otiai10/gosseract/v2"
)

const (
	inputDir  = "/Users/leo/dev/work/scanner/screenshots"
	outputDir = "/Users/leo/dev/work/scanner/cropped"
	topMargin    = 0.08  // 5% from top
	bottomMargin = 0.05  // 5% from bottom
)

func main() {
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
			fmt.Print(leftText)
			if !strings.HasSuffix(leftText, "\n") {
				fmt.Println()
			}
		}

		// Process right page
		client.SetImage(rightPath)
		rightText, err := client.Text()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error performing OCR on %s (right): %v\n", fileName, err)
		} else {
			rightText = cleanText(rightText)
			fmt.Print(rightText)
			if !strings.HasSuffix(rightText, "\n") {
				fmt.Println()
			}
		}
	}
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

func cleanText(text string) string {
	// Replace 3+ consecutive newlines with just 2
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}
	return text
}
