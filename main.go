package main

import (
	"bufio"
	"container/heap"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"image/jpeg"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
)

type urlHash struct {
	url  string
	hash string
}

type urlColor struct {
	url    string
	color1 string
	color2 string
	color3 string
}

type colors struct {
	color1 string
	color2 string
	color3 string
}

var urlHash_color_map = make(map[string]colors)

func main() {
	fmt.Println("Script Started")

	url_chan := readFile()
	urlHash_chan := hashUrl(url_chan)
	color_chan := downloadColor(urlHash_chan)
	writeFile(color_chan)

	fmt.Println("Script completed!")
}

func readFile() chan string {
	url_chan := make(chan string)

	sysArgs := os.Args
	var filename string

	if len(sysArgs) == 1 {
		fmt.Println("Filepath Argument not provided.")
		filename = "./input.txt"
	} else {
		filename = sysArgs[1]
	}

	go func() {
		file, err := os.Open("./" + filename)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		reader := bufio.NewReader(file)

		for {
			url, err := reader.ReadString('\n')
			if err == io.EOF {
				url_chan <- url
				break
			}
			url = strings.TrimSuffix(url, "\n")

			if err != nil {
				continue
			}
			url_chan <- url
		}
		close(url_chan)
	}()
	return url_chan
}

func GetMD5Hash(text string) string {
	hash := md5.Sum([]byte(text))
	return hex.EncodeToString(hash[:])
}

func hashUrl(url_chan chan string) chan urlHash {
	urlHash_chan := make(chan urlHash)

	go func() {
		for url := range url_chan {
			url_hash := urlHash{url, GetMD5Hash(url)}
			urlHash_chan <- url_hash
		}
		close(urlHash_chan)
	}()
	return urlHash_chan
}

func downloadColor(urlHash_chan chan urlHash) chan urlColor {

	urlColor_chan := make(chan urlColor)

	download_chan := make(chan urlHash)

	var wg_downloader sync.WaitGroup
	const numDownloaders = 5
	wg_downloader.Add(numDownloaders)

	// Bounded number of GoRoutines to download image. If not bounded, too many GoRoutines
	// can be spawned leading to memory issues.
	for i := 0; i < numDownloaders; i++ {
		go func() {
			downloader(download_chan, urlColor_chan, &wg_downloader)
		}()
	}

	go func() {
		for img := range urlHash_chan {
			if colors, url_exist := urlHash_color_map[img.hash]; url_exist {
				urlColor_chan <- urlColor{img.url, colors.color1, colors.color2, colors.color3}
			} else {
				download_chan <- urlHash{img.url, img.hash}
			}
		}
		close(download_chan)
		wg_downloader.Wait()
		close(urlColor_chan)
	}()

	return urlColor_chan
}

func downloader(download_chan chan urlHash, urlColor_chan chan urlColor, wg *sync.WaitGroup) {
	defer wg.Done()

	for url := range download_chan {
		image_colors := process_image(url)

		urlHash_color_map[url.hash] = colors{image_colors.color1, image_colors.color2, image_colors.color3}
		urlColor_chan <- image_colors
	}
}

func writeFile(color_chan chan urlColor) {
	f, _ := os.Create("output.txt")

	writer := bufio.NewWriter(f)

	for url_color := range color_chan {
		line_string := (url_color.url + "," + url_color.color1 + "," + url_color.color2 + "," + url_color.color3) + "\n"
		writer.WriteString(line_string)
	}
	writer.Flush()
}

// IMAGE PROCESSING
// Components used to decode an image and computer 3 most prevalent color

type RGB struct {
	red, green, blue int64
}

type kv struct {
	Key   string
	Value int
}
type KVHeap []kv

func (h KVHeap) Len() int           { return len(h) }
func (h KVHeap) Less(i, j int) bool { return h[i].Value > h[j].Value }
func (h KVHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *KVHeap) Push(x interface{}) {
	*h = append(*h, x.(kv))
}

func (h *KVHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

func getHeap(m map[string]int) *KVHeap {
	h := &KVHeap{}
	heap.Init(h)
	for k, v := range m {
		heap.Push(h, kv{k, v})
	}
	return h
}

func t2x(t int64) string {
	result := strconv.FormatInt(t, 16)
	if len(result) == 1 {
		result = "0" + result
	}
	return result
}

func rgb2hex(color RGB) string {
	r := t2x(color.red)
	g := t2x(color.green)
	b := t2x(color.blue)
	return strings.ToUpper("#" + (r + g + b))
}

func process_image(url urlHash) urlColor {
	resp, err := http.Get(url.url)
	if err != nil {
		fmt.Println("HTTP GET ERROR:", err)
	}
	defer resp.Body.Close()

	// Image not written to disk. Decoding done right after download to memory
	img, err := jpeg.Decode(resp.Body)
	if err != nil {
		fmt.Println("JPEG Decode Error: ", err, url.url)
		return urlColor{url.url, "#ERRORR", "#ERRORR", "#ERRORR"}
	}

	colorCounter := make(map[string]int)

	for y := 0; y < img.Bounds().Max.Y; y++ {
		for x := 0; x < img.Bounds().Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			r1, g1, b1 := r/257, g/257, b/257

			rgb_pixel := RGB{int64(r1), int64(g1), int64(b1)}
			hexCode := rgb2hex(rgb_pixel)

			colorCounter[hexCode] += 1
		}
	}
	h := getHeap(colorCounter)
	n := 3

	var colors []string

	for i := 0; i < n; i++ {
		color := heap.Pop(h)
		color_string := color.(kv).Key
		colors = append(colors, color_string)
	}
	urlColor_val := urlColor{url.url, colors[0], colors[1], colors[2]}
	return urlColor_val
}
