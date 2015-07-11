package orbit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"strings"

	"github.com/google/go-querystring/query"
	"github.com/hackedu/orbit/router"
)

type Client struct {
	BaseURL   *url.URL
	UserAgent string

	Projects ProjectsService
	Services ServicesService

	httpClient *http.Client
}

const (
	libraryVersion = "0.0.1"
	userAgent      = "orbit-client/" + libraryVersion
)

func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	c := &Client{
		BaseURL:    &url.URL{Scheme: "http", Host: "mew.hackedu.us:4000", Path: "/api/"},
		UserAgent:  userAgent,
		httpClient: httpClient,
	}
	c.Projects = &projectsService{c}
	c.Services = &servicesService{c}
	return c
}

var apiRouter = router.API()

func (c *Client) url(apiRouteName string, routeVars map[string]string, opt interface{}) (*url.URL, error) {
	route := apiRouter.Get(apiRouteName)
	if route == nil {
		return nil, fmt.Errorf("no API route named %q", apiRouteName)
	}

	routeVarsList := make([]string, 2*len(routeVars))
	i := 0
	for name, val := range routeVars {
		routeVarsList[i*2] = name
		routeVarsList[i*2+1] = val
	}
	url, err := route.URL(routeVarsList...)
	if err != nil {
		return nil, err
	}

	// make the route URL path relative to BaseURL by trimming the leading "/"
	url.Path = strings.TrimPrefix(url.Path, "/")

	if opt != nil {
		err = addOptions(url, opt)
		if err != nil {
			return nil, err
		}
	}

	return url, err
}

func (c *Client) NewRequest(method, urlStr string, body interface{}) (*http.Request, error) {
	rel, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}

	u := c.BaseURL.ResolveReference(rel)

	buf := new(bytes.Buffer)
	if body != nil {
		err := json.NewEncoder(buf).Encode(body)
		if err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequest(method, u.String(), buf)
	if err != nil {
		return nil, err
	}

	req.Header.Add("User-Agent", c.UserAgent)
	return req, nil
}

func (c *Client) Do(req *http.Request, v interface{}) (*http.Response, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := CheckResponse(resp); err != nil {
		return resp, err
	}

	if v != nil {
		if bp, ok := v.(*[]byte); ok {
			*bp, err = ioutil.ReadAll(resp.Body)
		} else {
			err = json.NewDecoder(resp.Body).Decode(v)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("error reading response from %s %s: %s", req.Method, req.URL.RequestURI(), err)
	}
	return resp, nil
}

func addOptions(u *url.URL, opt interface{}) error {
	v := reflect.ValueOf(opt)
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return nil
	}

	qs, err := query.Values(opt)
	if err != nil {
		return err
	}

	u.RawQuery = qs.Encode()
	return nil
}
