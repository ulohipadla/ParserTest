package main

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/antonholmquist/jason"
	"github.com/opesun/goquery"
	"golang.org/x/oauth2"

	"golang.org/x/oauth2/google"

	"google.golang.org/api/drive/v3"
)

var (
	WORKERS       int             = 2
	REPORT_PERIOD int             = 10
	DUP_TO_STOP   int             = 1
	HASH_FILE     string          = "hash.bin"
	QUOTES_FILE   string          = "quotes.txt"
	used          map[string]bool = make(map[string]bool)
)

const (
	Secret string = "client_secret.json"
	URL    string = "https://confluence.hflabs.ru/pages/viewpage.action?pageId=1181220999#app-switcher"
)

func getClient(ctx context.Context, config *oauth2.Config) *http.Client {
	cacheFile, err := tokenCacheFile()
	if err != nil {
		log.Fatalf("Unable to get path to cached credential file. %v", err)
	}
	tok, err := tokenFromFile(cacheFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(cacheFile, tok)
	}
	return config.Client(ctx, tok)
}

func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var code string
	if _, err := fmt.Scan(&code); err != nil {
		log.Fatalf("Unable to read authorization code %v", err)
	}

	tok, err := config.Exchange(oauth2.NoContext, code)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web %v", err)
	}
	return tok
}

func tokenCacheFile() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	tokenCacheDir := filepath.Join(usr.HomeDir, ".credentials")
	os.MkdirAll(tokenCacheDir, 0700)
	return filepath.Join(tokenCacheDir,
		url.QueryEscape("google-drive-golang.json")), err
}

func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	t := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(t)
	defer f.Close()
	return t, err
}

func saveToken(file string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", file)
	f, err := os.Create(file)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func randStr(strSize int, randType string) string {

	var dictionary string

	if randType == "alphanum" {
		dictionary = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	}

	if randType == "alpha" {
		dictionary = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	}

	if randType == "number" {
		dictionary = "0123456789"
	}

	var bytes = make([]byte, strSize)
	for k, v := range bytes {
		bytes[k] = dictionary[v%byte(len(dictionary))]
	}
	return string(bytes)
}

func readHashes() {
	if _, err := os.Stat(HASH_FILE); err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Creating new hash.")
			return
		}
	}

	fmt.Println("Reading hash...")
	hash_file, err := os.OpenFile(HASH_FILE, os.O_RDONLY, 0666)
	check(err)
	defer hash_file.Close()
	data := make([]byte, 16)
	for {
		n, err := hash_file.Read(data)
		if err != nil {
			if err == io.EOF {
				break
			}
			panic(err)
		}
		if n == 16 {
			used[hex.EncodeToString(data)] = true
		}
	}

	fmt.Println("Done. Hashes read: ", len(used))
}

func grab() <-chan string {
	c := make(chan string)
	for i := 0; i < WORKERS; i++ {
		go func() {
			for {
				x, err := goquery.ParseUrl(URL)
				s := x.Find("tr").Text()
				if err == nil {
					s = spaces(s)
					c <- s
				}
				time.Sleep(100 * time.Millisecond)
			}
		}()
	}
	fmt.Println("Started channels: ", WORKERS)
	return c
}
func inMass(ch rune) bool {
	capitals := []rune{'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z'}
	for k := range capitals {
		if ch == capitals[k] {
			return true
		}
	}
	return false
}
func isCode(a rune) bool {
	nums := []rune{'1', '2', '3', '4', '5', '6', '7', '8', '9', '0'}
	for k := range nums {
		if a == nums[k] {
			return true
		}
	}
	return false
}
func spaces(s string) string {
	res := []rune(s)
	st := ""
	for k := 0; k < len(res)-3; k++ {
		if unicode.IsUpper(res[k+1]) && !inMass(res[k+1]) || isCode(res[k]) && isCode(res[k+1]) && isCode(res[k]) && res[k-1] != ' ' && res[k-2] != ' ' && res[k-3] != ' ' {
			if isCode(res[k]) && isCode(res[k+1]) && isCode(res[k+2]) {
				st += "_" + string(res[k]) + string(res[k+1]) + string(res[k+2]) + "_"
				k += 2
			} else {
				st += string(res[k]) + "_" + string(res[k+1])
				k++
			}
		} else {
			st += string(res[k])
		}
	}
	return st + string(res[len(res)-3]) + string(res[len(res)-2]) + string(res[len(res)-1])
}
func check(e error) {
	if e != nil {
		panic(e)
	}
}
func fileprocessing() {
	readHashes()

	flag.IntVar(&WORKERS, "w", WORKERS, "ammount of channels")
	flag.IntVar(&REPORT_PERIOD, "r", REPORT_PERIOD, "refresh rate")
	flag.IntVar(&DUP_TO_STOP, "d", DUP_TO_STOP, "duplicates to stop")
	flag.StringVar(&HASH_FILE, "hf", HASH_FILE, "hash file")
	flag.StringVar(&QUOTES_FILE, "qf", QUOTES_FILE, "data file")
	flag.Parse()

	quotes_file, err := os.OpenFile(QUOTES_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	check(err)
	defer quotes_file.Close()

	hash_file, err := os.OpenFile(HASH_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	check(err)
	defer hash_file.Close()

	ticker := time.NewTicker(time.Duration(REPORT_PERIOD) * time.Second)
	defer ticker.Stop()

	key_chan := make(chan os.Signal, 1)
	signal.Notify(key_chan, os.Interrupt)

	hasher := md5.New()

	quotes_count, dup_count := 0, 0

	quotes_chan := grab()
	for {
		select {
		case quote := <-quotes_chan:
			quotes_count++
			hasher.Reset()
			io.WriteString(hasher, quote)
			hash := hasher.Sum(nil)
			hash_string := hex.EncodeToString(hash)
			if !used[hash_string] {
				used[hash_string] = true
				_, err = hash_file.Write(hash)
				s := strings.Split(quote, "_")
				fmt.Println(s)
				for k := 1; k < len(s)-2; k++ {
					_, err = quotes_file.WriteString(s[k-1] + ": " + s[k] + "\n\n")
					k++
					check(err)
				}
				check(err)
				dup_count = 0
			} else {
				if dup_count++; dup_count == DUP_TO_STOP {
					fmt.Println("Lines: ", len(used))
					return
				}
			}
		case <-key_chan:
			return
		case <-ticker.C:
			quotes_count = 0
		}
	}
}
func main() {
	fileprocessing()
	ctx := context.Background()

	credential, err := ioutil.ReadFile(Secret)
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	config, err := google.ConfigFromJSON(credential, drive.DriveScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}

	client := getClient(ctx, config)

	driveClientService, err := drive.New(client)
	if err != nil {
		log.Fatalf("Unable to initiate new Drive client: %v", err)
	}

	cacheFile, err := tokenCacheFile()
	if err != nil {
		log.Fatalf("Unable to get path to cached credential file. %v", err)
	}

	token, err := tokenFromFile(cacheFile)
	if err != nil {
		log.Fatalf("Unable to get token from file. %v", err)
	}

	fileName := "quotes.txt"
	fileBytes, err := ioutil.ReadFile(fileName)
	if err != nil {
		log.Fatalf("Unable to read file for upload: %v", err)
	}

	fileMIMEType := http.DetectContentType(fileBytes)

	postURL := "https://www.googleapis.com/upload/drive/v3/files?uploadType=multipart"

	authToken := token.AccessToken

	boundary := randStr(32, "alphanum")

	uploadData := []byte("\n" +
		"--" + boundary + "\n" +
		"Content-Type: application/json; charset=" + string('"') + "UTF-8" + string('"') + "\n\n" +
		"{ \n" +
		string('"') + "name" + string('"') + ":" + string('"') + fileName + string('"') + "\n" +
		"} \n\n" +
		"--" + boundary + "\n" +
		"Content-Type:" + fileMIMEType + "\n\n" +
		string(fileBytes) + "\n" +

		"--" + boundary + "--")

	request, _ := http.NewRequest("UPDATE", postURL, strings.NewReader(string(uploadData)))
	request.Header.Add("Host", "www.googleapis.com")
	request.Header.Add("Authorization", "Bearer "+authToken)
	request.Header.Add("Content-Type", "multipart/related; boundary="+string('"')+boundary+string('"'))
	request.Header.Add("Content-Length", strconv.FormatInt(request.ContentLength, 10))

	response, err := client.Do(request)
	if err != nil {
		log.Fatalf("Unable to be post to Google API: %v", err)
	}

	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)

	if err != nil {
		log.Fatalf("Unable to read Google API response: %v", err)
	}

	fmt.Println(string(body))

	jsonAPIreply, _ := jason.NewObjectFromBytes(body)

	uploadedFileID, _ := jsonAPIreply.GetString("id")
	fmt.Println("Uploaded file ID : ", uploadedFileID)

	renamedFile := drive.File{Name: fileName}

	if err != nil {
		log.Fatalf("Unable to rename(update) uploaded file in Drive:  %v", err)
	}

	filesListCall, err := driveClientService.Files.List().OrderBy("name").Do()

	if err != nil {
		log.Fatalf("Unable to list files in Drive:  %v", err)
	}

	if len(filesListCall.Files) > 0 {
		for _, file := range filesListCall.Files {
			if string(file.Name) == fileName {
				_, err = driveClientService.Files.Update(file.Id, &renamedFile).Do()
				check(err)
				break
			}
		}
	}
}
