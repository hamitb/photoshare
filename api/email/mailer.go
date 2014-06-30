package email

import (
	"bytes"
	"fmt"
	"github.com/danjac/photoshare/api/config"
	"log"
	"net/smtp"
	"strings"
	"text/template"
)

var mailer Mailer

type Message struct {
	Subject string
	Body    []byte
	To      []string
	From    string
}

func (msg *Message) String() string {
	return fmt.Sprintf("To:%s\r\nFrom:%s\r\nSubject: %s\r\nBody:%s",
		strings.Join(msg.To, ", "),
		msg.From,
		msg.Subject,
		string(msg.Body),
	)
}

// Creates a new message from a template
func MessageFromTemplate(subject string,
	to []string,
	from string,
	t *template.Template,
	data interface{}) (*Message, error) {

	msg := &Message{
		Subject: subject,
		To:      to,
		From:    from,
	}
	b := &bytes.Buffer{}
	if err := t.Execute(b, data); err != nil {
		return nil, err
	}
	msg.Body = b.Bytes()
	return msg, nil
}

type Mailer interface {
	Send(*Message) error
}

type smtpMailer struct {
	smtp.Auth
}

func (m *smtpMailer) Send(msg *Message) error {
	return smtp.SendMail(config.SmtpHost+":25", m.Auth, msg.From, msg.To, msg.Body)
}

type fakeMailer struct{}

func (m *fakeMailer) Send(msg *Message) error {
	log.Println(msg)
	return nil
}

func newSmtpMailer() Mailer {
	m := &smtpMailer{}
	m.Auth = smtp.PlainAuth("", config.SmtpName, config.SmtpPassword, config.SmtpHost)
	return m
}

func NewMailer() Mailer {
	return mailer
}

func init() {
	if config.SmtpName == "" || config.SmtpPassword == "" {
		log.Println("WARNING: using fake mailer, messages will not be sent by SMTP. " +
			"Set SMTP_NAME and SMTP_PASSWORD in environment to enable.")
		mailer = &fakeMailer{}
	} else {
		mailer = newSmtpMailer()
	}
}