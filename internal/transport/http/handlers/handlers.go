package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"golang.org/x/oauth2"

	"github.com/Yury132/Golang-Task-3/internal/models"
	"github.com/Yury132/Golang-Task-3/internal/utils"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/gorilla/websocket"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/rs/zerolog"
)

type Service interface {
	GetUserInfo(state string, code string) ([]byte, error)
	GetUsersList(ctx context.Context) ([]models.User, error)
	HandleUser(ctx context.Context, name string, email string) error
}

type Handler struct {
	log         zerolog.Logger
	oauthConfig *oauth2.Config
	service     Service
	js          jetstream.JetStream
}

// Для Google
var (
	// Любая строка
	oauthStateString = "pseudo-random"
	info             models.Content
	// Сессия
	store = sessions.NewCookieStore([]byte("super-secret-key"))
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Разрешение открытия WebSocket подключения всем клиентам
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Hub всех пользователей
var usersHub = make(map[string]*models.UserStruct, 0)

// Hub всех комнат
var roomsHub = make(map[int]*models.RoomStruct, 0)

// Hub всех чатов
var chatsHub = make(map[int]*models.ChatStruct, 0)

// Уникальный ID автоинкремент для чатов и комнат
var globalId = 1

// Стартовая страница
func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {

	tmpl, err := template.ParseFiles("./internal/templates/home_page.html")
	if err != nil {
		h.log.Error().Err(err).Msg("filed to show home page")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, nil)
}

// Авторизация через Гугл
func (h *Handler) Auth(w http.ResponseWriter, r *http.Request) {
	url := h.oauthConfig.AuthCodeURL(oauthStateString)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// Гугл перенаправляет сюда, когда пользователь успешно авторизовался, создаем сессию
func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	// Получаем данные из гугла
	content, err := h.service.GetUserInfo(r.FormValue("state"), r.FormValue("code"))
	if err != nil {
		h.log.Error().Err(err).Msg("callback...")
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	// Заполняем info, но не передаем ее на страницу
	if err = json.Unmarshal(content, &info); err != nil {
		h.log.Error().Err(err).Msg("filed to unmarshal struct")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Проверка существования пользователя в БД и его создание при необходимости
	if err = h.service.HandleUser(r.Context(), info.Name, info.Email); err != nil {
		h.log.Error().Err(err).Msg("filed to handle user")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Задаем жизнь сессии в секундах
	// 10 мин
	// store.Options = &sessions.Options{
	// 	MaxAge: 60 * 10,
	// }
	// Создаем сессию
	session, err := store.Get(r, "session-name")
	if err != nil {
		h.log.Error().Err(err).Msg("session create failed")
	}
	// Устанавливаем значения в сессию
	// Сохраняем данные пользователя
	session.Values["authenticated"] = true
	session.Values["Name"] = info.Name
	session.Values["Email"] = info.Email
	session.Values["ID"] = info.ID
	fmt.Println(info.ID, "First!!!")
	if err = session.Save(r, w); err != nil {
		h.log.Error().Err(err).Msg("filed to save session")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	tmpl, err := template.ParseFiles("./internal/templates/auth_page.html")
	if err != nil {
		h.log.Error().Err(err).Msg("filed to show home page")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, nil)
}

// Информация о пользователе
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {

	// Получаем сессию
	session, err := store.Get(r, "session-name")
	if err != nil {
		h.log.Error().Err(err).Msg("session failed")
		//w.WriteHeader(http.StatusInternalServerError)
		//return
	}

	// Проверяем, что пользователь залогинен
	if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
		// Если нет
		tmpl, err := template.ParseFiles("./internal/templates/error.html")
		w.WriteHeader(http.StatusUnauthorized)
		if err != nil {
			h.log.Error().Err(err).Msg("filed to show error page")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, nil)
	} else {
		// Если да
		// Читаем данные из сессии
		info.Name = session.Values["Name"].(string)
		info.Email = session.Values["Email"].(string)

		tmpl, err := template.ParseFiles("./internal/templates/auth_page.html")
		if err != nil {
			h.log.Error().Err(err).Msg("filed to show home page")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, info)
	}
}

// Выход из системы, удаление сессии
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	session, err := store.Get(r, "session-name")
	if err != nil {
		h.log.Error().Err(err).Msg("session failed")
	}
	// Удаляем сессию
	session.Options.MaxAge = -1
	session.Save(r, w)
	// Переадресуем пользователя на страницу логина
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Все пользователи в БД
func (h *Handler) GetUsersList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	users, err := h.service.GetUsersList(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		h.log.Error().Err(err).Msg("failed to get users list")
		return
	}

	data, err := json.Marshal(users)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		h.log.Error().Err(err).Msg("failed to marshal users list")
		return
	}

	w.Write(data)
}

// Для разных пользователей нужно открывать разные браузеры
// Страница после прохождения авторизации
func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {

	// ID и Name пользователя
	var userId string
	var userName string

	// Получаем сессию
	session, err := store.Get(r, "session-name")
	if err != nil {
		h.log.Error().Err(err).Msg("session failed")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Проверяем, что пользователь залогинен
	if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
		// Если нет
		tmpl, err := template.ParseFiles("./internal/templates/error.html")
		w.WriteHeader(http.StatusUnauthorized)
		if err != nil {
			h.log.Error().Err(err).Msg("filed to show error page")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, nil)
		return
	} else {
		// Если да
		// Читаем данные из сессии
		userName = session.Values["Name"].(string)
		fmt.Println("From Google", userName)
		userId = session.Values["ID"].(string)
		fmt.Println("From Google", userId)
	}

	// ID пользователя
	// userId, err := strconv.Atoi(r.URL.Query().Get("userId"))
	// if err != nil {
	// 	w.WriteHeader(http.StatusInternalServerError)
	// 	fmt.Println("failed to get id from string")
	// 	return
	// }

	// Проверка на то, что данного пользователя не было в карте
	_, userExist := usersHub[userId]

	// Если пользователя раньше не существовало
	if !userExist {
		// Создаем нового пользователя
		newUser := &models.UserStruct{UserId: userId, UserName: userName}

		// Добавляем нового пользователя в карту
		// Ключ карты - уникальный ID пользователя
		usersHub[userId] = newUser

		// // Создаем сессию
		// session, err := store.Get(r, "session-name")
		// if err != nil {
		// 	fmt.Println("session create failed")
		// }

		// // Сохраняем данные пользователя
		// // Будем получать userId и userName из гугл авторизации!!!!!!!!!!!!!!!!!!!!!!!!!!!Следить чтобы не пересохранялись пустые значения......
		// session.Values["userId"] = r.URL.Query().Get("userId")
		// session.Values["userName"] = r.URL.Query().Get("userName")
		// if err = session.Save(r, w); err != nil {
		// 	fmt.Println("filed to save session")
		// 	w.WriteHeader(http.StatusInternalServerError)
		// 	return
		// }
	}

	tmpl, err := template.ParseFiles("./internal/templates/start.html")
	if err != nil {
		fmt.Println("filed to show start page")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	// Передаем на страницу карту всех чатов
	tmpl.Execute(w, roomsHub)
}

// Создание чата
func (h *Handler) CreateChat(w http.ResponseWriter, r *http.Request) {

	// Название чата из формы POST запрос
	getRoomName := r.FormValue("chatName")
	if getRoomName == "" {
		fmt.Println("Название чата пустое")
		// Переадресуем пользователя на ту же страницу
		// Костыль userId == -1
		http.Redirect(w, r, "/start", http.StatusSeeOther)
		return
	}
	// http.Redirect(w, r, "/start?userId=-1&userName=xxx", http.StatusSeeOther) &&&&&&&&&&&&&&&&&&&&&&&&&&&&&&&&&&&&&&&&&&&&&&&&&
	// Создаем новую комнату
	newRoom := &models.RoomStruct{RoomId: globalId, RoomName: getRoomName}

	// Добавляем новую комнату в карту
	roomsHub[globalId] = newRoom

	// Создаем новый чат
	newChat := &models.ChatStruct{ChatId: globalId, Room: newRoom, Ws: make([]*websocket.Conn, 0), User: make([]*models.UserStruct, 0)}

	// Добавляем новый чат в карту
	chatsHub[globalId] = newChat

	// Счетчик - уникальный автоинкремент - ID чатов и комнат
	// Кол-во существующих комнат и чатов совпадает
	globalId++

	// Переадресуем пользователя на ту же страницу
	// Костыль userId == -1
	http.Redirect(w, r, "/start", http.StatusSeeOther)
}

// Переходим в конкретный чат
func (h *Handler) GoChat(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)

	// ID чата
	chatId, err := strconv.Atoi(vars["chatId"])
	if err != nil || chatId < 1 {
		http.NotFound(w, r)
		return
	}

	// Проверка на переход в уже удаленный чат
	if _, chatExist := chatsHub[chatId]; !chatExist {
		/// Переадресуем пользователя на ту же страницу
		// Костыль userId == -1
		http.Redirect(w, r, "/start", http.StatusSeeOther)
		return
	}

	// Получаем сессию
	session, err := store.Get(r, "session-name")
	if err != nil {
		fmt.Println("session failed")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Из сессии читаем ID пользователя
	userId := session.Values["ID"].(string)
	// Переводим в int
	// userIdInt, err := strconv.Atoi(userIdString)
	// if err != nil || userIdInt < 1 {
	// 	http.NotFound(w, r)
	// 	return
	// }

	fmt.Println("ID пользователя из Сессии: ", userId)

	// Из сессии читаем Name пользователя
	userName := session.Values["Name"].(string)

	// Формируем структуру
	data := models.UserAndRoomStruct{UserId: userId, UserName: userName, RoomId: chatId, RoomName: chatsHub[chatId].Room.RoomName}

	tmpl, err := template.ParseFiles("./internal/templates/chat.html")
	if err != nil {
		fmt.Println("filed to show home page")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	// Передаем данные
	tmpl.Execute(w, data)
}

// Подключение через WebSocket
// Сюда приходят все клиенты
func (h *Handler) WsEndpoint(w http.ResponseWriter, r *http.Request) {

	// ID пользователя
	// getUserId, err := strconv.Atoi(r.URL.Query().Get("userId"))
	// if err != nil || getUserId < 1 {
	// 	http.NotFound(w, r)
	// 	return
	// }
	getUserId := r.URL.Query().Get("userId")

	// ID комнаты (чата)
	getRoomId, err := strconv.Atoi(r.URL.Query().Get("roomId"))
	if err != nil || getRoomId < 1 {
		http.NotFound(w, r)
		return
	}

	// Уникальное подключение *websocket.Conn
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
	}

	log.Println("Подключился новый пользователь, userId - ", getUserId, "roomId - ", getRoomId)

	// Проверка на существование пользователя в карте
	if _, ok := usersHub[getUserId]; !ok {
		fmt.Println("invalid userId")
		return
	}

	// Проверка на существование чата в карте
	if _, ok := chatsHub[getRoomId]; !ok {
		fmt.Println("invalid chatID")
		return
	}

	// Добавляем пользователя к конкретному чату
	chatsHub[getRoomId].User = append(chatsHub[getRoomId].User, usersHub[getUserId])
	// Добавляем подключение к конкретному чату
	chatsHub[getRoomId].Ws = append(chatsHub[getRoomId].Ws, conn)

	fmt.Println("Вывод структуры конкретного чата:")
	fmt.Printf("%+v\n", chatsHub[getRoomId])
	fmt.Println("Присоединились к чату ID: ", chatsHub[getRoomId].Room.RoomId, " - ", chatsHub[getRoomId].Room.RoomName)

	fmt.Printf("Количество подключений в данном чате: %v\n", len(chatsHub[getRoomId].Ws))

	// Готовим сообщение JSON для отправки
	msg := models.MessageOnScreen{
		Msg:    "Добро пожаловать в чат!",
		Author: usersHub[getUserId].UserName,
	}

	// Кодируем
	b, err := json.Marshal(msg)
	if err != nil {
		fmt.Println("js message marshal err")
		return
	}

	// Сообщение клиенту
	err = conn.WriteMessage(1, b)
	if err != nil {
		log.Println(err)
	}

	// В бесконечном цикле прослушиваем входящие сообщения от каждого подключенного клиента
	// Передаем ID пользователя, ID комнаты (ID чата)
	h.reader(conn, getUserId, getRoomId)
}

// В бесконечном цикле прослушиваем входящие сообщения от каждого подключенного клиента
// Передаем ID пользователя, ID комнаты (ID чата) - Уникальные данные для каждого клиента, чьи сообщения мы прослушиваем
// Это и есть уникальные ключи к картам
// Имеем соответствие *websocket.Conn с конкретными комнатми (чатами)
func (h *Handler) reader(conn *websocket.Conn, userId string, chatId int) {
	// Этот бесконечный цикл запускаетя для каждого клиента с открытым WebSocket подключением
	for {
		// Ждем сообщение от клиента
		messageType, p, err := conn.ReadMessage()

		// При ошибке закрываем соединение - удаляем подключение и клиента из массива
		if err != nil {
			log.Println("Ошибка при чтении: ", err)

			// Если в DeleteChat уже закрыли все подключения тут необходимо проверять, закрыто ли оно... чтобы повторно не вызывать err = conn.Close()
			// Проверяем тип ошибки
			// if err, ok := err.(*websocket.CloseError); ok {
			// 	log.Printf("connection closed, code: %d, text: %q", err.Code, err.Text)
			// 	break
			// }

			// Или так
			// if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
			// 	log.Printf("WWWWWWWerror: %v", err)
			// }

			err = conn.Close()
			if err != nil {
				log.Println("Ошибка при закрытии соединения: ", err)
				return
			}

			// Проверка на существование чата (вдруг кто-то удалил)
			if _, chatExist := chatsHub[chatId]; !chatExist {
				return
			}

			// Вычисляем индекс удаляемого подключения из чата
			indexConn := utils.IndexOfConn(conn, chatsHub[chatId].Ws)
			// Удаляем подключение из массива по индексу
			chatsHub[chatId].Ws = append(chatsHub[chatId].Ws[:indexConn], chatsHub[chatId].Ws[indexConn+1:]...)
			fmt.Println("Удалили одно подключение из чата")

			// Вычисляем индекс удаляемого пользователя из чата
			indexUser := utils.IndexOfUser(usersHub[userId], chatsHub[chatId].User)
			// Удаляем пользователя из чата по индексу
			chatsHub[chatId].User = append(chatsHub[chatId].User[:indexUser], chatsHub[chatId].User[indexUser+1:]...)
			fmt.Println("Удалили одного пользователя из чата")

			// Выводим структуру
			fmt.Println("Структура чата после удаления:")
			fmt.Printf("%+v\n", chatsHub[chatId])
			fmt.Printf("Количество подключений в данном чате: %v\n", len(chatsHub[chatId].Ws))
			return
		}

		log.Println("Пришло сообщение: ", string(p), " от пользователя ID ", usersHub[userId].UserId, " - ", usersHub[userId].UserName)

		// Готовим сообщение для отправки
		msg := models.SendMessage{
			Msg:         string(p),
			Author:      usersHub[userId].UserName,
			MessageType: messageType,
			ChatId:      chatId,
		}

		// Кодируем
		b, err := json.Marshal(msg)
		if err != nil {
			fmt.Println("js message marshal err")
			return
		}

		// Отправляем полученное сообщение в Nats------------------------------------------------------------------------------------------------
		if _, err = h.js.Publish(context.Background(), "events.us.page_loaded", b); err != nil {
			fmt.Println("failed to publish message", err)
			return
		}
		//
		// Без Nats
		//
		// // Готовим сообщение JSON для отправки
		// msg := models.MessageOnScreen{
		// 	Msg:    string(p),
		// 	Author: usersHub[userId].UserName,
		// }

		// // Кодируем
		// b, err := json.Marshal(msg)
		// if err != nil {
		// 	fmt.Println("js message marshal err")
		// 	return
		// }
		// // Рассылка сообщения всем участникам чата---------------------------------------------------------------------------------------------
		// for i, conn := range chatsHub[chatId].Ws {
		// 	if err := conn.WriteMessage(messageType, b); err != nil {
		// 		log.Println("Ошибка при рассылке, ID подключения - ", i, " Ошибка:  ", err)
		// 		return
		// 	}
		// }

	}
}

// Удаление конкретного чата
func (h *Handler) DeleteChat(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)

	// ID чата
	chatId, err := strconv.Atoi(vars["chatId"])
	if err != nil || chatId < 1 {
		http.NotFound(w, r)
		return
	}

	fmt.Println("Удаляем чат", chatId)

	// // Необходимо закрыть все подключения в удаляемом чате
	// for i, conn := range chatsHub[chatId].Ws {
	// 	err = conn.Close()
	// 	if err != nil {
	// 		log.Println(i, " - Ошибка при закрытии соединения: ", err)
	// 	}
	// }

	// Удаляем чат и комнату из карты
	delete(chatsHub, chatId)
	delete(roomsHub, chatId)

	// Переадресуем пользователя на ту же страницу
	// Костыль userId == -1
	http.Redirect(w, r, "/start", http.StatusSeeOther)
}

// Изменение названия чата
func (h *Handler) EditChat(w http.ResponseWriter, r *http.Request) {

	// ID пользователя из формы POST запрос
	// getUserID, err := strconv.Atoi(r.FormValue("userID"))
	// if err != nil {
	// 	w.WriteHeader(http.StatusInternalServerError)
	// 	fmt.Println("failed to get getUserID from string")
	// 	http.Redirect(w, r, "/start", http.StatusSeeOther)
	// 	return
	// }

	getUserID := r.FormValue("userID")

	// ID чата из формы POST запрос
	getRoomID, err := strconv.Atoi(r.FormValue("chatID"))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Println("failed to get getRoomID from string")
		http.Redirect(w, r, "/start", http.StatusSeeOther)
		return
	}

	// Название чата из формы POST запрос
	getRoomName := r.FormValue("chatName")
	if getRoomName == "" {
		fmt.Println("Название чата пустое")
		// Переадресуем пользователя на ту же страницу
		// Костыль userId == -1
		http.Redirect(w, r, "/start", http.StatusSeeOther)
		return
	}

	// Проверка на существование чата (вдруг кто-то удалил)
	if _, chatExist := chatsHub[getRoomID]; !chatExist {
		http.Redirect(w, r, "/start", http.StatusSeeOther)
		return
	}

	// Готовим сообщение для отправки
	msg := models.SendMessage{
		Msg:         "Новое название чата - " + getRoomName,
		Author:      usersHub[getUserID].UserName,
		MessageType: 1,
		ChatId:      getRoomID,
	}

	// Кодируем
	b, err := json.Marshal(msg)
	if err != nil {
		fmt.Println("js message marshal err")
		return
	}

	// Отправляем полученное сообщение в Nats----------------------------------------------------------------Nats--------------------------------
	if _, err = h.js.Publish(context.Background(), "events.us.page_loaded", b); err != nil {
		fmt.Println("failed to publish message", err)
		return
	}

	// Изменяем название текущего чата (название комнаты) в карте
	chatsHub[getRoomID].Room.RoomName = getRoomName

	// Перезаходим в чат
	http.Redirect(w, r, "/go-chat/"+strconv.Itoa(getRoomID), http.StatusSeeOther)
}

// Вывод всех чатов
func (h *Handler) GetChats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	response, err := json.Marshal(chatsHub)
	if err != nil {
		fmt.Println("filed to marshal response data")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write(response)
}

func (h *Handler) GetRooms(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	response, err := json.Marshal(roomsHub)
	if err != nil {
		fmt.Println("filed to marshal response data")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write(response)
}

// Вывод всех пользователей
func (h *Handler) GetUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	response, err := json.Marshal(usersHub)
	if err != nil {
		fmt.Println("filed to marshal response data")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write(response)
}

func New(log zerolog.Logger, oauthConfig *oauth2.Config, service Service, js jetstream.JetStream) *Handler {
	return &Handler{
		log:         log,
		oauthConfig: oauthConfig,
		service:     service,
		js:          js,
	}
}

// Воркер
// Должен видеть chatsHub !!!!!!!!!!!!!!!!!!!!!
func (h *Handler) Worker(id int, jobs <-chan *models.SendMessage) {
	// Ожидаем получения данных для работы
	// Если данных нет в канале - блокировка
	for j := range jobs {
		fmt.Println("worker", id, "принял сообщение: ", j)

		// Проверка на существование чата (вдруг кто-то удалил)
		if _, chatExist := chatsHub[j.ChatId]; !chatExist {
			continue
		}

		// Рассылка сообщения всем участникам чата
		for i, conn := range chatsHub[j.ChatId].Ws {

			// Готовим сообщение JSON для отправки
			msg := models.MessageOnScreen{
				Msg:    j.Msg,
				Author: j.Author,
			}

			// Кодируем
			b, err := json.Marshal(msg)
			if err != nil {
				fmt.Println("js message marshal err")
				return
			}

			if err := conn.WriteMessage(j.MessageType, b); err != nil {
				log.Println("Ошибка при рассылке, ID подключения - ", i, " Ошибка:  ", err)
				continue
			}
		}
		fmt.Println("worker", id, "разослал сообщение: ", j)
	}

}
