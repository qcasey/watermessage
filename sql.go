package main

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog/log"
)

func query(SQL string) (*sql.Rows, error) {
	log.Debug().Msg(SQL)

	if server.DB == nil {
		log.Info().Msg("Creating new DB connection")
		var err error
		server.DB, err = sql.Open("sqlite3", server.SQLiteFile)
		if err != nil {
			return nil, err
		}
		defer server.DB.Close()
	}

	rows, err := server.DB.Query(SQL)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func parseMessageRows(rows *sql.Rows) []Message {
	var out []Message
	if rows == nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		m := Message{}
		err := rows.Scan(&m.RowID, &m.Handle.ID, &m.Text, &m.IsFromMe, &m.HasAttachment, &m.Delivered, &m.Date, &m.DateDelivered, &m.DateRead)
		if err != nil {
			log.Error().Msg(err.Error())
		}
		if m.Handle.ID == nil {
			newID := "me"
			m.Handle.ID = &newID
		}
		if m.HasAttachment {
			attachments, err := getAttachment(m.RowID)
			if err == nil {
				m.Attachments = attachments
			} else {
				log.Error().Msg(err.Error())
			}
		}
		out = append(out, m)
	}
	return out
}

func parseAttachmentRows(rows *sql.Rows) []Attachment {
	var out []Attachment
	if rows == nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		m := Attachment{}
		err := rows.Scan(&m.MessageID, &m.ID, &m.Filename, &m.MIMEType, &m.TransferState, &m.TotalBytes)
		if err != nil {
			log.Error().Msg(err.Error())
		}
		out = append(out, m)

		// Ensure attachment is registered with the server
		server.Attachments[*m.MessageID] = m
	}
	return out
}

func parseChatRows(rows *sql.Rows) []*Chat {
	var out []*Chat
	if rows == nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		c := Chat{}
		rows.Scan(&c.RowID, &c.ID, &c.Name, &c.DisplayName)
		out = append(out, &c)
	}
	//log.Info().Msg(fmt.Sprintf("Counted %d chats", len(out)))
	return out
}

func getAttachment(messageID string) ([]Attachment, error) {
	sql := fmt.Sprintf("SELECT message_attachment_join.message_id, attachment.ROWID, attachment.filename, attachment.mime_type, attachment.transfer_state, attachment.total_bytes FROM attachment LEFT OUTER JOIN message_attachment_join ON message_attachment_join.attachment_id = attachment.ROWID WHERE message_attachment_join.message_id = %s ORDER BY attachment.created_date DESC", messageID)
	rows, err := query(sql)
	if err != nil {
		return nil, err
	}
	return parseAttachmentRows(rows), nil
}

func getAllMessagesInChat(chatID string) ([]Message, error) {
	// Default to handle ID, check if it's a group chat
	selector := "chat.room_name IS NULL AND handle.id"
	if strings.Contains(chatID, "chat") {
		selector = "chat.chat_identifier"
	}

	sql := fmt.Sprintf("SELECT DISTINCT message.ROWID, handle.id, message.text, message.is_from_me, message.cache_has_attachments, message.is_delivered, message.date, message.date_delivered, message.date_read FROM message LEFT OUTER JOIN chat ON chat.room_name = message.cache_roomnames LEFT OUTER JOIN handle ON handle.ROWID = message.handle_id WHERE message.service = 'iMessage' AND %s = '%s' ORDER BY message.date DESC LIMIT 50", selector, chatID)
	rows, err := query(sql)
	if err != nil {
		return nil, err
	}
	return parseMessageRows(rows), nil
}

func getLastMessageInChat(chatID string) (Message, error) {
	// Default to handle ID, check if it's a group chat
	selector := "handle.id"
	if strings.Contains(chatID, "chat") {
		selector = "chat.chat_identifier"
	}

	sql := fmt.Sprintf("SELECT DISTINCT message.ROWID, handle.id, message.text, message.is_from_me, message.cache_has_attachments, message.is_delivered, message.date, message.date_delivered, message.date_read FROM message LEFT OUTER JOIN chat ON chat.room_name = message.cache_roomnames LEFT OUTER JOIN handle ON handle.ROWID = message.handle_id WHERE message.service = 'iMessage' AND %s = '%s' ORDER BY message.date DESC LIMIT 1", selector, chatID)
	rows, err := query(sql)
	if err != nil {
		return Message{}, err
	}

	messages := parseMessageRows(rows)
	if messages == nil {
		return Message{}, nil
	}
	return messages[0], nil
}

func isNewMessage(chatID string) bool {
	serverChat, ok := server.ChatMap[chatID]
	if !ok {
		return true
	}

	sql := fmt.Sprintf("SELECT message_date FROM chat_message_join LEFT OUTER JOIN chat ON chat.ROWID = chat_message_join.chat_id WHERE chat_id = %d AND chat.service_name = 'iMessage' ORDER BY message_date DESC LIMIT 1", serverChat.RowID)
	rows, err := query(sql)
	if err != nil {
		return true
	}
	defer rows.Close()
	var lastMessageDate int
	for rows.Next() {
		rows.Scan(&lastMessageDate)
	}

	if serverChat.LastMessageDate != lastMessageDate {
		log.Info().Msg(fmt.Sprintf("Row %d not found, last recorded date is %d while the server shows %d", serverChat.RowID, lastMessageDate, serverChat.LastMessageDate))
		return true
	}

	return false
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
		if isNewMessage(chat.ID) {
			updatedChats++
			chat.Messages, err = getAllMessagesInChat(chat.ID)
			if err != nil {
				log.Error().Msg(err.Error())
			}
			if len(chat.Messages) > 0 {
				chat.LastMessageDate = chat.Messages[0].Date
				server.ChatMap[chat.ID] = chat
			} else {
				log.Debug().Msg(fmt.Sprintf("0 messages in row %d, %s", chat.RowID, chat.ID))
			}
		}
	}

	log.Debug().Msg(fmt.Sprintf("Took %dms to parse %d rows", time.Since(start).Milliseconds(), updatedChats))
}
