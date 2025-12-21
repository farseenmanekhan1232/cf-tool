package client

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net/url"
	"regexp"
	"strings"
	"time"

	http "github.com/bogdanfinn/fhttp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/fatih/color"
	"github.com/xalanq/cf-tool/cookiejar"
	"github.com/xalanq/cf-tool/util"
)

// genFtaa generate a random one
func genFtaa() string {
	return util.RandString(18)
}

// genBfaa generate a bfaa
func genBfaa() string {
	return "f1b3f18c715565b589b7823cda7448ce"
}

// ErrorNotLogged not logged in
var ErrorNotLogged = "Not logged in"

// findHandle if logged return (handle, nil), else return ("", ErrorNotLogged)
func findHandle(body []byte) (string, error) {
	reg := regexp.MustCompile(`handle = "([\s\S]+?)"`)
	tmp := reg.FindSubmatch(body)
	if len(tmp) < 2 {
		return "", errors.New(ErrorNotLogged)
	}
	return string(tmp[1]), nil
}

func findCsrf(body []byte) (string, error) {
	reg := regexp.MustCompile(`csrf='(.+?)'`)
	tmp := reg.FindSubmatch(body)
	if len(tmp) < 2 {
		return "", errors.New("Cannot find csrf")
	}
	return string(tmp[1]), nil
}

// Login codeforces with handler and password
func (c *Client) Login() (err error) {
	color.Cyan("Login %v...\n", c.HandleOrEmail)

	password, err := c.DecryptPassword()
	if err != nil {
		return
	}

	jar, _ := cookiejar.New(nil)
	c.client.SetCookieJar(jar)
	body, err := util.GetBody(c.client, c.host+"/enter")
	if err != nil {
		return
	}

	csrf, err := findCsrf(body)
	if err != nil {
		return
	}

	ftaa := genFtaa()
	bfaa := genBfaa()

	body, err = util.PostBody(c.client, c.host+"/enter", url.Values{
		"csrf_token":    {csrf},
		"action":        {"enter"},
		"ftaa":          {ftaa},
		"bfaa":          {bfaa},
		"handleOrEmail": {c.HandleOrEmail},
		"password":      {password},
		"_tta":          {"176"},
		"remember":      {"on"},
	})
	if err != nil {
		return
	}

	handle, err := findHandle(body)
	if err != nil {
		return
	}

	c.Ftaa = ftaa
	c.Bfaa = bfaa
	c.Handle = handle
	c.Jar = jar
	color.Green("Succeed!!")
	color.Green("Welcome %v~", handle)
	return c.save()
}

func createHash(key string) []byte {
	hasher := md5.New()
	hasher.Write([]byte(key))
	return hasher.Sum(nil)
}

func encrypt(handle, password string) (ret string, err error) {
	block, err := aes.NewCipher(createHash("glhf" + handle + "233"))
	if err != nil {
		return
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return
	}
	text := gcm.Seal(nonce, nonce, []byte(password), nil)
	ret = hex.EncodeToString(text)
	return
}

func decrypt(handle, password string) (ret string, err error) {
	data, err := hex.DecodeString(password)
	if err != nil {
		err = errors.New("Cannot decode the password")
		return
	}
	block, err := aes.NewCipher(createHash("glhf" + handle + "233"))
	if err != nil {
		return
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return
	}
	nonceSize := gcm.NonceSize()
	nonce, text := data[:nonceSize], data[nonceSize:]
	plain, err := gcm.Open(nil, nonce, text, nil)
	if err != nil {
		return
	}
	ret = string(plain)
	return
}

// DecryptPassword get real password
func (c *Client) DecryptPassword() (string, error) {
	if len(c.Password) == 0 || len(c.HandleOrEmail) == 0 {
		return "", errors.New("You have to configure your handle and password by `cf config`")
	}
	return decrypt(c.HandleOrEmail, c.Password)
}

// ConfigLogin configure login via browser
func (c *Client) ConfigLogin() (err error) {
	if c.Handle != "" {
		color.Green("Current user: %v", c.Handle)
	}
	return c.LoginBrowser()
}

// LoginBrowser opens Chrome browser for user to login

func (c *Client) LoginBrowser() (err error) {
	color.Cyan("Opening browser for Codeforces login...")
	color.Cyan("Please log in to Codeforces in the browser window.")
	color.Cyan("The window will close automatically after login is detected.")

	// Create a new browser context with visible window and stealth options
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false),
		chromedp.Flag("disable-gpu", false),
		chromedp.Flag("start-maximized", true),
		// Stealth options to avoid automation detection
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("exclude-switches", "enable-automation"),
		chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// Quick test to see if Chrome is available
	if err := chromedp.Run(ctx); err != nil {
		color.Red("Could not launch Chrome/Chromium browser.")
		color.Yellow("Please install one of the following:")
		color.Yellow("  • Google Chrome: https://www.google.com/chrome/")
		color.Yellow("  • Chromium (lightweight): https://www.chromium.org/getting-involved/download-chromium/")
		return err
	}

	// Set a timeout for the entire operation
	ctx, cancel = context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	var cookies []*network.Cookie

	// Navigate to login page and wait for successful login
	err = chromedp.Run(ctx,
		chromedp.Navigate(c.host+"/enter"),
		// Wait for successful login by checking for handle in page
		chromedp.ActionFunc(func(ctx context.Context) error {
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
					// Check if logged in by looking for handle element
					var handle string
					err := chromedp.Evaluate(`
						(function() {
							var el = document.querySelector('a[href^="/profile/"]');
							if (el) return el.textContent.trim();
							return '';
						})()
					`, &handle).Do(ctx)
					if err == nil && handle != "" {
						color.Green("Detected login as: %v", handle)
						c.Handle = handle
						return nil
					}
					time.Sleep(500 * time.Millisecond)
				}
			}
		}),
		// Extract all cookies
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			cookies, err = network.GetCookies().Do(ctx)
			return err
		}),
	)

	if err != nil {
		color.Red("Browser login failed: %v", err)
		return err
	}

	// Convert cookies to jar format
	jar, _ := cookiejar.New(nil)
	u, _ := url.Parse(c.host)
	httpCookies := []*http.Cookie{}

	for _, cookie := range cookies {
		if strings.Contains(c.host, cookie.Domain) || cookie.Domain == ".codeforces.com" || cookie.Domain == "codeforces.com" {
			httpCookies = append(httpCookies, &http.Cookie{
				Name:   cookie.Name,
				Value:  cookie.Value,
				Domain: cookie.Domain,
				Path:   cookie.Path,
			})
		}
	}

	jar.SetCookies(u, httpCookies)
	c.client.SetCookieJar(jar)
	c.Jar = jar

	color.Green("Succeed!!")
	color.Green("Welcome %v~", c.Handle)
	return c.save()
}
