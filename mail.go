package helper

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/bincooo/emit.io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

func randomMail(context context.Context, proxies string, jar http.CookieJar) (mail string, err error) {
	response, err := emit.ClientBuilder().
		Context(context).
		Proxies(proxies).
		CookieJar(jar).
		GET("https://www.guerrillamail.com/inbox").
		DoS(http.StatusOK)
	if err != nil {
		return
	}

	text := emit.TextResponse(response)
	{ // 获取mail
		c := regexp.MustCompile(`Email: ([a-zA-Z]+@sharklasers.com)`)
		values := c.FindStringSubmatch(text)
		if len(values) < 2 {
			return "", errors.New("fetch mail failed")
		}
		mail = values[1]
	}
	{ // 获取token
		c := regexp.MustCompile(`api_token : '([a-zA-Z0-9]+)',`)
		values := c.FindStringSubmatch(text)
		if len(values) < 2 {
			return "", errors.New("fetch mail failed")
		}
		//token = values[1]
	}
	{ // req
		c := regexp.MustCompile(`"mail_id":([0-9]+)`)
		values := c.FindStringSubmatch(text)
		if len(values) < 2 {
			return "", errors.New("fetch mail failed")
		}
		//seq = values[1]
	}
	return
}

func fetchMailMessage(context context.Context, proxies, mail string, jar http.CookieJar) (string, error) {
	values := strings.Split(mail, "@")
	if len(values) < 2 {
		return "", errors.New("mail is error")
	}

	retry := 20
	waiting := 3 * time.Second
label:
	retry--
	response, err := emit.ClientBuilder().
		Context(context).
		Proxies(proxies).
		CookieJar(jar).
		GET("https://www.guerrillamail.com/inbox").
		DoS(http.StatusOK)
	if err != nil {
		return "", err
	}

	text := emit.TextResponse(response)
	c := regexp.MustCompile(`result: \{.+},`)
	values = c.FindStringSubmatch(text)
	if len(values) < 1 {
		if retry > 0 {
			time.Sleep(waiting)
			goto label
		}
		return "", errors.New("fetch code failed")
	}

	text = strings.TrimPrefix(values[0], "result: ")
	text = strings.TrimSuffix(text, ",")
	var obj map[string]interface{}
	if err = json.Unmarshal([]byte(text), &obj); err != nil {
		return "", err
	}

	if count, ok := obj["count"]; !ok || count == "0" {
		if retry > 0 {
			time.Sleep(waiting)
			goto label
		}
		return "", errors.New("fetch code failed")
	}

	list := obj["list"].([]interface{})
	for _, value := range list {
		subject, ok := value.(map[string]interface{})
		if !ok {
			continue
		}

		if subject["mail_from"] != "login@you.com" {
			continue
		}

		text, ok = subject["mail_excerpt"].(string)
		if !ok {
			return "", errors.New("fetch code failed")
		}

		{
			c = regexp.MustCompile("[0-9]{6}")
			values = c.FindStringSubmatch(text)
			if len(values) == 0 {
				return "", errors.New("fetch code failed")
			}
			return values[0], nil
		}
	}

	if retry > 0 {
		time.Sleep(5 * time.Second)
		goto label
	}
	return "", errors.New("fetch code failed")
}
