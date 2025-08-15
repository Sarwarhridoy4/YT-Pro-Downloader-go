package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	RED     = "\033[31m"
	GREEN   = "\033[32m"
	YELLOW  = "\033[33m"
	CYAN    = "\033[36m"
	MAGENTA = "\033[35m"
	BOLD    = "\033[1m"
	DIM     = "\033[2m"
	RESET   = "\033[0m"

	HIDE_CURSOR = "\033[?25l"
	SHOW_CURSOR = "\033[?25h"
)

type plItem struct {
	Index    string
	Title    string
	Duration string
}

func main() {
	clearScreen()
	defer fmt.Print(SHOW_CURSOR, RESET)
	fmt.Print(HIDE_CURSOR)
	banner()

	logDir := filepath.Join(os.TempDir(), "ytpro")
	_ = os.MkdirAll(logDir, 0o755)

	// Dependencies
	if err := ensureDeps(logDir); err != nil {
		failExit(err.Error())
	}

	fmt.Printf("%s✅ All dependencies are installed.%s\n\n", GREEN, RESET)

	// Mode selection
	mode := prompt("Select download mode:\n  1) Single Video\n  2) Playlist\nEnter choice (1 or 2): ")
	var (
		videoURL       string
		playListFlag   string
		rangeFlag      string
		firstItemIndex = 1
		outputTemplate string
	)

	switch strings.TrimSpace(mode) {
	case "1":
		videoURL = prompt("🎯 Enter video URL: ")
		playListFlag = "--no-playlist"
		outputTemplate = "%(title)s.%(ext)s"
		fmt.Printf("\n%s📡 Fetching available formats…%s\n", YELLOW, RESET)
		runPassthrough("yt-dlp", "-F", videoURL)
	case "2":
		videoURL = prompt("📜 Enter playlist URL: ")
		fmt.Printf("\n%s📡 Fetching playlist details…%s\n", YELLOW, RESET)
		items, err := fetchPlaylistItems(videoURL)
		if err != nil {
			failExit(err.Error())
		}
		selections := paginateSelect(items, 10)
		if len(selections) > 0 {
			rangeFlag = "--playlist-items " + strings.Join(selections, ",")
			firstItemIndex = firstFromRanges(selections[0])
		}
		playListFlag = "--yes-playlist"
		outputTemplate = "%(playlist_title)s/%(playlist_index)02d - %(title)s.%(ext)s"

		fmt.Printf("\n%s📡 Fetching formats for playlist item %d…%s\n", YELLOW, firstItemIndex, RESET)
		runPassthrough("yt-dlp", "-F", "--playlist-items", strconv.Itoa(firstItemIndex), videoURL)
	default:
		failExit("Invalid choice.")
	}

	// Format selection
	code := strings.TrimSpace(prompt("🎥 Enter format code (blank=best): "))
	var dlFormat string
	if code == "" {
		dlFormat = "bv*+ba"
	} else {
		isVideoOnly := checkVideoOnly(code, videoURL, firstItemIndex)
		if isVideoOnly {
			fmt.Printf("%s🎧 Adding best audio…%s\n", CYAN, RESET)
			dlFormat = code + "+ba"
		} else {
			dlFormat = code
		}
	}

	// Download
	downloadList := filepath.Join(logDir, "downloaded_files.txt")
	_ = os.WriteFile(downloadList, []byte{}, 0o644)
	fmt.Printf("\n%s🚀 Starting download…%s\n\n", GREEN, RESET)

	if err := runYtDlpWithTextProgress(dlFormat, playListFlag, rangeFlag, outputTemplate, videoURL, downloadList); err != nil {
		failExit(err.Error())
	}

	fmt.Printf("\n%s✅ Download(s) finished.%s\n", GREEN, RESET)

	// Conversion
	convert := strings.TrimSpace(prompt("🔄 Convert file(s)? (y/n): "))
	if strings.EqualFold(convert, "y") {
		outFmt := strings.TrimSpace(prompt("🎯 Enter output format: "))
		files := mustLoadDownloaded(downloadList)
		if len(files) == 0 {
			files = guessRecentFiles(".")
		}
		for _, inFile := range files {
			if _, err := os.Stat(inFile); err != nil {
				continue
			}
			outFile := replaceExt(inFile, outFmt)
			fmt.Printf("\n%s🔄 Converting:%s %s → %s\n", CYAN, RESET, inFile, outFile)
			dur := probeDuration(inFile)
			if err := runFfmpegWithProgress(inFile, outFile, dur); err != nil {
				fmt.Printf("%s✖ Convert failed:%s %s\n", RED, RESET, outFile)
				continue
			}
			fmt.Printf("%s✔ Converted:%s %s\n", GREEN, RESET, outFile)
		}
	} else {
		fmt.Printf("%s✅ Download completed without conversion.%s\n", GREEN, RESET)
	}

	footer()
}

// ---------- UI ----------

func banner() {
	fmt.Printf("%s=============================================%s\n", CYAN, RESET)
	fmt.Printf("%s%s         YT Pro Downloader v2.6%s\n", GREEN, BOLD, RESET)
	fmt.Printf("%s     Powered by yt-dlp + ffmpeg%s\n", YELLOW, RESET)
	fmt.Printf("%s=============================================%s\n\n", CYAN, RESET)
}

func footer() {
	fmt.Printf("\n%s=============================================%s\n", CYAN, RESET)
	fmt.Printf("%s%s   🎉 Thank you for using YT Pro!%s\n", GREEN, BOLD, RESET)
	fmt.Printf("%s=============================================%s\n", CYAN, RESET)
}

func clearScreen() {
	fmt.Print("\033[2J\033[H")
}

func prompt(msg string) string {
	fmt.Print(msg)
	reader := bufio.NewReader(os.Stdin)
	text, _ := reader.ReadString('\n')
	return strings.TrimSpace(text)
}

func failExit(msg string) {
	fmt.Printf("%s✖ %s%s\n", RED, msg, RESET)
	os.Exit(1)
}

// ---------- Dependency Management ----------

func ensureDeps(logDir string) error {
	need := false
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		need = true
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		need = true
	}
	if !need {
		return nil
	}

	fmt.Printf("%sChecking & installing dependencies…%s\n", YELLOW, RESET)
	osType := runtime.GOOS

	switch osType {
	case "linux":
		switch {
		case hasCmd("apt"):
			if err := runStepSpinner(logDir, "Refreshing apt", "sudo", "apt", "update", "-y"); err != nil {
				return err
			}
			if err := runStepSpinner(logDir, "Installing yt-dlp & ffmpeg", "sudo", "apt", "install", "-y", "yt-dlp", "ffmpeg"); err != nil {
				return err
			}
		case hasCmd("dnf"):
			if err := runStepSpinner(logDir, "Installing yt-dlp & ffmpeg", "sudo", "dnf", "install", "-y", "yt-dlp", "ffmpeg"); err != nil {
				return err
			}
		case hasCmd("pacman"):
			if err := runStepSpinner(logDir, "Syncing pacman", "sudo", "pacman", "-Sy", "--noconfirm"); err != nil {
				return err
			}
			if err := runStepSpinner(logDir, "Installing yt-dlp & ffmpeg", "sudo", "pacman", "-S", "--noconfirm", "yt-dlp", "ffmpeg"); err != nil {
				return err
			}
		default:
			return errors.New("Unsupported Linux package manager.")
		}
	case "darwin":
		if !hasCmd("brew") {
			if err := runStepSpinner(logDir, "Installing Homebrew", "/bin/bash", "-c", "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"); err != nil {
				return err
			}
		}
		if err := runStepSpinner(logDir, "Installing yt-dlp", "brew", "install", "yt-dlp"); err != nil {
			return err
		}
		if err := runStepSpinner(logDir, "Installing ffmpeg", "brew", "install", "ffmpeg"); err != nil {
			return err
		}
	case "windows":
		if !hasCmd("winget") {
			return errors.New("winget not found.")
		}
		if err := runStepSpinner(logDir, "Installing yt-dlp", "winget", "install", "--id=yt-dlp.yt-dlp", "-e", "--accept-package-agreements", "--accept-source-agreements"); err != nil {
			return err
		}
		if err := runStepSpinner(logDir, "Installing FFmpeg", "winget", "install", "--id=Gyan.FFmpeg", "-e", "--accept-package-agreements", "--accept-source-agreements"); err != nil {
			return err
		}
	default:
		return fmt.Errorf("Unsupported OS: %s", osType)
	}
	return nil
}

func hasCmd(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func runStepSpinner(logDir, msg string, name string, args ...string) error {
	logFile := filepath.Join(logDir, fmt.Sprintf("step_%d.log", time.Now().UnixNano()))
	f, err := os.Create(logFile)
	if err != nil {
		return err
	}
	defer f.Close()

	cmd := exec.Command(name, args...)
	cmd.Stdout = f
	cmd.Stderr = f

	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()

	spinChars := []rune{'|', '/', '-', '\\'}
	i := 0
	fmt.Print(DIM)
	for {
		select {
		case err := <-done:
			fmt.Print("\r\033[K", RESET)
			if err != nil {
				fmt.Printf("%s✖ %s failed.%s\n", RED, msg, RESET)
				fmt.Printf("%sSee log:%s %s\n", DIM, RESET, logFile)
				printTail(logFile, 15)
				return err
			}
			fmt.Printf("%s✔ %s%s\n", GREEN, msg, RESET)
			return nil
		default:
			fmt.Printf("\r%s %s…", string(spinChars[i%len(spinChars)]), msg)
			time.Sleep(120 * time.Millisecond)
			i++
		}
	}
}

func printTail(path string, n int) {
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	lines := strings.Split(string(b), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	for _, l := range lines {
		fmt.Println(l)
	}
}

// ---------- Playlist ----------

func fetchPlaylistItems(url string) ([]plItem, error) {
	cmd := exec.Command("yt-dlp", "--flat-playlist", "--print", "%(playlist_index)03d|%(title)s|%(duration_string)s", url)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch playlist: %v", err)
	}
	var items []plItem
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		parts := strings.SplitN(sc.Text(), "|", 3)
		if len(parts) != 3 {
			continue
		}
		items = append(items, plItem{Index: parts[0], Title: parts[1], Duration: parts[2]})
	}
	return items, nil
}

func paginateSelect(items []plItem, pageSize int) []string {
	total := len(items)
	start := 0
	var selections []string
	reader := bufio.NewReader(os.Stdin)

	for start < total {
		clearScreen()
		fmt.Printf("%s%sPlaylist Videos (Items %d to %d of %d):%s\n", CYAN, BOLD, start+1, min(start+pageSize, total), total, RESET)
		for i := start; i < min(start+pageSize, total); i++ {
			fmt.Printf("%s%s%s) %s %s[%s]%s\n", MAGENTA, items[i].Index, RESET, items[i].Title, DIM, items[i].Duration, RESET)
		}
		fmt.Println("\nn) Load next 10 items\n0) Done selecting")
		fmt.Print("🎯 Enter selections (e.g., 1,3,5-7): ")

		text, _ := reader.ReadString('\n')
		text = strings.TrimSpace(text)

		if text == "n" || text == "N" {
			start += pageSize
			continue
		} else if text == "0" {
			break
		} else if text != "" {
			selections = append(selections, text)
		}
		start += pageSize
	}
	return selections
}

func firstFromRanges(s string) int {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, ",")
	if len(parts) == 0 {
		return 1
	}
	first := strings.TrimSpace(parts[0])
	if strings.Contains(first, "-") {
		first = strings.SplitN(first, "-", 2)[0]
	}
	n, _ := strconv.Atoi(first)
	if n <= 0 {
		n = 1
	}
	return n
}

// ---------- Formats ----------

func checkVideoOnly(code, url string, firstItem int) bool {
	cmd := exec.Command("yt-dlp", "-F", "--playlist-items", strconv.Itoa(firstItem), url)
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	sc := bufio.NewScanner(bytes.NewReader(out))
	re := regexp.MustCompile(`^\s*` + regexp.QuoteMeta(code) + `\b.*video\s+only`)
	for sc.Scan() {
		if re.MatchString(sc.Text()) {
			return true
		}
	}
	return false
}

// ---------- Download Progress (text) ----------

func runYtDlpWithTextProgress(dlFormat, playlistFlag, rangeFlag, outTpl, url, listPath string) error {
	args := []string{"-f", dlFormat, "-o", outTpl, "--newline", url}
	if playlistFlag != "" {
		args = append(args, playlistFlag)
	}
	if rangeFlag != "" {
		parts := strings.Split(rangeFlag, " ")
		args = append(args, parts...)
	}

	cmd := exec.Command("yt-dlp", args...)
	stdout, _ := cmd.StdoutPipe()
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		return err
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Println(line)
		if strings.HasPrefix(line, "[download] Destination:") {
			filePath := strings.TrimSpace(strings.TrimPrefix(line, "[download] Destination:"))
			appendLine(listPath, filePath)
		}
	}

	return cmd.Wait()
}

// ---------- Conversion ----------

func probeDuration(path string) int {
	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1", path)
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	f, _ := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	return int(f + 0.5)
}

func runFfmpegWithProgress(inFile, outFile string, durationSec int) error {
	cmd := exec.Command("ffmpeg", "-y", "-hide_banner", "-loglevel", "error", "-i", inFile, outFile, "-progress", "pipe:1")
	stdout, _ := cmd.StdoutPipe()
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		return err
	}

	reader := bufio.NewReader(stdout)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		line = strings.TrimSpace(line)
		if line != "" {
			fmt.Println(line)
		}
	}

	return cmd.Wait()
}

// ---------- Utilities ----------

func runPassthrough(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

func mustLoadDownloaded(path string) []string {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []string
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		s := strings.TrimSpace(sc.Text())
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func guessRecentFiles(root string) []string {
	type entry struct {
		Mod  time.Time
		Path string
	}
	var list []entry
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.HasPrefix(filepath.Base(path), ".") {
			return nil
		}
		if info, e := d.Info(); e == nil {
			list = append(list, entry{Mod: info.ModTime(), Path: path})
		}
		return nil
	})
	sort.Slice(list, func(i, j int) bool { return list[i].Mod.After(list[j].Mod) })
	var out []string
	for i := 0; i < min(10, len(list)); i++ {
		out = append(out, list[i].Path)
	}
	return out
}

func replaceExt(path, newExt string) string {
	ext := filepath.Ext(path)
	if !strings.HasPrefix(newExt, ".") {
		newExt = "." + newExt
	}
	return strings.TrimSuffix(path, ext) + newExt
}

func appendLine(path, line string) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|
	os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(line + "\n")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
