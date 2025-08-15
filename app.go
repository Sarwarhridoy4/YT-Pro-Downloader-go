package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
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

type stepResult struct {
	LogPath string
	Err     error
}

func main() {
	clearScreen()
	defer func() { fmt.Print(SHOW_CURSOR, RESET) }()
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
		// Is selected code video-only? Then combine with best audio.
		isVideoOnly := checkVideoOnly(code, videoURL, firstItemIndex)
		if isVideoOnly {
			fmt.Printf("%s🎧 Adding best audio…%s\n", CYAN, RESET)
			dlFormat = code + "+ba"
		} else {
			dlFormat = code
		}
	}

	// Download with live progress
	downloadList := filepath.Join(logDir, "downloaded_files.txt")
	_ = os.WriteFile(downloadList, []byte{}, 0o644)
	fmt.Printf("\n%s🚀 Starting download…%s\n", GREEN, RESET)
	fmt.Println()
	fmt.Println()

	if err := runYtDlpWithProgress(dlFormat, playListFlag, rangeFlag, outputTemplate, videoURL, downloadList); err != nil {
		failExit(err.Error())
	}
	fmt.Printf("%s✅ Download(s) finished.%s\n", GREEN, RESET)

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
			baseIn := filepath.Base(inFile)
			baseOut := filepath.Base(outFile)
			fmt.Printf("\n%sConverting:%s %s%s%s → %s%s%s\n", CYAN, RESET, MAGENTA, baseIn, RESET, MAGENTA, baseOut, RESET)
			fmt.Println()
			fmt.Println()

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

// ---------- UI Helpers ----------

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
	return strings.TrimRight(text, "\r\n")
}

func failExit(msg string) {
	fmt.Printf("%s✖ %s%s\n", RED, msg, RESET)
	os.Exit(1)
}

func termCols() int {
	if v := os.Getenv("COLUMNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 40 {
			return n
		}
	}
	// Simple default; avoids external deps
	return 80
}

func barWidth() int {
	cols := termCols()
	if cols > 70 {
		return 50
	}
	return 40
}

func drawBar(percent int, width int) string {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	filled := percent * width / 100
	empty := width - filled
	var b strings.Builder
	b.WriteString("[")
	for i := 0; i < filled; i++ {
		b.WriteString("█")
	}
	for i := 0; i < empty; i++ {
		b.WriteString("░")
	}
	b.WriteString("] ")
	b.WriteString(fmt.Sprintf("%3d%%", percent))
	return b.String()
}

func update2LineUI(header string, percent int, tail string) {
	// Move cursor up two lines, clear, print header and bar+tail
	fmt.Print("\033[2A") // up 2
	fmt.Print("\033[K")  // clear line
	fmt.Println(header)
	fmt.Print("\033[K")
	fmt.Print(drawBar(percent, barWidth()))
	if tail != "" {
		fmt.Print("  ", tail)
	}
	fmt.Println()
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
		// Detect package manager
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
		// winget only available in modern Windows
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

	// spinner
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

type plItem struct {
	Index    string
	Title    string
	Duration string
}

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
		fmt.Println()
		fmt.Println("n) Load next 10 items")
		fmt.Println("0) Done selecting")
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
	// "1,3,5-7" => 1; "5-7" => 5
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

// ---------- Download Progress (yt-dlp) ----------

func runYtDlpWithProgress(dlFormat, playlistFlag, rangeFlag, outTpl, url, listPath string) error {
	args := []string{"-f", dlFormat}
	if playlistFlag != "" {
		args = append(args, playlistFlag)
	}
	if rangeFlag != "" {
		parts := strings.Split(rangeFlag, " ")
		args = append(args, parts...)
	}
	args = append(args, "-o", outTpl, url,
		"--newline",
		"--progress-template", "%(progress._percent_str)s|%(progress._speed_str)s|%(progress._eta_str)s|%(filename)s",
		"--print", "before_dl:START|%(filename)s",
		"--print", "after_move:FILE|%(filepath)s",
	)

	cmd := exec.Command("yt-dlp", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		return err
	}

	header := ""
	tail := ""
	percent := 0

	sc := bufio.NewScanner(stdout)
	// Print two empty lines for UI space
	fmt.Println()
	fmt.Println()
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "START|"):
			file := strings.TrimPrefix(line, "START|")
			header = fmt.Sprintf("%s📥 Downloading:%s %s%s%s", CYAN, RESET, MAGENTA, filepath.Base(file), RESET)
			tail = fmt.Sprintf("%sSpeed:%s --  %sETA:%s --", YELLOW, RESET, YELLOW, RESET)
			update2LineUI(header, 0, tail)
		case strings.HasPrefix(line, "FILE|"):
			fp := strings.TrimPrefix(line, "FILE|")
			appendLine(listPath, fp)
		default:
			// expect percent|speed|eta|filename
			parts := strings.SplitN(line, "|", 4)
			if len(parts) == 4 {
				pct := sanitizePercent(parts[0])
				spd := parts[1]
				eta := parts[2]
				file := filepath.Base(parts[3])
				header = fmt.Sprintf("%s📥 Downloading:%s %s%s%s", CYAN, RESET, MAGENTA, file, RESET)
				tail = fmt.Sprintf("%sSpeed:%s %s  %sETA:%s %s", YELLOW, RESET, nz(spd, "N/A"), YELLOW, RESET, nz(eta, "N/A"))
				percent = pct
				update2LineUI(header, percent, tail)
			}
		}
	}
	if err := cmd.Wait(); err != nil {
		return err
	}
	return nil
}

func sanitizePercent(s string) int {
	s = strings.TrimSpace(s)
	s = strings.TrimRight(s, "%")
	s = strings.TrimSpace(s)
	// keep only digits and dot
	buf := make([]rune, 0, len(s))
	for _, r := range s {
		if (r >= '0' && r <= '9') || r == '.' {
			buf = append(buf, r)
		}
	}
	if len(buf) == 0 {
		return 0
	}
	f, err := strconv.ParseFloat(string(buf), 64)
	if err != nil {
		return 0
	}
	return int(math.Floor(f + 0.00001))
}

func nz(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

func appendLine(path, line string) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(line + "\n")
}

// ---------- Conversion (ffmpeg) ----------

func probeDuration(path string) int {
	// returns integer seconds
	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", path)
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	s := strings.TrimSpace(string(out))
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	if f <= 0 {
		return 0
	}
	return int(math.Round(f))
}

func runFfmpegWithProgress(inFile, outFile string, durationSec int) error {
	// ffmpeg -progress pipe:1
	cmd := exec.Command("ffmpeg", "-y", "-hide_banner", "-loglevel", "error", "-i", inFile, outFile, "-progress", "pipe:1")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		return err
	}

	// Two lines for UI
	fmt.Println()
	fmt.Println()

	header := fmt.Sprintf("%s🔄 Converting:%s %s%s%s → %s%s%s", CYAN, RESET, MAGENTA, filepath.Base(inFile), RESET, MAGENTA, filepath.Base(outFile), RESET)
	update2LineUI(header, 0, "")

	reader := bufio.NewReader(stdout)
	var percent int

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		kv := strings.SplitN(line, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key, val := kv[0], kv[1]
		switch key {
		case "out_time_ms":
			if durationSec > 0 {
				// out_time_ms is microseconds
				v, _ := strconv.ParseInt(val, 10, 64)
				secs := int(v / 1_000_000)
				if durationSec <= 0 {
					durationSec = 1
				}
				p := secs * 100 / max(1, durationSec)
				if p < 0 {
					p = 0
				}
				if p > 100 {
					p = 100
				}
				percent = p
			} else {
				percent = 0
			}
			update2LineUI(header, percent, "")
		case "progress":
			if val == "end" {
				update2LineUI(header, 100, "")
			}
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
