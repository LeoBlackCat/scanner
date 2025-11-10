package main

import (
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	// "github.com/go-vgo/robotgo"
	hook "github.com/robotn/gohook"
	"github.com/kbinani/screenshot"
)

const (
	screenshotDir = "screenshots"
	// Similarity threshold: 0.0 = identical, 1.0 = completely different
	// Adjust this value based on testing (higher = more tolerant of differences)
	similarityThreshold = 0.01
)

var lastScreenshot *image.RGBA

func main() {
	// Create screenshots directory if it doesn't exist
	if err := os.MkdirAll(screenshotDir, 0755); err != nil {
		fmt.Printf("Error creating directory: %v\n", err)
		return
	}

	fmt.Println("Screenshot capture app started!")
	fmt.Println("Press Cmd+Shift+S to take a screenshot")
	fmt.Println("Press Ctrl+C to quit")

	// Register global hotkey using gohook
	// Cmd+Shift+S (keys "cmd", "shift", "s")
	hook.Register(hook.KeyDown, []string{"cmd", "shift", "s"}, func(e hook.Event) {
		fmt.Println("Hotkey triggered!")
		handleScreenshot()
	})

	s := hook.Start()
	<-hook.Process(s)
}

func handleScreenshot() {
	// Capture screenshot first
	var img *image.RGBA
	if !captureKindleWindow(&img) {
		captureFullScreen(&img)
	}

	if img == nil {
		fmt.Println("Failed to capture screenshot")
		return
	}

	// Check if similar to last screenshot
	if lastScreenshot != nil && isSimilar(lastScreenshot, img) {
		fmt.Println("Screenshot is similar to previous one, skipping...")
		// Still press right arrow to advance
		// robotgo.KeyTap("right")
		return
	}

	// Play sound
	playSound()

	// Save the screenshot
	saveScreenshotImg(img)

	// Store as last screenshot
	lastScreenshot = img

	// Press right arrow key to go to next page
	// robotgo.KeyTap("right")
	// fmt.Println("Pressed right arrow key")
}

func playSound() {
	// Play system beep sound
	go func() {
		exec.Command("afplay", "/System/Library/Sounds/Glass.aiff").Run()
	}()
}

func isSimilar(img1, img2 *image.RGBA) bool {
	// Check if dimensions match
	if img1.Bounds() != img2.Bounds() {
		return false
	}

	bounds := img1.Bounds()

	// Sample-based comparison for performance
	// Compare every 10th pixel to speed up comparison
	sampleRate := 10
	totalSamples := 0
	differentPixels := 0

	for y := bounds.Min.Y; y < bounds.Max.Y; y += sampleRate {
		for x := bounds.Min.X; x < bounds.Max.X; x += sampleRate {
			totalSamples++

			r1, g1, b1, _ := img1.At(x, y).RGBA()
			r2, g2, b2, _ := img2.At(x, y).RGBA()

			// Calculate color difference (Euclidean distance in RGB space)
			// Normalize to 0-255 range
			dr := float64(r1>>8) - float64(r2>>8)
			dg := float64(g1>>8) - float64(g2>>8)
			db := float64(b1>>8) - float64(b2>>8)

			distance := math.Sqrt(dr*dr + dg*dg + db*db)

			// If color difference > threshold, count as different
			if distance > 30 { // Threshold for individual pixel difference
				differentPixels++
			}
		}
	}

	// Calculate percentage of different pixels
	diffRatio := float64(differentPixels) / float64(totalSamples)

	fmt.Printf("Similarity check: %.2f%% different pixels\n", diffRatio*100)

	return diffRatio < similarityThreshold
}

func captureKindleWindow(imgOut **image.RGBA) bool {
	// Use screencapture with window selection for Kindle
	// First try to use macOS screencapture with window ID

	// Get window ID using Python script (more reliable than AppleScript for Kindle)
	cmd := exec.Command("python3", "-c", `
import Quartz
import sys

window_list = Quartz.CGWindowListCopyWindowInfo(
    Quartz.kCGWindowListOptionOnScreenOnly | Quartz.kCGWindowListExcludeDesktopElements,
    Quartz.kCGNullWindowID
)

for window in window_list:
    owner = window.get('kCGWindowOwnerName', '')
    if 'Kindle' in owner:
        bounds = window['kCGWindowBounds']
        wid = window['kCGWindowNumber']
        print(f"{int(bounds['X'])},{int(bounds['Y'])},{int(bounds['Width'])},{int(bounds['Height'])},{wid}")
        sys.exit(0)
sys.exit(1)
`)

	output, err := cmd.Output()
	if err != nil {
		fmt.Println("Kindle window not found, capturing full screen instead")
		return false
	}

	// Parse window bounds and ID
	parts := strings.Split(strings.TrimSpace(string(output)), ",")
	if len(parts) != 5 {
		fmt.Printf("Invalid output format: %s\n", string(output))
		return false
	}

	x, _ := strconv.Atoi(parts[0])
	y, _ := strconv.Atoi(parts[1])
	w, _ := strconv.Atoi(parts[2])
	h, _ := strconv.Atoi(parts[3])

	fmt.Printf("Capturing Kindle window at (%d,%d) size %dx%d\n", x, y, w, h)

	// Capture the specific region
	img, err := screenshot.CaptureRect(image.Rect(x, y, x+w, y+h))
	if err != nil {
		fmt.Printf("Error capturing Kindle window: %v\n", err)
		return false
	}

	*imgOut = img
	return true
}

func captureFullScreen(imgOut **image.RGBA) {
	// Capture the primary display
	bounds := screenshot.GetDisplayBounds(0)
	img, err := screenshot.CaptureRect(bounds)
	if err != nil {
		fmt.Printf("Error capturing screenshot: %v\n", err)
		return
	}

	fmt.Println("Captured full screen")
	*imgOut = img
}

func saveScreenshotImg(img *image.RGBA) {
	// Determine prefix based on image size (Kindle window vs full screen)
	prefix := "screen"
	bounds := screenshot.GetDisplayBounds(0)
	if img.Bounds().Dx() < bounds.Dx() || img.Bounds().Dy() < bounds.Dy() {
		prefix = "kindle"
	}

	// Get next available filename
	filename := getNextFilename(prefix)
	filepath := filepath.Join(screenshotDir, filename)

	// Save the image
	file, err := os.Create(filepath)
	if err != nil {
		fmt.Printf("Error creating file: %v\n", err)
		return
	}
	defer file.Close()

	if err := png.Encode(file, img); err != nil {
		fmt.Printf("Error encoding PNG: %v\n", err)
		return
	}

	fmt.Printf("Screenshot saved to: %s\n", filepath)
}

func getNextFilename(prefix string) string {
	// Find the highest number in existing files
	files, err := os.ReadDir(screenshotDir)
	maxNum := 0

	if err == nil {
		for _, file := range files {
			if strings.HasPrefix(file.Name(), prefix) && strings.HasSuffix(file.Name(), ".png") {
				// Extract number from filename like "screen_001.png" or "kindle_001.png"
				name := strings.TrimSuffix(file.Name(), ".png")
				parts := strings.Split(name, "_")
				if len(parts) == 2 {
					if num, err := strconv.Atoi(parts[1]); err == nil && num > maxNum {
						maxNum = num
					}
				}
			}
		}
	}

	// Return next filename
	return fmt.Sprintf("%s_%03d.png", prefix, maxNum+1)
}
