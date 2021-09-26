package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"github.com/jackc/pgx/v4"
	"gopkg.in/yaml.v3"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"
)

type config struct {
	Database string `yaml:"database"`
	Key      string `yaml:"key"`
}

type note struct {
	ID   int
	Note string
	Date time.Time
}

func readConfigFile() (c *config) {
	f, err := ioutil.ReadFile("config.yml")
	if err != nil {
		os.Exit(1)
	}
	c = &config{}
	err = yaml.Unmarshal(f, c)
	if err != nil {
		os.Exit(1)
	}
	return
}

func connectToDB(c *config) (conn *pgx.Conn) {
	conn, err := pgx.Connect(context.Background(), c.Database)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	return
}

func insertNote(conn *pgx.Conn, note string) {
	queryString := "INSERT INTO public.notes (note) VALUES($1);"
	_, err := conn.Exec(context.Background(), queryString, note)
	if err != nil {
		fmt.Fprintf(os.Stderr, "InsertRow failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Note saved!")
}

func listNotes(conn *pgx.Conn, count int, key string) {
	queryString := "SELECT * from notes ORDER BY pubtime DESC LIMIT $1;"
	rows, err := conn.Query(context.Background(), queryString, count)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Query failed: %v\n", err)
		os.Exit(1)
	}

	var n note
	var decNote string
	for rows.Next() {
		rows.Scan(&n.ID, &n.Note, &n.Date)
		decNote = decrypt(n.Note, key)
		fmt.Printf("%v: %v (%v)\n", n.Date.Format("02/01/06 15:04"), decNote, n.ID)
	}
	rowsAffected, err := strconv.Atoi(strings.Split(string(rows.CommandTag()), " ")[1])
	if err == nil && rowsAffected == 0 {
		fmt.Println("You have no notes!")
	}
}

func deleteNote(conn *pgx.Conn, idList []int32) {
	queryString := "DELETE FROM notes WHERE id=any($1);"
	_, err := conn.Query(context.Background(), queryString, idList)
	if err != nil {
		fmt.Fprintf(os.Stderr, "DeleteRow failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Notes %v deleted from server \n", idList)
}

func encrypt(text, key string) string {
	bKey, bText := []byte(key), []byte(text)

	block, err := aes.NewCipher(bKey)
	if err != nil {
		panic(err.Error())
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		panic(err.Error())
	}

	nonce := make([]byte, aesGCM.NonceSize())
	_, err = io.ReadFull(rand.Reader, nonce)
	if err != nil {
		panic(err.Error())
	}

	encText := aesGCM.Seal(nonce, nonce, bText, nil)
	return base64.StdEncoding.EncodeToString(encText)
}

func decrypt(encText, key string) string {
	bKey := []byte(key)
	bEncText, _ := base64.StdEncoding.DecodeString(encText)
	block, err := aes.NewCipher(bKey)
	if err != nil {
		panic(err.Error())
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		panic(err.Error())
	}

	nonceSize := aesGCM.NonceSize()
	nonce, cipher := bEncText[:nonceSize], bEncText[nonceSize:]

	text, err := aesGCM.Open(nil, nonce, cipher, nil)
	return string(text)
}

func main() {
	c := readConfigFile()
	conn := connectToDB(c)
	defer conn.Close(context.Background())

	if len(os.Args) < 2 {
		listNotes(conn, 5, c.Key)
		os.Exit(0)
	}

	switch os.Args[1] {
	case "-l":
		if len(os.Args) == 2 {
			listNotes(conn, 5, c.Key)
		} else {
			count, err := strconv.Atoi(os.Args[2])
			if err != nil || count == 0 {
				listNotes(conn, 5, c.Key)
			} else {
				listNotes(conn, count, c.Key)
			}
		}
	case "-d":
		if len(os.Args) >= 3 {
			idList := make([]int32, 0)
			for _, v := range os.Args[2:] {
				id, err := strconv.Atoi(v)
				idList = append(idList, int32(id))
				if err != nil {
					fmt.Println("Wrong id format! Only numbers are allowed!")
					os.Exit(1)
				}
			}
			deleteNote(conn, idList)
		} else {
			fmt.Println("No note ids given!")
			os.Exit(1)
		}
	case "-h":
		fmt.Println("=====USAGE=====")
		fmt.Println("Add new note: pgn here is my note text")
		fmt.Println("List last 5 notes: pgn")
		fmt.Println("List last 10 notes: pgn -l 10")
		fmt.Println("Delete notes: pgn -d id id id")
	default:
		note := strings.Join(os.Args[1:], " ")
		encNote := encrypt(note, c.Key)
		insertNote(conn, encNote)
	}
}
