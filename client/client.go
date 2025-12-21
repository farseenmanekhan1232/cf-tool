package client

import (
	"encoding/json"
	http "github.com/bogdanfinn/fhttp"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings" // Added strings import

	"github.com/fatih/color"
	"github.com/xalanq/cf-tool/cookiejar"
	"github.com/xalanq/cf-tool/util"

	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

// Client codeforces client
type Client struct {
	Jar            *cookiejar.Jar `json:"cookies"`
	Handle         string         `json:"handle"`
	HandleOrEmail  string         `json:"handle_or_email"`
	Password       string         `json:"password"`
	Ftaa           string         `json:"ftaa"`
	Bfaa           string         `json:"bfaa"`
	LastSubmission *Info          `json:"last_submission"`
	host           string
	proxy          string
	path           string
	client         util.HttpClient
	UserAgent      string `json:"user_agent"`
}

// Instance global client
var Instance *Client

type TlsClientWrapper struct {
	client tls_client.HttpClient
	c      *Client
}

func (w *TlsClientWrapper) Do(req *http.Request) (*http.Response, error) {
	if w.c.UserAgent != "" {
		req.Header.Set("User-Agent", w.c.UserAgent)
	}
	return w.client.Do(req)
}

func (w *TlsClientWrapper) Get(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	return w.Do(req)
}

func (w *TlsClientWrapper) PostForm(url string, data url.Values) (*http.Response, error) {
	req, err := http.NewRequest("POST", url, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return w.client.Do(req)
}

func (w *TlsClientWrapper) SetCookieJar(jar http.CookieJar) {
	w.client.SetCookieJar(jar)
}

// Init initialize
func Init(path, host, proxy string) {
	jar, _ := cookiejar.New(nil)
	c := &Client{Jar: jar, LastSubmission: nil, path: path, host: host, proxy: proxy, client: nil}
	if err := c.load(); err != nil {
		color.Red(err.Error())
		color.Green("Create a new session in %v", path)
	}

	options := []tls_client.HttpClientOption{
		tls_client.WithTimeoutSeconds(30),
		tls_client.WithClientProfile(profiles.Chrome_133),
		tls_client.WithNotFollowRedirects(),
		tls_client.WithCookieJar(c.Jar), // Use shared cookie jar
	}

	if len(proxy) > 0 {
		options = append(options, tls_client.WithProxyUrl(proxy))
	}

	client, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), options...)
	if err != nil {
		color.Red(err.Error())
		return
	}

	c.client = &TlsClientWrapper{client: client, c: c}
	if err := c.save(); err != nil {
		color.Red(err.Error())
	}
	Instance = c
}

// load from path
func (c *Client) load() (err error) {
	file, err := os.Open(c.path)
	if err != nil {
		return
	}
	defer file.Close()

	bytes, err := ioutil.ReadAll(file)

	if err != nil {
		return err
	}

	return json.Unmarshal(bytes, c)
}

// save file to path
func (c *Client) save() (err error) {
	data, err := json.MarshalIndent(c, "", "  ")
	if err == nil {
		os.MkdirAll(filepath.Dir(c.path), os.ModePerm)
		err = ioutil.WriteFile(c.path, data, 0644)
	}
	if err != nil {
		color.Red("Cannot save session to %v\n%v", c.path, err.Error())
	}
	return
}
