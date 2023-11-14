package utils

import (
	"github.com/Yury132/Golang-Task-3/internal/models"
	"github.com/gorilla/websocket"
)

// Вычисляем индекс элемента в срезе для последующего удаления - Подключения
func IndexOfConn(conn *websocket.Conn, data []*websocket.Conn) int {
	for k, v := range data {
		if conn == v {
			return k
		}
	}
	return -1
}

// Вычисляем индекс элемента в срезе для последующего удаления - Пользователи
func IndexOfUser(user *models.UserStruct, data []*models.UserStruct) int {
	for k, v := range data {
		if user == v {
			return k
		}
	}
	return -1
}
