

# 📥 YT Pro Downloader (Go Version)

A **professional terminal application** written in **Go** for downloading videos and playlists from YouTube (and 1000+ other sites supported by [yt-dlp](https://github.com/yt-dlp/yt-dlp)), with **automatic audio merging, format selection, and built-in conversion** via [FFmpeg](https://ffmpeg.org/).

---

## 🚀 Features

- ✅ **Interactive terminal UI** with colorful prompts
- ✅ **Single video or full playlist** download modes
- ✅ **Paged playlist selection** — loads **10 items at a time**, lets you pick from multiple pages, and accumulates selections before download
- ✅ **Automatic detection** of video-only formats → merges with best audio
- ✅ **Lists all available formats** before download
- ✅ **Best quality by default** if no format is specified
- ✅ **Custom output format conversion** (MP4, MP3, MKV, WAV, etc.) via FFmpeg
- ✅ **Playlist organization** into a named folder
- ✅ **Works on Windows, Linux, macOS**
- ✅ **Cross-platform builds** available via provided build scripts

---

## 📦 Installation

### 1️⃣ Download the Executable

- **Windows** → `myprogram-windows-amd64.exe`  
- **Linux** → `myprogram-linux-amd64`  
- **macOS** → `myprogram-darwin-amd64`  

*(Use the build scripts if you want to compile it yourself.)*

---

### 2️⃣ Build from Source (Optional)

**Windows:**
```cmd
build-all.bat
````

**Linux/macOS:**

```bash
chmod +x build-all.sh
./build-all.sh
```

All builds are output to the `build/` folder.

---

### 3️⃣ Run the Executable

**Windows:**

```cmd
.\build\myprogram-windows-amd64.exe
```

**Linux/macOS:**

```bash
chmod +x build/myprogram-linux-amd64   # First time only
./build/myprogram-linux-amd64
```

---

## 🛠 Dependencies

* **yt-dlp** (for downloading videos)
* **FFmpeg** (for audio/video merging and conversion)

> Make sure `yt-dlp` and `ffmpeg` are installed and added to your PATH.
> On Windows, download [yt-dlp.exe](https://github.com/yt-dlp/yt-dlp/releases) and [ffmpeg.exe](https://ffmpeg.org/download.html).
> On Linux/macOS, install via `apt`, `dnf`, `pacman`, or `brew`.

---

## 📖 User Guide

After running the executable, you’ll see:

```plaintext
=============================================
      YT Pro Downloader v2.6 (Go Version)
  Powered by yt-dlp + ffmpeg
=============================================
```

---

### Step 1: Choose Mode

```plaintext
Select download mode:
1) Single Video
2) Playlist
Enter choice (1 or 2):
```

---

### Step 2: Playlist Mode – Paged Selection

* Loads **10 videos at a time** from the playlist.
* Options:

  * Enter video numbers: `1,3,5-7`
  * Press `n` → next 10 videos
  * Press `0` → done selecting

Selections **accumulate across pages** until you choose `0`.

---

### Step 3: Choose Format

* Lists **all available formats** for the first selected video:

```plaintext
137 mp4 1920x1080 video only
140 m4a audio only
...
Enter format code (leave blank for best quality):
```

* Leave blank → downloads **best video + best audio**
* Video-only formats → automatically merges with best audio

---

### Step 4: Download

* Single videos → current folder
* Playlists → folder named after playlist

```plaintext
📥 Downloading: My Video Title.mp4
[████████████░░░░░░░░░░░░]  65%  2.3MiB/s  ETA: 00:20
```

---

### Step 5: Optional Conversion

```plaintext
Do you want to convert the file(s) to another format? (y/n): y
Enter output format (e.g., mp4, mp3, mkv, wav): mp3
✔ Converted: myvideo.mp3
```

---

## 💡 Examples

**Download best quality video + audio**

```bash
./myprogramYT-Pro-Downloader-windows-386-linux-amd64
# Leave format blank when prompted
```

**Download specific videos from a playlist**

```bash
./myprogram-linux-amd64
# Choose "Playlist" mode
# Pick videos across pages using the 10-at-a-time system
```

**Download specific format and convert to MP3**

```bash
./YT-Pro-Downloader-linux-amd64
# Enter video-only format code (e.g., 137)
# Enter output format mp3
```

---

## ⚠️ Legal Notice

Downloading videos from YouTube or other platforms may violate their **Terms of Service**.
This tool is intended for **personal, non-commercial use** with content you have rights to download.
The author is **not responsible** for misuse.

---

## 📝 License

MIT License — You are free to modify, build, and distribute, but **use responsibly**.

---
