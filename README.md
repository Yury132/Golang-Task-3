<h1 align="center">Задание №3</h1>
<h3 align="left">Необходимо создать чат-сервер.

Сервис должен выполнять следующий сценарий:
1.	Пользователь авторизуется в систему через Google
2.	После авторизации пользователь с помощью API может получить список чатов, а также создать новый чат, отредактировать или удалить существующий
3.	Пользователь может отправить новое текстовое сообщение с помощью API. После добавления сообщения в базу сервис должен добавить задачу в очередь на рассылку данного сообщения всем пользователям подключенным к websocket
4.	Сервис поддерживает подключение по websocket для получения информации о новых сообщениях

Требования:
1.	Для реализации авторизации использовать код из задания 1
2.	Использовать PostgreSQL в качестве базы данныхи
3.	Использовать Nats JetStream в качестве инструмента для реализации очереди
4.	Использовать идеологию REST при проектировании методов API
</h3>

<h1 align="center">Развертка</h1>

- Склонировать репозиторий
```
git clone https://github.com/Yury132/Golang-Task-3.git
```
- Установить PostgreSQL в Docker контейнер, используя docker-compose.yml файл из проекта
  
1. Скопировать docker-compose.yml в новую папку "postgresql"
  
2. Выполнить в терминале команду
```
docker compose up
```
- Подключиться к базе данных PostgreSQL (Например, через DBeaver)

POSTGRES_DB: mydb

POSTGRES_USER: root

POSTGRES_PASSWORD: mydbpass

Port: 5432

Host: localhost

- Установить NATS в Docker контейнер командами
```
docker pull nats:latest
```
```
docker run -p 4222:4222 -ti nats:latest -js
```
- Скопировать полученный файл .env по пути Golang-Task-3/internal/config
- Запустить веб-приложение командой
```
go run cmd/main.go
```

<h1 align="center">Тестирование</h1>

- Используя разные браузеры, например, Яндекс Браузер и Google Chrome, перейти по адресу

```
http://localhost:8080/
```

- Авторизоваться через Google под разными аккаунтами
- Нажать на кнопку "Чаты"
- Создать чат с любым названием
  
![alt text](https://github.com/Yury132/Golang-Task-3/blob/main/forREADME/1.PNG?raw=true)
  
![alt text](https://github.com/Yury132/Golang-Task-3/blob/main/forREADME/2.PNG?raw=true)

- Авторизованным пользователям перейти в один и тот же чат и начать обмениваться сообщениями
    
![alt text](https://github.com/Yury132/Golang-Task-3/blob/main/forREADME/4.PNG?raw=true)

- При необходимости название чата можно изменить
    
![alt text](https://github.com/Yury132/Golang-Task-3/blob/main/forREADME/3.PNG?raw=true)

- Существующие чаты можно удалять, нажимая на крестик рядом с названием конкретного чата

