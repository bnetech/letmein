package main

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func HandleNewHelpRequest(rw http.ResponseWriter, req *http.Request) {
	err := req.ParseForm()
	if err != nil {
		log.Printf("[ERROR] Error parsing form: %s\n", err.Error())
		http.Error(rw, INTERNAL_SERVER_ERROR_MESSAGE, http.StatusInternalServerError)
		return
	}

	now := time.Now().UTC()

	pageOpened := req.Form.Get("help-opended")
	honeypot := req.Form.Get("help-honeypot")
	text := req.Form.Get("help-text")
	contact := req.Form.Get("help-contact")

	t, err := time.Parse(time.RFC3339, pageOpened)
	if err != nil {
		log.Printf("[ERROR] Failed to parse date (%s): %s", pageOpened, err.Error())
		http.Error(rw, INTERNAL_SERVER_ERROR_MESSAGE, http.StatusInternalServerError)
		return
	}
	log.Printf("[INFO] INF01 Time Diff = %vs", now.Sub(t.UTC()).Seconds())

	if honeypot != "" {
		log.Printf("[ERROR] Failed to parse date (%s): %s", pageOpened, err.Error())
		http.Error(rw, "Looks like you're a robot :(", http.StatusBadRequest)
		return
	}

	if text == "" {
		log.Printf("[INFO] No text was submitted (%s): %s", t, err.Error())
		http.Error(rw, "Oops. We need to know what you need help with.", http.StatusBadRequest)
		return
	}

	if contact == "" {
		log.Printf("[INFO] No contact info was submitted (%s): %s", t, err.Error())
		http.Error(rw, "Oops. We need some form of contact information.", http.StatusBadRequest)
		return
	}

	if db == nil {
		log.Println("[WARN] Not writing to database. Check logs")
		http.Error(rw, "Oops. We need some form of contact information.", http.StatusBadRequest)
		return
	}

	var id int
	err = db.QueryRow("INSERT INTO help (HelpText, HelpContact, PageOpened, FormSubmitted) VALUES ($1,$2,$3,$4) RETURNING HelpId;", text, contact, t, now).Scan(&id)
	if err != nil {
		log.Printf("[ERROR] Failed to write to DB: %s", err.Error())
		http.Error(rw, INTERNAL_SERVER_ERROR_MESSAGE, http.StatusInternalServerError)
		return
	}

	log.Printf("[INFO] New help request: %v", id)

}

type SlackSlashCommand struct {
	Token       string `schema:"token" json:"token"`
	TeamID      string `schema:"team_id"`
	TeamDomain  string `schema:"team_domain"`
	ChannelId   string `schema:"channel_id"`
	ChannelName string `schema:"channel_name"`
	UserId      string `schema:"user_id"`
	UserName    string `schema:"user_name"`
	Command     string `schema:"command"`
	Text        string `schema:"text"`
}

type HelpRequest struct {
	Text     string `json:"text"`
	Contact  string `json:"contact"`
	Approved bool   `json:"approved"`
}

func inArray(in string, array []string) (bool, int) {
	for index, value := range array {
		if in == value {
			return true, index
		}
	}
	return false, 0
}

func HandleHelpRequestApproval(rw http.ResponseWriter, req *http.Request) {

	err := req.ParseForm()
	if err != nil {
		log.Printf("[ERROR] Error decoding slack slash request form  (%s): %v\n", err.Error(), req.Body)
		http.Error(rw, INTERNAL_SERVER_ERROR_MESSAGE, http.StatusInternalServerError)
	}

	ss := &SlackSlashCommand{}
	err = formDecoder.Decode(ss, req.PostForm)
	if err != nil {
		log.Printf("[ERROR] Error decoding slack slash request (%s): %v\n", err.Error(), req.Body)
		http.Error(rw, INTERNAL_SERVER_ERROR_MESSAGE, http.StatusInternalServerError)
		return
	}

	if ss.Token != *slackSlashToken {
		log.Printf("[ERROR] An incorrect token was sent with the request: %s", ss.Token)
		http.Error(rw, "Sorry, we don't recognise that token.", http.StatusUnauthorized)
		return
	}

	if ss.Text == "" {
		log.Printf("[ERROR] No text was sent with slash request")
		http.Error(rw, "Oops. We need an ID to approve i.e. /approve 123", http.StatusBadRequest)
		return
	}

	if ok, _ := inArray(ss.UserName, []string{"mnbbrown", "smitec"}); !ok {
		log.Printf("[ERROR] Invalid approver: %s", ss.UserName)
		http.Error(rw, "Oops. Only moderators can approve help request.", http.StatusUnauthorized)
		return
	}

	id, err := strconv.Atoi(strings.TrimSpace(ss.Text))
	if err != nil {
		log.Printf("[DEBUG] Not a number: %v (%s)\n", ss.Text, err.Error())
		http.Error(rw, fmt.Sprintf("%s is not a valid approval (%s)", ss.Text, err.Error()), http.StatusBadRequest)
		return
	}

	request := &HelpRequest{}
	err = db.QueryRow("SELECT HelpText, HelpContact FROM help WHERE HelpId = $1", id).Scan(&request.Text, &request.Contact)
	if err != nil {
		switch {
		case err == sql.ErrNoRows:
			log.Printf("[ERROR] Help Request %v not found", id)
			http.Error(rw, fmt.Sprintf("Help request %v wasn't found :(", id), http.StatusNotFound)
		default:
			log.Printf("[ERROR] DB Error: %s\n", err.Error())
			http.Error(rw, INTERNAL_SERVER_ERROR_MESSAGE, http.StatusInternalServerError)
		}
		return
	}

	_, err = db.Exec(db.Rebind("UPDATE help SET approved = ?, approvedby = ? WHERE HelpId = ?"), true, ss.UserName, id)
	if err != nil {
		log.Printf("[ERROR] [DB] Failed setting help request(%v) as approved by %s ", id, ss.UserName)
		http.Error(rw, INTERNAL_SERVER_ERROR_MESSAGE, http.StatusInternalServerError)
		return
	}

	resp, err := SendRequest("POST", *slackURL, "/api/chat.postMessage", map[string]string{
		"channel": *slackJobChannel,
		"text":    fmt.Sprintf("*Help Wanted*: \n %s \n\n *Contact Details*: %s", request.Text, request.Contact),
		"token":   *slackToken,
	})

	resptest, _ := ioutil.ReadAll(resp.Body)

	if err != nil {
		log.Printf("[ERROR] [SLACKAPI] Failed to post to channel (%v): %v", string(resptest))
		http.Error(rw, INTERNAL_SERVER_ERROR_MESSAGE, http.StatusInternalServerError)
		return
	}

	respToApprovalChannel := fmt.Sprintf("Approved request %v", id)
	rw.Header().Set("Content-Type", "text/plain")
	rw.Header().Set("Content-Length", strconv.Itoa(len(respToApprovalChannel)))
	fmt.Fprint(rw, respToApprovalChannel)
}
