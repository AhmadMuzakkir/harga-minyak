package api

import (
	"github.com/getsentry/raven-go"
	"github.com/ahmadmuzakkir/harga-minyak/model"
	"github.com/ahmadmuzakkir/harga-minyak/provider"
	"net/http"
	"sync"
	"time"
	"github.com/go-chi/chi"
	"github.com/go-chi/render"
)

type HargaHandler struct {
	provider *provider.MySumberClient

	cache *Cache
}

func NewHandler(p *provider.MySumberClient) *HargaHandler {
	h := &HargaHandler{
		provider: p,
		cache:    &Cache{duration: 4 * time.Hour},
	}
	return h
}

type Cache struct {
	duration time.Duration

	mu     sync.RWMutex
	update time.Time
	data   *model.Data
}

func (c *Cache) Get() *model.Data {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if time.Now().Sub(c.update) > c.duration {
		return nil
	}

	data := c.data
	return data
}

func (c *Cache) Set(data *model.Data) {
	c.mu.Lock()
	c.update = time.Now()
	c.data = data
	c.mu.Unlock()
}

func (c *Cache) Clear() {
	c.mu.Lock()
	c.update = time.Now()
	c.data = nil
	c.mu.Unlock()
}

func (h *HargaHandler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Get("/getLatest", h.GetLatest)
	r.Get("/getAll", h.GetAll)
	return r
}

func (h *HargaHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	var err error
	sc, err := h.GetData()

	if sc == nil && err != nil {
		raven.CaptureError(err, map[string]string{"module": "API", "API": "GetAll"})
		render.Render(w, r, model.ErrServer(err))
		return
	}

	v := sc.All
	render.Render(w, r, v)
}

func (h *HargaHandler) GetLatest(w http.ResponseWriter, r *http.Request) {
	var err error
	sc, err := h.GetData()

	if sc == nil && err != nil {
		raven.CaptureError(err, map[string]string{"module": "API", "API": "GetLatest"})
		render.Render(w, r, model.ErrServer(err))
		return
	}

	v := sc.Latest
	render.Render(w, r, v)
}

func (h *HargaHandler) RefreshCache() error {
	h.cache.Clear()

	sc, err := h.provider.Scrape()
	if err != nil {
		raven.CaptureError(err, map[string]string{"module": "API", "API": "RefreshCache"})
	}

	h.cache.Set(sc)

	return err
}

func (h *HargaHandler) GetData() (*model.Data, error) {
	var err error

	data := h.cache.Get()

	if data != nil {
		return data, nil
	}

	err = h.RefreshCache()

	return h.cache.Get(), err
}
