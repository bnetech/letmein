package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/drone/config"
	"github.com/gorilla/schema"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"time"
)

const (
	INTERNAL_SERVER_ERROR_MESSAGE = "Ooops, Something went wrong."
)

var (
	slackURL        = config.String("slack-url", "https://bnetech.slack.com/")
	slackToken      = config.String("slack-token", "")
	slackSlashToken = config.String("slack-slash-token", "")
	slackJobChannel = config.String("slack-job-channel", "#jobs")
	postgresDSN     = config.String("postgres-url", os.Getenv("DATABASE_URL"))
)

func SendRequest(method string, baseurl string, resource string, data map[string]string) (*http.Response, error) {
	u, _ := url.ParseRequestURI(baseurl)
	u.Path = resource

	var r *http.Request

	switch {
	case method == "POST":

		d := url.Values{}
		for k, v := range data {
			d.Add(k, v)
		}
		urlStr := fmt.Sprintf("%v", u.String())
		log.Println(urlStr)
		r, _ = http.NewRequest(method, urlStr, bytes.NewBufferString(d.Encode()))
		r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Add("Content-Length", strconv.Itoa(len(d.Encode())))

	case method == "GET":
		query := u.Query()
		for k, v := range data {
			query.Set(k, v)
		}
		u.RawQuery = query.Encode()
		urlStr := fmt.Sprintf("%v", u)
		log.Println(urlStr)
		r, _ = http.NewRequest(method, urlStr, nil)

	}

	return http.DefaultClient.Do(r)
}

type slackResponse struct {
	Ok          bool   `json:"ok"`
	ErrorString string `json:"error"`
}

func (s *slackResponse) Error() string {
	return s.ErrorString
}

func SaveRequest(req *http.Request, email string, pageOpened time.Time, formSubmitted time.Time, honeypot string) {
	if db == nil {
		log.Println("[WARN] Not writing to database. Check logs")
		return
	}
	_, err := db.Exec("INSERT INTO submissions (IPAddress, Email, PageOpened, FormSubmitted, HoneyPot) VALUES ($1,$2,$3,$4,$5)",
		req.RemoteAddr, email, pageOpened, formSubmitted, honeypot)
	if err != nil {
		log.Printf("[ERROR] Failed to write to DB: %s", err.Error())
		return
	}
}

type slackMembersResponse struct {
	Ok      bool          `json:"ok"`
	Members []interface{} `json:"members"`
}

func ReadJSONResponse(resp *http.Response, v interface{}) error {
	contents, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	err = json.Unmarshal(contents, v)
	if err != nil {
		return err
	}
	return nil
}

type groupStats struct {
}

func HandleStatsRequest(rw http.ResponseWriter, req *http.Request) {

	resp, err := SendRequest("GET", *slackURL, "/api/users.list", map[string]string{
		"token": *slackToken,
	})

	if err != nil {
		log.Printf("[ERROR] Stats Request %s\n", err.Error())
		http.Error(rw, INTERNAL_SERVER_ERROR_MESSAGE, http.StatusInternalServerError)
		return
	}

	var memberInfo *slackMembersResponse
	err = ReadJSONResponse(resp, &memberInfo)
	if err != nil {
		log.Printf("[ERROR] Stats Request %s\n", err.Error())
		http.Error(rw, INTERNAL_SERVER_ERROR_MESSAGE, http.StatusInternalServerError)
	}

	b, err := json.Marshal(struct {
		Users    int `json:"users"`
		Channels int `json:"channels"`
	}{
		len(memberInfo.Members),
		0,
	})
	if err != nil {
		log.Printf("[ERROR] Stats Request %s\n", err.Error())
		http.Error(rw, INTERNAL_SERVER_ERROR_MESSAGE, http.StatusInternalServerError)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	rw.Header().Set("Content-Length", strconv.Itoa(len(b)))
	rw.Write(b)
}

func HandleInviteRequest(rw http.ResponseWriter, req *http.Request) {
	err := req.ParseForm()
	if err != nil {
		log.Printf("[ERROR]: %s\n", err.Error())
		http.Error(rw, INTERNAL_SERVER_ERROR_MESSAGE, http.StatusInternalServerError)
		return
	}

	now := time.Now().UTC()

	pageOpened := req.Form.Get("page-opended")
	honeypot := req.Form.Get("honeypot")
	email := req.Form.Get("email")

	t, err := time.Parse(time.RFC3339Nano, pageOpened)
	if err != nil {
		log.Printf("[ERROR] Failed to parse date (%s): %s", t, err.Error())
		http.Error(rw, INTERNAL_SERVER_ERROR_MESSAGE, http.StatusInternalServerError)
		return
	}
	log.Printf("[INFO] INF01 Time Diff = %vs", now.Sub(t.UTC()).Seconds())

	go SaveRequest(req, email, t, now, honeypot)

	if honeypot != "" {
		log.Printf("[ERROR] Failed to parse date (%s): %s", pageOpened, err.Error())
		http.Error(rw, "Looks like you're a robot :(", http.StatusBadRequest)
		return
	}

	if email == "" {
		http.Error(rw, "Ooops. We need your email address to send the invite.", http.StatusBadRequest)
		return
	}

	if ok, _ := regexp.MatchString("^[^@\\s]+@[^@\\s]+\\.[^@\\s]+$", email); !ok {
		http.Error(rw, "Ooops. That doesn't appear to be a valid email address", http.StatusBadRequest)
		return
	}

	resp, _ := SendRequest("POST", *slackURL, "/api/users.admin.invite", map[string]string{
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
			log.Printf("[DEBUG] Status Code: %v\n", resp.StatusCode)
			log.Printf("[DEBUG] Ok: %v\n", serr.Ok)
			log.Printf("[DEBUG] Error String: %v\n", serr.ErrorString)
			log.Printf("[DEBUG] Body: %s\n", string(contents))
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
				log.Printf("[DEBUG] Already invited (%s)\n", email)
				http.Error(rw, "Looks like you've already requested an invite. Check your inbox (or your spam) again.", http.StatusBadRequest)
				return
			case serr.ErrorString == "already_in_team":
				log.Printf("[DEBUG] Already a member (%s)\n", email)
				http.Error(rw, "Looks like you're already a member!", http.StatusBadRequest)
			default:
				log.Printf("[ERROR] Unknown error (%s)\n", serr.Error())
				http.Error(rw, INTERNAL_SERVER_ERROR_MESSAGE, http.StatusInternalServerError)
				return
			}
		}
	}
}

var db *sqlx.DB
var formDecoder *schema.Decoder

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

	if *slackSlashToken == "" {
		log.Fatalln("IWANTIN_SLACK_SLASH_TOKEN must be set.")
	}

	log.Println("Starting Auto-Inviter")
	log.Printf("Slack URL: %s\n", *slackURL)

	if *postgresDSN == "" {
		log.Println("[WARN] IWANTIN_DATABASE_URL or DATABASE_URL is not set: not storing submissions.")
	} else {
		db, err = sqlx.Connect("postgres", *postgresDSN)
		if err != nil {
			log.Fatalln(err.Error())
		}
		log.Printf("Database: %s", *postgresDSN)
	}

	formDecoder = schema.NewDecoder()

	http.Handle("/", http.FileServer(http.Dir("./static")))
	http.HandleFunc("/invite", HandleInviteRequest)
	http.HandleFunc("/stats", HandleStatsRequest)
	http.HandleFunc("/help/request", HandleNewHelpRequest)
	http.HandleFunc("/help/approve", HandleHelpRequestApproval)
	err = http.ListenAndServe(":"+os.Getenv("PORT"), nil)
	if err != nil {
		panic(err)
	}
}
