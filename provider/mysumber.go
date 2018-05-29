package provider

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/ahmadmuzakkir/harga-minyak/model"
	"github.com/pkg/errors"
	"log"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

type MySumberClient struct {
	httpClient *http.Client
}

func NewClient() *MySumberClient {
	var netTransport = &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 30 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 30 * time.Second,
	}
	hc := &http.Client{
		Transport: netTransport,
		Timeout:   time.Second * 30,
	}

	return &MySumberClient{httpClient: hc}
}

func (c *MySumberClient) Scrape() (*model.Data, error) {
	log.Println("Scraping...")
	startScrape := time.Now()

	var err error
	req, err := http.NewRequest("GET", "https://www.mysumber.com/minyak.html", nil)
	if err != nil {
		return nil, errors.Wrap(err, "Error http.NewRequest()")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "Error http.Do()")
	}

	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		return nil, errors.Wrap(err, "Error NewDocumentFromResponse")
	}

	result := &model.Data{}

	all, err := scrapeWeekList(doc)
	if err != nil {
		return nil, errors.Wrap(err, "Error scrape week list")
	}

	latest, err := scrapeLatest(doc)
	if err != nil {
		return nil, errors.Wrap(err, "Error scrape latest")
	}

	log.Printf("Finished scraping in %s \n", time.Now().Sub(startScrape).Round(time.Millisecond))

	now := time.Now()
	result.All = &model.WeekList{LastUpdate: &now, List: all}

	result.Latest = &model.Latest{LastUpdate: &now, Week: latest}

	return result, nil
}

func scrapeWeekList(doc *goquery.Document) ([]*model.Week, error) {
	var weeklyPrices []*model.Week
	var err error

	doc.Find("table[border='1'][width='100%']:contains('Harga Minyak Mingguan')").
		Each(func(i int, s *goquery.Selection) {
		s.Find("thead").Find("tr").
			Each(func(i int, s *goquery.Selection) {
			if i >= 2 {
				var week *model.Week
				week, err = parseWeekListColumn(s)
				if err != nil {
					log.Println(err)
				}
				if week != nil && week.HasPrice() {
					weeklyPrices = append(weeklyPrices, week)
				}
			}
		})
	})

	// Sort the list ascendingly based on start date
	sort.Slice(weeklyPrices, func(i, j int) bool {
		return weeklyPrices[i].StartDate.Before(*weeklyPrices[j].StartDate)
	})

	oneDayDuration := 24 * time.Hour

	// Set the price differences. This assumes the list is sorted ascendingly based on the date.
	for i, week := range weeklyPrices {
		if i > 0 {
			// Make sure the weeks are consecutive before setting the diff, in case there's gap in the list.
			if week.StartDate.Sub(*weeklyPrices[i-1].EndDate) <= oneDayDuration {
				week.SetDiff(weeklyPrices[i-1])
			}
		}
	}

	if err != nil {
		return nil, err
	}
	return weeklyPrices, nil

}

func scrapeLatest(doc *goquery.Document) (*model.Week, error) {
	ron95 := &model.Price{}
	ron97 := &model.Price{}
	diesel := &model.Price{}

	doc.Find("table[border='1'][width='100%']:contains('Perbandingan')").
		Find("thead").
		Find("tr:nth-child(2)").
		Find("td").
		Find("table[border='1'][width='100%']").
		Find("thead").
		Find("tr").
		Each(func(i int, s *goquery.Selection) {

		if i < 3 {
			// Ron 95
			if i == 0 {
				ron95 = model.NewPrice(s.Text())
			} else if i == 1 {
				diff := strings.TrimSpace(strings.Replace(s.Text(), "sen", "", 1))

				var decrease bool = false
				s.Find("img[title='Minyak Turun Harga']").Each(func(i int, s *goquery.Selection) {
					decrease = true
				})

				if decrease {
					ron95.Diff = "-" + diff
				} else {
					ron95.Diff = diff
				}
			}
		} else if i < 6 {
			// Ron 97
			if i == 3 {
				ron97 = model.NewPrice(s.Text())
			} else if i == 4 {
				diff := strings.TrimSpace(strings.Replace(s.Text(), "sen", "", 1))

				var decrease bool = false
				s.Find("img[title='Minyak Turun Harga']").Each(func(i int, s *goquery.Selection) {
					decrease = true
				})

				if decrease {
					ron97.Diff = "-" + diff
				} else {
					ron97.Diff = diff
				}
			}

		} else if i < 9 {
			// Diesel
			if i == 6 {
				diesel = model.NewPrice(s.Text())
			} else if i == 7 {
				diff := strings.TrimSpace(strings.Replace(s.Text(), "sen", "", 1))

				var decrease bool = false
				s.Find("img[title='Minyak Turun Harga']").Each(func(i int, s *goquery.Selection) {
					decrease = true
				})

				if decrease {
					diesel.Diff = "-" + diff
				} else {
					diesel.Diff = diff
				}
			}
		}

	})

	week := &model.Week{}
	week.PriceRon95 = ron95
	week.PriceRon97 = ron97
	week.PriceDiesel = diesel
	return week, nil
}

func parseWeekListColumn(s *goquery.Selection) (*model.Week, error) {
	var week *model.Week
	var err error

	s.Find("td").Each(func(i int, s *goquery.Selection) {

		if i == 0 {
			week, err = parseWeekList(s.Text())
		} else if i == 1 {
			if week != nil {
				week.PriceRon95 = model.NewPrice(s.Text())
			}
		} else if i == 2 {
			if week != nil {
				week.PriceRon97 = model.NewPrice(s.Text())
			}
		} else if i == 3 {
			if week != nil {
				week.PriceDiesel = model.NewPrice(s.Text())
			}
		}
	})
	if err != nil {
		return nil, err
	}
	return week, nil
}

/*
Sample raw: "30 Mac – 5 Apr", "6 – 12 April"
*/
func parseWeekList(raw string) (*model.Week, error) {
	// Split into two parts, the start date and the end date

	splits := strings.Split(raw, "–")
	// Sometimes they use different separator.
	if len(splits) == 1 {
		splits = strings.Split(raw, "-")
	}

	if splits == nil || len(splits) < 1 {
		return nil, errors.New(fmt.Sprintf("Could not parse the date from '%v'", raw))
	}

	var err error

	startRaw := strings.TrimSpace(splits[0])
	// The day and the month is separated by a whitespace.
	start := strings.Fields(startRaw)

	var startDay int
	var startMonth time.Month

	/*
	 Sometimes the start date does not have a month. e.g. "6 – 12 April"
	 If so, we will get the month from the end date later.
	*/
	if len(start) == 1 {
		//log.Println("start: ", start[0])
		startDay, err = strconv.Atoi(start[0])
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Could not parse the start day from '%v'", start[0]))
		}
	} else if len(start) == 2 {
		//log.Println("start: ", start[0])
		startDay, err = strconv.Atoi(start[0])
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Could not parse the start day from '%v'", start[0]))
		}

		startMonth, err = getMonth(start[1])
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Could not parse the start month from '%v'", start[1]))
		}
	} else {
		return nil, errors.New(fmt.Sprintf("Could not parse the start date from '%v'", startRaw))
	}

	endRaw := strings.TrimSpace(splits[1])
	// The day and the month is separated by a whitespace.
	end := strings.Fields(endRaw)

	var endDay int
	var endMonth time.Month
	if len(end) == 2 {
		endDay, err = strconv.Atoi(end[0])
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Could not parse the end day from '%v'", end[0]))
		}

		endMonth, err = getMonth(end[1])
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Could not parse the end month from '%v'", end[1]))
		}
	} else {
		return nil, errors.New(fmt.Sprintf("Could not parse the end date from '%v'", endRaw))
	}

	// If start month is not set, we assume start and end has the same month
	if startMonth == 0 {
		startMonth = endMonth
	}

	now := time.Now()

	startDate := time.Date(now.Year(), startMonth, startDay, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(now.Year(), endMonth, endDay, 0, 0, 0, 0, time.UTC)

	week := &model.Week{}
	week.StartDate = &startDate
	week.EndDate = &endDate

	return week, nil
}

func getMonth(s string) (time.Month, error) {
	switch s {
	case "Feb":
		return time.February, nil
	case "Jan":
		return time.January, nil
	case "Mar":
		fallthrough
	case "Mac":
		return time.March, nil
	case "Apr":
		fallthrough
	case "April":
		return time.April, nil
	case "Mei":
		return time.May, nil
	case "Jun":
		return time.June, nil
	case "Jul":
		fallthrough
	case "Julai":
		return time.July, nil
	case "Ogs":
		fallthrough
	case "Ogos":
		return time.August, nil
	case "Sep":
		fallthrough
	case "Sept":
		fallthrough
	case "September":
		return time.September, nil
	case "Okt":
		fallthrough
	case "Oktober":
		return time.October, nil
	case "Nov":
		fallthrough
	case "November":
		return time.November, nil
	case "Dis":
		fallthrough
	case "Dec":
		fallthrough
	case "Disember":
		return time.December, nil
	}
	return -1, errors.New(fmt.Sprintf("Could not parse the month from '%v'", s))
}
