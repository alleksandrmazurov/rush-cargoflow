package main

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const defaultURL = "https://www.michaelfogleman.com/static/rush/rush.txt.gz"

func main() {
	outDir := flag.String("out", "data/external/rush", "output directory")
	url := flag.String("url", defaultURL, "download URL for rush.txt.gz")
	skipDownload := flag.Bool("skip-download", false, "only verify/hash existing files")
	flag.Parse()

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fatal(err)
	}
	gzPath := filepath.Join(*outDir, "rush.txt.gz")
	txtPath := filepath.Join(*outDir, "rush.txt")

	if !*skipDownload {
		if _, err := os.Stat(gzPath); err == nil {
			fmt.Printf("already present: %s\n", gzPath)
		} else {
			fmt.Printf("Downloading %s ...\n", *url)
			if err := downloadFile(*url, gzPath); err != nil {
				fmt.Fprintf(os.Stderr, "\nAutomatic download failed: %v\n\n", err)
				fmt.Fprintf(os.Stderr, "Manual steps:\n")
				fmt.Fprintf(os.Stderr, "  1. Download: %s\n", defaultURL)
				fmt.Fprintf(os.Stderr, "  2. Save as:  %s\n", gzPath)
				fmt.Fprintf(os.Stderr, "  3. Re-run: go run ./cmd/rush-db-fetch --skip-download --out %s\n", *outDir)
				os.Exit(1)
			}
		}
	}

	if _, err := os.Stat(gzPath); err != nil {
		fatal(fmt.Errorf("missing %s — place official rush.txt.gz there", gzPath))
	}

	gzSize, gzSHA, err := fileMeta(gzPath)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("gzip: size=%d sha256=%s\n", gzSize, gzSHA)

	needDecompress := true
	if st, err := os.Stat(txtPath); err == nil && st.Size() > 0 {
		needDecompress = false
		fmt.Printf("uncompressed already present: %s\n", txtPath)
	}
	if needDecompress {
		fmt.Println("Decompressing...")
		if err := gunzipFile(gzPath, txtPath); err != nil {
			fatal(err)
		}
	}
	txtSize, txtSHA, err := fileMeta(txtPath)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("txt:  size=%d sha256=%s\n", txtSize, txtSHA)

	prov := map[string]interface{}{
		"sourceName":            "Michael Fogleman Rush Hour puzzle database",
		"officialPage":          "https://www.michaelfogleman.com/rush/",
		"downloadURL":           *url,
		"retrievalDate":         time.Now().UTC().Format("2006-01-02"),
		"filenameGz":            "rush.txt.gz",
		"filenameTxt":           "rush.txt",
		"compressedSizeBytes":   gzSize,
		"uncompressedSizeBytes": txtSize,
		"sha256Gz":              gzSHA,
		"sha256Txt":             txtSHA,
		"licensingNote":         "Software upstream is MIT. Separate explicit license file for the puzzle database alone was not found; do not invent one. Author publishes the DB for download.",
	}
	data, _ := json.MarshalIndent(prov, "", "  ")
	provPath := filepath.Join(*outDir, "PROVENANCE.json")
	if err := os.WriteFile(provPath, data, 0o644); err != nil {
		fatal(err)
	}
	fmt.Printf("Wrote %s\n", provPath)
	fmt.Println("OK")
}

func downloadFile(url, path string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	tmp := path + ".partial"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	n, err := io.Copy(f, resp.Body)
	cerr := f.Close()
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if cerr != nil {
		_ = os.Remove(tmp)
		return cerr
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	fmt.Printf("downloaded %d bytes\n", n)
	return nil
}

func gunzipFile(gzPath, outPath string) error {
	f, err := os.Open(gzPath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, gz)
	cerr := out.Close()
	if err != nil {
		return err
	}
	return cerr
}

func fileMeta(path string) (int64, string, error) {
	st, err := os.Stat(path)
	if err != nil {
		return 0, "", err
	}
	f, err := os.Open(path)
	if err != nil {
		return 0, "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return 0, "", err
	}
	return st.Size(), hex.EncodeToString(h.Sum(nil)), nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
