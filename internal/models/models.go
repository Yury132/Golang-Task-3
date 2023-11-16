package models

import "github.com/gorilla/websocket"

type User struct {
	ID    uint64 `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Данные от Гугла
type Content struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
	Locale        string `json:"locale"`
}

// Вспомогательная структура - ID пользователя и комнаты для перехода в конкретный чат
type UserAndRoomStruct struct {
	UserId   string `json:"user_id"`
	UserName string `json:"user_name"`
	RoomId   int    `json:"room_id"`
	RoomName string `json:"room_name"`
}

// Комната
type RoomStruct struct {
	RoomId   int    `json:"room_id"`
	RoomName string `json:"room_name"`
}

// Пользователь
type UserStruct struct {
	UserId   string `json:"user_id"`
	UserName string `json:"user_name"`
}

// Чат
type ChatStruct struct {
	ChatId int               `json:"chat_id"`
	Room   *RoomStruct       `json:"room"`
	Ws     []*websocket.Conn `json:"ws"`
	User   []*UserStruct     `json:"user"`
}

// Передаваемое сообщение в Nats
type SendMessage struct {
	Msg         string `json:"msg"`
	Author      string `json:"author"`
	MessageType int    `json:"messageType"`
	ChatId      int    `json:"chatId"`
}

// Передаваемое сообщение по WebSocket клиету для отображения на странице
type MessageOnScreen struct {
	Msg    string `json:"msg"`
	Author string `json:"author"`
}
