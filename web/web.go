package web

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

const (
	browser = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/87.0.4280.60 Safari/537.36"
)

var (
	c = http.Client{
		Timeout: time.Second * 10,
	}
)

// GetBody fetches and returns the body of a page at URL `url`.
func GetBody(url string) ([]byte, error) {
	res, err := getResponse(http.MethodGet, url, "")
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return body, fmt.Errorf("Non-2xx response code: %d. Response: %s", res.StatusCode, body)
	}
	return body, nil
}

// GetJSON fetches a URL and returns the JSON as a string-to-interface map.
func GetJSON(url string) (map[string]interface{}, error) {
	b, err := GetBody(url)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	err = json.Unmarshal(b, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func getResponse(httpMethod string, url string, referrer string) (*http.Response, error) {
	req, err := http.NewRequest(httpMethod, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", browser)
	if referrer != "" {
		req.Header.Set("Referer", referrer)
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
	}
	res, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	return res, nil
}
