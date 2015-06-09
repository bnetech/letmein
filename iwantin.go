package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/drone/config"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
)

const (
	INTERNAL_SERVER_ERROR_MESSAGE = "Ooops, Something went wrong."
)

var (
	slackURL      = config.String("slack-url", "https://bnetech.slack.com/")
	slackToken    = config.String("slack-token", "")
	captchaSecret = config.String("captcha-secret", "")
)

func SendRequest(baseurl string, resource string, data map[string]string) (*http.Response, error) {
	d := url.Values{}
	for k, v := range data {
		d.Set(k, v)
	}
	//d.Set("token", *slackToken) moved out to make this more general
	u, _ := url.ParseRequestURI(baseurl)
	u.Path = resource
	urlStr := fmt.Sprintf("%v", u) // "https://api.com/user/"

	r, _ := http.NewRequest("POST", urlStr, bytes.NewBufferString(d.Encode()))
	r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Add("Content-Length", strconv.Itoa(len(d.Encode())))
	return http.DefaultClient.Do(r)
}

type slackResponse struct {
	Ok          bool   `json:"ok"`
	ErrorString string `json:"error"`
}

func (s *slackResponse) Error() string {
	return s.ErrorString
}

type captchaResponse struct {
	Success    bool     `json:"success"`
	ErrorCodes []string `json:"error-codes,omitempty"`
}

func (c *captchaResponse) CaptchaStatus() bool {
	return c.Success
}

func HandleInviteRequest(rw http.ResponseWriter, req *http.Request) {
	err := req.ParseForm()
	if err != nil {
		log.Printf("[ERROR]: %s\n", err.Error())
		http.Error(rw, INTERNAL_SERVER_ERROR_MESSAGE, http.StatusInternalServerError)
		return
	}

	email := req.Form.Get("email")
	if email == "" {
		http.Error(rw, "Ooops. We need your email address to send the invite.", http.StatusBadRequest)
		return
	}

	if ok, _ := regexp.MatchString("^[^@\\s]+@[^@\\s]+\\.[^@\\s]+$", email); !ok {
		http.Error(rw, "Ooops. That doesn't appear to be a valid email address", http.StatusBadRequest)
		return
	}

	captcha := req.Form.Get("g-recaptcha-response")
	if captcha == "" {
		http.Error(rw, "Something went wrong with the Captcha, are you a robot?", http.StatusBadRequest)
		return
	}

	// Ask google if the captcha was okay
	resp, _ := SendRequest("https://www.google.com/recaptcha", "/api/sitverify", map[string]string{
		"secret":   *captchaSecret,
		"response": captcha,
	})

	// Note, can add remoteip but I dunno how :)
	if resp == nil {
		log.Println("[Error] Error submitting captcha")
		http.Error(rw, INTERNAL_SERVER_ERROR_MESSAGE, http.StatusInternalServerError)
		return
	}
	// it worked but we need to check the result
	contents, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[Error] Error decoding captcha response (%s)\n", err.Error())
		http.Error(rw, INTERNAL_SERVER_ERROR_MESSAGE, http.StatusInternalServerError)
		return
	}
	var cap captchaResponse
	err = json.Unmarshal(contents, &cap)
	if err != nil {
		log.Printf("[Error] Error decoding captcha error (%s)\n", err.Error())
		// I have a feeling this wont work because I didnt implement Error ^
		// @mnbbrown
	}

	if cap.Success != true {
		log.Println("[Captcha] Captcha Failed")
		http.Error(rw, "Captcha Failed", http.StatusBadRequest)
		// @mnbbrown should maybe print the error strings, I dunno
		return
	}

	resp, _ = SendRequest(*slackURL, "/api/users.admin.invite", map[string]string{
		"email": email,
		"token": *slackToken,
	})
	if resp == nil {

		log.Println("[ERROR] Error decoding slack error")
		http.Error(rw, INTERNAL_SERVER_ERROR_MESSAGE, http.StatusInternalServerError)
		return

	} else {

		contents, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Printf("[ERROR] Error decoding slack error (%s)\n", err.Error())
			http.Error(rw, INTERNAL_SERVER_ERROR_MESSAGE, http.StatusInternalServerError)
			return
		}

		var serr slackResponse
		err = json.Unmarshal(contents, &serr)
		if err != nil {
			log.Printf("[ERROR] Error decoding slack error (%s)\n", err.Error())
			log.Printf("Status Code: %v\n", resp.StatusCode)
			log.Printf("Ok: %v\n", serr.Ok)
			log.Printf("Error String: %v\n", serr.ErrorString)
			log.Printf("Body: %s\n", string(contents))
			http.Error(rw, INTERNAL_SERVER_ERROR_MESSAGE, http.StatusInternalServerError)
			return
		}
		if serr.Ok != true {
			switch {
			case serr.ErrorString == "not_authed":
				log.Printf("[ERROR] Invalid Slack Token (%s)\n", *slackToken)
				http.Error(rw, INTERNAL_SERVER_ERROR_MESSAGE, http.StatusInternalServerError)
				return
			case serr.ErrorString == "already_invited" || serr.ErrorString == "sent_recently":
				log.Printf("[ERROR] Already invited (%s)\n", email)
				http.Error(rw, "Looks like you've already requested an invite. Check your inbox (or your spam) again.", http.StatusBadRequest)
				return
			case serr.ErrorString == "already_in_team":
				log.Printf("[ERROR] Already a member (%s)\n", email)
				http.Error(rw, "Looks like you're already a member!", http.StatusBadRequest)
			default:
				log.Printf("[ERROR] Unknown error (%s)\n", serr.Error())
				http.Error(rw, INTERNAL_SERVER_ERROR_MESSAGE, http.StatusInternalServerError)
				return
			}
		}
	}
}

func main() {
	c := flag.String("c", "", "Location of the configuration file.")
	config.SetPrefix("IWANTIN_")
	err := config.Parse(*c)
	if err != nil {
		log.Fatalln(err.Error())
	}

	if *slackURL == "" {
		log.Fatalln("IWANTIN_SLACK_URL must be set.")
	}
	if *slackToken == "" {
		log.Fatalln("IWANTIN_SLACK_TOKEN must be set.")
	}
	if *captchaSecret == "" {
		log.Fatalln("IWANTIN_CAPTCHA_SECRET must be set.")
	}

	log.Println("Starting Auto-Inviter")
	log.Printf("Slack URL: %s\n", *slackURL)

	http.Handle("/", http.FileServer(http.Dir("./static")))
	http.HandleFunc("/invite", HandleInviteRequest)
	err = http.ListenAndServe(":"+os.Getenv("PORT"), nil)
	if err != nil {
		panic(err)
	}
}
