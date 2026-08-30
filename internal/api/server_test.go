//nolint:errcheck,gochecknoglobals,noctx
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/saphka/link-shortener/internal/repository/link"
	"github.com/stretchr/testify/assert"
)

type inMemoryLinkRepository struct {
	lock      *sync.RWMutex
	idCounter int64
	data      map[string]link.ShortLink
}

// CreateLink implements [api.LinkRepository].
func (i *inMemoryLinkRepository) CreateLink(
	ctx context.Context,
	url string,
) (link.ShortLink, error) {
	i.lock.Lock()
	defer i.lock.Unlock()
	i.idCounter++
	id := i.idCounter
	result := link.ShortLink{
		Id:  id,
		Key: fmt.Sprintf("randomKkey%d", id),
		Url: url,
	}
	if _, found := i.data[result.Key]; found {
		return result, link.ErrLinkKeyConflict
	}
	i.data[result.Key] = result
	return result, nil
}

// GetLink implements [api.LinkRepository].
func (i *inMemoryLinkRepository) GetLink(ctx context.Context, key string) (link.ShortLink, error) {
	i.lock.RLock()
	defer i.lock.RUnlock()
	if result, found := i.data[key]; found {
		return result, nil
	} else {
		return link.ShortLink{}, link.ErrLinkNotFound
	}
}

var server *httptest.Server
var client *http.Client
var repo *inMemoryLinkRepository

func TestMain(m *testing.M) {
	repo = &inMemoryLinkRepository{
		lock: &sync.RWMutex{},
		data: make(map[string]link.ShortLink),
	}

	mux := http.NewServeMux()
	linkServer, err := NewServer(mux, repo)
	if err != nil {
		log.Fatalf("cannot create handler: %v", err)
	}

	server = httptest.NewServer(linkServer)
	defer server.Close()

	client = &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	exitCode := m.Run()
	os.Exit(exitCode)
}

func TestCreateLink(t *testing.T) {
	req, err := http.NewRequest(
		http.MethodPost,
		server.URL+"/link",
		strings.NewReader(`{"url":"http://example.com"}`),
	)
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var respBody struct {
		Key string `json:"key"`
		Url string `json:"url"`
	}
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	assert.NoError(t, err)
	assert.True(t, strings.Contains(respBody.Key, "randomKkey"))

	repo.lock.RLock()
	defer repo.lock.RUnlock()
	_, found := repo.data[respBody.Key]
	assert.True(t, found)
}

func TestCreateLinkConflict(t *testing.T) {
	repo.lock.RLock()
	nextId := repo.idCounter + 1
	repo.lock.RUnlock()

	nextKey := fmt.Sprintf("randomKkey%d", nextId+1)
	createLink(nextKey, "http://example4.com")

	req, err := http.NewRequest(
		http.MethodPost,
		server.URL+"/link",
		strings.NewReader(`{"url":"http://example5.com"}`),
	)
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	repo.lock.RLock()
	defer repo.lock.RUnlock()
	val, found := repo.data[nextKey]
	assert.True(t, found)
	assert.Equal(t, "http://example4.com", val.Url)
}

func TestReadLink(t *testing.T) {
	createLink("testLink01", "http://example2.com")

	req, err := http.NewRequest(http.MethodGet, server.URL+"/l/testLink01", strings.NewReader(""))
	assert.NoError(t, err)

	resp, err := client.Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusFound, resp.StatusCode)
	assert.Equal(t, "http://example2.com", resp.Header.Get("Location"))
}

func TestErrorReadLink(t *testing.T) {
	createLink("testLink02", "http://example3.com")

	req, err := http.NewRequest(http.MethodGet, server.URL+"/l/tesOther01", strings.NewReader(""))
	assert.NoError(t, err)

	resp, err := client.Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
}

func TestBadRequest(t *testing.T) {
	req, err := http.NewRequest(
		http.MethodPost,
		server.URL+"/link",
		strings.NewReader(`{"url":null}`),
	)
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var respBody ErrorResponse
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	assert.NoError(t, err)
	assert.Contains(t, respBody.Message, "request body has an error")
}

func TestBadUrl(t *testing.T) {
	req, err := http.NewRequest(
		http.MethodPost,
		server.URL+"/link",
		strings.NewReader(`{"url":":iAmNotAUrl"}`),
	)
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var respBody ErrorResponse
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	assert.NoError(t, err)
	assert.Contains(t, respBody.Message, "cannot parse url")
}

func TestBadUrlScheme(t *testing.T) {
	req, err := http.NewRequest(
		http.MethodPost,
		server.URL+"/link",
		strings.NewReader(`{"url":"fille:///etc/hosts"}`),
	)
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var respBody ErrorResponse
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	assert.NoError(t, err)
	assert.Contains(t, respBody.Message, "unsupported url scheme")
}

func TestBadHost(t *testing.T) {
	req, err := http.NewRequest(
		http.MethodPost,
		server.URL+"/link",
		strings.NewReader(`{"url":"http://"}`),
	)
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var respBody ErrorResponse
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	assert.NoError(t, err)
	assert.Contains(t, respBody.Message, "link has no host")
}

func createLink(key, url string) {
	repo.lock.Lock()
	defer repo.lock.Unlock()
	repo.idCounter++
	id := repo.idCounter
	result := link.ShortLink{
		Id:  id,
		Key: key,
		Url: url,
	}
	repo.data[key] = result
}
