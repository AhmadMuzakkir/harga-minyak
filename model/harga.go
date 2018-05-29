package model

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"
)

type Data struct {
	All    *WeekList
	Latest *Latest
}

type WeekList struct {
	LastUpdate *time.Time `json:"last_update,omitempty"`
	List       []*Week    `json:"list,omitempty"`
}

type Latest struct {
	LastUpdate *time.Time `json:"last_update,omitempty"`
	Week       *Week      `json:"latest,omitempty"`
}

func (s *Latest) Render(rw http.ResponseWriter, r *http.Request) error {
	return nil
}

func (s *Latest) MarshalJSON() ([]byte, error) {
	type Alias Week
	return json.Marshal(&struct {
		LastUpdate string `json:"last_update,omitempty"`
		*Alias
	}{
		LastUpdate: s.LastUpdate.Format("2006-01-02T15:04:05-0700"),
		Alias:      (*Alias)(s.Week),
	})
}

type Week struct {
	StartDate   *time.Time `json:"start_date,omitempty"`
	EndDate     *time.Time `json:"end_date,omitempty"`
	PriceRon95  *Price     `json:"ron95,omitempty"`
	PriceRon97  *Price     `json:"ron97,omitempty"`
	PriceDiesel *Price     `json:"diesel,omitempty"`
}

func (s *WeekList) MarshalJSON() ([]byte, error) {
	type Alias WeekList
	return json.Marshal(&struct {
		LastUpdate string `json:"last_update,omitempty"`
		*Alias
	}{
		LastUpdate: s.LastUpdate.Format("2006-01-02T15:04:05-0700"),
		Alias:      (*Alias)(s),
	})
}

func (w *Week) MarshalJSON() ([]byte, error) {
	type Alias Week

	temp := &struct {
		StartDate string `json:"start_date,omitempty"`
		EndDate   string `json:"end_date,omitempty"`
		*Alias
	}{
		Alias: (*Alias)(w),
	}

	if w.StartDate != nil {
		temp.StartDate = w.StartDate.Format("2006-01-02T15:04:05-0700")
	}

	if w.EndDate != nil {
		temp.EndDate = w.EndDate.Format("2006-01-02T15:04:05-0700")
	}

	return json.Marshal(temp)
}

func (s *WeekList) Render(rw http.ResponseWriter, r *http.Request) error {
	//rw.WriteHeader(http.StatusNotModified)
	return nil
}

func (w *Week) Render(rw http.ResponseWriter, r *http.Request) error {
	return nil
}

func (w *Week) HasPrice() bool {
	return w.PriceRon95.HasPrice() && w.PriceRon97.HasPrice() && w.PriceDiesel.HasPrice()
}

func (w *Week) SetDiff(w2 *Week) {
	var err error
	err = w.PriceRon95.setDiff(w2.PriceRon95)
	if err != nil {
		log.Println(err)
	}

	err = w.PriceRon97.setDiff(w2.PriceRon97)
	if err != nil {
		log.Println(err)
	}

	err = w.PriceDiesel.setDiff(w2.PriceDiesel)
	if err != nil {
		log.Println(err)
	}
}

type Price struct {
	Price string `json:"price,omitempty"`
	price float64
	Diff  string `json:"diff,omitempty"`
}

func NewPrice(priceStr string) *Price {
	price, _ := strconv.ParseFloat(priceStr, 32)
	return &Price{
		Price: priceStr,
		price: price,
	}
}

func (p *Price) String() string {
	return p.Price + " " + p.Diff
}

func (p *Price) HasPrice() bool {
	return p.price != 0
}

func (p *Price) setDiff(p2 *Price) error {
	if !p.HasPrice() || !p2.HasPrice() {
		return nil
	}

	p.Diff = strconv.FormatFloat(p.price-p2.price, 'f', 2, 32)
	return nil
}
