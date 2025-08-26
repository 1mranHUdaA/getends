# 🔗 GETENDS

A fast and lightweight **Go tool** to extract links (including `.js` files) from web pages with filtering, error handling, and DNS failover.  
It’s useful for recon, bug bounty hunting, or general link scraping.  

```
       __  ____      __
 ___ ____ / /_/ __/__  ___/ /__
 / _  / -_) __/ _// _  / _  (_-<
 \_, /\__/\__/___/_//_/\_,_/___/
/___/   - Links Extractor
```

---

## ✨ Features
- Extracts links (`<a>`, `<script>`, `<link>`) from HTML pages.  
- Supports **single URL** or **list of URLs** input.  
- Filters:
  - Only same-domain links (`-d`)  
  - Only `.js` files (`-j`)  
  - Excludes junk/media files (`.css`, `.png`, `.pdf`, etc.)  
- Outputs results to a file (default: `extracted.txt`).  

---

## ⚙️ Installation
Make sure you have [Go installed](https://go.dev/dl/).  

```bash
git clone https://github.com/yourusername/links-extractor.git
cd links-extractor
go build -o links-extractor
```

---

## 🚀 Usage

### Single URL
```bash
./links-extractor -u https://example.com
```

### Multiple URLs from a file
```bash
./links-extractor -l urls.txt
```

### Output to a custom file
```bash
./links-extractor -u https://example.com -o results.txt
```

### Extract only same-domain links
```bash
./links-extractor -u https://example.com -d
```

### Extract only `.js` files
```bash
./links-extractor -u https://example.com -j
```

### Skip sending `Accept` header
```bash
./links-extractor -u https://example.com --no-accept
```

---

## 📂 Example Output
When extracting from `https://example.com`:

```
--- [INFO] Processing https://example.com ---
[EXTRACTED] https://example.com/app.js
[EXTRACTED] https://example.com/dashboard
[EXTRACTED] https://static.example.com/script/main.js
--- [OUTPUT] Extracted URLs written to extracted.txt ---
```

---

## 🛠 Flags

| Flag          | Description |
|---------------|-------------|
| `-u`          | Single URL to fetch |
| `-l`          | File with list of URLs |
| `-o`          | Output file (default: `extracted.txt`) |
| `-d`          | Extract only same-domain links |
| `-j`          | Extract only `.js` files |
| `--no-accept` | Do not send the `Accept` header |

---

## 🧑‍💻 Example Workflow (Bug Bounty Recon)

```bash
# Crawl multiple URLs and collect JS files only
./links-extractor -l targets.txt -j -o js-links.txt

# Combine with tools like httpx or gau
cat js-links.txt | httpx -mc 200 -o live-js.txt
```

---

## ⚡️ Notes
- Junk/static files are filtered automatically.  

---
