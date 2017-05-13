package main

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/mail"
	"net/smtp"
	"os"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	// Перенаправление логов в файл
	// создать файл лога, установить права доступв
	l, err := os.OpenFile("/var/log/klshmail.log", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	logFatal(err)
	defer l.Close()

	log.SetOutput(l)

	// Заголовки исходного сообщения, которые нужно оставить
	hh := []string{
		"MIME-Version",
		"Message-Id",
		"Subject",
		"Content-Type",
		"In-Reply-To",
		"References",
	}

	// Читаем сообщение из стандартного ввода
	m, err := mail.ReadMessage(os.Stdin)
	logFatal(err)
	log.Println("New message accepted...")

	// Разбор сообщения
	to, err := mail.ParseAddress(m.Header.Get("to"))
	logFatal(err)
	log.Println("To:", to.String())

	// Разбор заголовков сообщения
	from, err := mail.ParseAddress(m.Header.Get("from"))
	logFatal(err)
	log.Println("From:", from.String())

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
		WHERE LOWER(list.email)=LOWER(?)
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
		ON (
			user.id=user_list.uid AND
			user_list.canwrite AND
			user_list.lid=?
		)
    WHERE LOWER(user.email)=LOWER(?)
    AND user.active
		`, lid, from.Address).Scan(&uid)
	if err != nil {
		log.Println("User", from.Address, "can't send messages to", to.Address)
		os.Exit(0)
	}

	// Формирование заголовков нового сообщения
	var newmessage string

	for _, k := range hh {
		if h := m.Header.Get(k); h != "" {
			newmessage += k + ": " + h + "\r\n"
		}
	}

	// from.Name = "[" + lname + "] " + from.Name
	// from.Address = to.Address
	// newmessage += "From: " + from.String() + "\r\n"
	// newmessage += "Reply-To: <" + from.Address + ">\r\n"
	// newmessage += "X-KLSH-Sender: " + uid + "\r\n"
	// newmessage += "\r\n"

	from.Name = lprefix + " " + from.Name
	from.Address = to.Address
	newmessage += fmt.Sprintf("From: %s\r\n", from.String())
	newmessage += fmt.Sprintf("Reply-To: <%s>\r\n", from.Address)
	newmessage += fmt.Sprintf("X-KLSH-Sender: %v\r\n", uid)
	newmessage += "\r\n"

	// Подключение к SMTP серверу
	c, err := smtp.Dial("127.0.0.1:25")
	if err != nil {
		log.Println("SMTP connection error")
		log.Fatal(err)
	}
	defer c.Close()

	// Отправка сообщения
	c.Mail(to.Address)
	// c.Rcpt(lid + "@klshmail")
	c.Rcpt(fmt.Sprintf("%v@klshmail", lid))

	wc, err := c.Data()
	logFatal(err)
	defer wc.Close()

	// Отправка заголовков сообщения
	if _, err = wc.Write([]byte(newmessage)); err != nil {
		log.Println("SMTP send headers error")
		log.Fatalln(err)
	}

	// Отправка тела сообщения
	if _, err = io.Copy(wc, m.Body); err != nil {
		log.Println("SMTP send body error")
		log.Fatalln(err)
	}

	// Завершение работы
	log.Println("Message processing done.")

}

func logFatal(e error) {
	if e != nil {
		log.Fatalln(e)
	}
}

