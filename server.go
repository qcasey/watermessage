package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog/log"
)

// ServerType holds global state of the iMessage server
type ServerType struct {
	Chats        []*Chat
	ChatMap      map[string]*Chat
	DB           *sql.DB
	SQLiteFile   string
	LastModified time.Time
	Attachments  map[string]Attachment
	lock         sync.RWMutex
}

func (s *ServerType) openDB() {
	var err error
	server.DB, err = sql.Open("sqlite3", server.SQLiteFile)
	if err != nil {
		log.Error().Msg(err.Error())
		return
	}
}

func query(SQL string) (*sql.Rows, error) {
	log.Debug().Msg(SQL)

	// Open new connection to the DB if it's been closed.
	// Some connections are persistent, to speed up rapid fire queries
	if err := server.DB.Ping(); err != nil {
		log.Info().Msg("Creating new DB connection")
		server.openDB()
		defer server.DB.Close()
	}

	rows, err := server.DB.Query(SQL)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func hasBeenModified() bool {
	info, err := os.Stat(server.SQLiteFile)
	if err != nil {
		log.Error().Msg(err.Error())
	}
	if info.ModTime().After(server.LastModified) {
		server.LastModified = info.ModTime()
		return true
	}
	return false
}

func readBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	body, err := ioutil.ReadAll(r.Body)

	if err != nil {
		http.Error(w, "can't read body", http.StatusBadRequest)
		return nil, fmt.Errorf("Error reading body: %v", err)
	}

	// Put body back
	r.Body.Close() //  must close
	r.Body = ioutil.NopCloser(bytes.NewBuffer(body))

	return body, nil
}

// Get group chats?
//sql := "SELECT DISTINCT chat.ROWID, chat.chat_identifier, chat.guid, chat.display_name FROM message LEFT OUTER JOIN chat ON chat.room_name = message.cache_roomnames LEFT OUTER JOIN handle ON handle.ROWID = message.handle_id WHERE message.is_from_me = 0 AND chat.service_name = 'iMessage' AND message.handle_id > 0 ORDER BY message.date DESC"

func refreshChats() {
	server.lock.Lock()
	defer server.lock.Unlock()

	var err error
	server.DB, err = sql.Open("sqlite3", server.SQLiteFile)
	if err != nil {
		log.Error().Msg(err.Error())
		return
	}
	defer server.DB.Close()

	sql := "SELECT DISTINCT chat.ROWID, chat.chat_identifier, chat.guid, chat.display_name FROM chat LEFT OUTER JOIN chat_message_join ON chat.ROWID = chat_message_join.chat_id WHERE chat.service_name = 'iMessage' ORDER BY chat_message_join.message_date DESC"
	rows, err := query(sql)
	if err != nil {
		log.Error().Msg(err.Error())
		return
	}
	newChats := parseChatRows(rows)

	start := time.Now()
	updatedChats := 0
	for _, chat := range newChats {
		// Only fetch entire chat history if there are new messages
		if !hasNewMessages(chat.ID) {
			continue
		}

		updatedChats++
		chat.Messages, err = getAllMessages(chat.ID)
		if err != nil {
			log.Error().Msg(err.Error())
		}
		if len(chat.Messages) > 0 {
			chat.LastMessageDate = chat.Messages[0].Date
			server.ChatMap[chat.ID] = chat
		}
	}

	log.Debug().Msg(fmt.Sprintf("Took %dms to parse %d rows", time.Since(start).Milliseconds(), updatedChats))
}
