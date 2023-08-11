package scrapper

import (
	"encoding/csv"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type extractedJob struct {
	id        string
	title     string
	condition string
}

func Scrape(term string) {
	var baseURL string = "https://www.saramin.co.kr/zf_user/search/recruit?&searchword=" + term

	var jobs []extractedJob
	c := make(chan []extractedJob)
	totalPages, countsPerPage := getPages(baseURL)

	for i := 0; i < totalPages; i++ {
		go getPage(baseURL, i, countsPerPage, c)
	}

	for i := 0; i < totalPages; i++ {
		extractedJobs := <-c
		jobs = append(jobs, extractedJobs...)
	}

	writeJobs(jobs)
	fmt.Println("Done, extracted : ", len(jobs))
}

func getPage(baseURL string, page int, countsPerPage int, mainC chan<- []extractedJob) {
	var jobs []extractedJob
	c := make(chan extractedJob)

	pageURL := baseURL + "&recruitPage=" + strconv.Itoa(page+1) + "&recruitSort=relation&recruitPageCount=" + strconv.Itoa(countsPerPage) + "&inner_com_type=&company_cd=0%2C1%2C2%2C3%2C4%2C5%2C6%2C7%2C9%2C10&show_applied=&quick_apply=&except_read=&ai_head_hunting=&mainSearch=n"
	fmt.Println("Requesting URL:", pageURL)

	res, err := http.Get(pageURL)
	checkErr(err)
	checkCode(res)

	defer res.Body.Close()

	doc, err := goquery.NewDocumentFromReader(res.Body)
	checkErr(err)

	searchCards := doc.Find(".item_recruit")
	searchCards.Each(func(i int, card *goquery.Selection) {
		go extractJob(card, c)
	})

	for i := 0; i < searchCards.Length(); i++ {
		job := <-c
		jobs = append(jobs, job)
	}

	mainC <- jobs
}

func extractJob(card *goquery.Selection, c chan<- extractedJob) {
	id, _ := card.Attr("value")
	title := CleanString(card.Find(".job_tit>a").Text())
	condition := CleanString(card.Find(".job_condition").Text())
	c <- extractedJob{
		id:        id,
		title:     title,
		condition: condition,
	}
}

func getPages(baseURL string) (int, int) {
	pages := 0
	countsPerPage := 0

	res, err := http.Get(baseURL)
	checkErr(err)
	checkCode(res)

	defer res.Body.Close()

	doc, err := goquery.NewDocumentFromReader(res.Body)
	checkErr(err)

	re := regexp.MustCompile("\\d+")

	// 전체 개수를 구한다.
	totalCountText := doc.Find(".cnt_result").Text()
	totalCountString := strings.Join(re.FindAllString(totalCountText, -1), "")

	// 목록에 나타나는 데이터의 개수를 구한다.
	countPerPageText := doc.Find(".wrap_result_filter").Children().Last().Find(".btn_filter").Text()
	countPerPageString := strings.Join(re.FindAllString(countPerPageText, -1), "")

	totalCountFloat, _ := strconv.ParseFloat(totalCountString, 64)
	countPerPageFloat, _ := strconv.ParseFloat(countPerPageString, 64)

	pages = int(math.Ceil(totalCountFloat / countPerPageFloat))
	countsPerPage = int(countPerPageFloat)

	return pages, countsPerPage
}

func checkErr(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}

func checkCode(res *http.Response) {
	if res.StatusCode != 200 {
		log.Fatalln("Request failed with Status: ", res.StatusCode)
	}
}

func CleanString(str string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(str)), " ")
}

func writeJobs(jobs []extractedJob) {
	file, err := os.Create("jobs.csv")
	checkErr(err)
	utf8bom := []byte{0xEF, 0xBB, 0xBF}
	file.Write(utf8bom)

	w := csv.NewWriter(file)
	defer w.Flush()
	defer file.Close()

	headers := []string{"url", "title", "condition"}

	wErr := w.Write(headers)
	checkErr(wErr)

	for _, job := range jobs {
		applyBaseUrl := "https://www.saramin.co.kr/zf_user/jobs/relay/view?rec_idx="
		jobSlice := []string{applyBaseUrl + job.id, job.title, job.condition}
		jwErr := w.Write(jobSlice)
		checkErr(jwErr)
	}
}
