package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/transform"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"sync"
	"time"
)

func main() {

	for i := 1; i <= pageNumber; i++ {
		getGirlList(fmt.Sprintf("%s/%d", baseUrl, i))
	}
	wg.Wait()
}

const (
	baseUrl    = "https://mzitu.com/page"
	albumUrl   = "https://www.mzitu.com"
	girlUrl    = "https://www.mzitu.com/"
	saveDir    = "./meizi/"
	referer    = "https://www.mzitu.com/208319/34"
	pageNumber = 1
	userAgent  = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/84.0.4147.135 Safari/537.36"
	cookie     = "Hm_lvt_cb7f29be3c304cd3bb0c65a4faa96c30=1599050899; Hm_lpvt_cb7f29be3c304cd3bb0c65a4faa96c30=1599235057"
)

// 获取 uids 正则
var uidPattern = regexp.MustCompile(`<li><a href="https://www.mzitu.com/(\d{6}).*?alt='(.+)' width=.*?/>`)

// 获取 主图片地址 正则
var imgPattern = regexp.MustCompile(`<img class="blur" src="(.*?\.jpg)`)

var wg sync.WaitGroup

//创建文件夹
func CreateDir(dirName string) {
	_ = os.Mkdir(dirName, 0755)
}

func SaveFile(url string, savePath string) (httpStatusCode int) {
	method := "GET"
	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		fmt.Println(err)
	}
	req.Header.Add("Referer", referer)
	req.Header.Add("user-agent", userAgent)
	req.Header.Add("cookie", cookie)

	res, err := client.Do(req)
	if err != nil {
		time.Sleep(500 * time.Millisecond)
		return 200
	}
	defer res.Body.Close()
	httpStatusCode = res.StatusCode
	if httpStatusCode == 200 {
		body, _ := ioutil.ReadAll(res.Body)
		_ = ioutil.WriteFile(savePath, body, 0755)
		log.Printf("保存成功：%s", url)
	}
	return
}

func getReponseWithGlobalHeaders(url string) *http.Response {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Referer", referer)
	req.Header.Add("user-agent", userAgent)
	req.Header.Add("cookie", cookie)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		panic("连接超时")
	}
	return res
}

func getHtmlFromUrl(url string) []byte {
	response := getReponseWithGlobalHeaders(url)

	reader := response.Body
	// 返回的内容被压缩成gzip格式了，需要解压一下
	if response.Header.Get("Content-Encoding") == "gzip" {
		reader, _ = gzip.NewReader(response.Body)
	}
	// 此时htmlContent还是gbk编码，需要转换成utf8编码
	htmlContent, _ := ioutil.ReadAll(reader)

	oldReader := bufio.NewReader(bytes.NewReader(htmlContent))
	peekBytes, _ := oldReader.Peek(1024)
	e, _, _ := charset.DetermineEncoding(peekBytes, "")
	utf8reader := transform.NewReader(oldReader, e.NewDecoder())
	// 此时htmlContent就已经是utf8编码了
	htmlContent, _ = ioutil.ReadAll(utf8reader)

	if err := response.Body.Close(); err != nil {
		fmt.Println("error happened when closing response body!", err)
	}
	return htmlContent
}

func getFirstImgUrl(uid string) string {
	firstPage := fmt.Sprintf("%s/%s", albumUrl, uid)
	html := getHtmlFromUrl(firstPage)

	firstImgUrl := imgPattern.FindAllStringSubmatch(string(html), len(html)+1)[0][1]
	return firstImgUrl
}

func saveAlbum(girlInfo GirlInfo) {
	// 以标题创建相册文件夹
	log.Printf("相册id:%s 开始", girlInfo.Uid)
	dir := saveDir + girlInfo.Title
	CreateDir(dir)
	firstImgUrl := getFirstImgUrl(girlInfo.Uid)
	albumBaseUrl := firstImgUrl[:35]
	i := 1
	for {
		albumDir := fmt.Sprintf("%s/%d.jpg", dir, i)
		imgUrl := fmt.Sprintf("%s%02d.jpg", albumBaseUrl, i)
		httpStatusCode := SaveFile(imgUrl, albumDir)
		if httpStatusCode == 404 {
			wg.Done()
			break
		} else if httpStatusCode == 200 {
			i++
		} else {
			time.Sleep(1 * time.Second)
		}
		time.Sleep(1 * time.Second)
	}

}

type GirlInfo struct {
	Uid          string
	Title        string
	FirstPageUrl string
}

func getGirlList(url string) {
	html := getHtmlFromUrl(url)
	girlMatch := uidPattern.FindAllStringSubmatch(string(html), len(html)+1)
	girlNumber := len(girlMatch)
	girlList := make([]GirlInfo, girlNumber)
	if girlNumber > 0 {
		for k, v := range girlMatch {
			girlList[k] = GirlInfo{
				Uid:          v[1],
				Title:        v[2],
				FirstPageUrl: girlUrl + v[1],
			}
		}
	}
	//开协程保存相册
	for _, v := range girlList {
		wg.Add(1)
		go saveAlbum(v)
	}
}
