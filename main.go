package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"log"
	"mime"
	"net/mail"
	"net/smtp"
	"os"
	"strconv"
	"strings"

	message "github.com/matsuev/go-message"
	charset "github.com/matsuev/go-message/charset"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	// Перенаправление логов в файл
	// создать файл лога, установить права доступа
	// l, err := os.OpenFile("./klshmail.log", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	// l, err := os.OpenFile("/var/log/klshmail.log", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	// logFatal(err)
	// defer l.Close()

	// log.SetOutput(l)

	// Заголовки исходного сообщения,
	// которые нужно оставить
	hh := []string{
		"MIME-Version",
		"Message-Id",
		"Content-Type",
		"Content-Transfer-Encoding",
		"In-Reply-To",
		"References",
		// "Subject",
	}

	// r, err := os.Open("./testmessage.eml")
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// defer r.Close()

	// Читаем сообщение из стандартного ввода
	msg, err := message.Read(os.Stdin)
	// msg, err := message.Read(r)
	logFatal(err)
	log.Println("New message accepted...")

	// Разбор заголовков сообщения
	to, err := getMailHeader("To", msg.Header)
	logFatal(err)
	log.Printf("To: %s <%s>\n", to.Name, to.Address)

	from, err := getMailHeader("From", msg.Header)
	logFatal(err)
	log.Printf("From: %s <%s>\n", from.Name, from.Address)

	hSubject, err := charset.DecodeHeader(msg.Header.Get("Subject"))
	logFatal(err)

	subj := mime.BEncoding.Encode("utf-8", hSubject)

	if to.Address == "" || from.Address == "" {
		log.Fatalln("Empty address. Reject message.")
	}

	// Сокдинение с сервером БД
	db, err := sql.Open("mysql", "klshmail:euXoe8uSha1xu4sh@/klshmail?charset=utf8")
	logFatal(err)

	// Проверка соединения с сервером БД
	err = db.Ping()
	logFatal(err)

	// Запрос данных о списке рассылки
	var lid uint64
	var lprefix string

	err = db.QueryRow(`
		SELECT list.id, list.prefix
		FROM list
		WHERE LCASE(list.email)=TRIM(LCASE(?))
		AND list.active
		`, to.Address).Scan(&lid, &lprefix)
	if err != nil {
		log.Println("No list with address:", to.Address)
		os.Exit(0)
	}

	// Запрос на проверку прав пользователя на отправку сообщений в список
	var uid uint64
	err = db.QueryRow(`
		SELECT user.id
		FROM user
		INNER JOIN user_list
		ON (user_list.lid=?
			AND user.id=user_list.uid
			AND user_list.canwrite
		)
		WHERE LCASE(user.email)=TRIM(LCASE(?))
		AND user.active
		`, lid, from.Address).Scan(&uid)
	if err != nil {
		log.Println("User", from.Address, "can't send messages to", to.Address)
		os.Exit(0)
	}

	// Формирование заголовков нового сообщения
	newHeader := make(message.Header)
	for _, hk := range hh {
		if hv := msg.Header.Get(hk); hv != "" {
			newHeader.Set(hk, hv)
		}
	}

	sender := new(mail.Address)
	sender.Name = from.Name
	sender.Address = from.Address

	from.Name = fmt.Sprintf("%s", lprefix)
	from.Address = to.Address

	newHeader.Set("From", from.String())
	newHeader.Set("Reply-To", to.Address)
	newHeader.Set("Subject", subj)
	newHeader.Set("X-KLSH-Sender", strconv.FormatUint(uid, 10))

	var b bytes.Buffer
	w, err := message.CreateWriter(&b, newHeader)
	if err != nil {
		log.Fatal(err)
	}
	defer w.Close()

	if err := transform(w, msg, sender); err != nil {
		log.Fatal(err)
	}

	// Подключение к SMTP серверу
	c, err := smtp.Dial("127.0.0.1:25")
	if err != nil {
		log.Println("SMTP connection error")
		log.Fatal(err)
	}
	defer c.Close()

	// Заголовки для отправки
	c.Mail(to.Address)
	c.Rcpt(fmt.Sprintf("%v@klshmail", lid))

	wc, err := c.Data()
	logFatal(err)
	defer wc.Close()

	// Отправка сообщения
	br := bytes.NewReader(b.Bytes())
	if _, err = io.Copy(wc, br); err != nil {
		log.Println("SMTP send body error")
		log.Fatalln(err)
	}

	// fmt.Println(b.String())

	// Завершение работы
	log.Println("Message processing done.")

}

func logFatal(e error) {
	if e != nil {
		log.Fatalln(e)
	}
}

const senderHtml string = `<p><b>Сообщение от:</b> %s &lt;<a href="mailto:%s">%s</a>&gt;<p>`
const senderPlain string = "| Сообщение от:  %s <%s>\n——\n\n"

func transform(w *message.Writer, e *message.Entity, sender *mail.Address) error {
	if mr := e.MultipartReader(); mr != nil {
		// This is a multipart entity, transform each of its parts
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			} else if err != nil {
				return err
			}

			fmt.Println(p.Header)

			p.Header.Add("Content-Transfer-Encoding", "base64")
			pw, err := w.CreatePart(p.Header)
			if err != nil {
				return err
			}

			if err := transform(pw, p, sender); err != nil {
				return err
			}

			pw.Close()
		}
		return nil
	} else {
		body := e.Body
		var newLine string
		if strings.HasPrefix(e.Header.Get("Content-Type"), "text/plain") {
			newLine = fmt.Sprintf(senderPlain, sender.Name, sender.Address)
		}
		if strings.HasPrefix(e.Header.Get("Content-Type"), "text/html") {
			newLine = fmt.Sprintf(senderHtml, sender.Name, sender.Address, sender.Address)
		}
		body = io.MultiReader(strings.NewReader(newLine), body)

		// rb := make([]byte, 0)
		// io.ReadFull(body, rb)
		//
		// encoder := base64.NewEncoder(base64.StdEncoding, w)
		// _, err := encoder.Write(rb)
		_, err := io.Copy(w, body)
		return err
	}
}

func getMailHeader(k string, h message.Header) (*mail.Address, error) {
	dh, err := charset.DecodeHeader(h.Get(k))
	if err != nil {
		return nil, err
	}

	rh, err := mail.ParseAddress(dh)
	return rh, err
}
