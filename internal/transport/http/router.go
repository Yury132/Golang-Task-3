package http

import (
	"net/http"

	"github.com/Yury132/Golang-Task-3/internal/transport/http/handlers"
	"github.com/gorilla/mux"
)

func InitRoutes(h *handlers.Handler) *mux.Router {
	r := mux.NewRouter()

	r.HandleFunc("/", h.Home).Methods(http.MethodGet)
	r.HandleFunc("/auth", h.Auth).Methods(http.MethodGet)
	r.HandleFunc("/callback", h.Callback).Methods(http.MethodGet)
	r.HandleFunc("/me", h.Me).Methods(http.MethodGet)
	r.HandleFunc("/logout", h.Logout).Methods(http.MethodGet)
	r.HandleFunc("/users-list", h.GetUsersList).Methods(http.MethodGet)
	// Открываем подключение для каждого клиента по WebSocket
	r.HandleFunc("/ws", h.WsEndpoint)
	// Создание чата
	r.HandleFunc("/create-chat", h.CreateChat).Methods(http.MethodPost)
	// Вывод всех комнат
	r.HandleFunc("/get-rooms", h.GetRooms).Methods(http.MethodGet)
	// Вывод всех чатов
	r.HandleFunc("/get-chats", h.GetChats).Methods(http.MethodGet)
	// Вывод всех пользователей
	r.HandleFunc("/get-users", h.GetUsers).Methods(http.MethodGet)
	// Страница после прохождения авторизации
	r.HandleFunc("/start", h.Start)
	// Переход в конкретный чат
	r.HandleFunc("/go-chat/{chatId:[0-9]+}", h.GoChat)
	// Удаление конкретного чата
	r.HandleFunc("/delete-chat/{chatId:[0-9]+}", h.DeleteChat)
	http.Handle("/", r)

	return r
}
