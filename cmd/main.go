package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Yury132/Golang-Task-3/internal/client/google"
	"github.com/Yury132/Golang-Task-3/internal/config"
	"github.com/Yury132/Golang-Task-3/internal/models"
	"github.com/Yury132/Golang-Task-3/internal/service"
	"github.com/Yury132/Golang-Task-3/internal/storage"
	transport "github.com/Yury132/Golang-Task-3/internal/transport/http"
	"github.com/Yury132/Golang-Task-3/internal/transport/http/handlers"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/pressly/goose/v3"
)

const (
	dialect        = "pgx"
	commandUp      = "up"
	commandDown    = "down"
	migrationsPath = "./internal/migrations"
)

func main() {

	// Конфигурации
	cfg, err := config.Parse()
	if err != nil {
		panic(err)
	}

	// Логгер
	logger := cfg.Logger()

	// Миграции
	db, err := goose.OpenDBWithDriver(dialect, cfg.GetDBConnString())
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to open db by goose")
	}

	if err = goose.Run(commandUp, db, migrationsPath); err != nil {
		logger.Fatal().Msgf("migrate %v: %v", commandUp, err)
	}

	if err = db.Close(); err != nil {
		logger.Fatal().Err(err).Msg("failed to close db connection by goose")
	}

	// Настройка БД
	poolCfg, err := cfg.PgPoolConfig()
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to DB")
	}

	// Подключение к БД
	conn, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to db")
	}

	// Гугл
	oauthCfg := cfg.SetupConfig()
	googleAPI := google.New(logger)

	//-------------------------------------------------------Настройка Nats и канала для воркеров------------------------------
	// Канал для воркеров
	jobs := make(chan *models.SendMessage, 100)

	// Адрес сервера nats
	url := os.Getenv("NATS_URL")
	if url == "" {
		url = nats.DefaultURL
	}

	// Подключаемся к серверу
	nc, err := nats.Connect(url)
	if err != nil {
		fmt.Println("Ошибка при Connect...")
	}
	defer nc.Drain()

	// Создаем Jetstream
	js, err := jetstream.New(nc)
	if err != nil {
		fmt.Println("Ошибка при jetstream.New...")
	}

	cfgJS := jetstream.StreamConfig{
		Name: "EVENTS",
		// Очередь
		Retention: jetstream.WorkQueuePolicy,
		Subjects:  []string{"events.>"},
	}

	// Создаем поток
	stream, err := js.CreateStream(context.Background(), cfgJS)
	if err != nil {
		fmt.Println("Ошибка при js.CreateStream...")
	}

	// Создаем получателя
	cons, err := stream.CreateOrUpdateConsumer(context.Background(), jetstream.ConsumerConfig{
		Name: "processor-1",
	})
	if err != nil {
		fmt.Println("Ошибка при stream.CreateOrUpdateConsumer...")
	}

	// В горутине получатель беспрерывно ждет входящих сообщений
	// При получении сообщений, передает задачи-данные-смс воркерам для последующей рассылки
	go func() {
		_, err := cons.Consume(func(msg jetstream.Msg) {
			// Декодируем
			var info = new(models.SendMessage)
			if err := json.Unmarshal(msg.Data(), info); err != nil {
				fmt.Println("Ошибка при декодировании....")
			} else {
				fmt.Println("Полученные данные Consume: ", info)
				//fmt.Println("Отправляем данные в канал")
				// Заполняем канал данными
				// Воркеры начнут работать
				jobs <- info
			}
			// Подтверждаем получение сообщения
			err := msg.DoubleAck(context.Background())
			if err != nil {
				fmt.Println("Ошибка при DoubleAck...")
			}
		})
		if err != nil {
			fmt.Println("Ошибка при Consume...")
		}
	}()
	//-------------------------------------------------------Настройка Nats и канала для воркеров------------------------------

	strg := storage.New(conn)
	svc := service.New(logger, oauthCfg, googleAPI, strg)
	// Прокидываем также Jetstream
	handler := handlers.New(logger, oauthCfg, svc, js)
	srv := transport.New(":8080").WithHandler(handler)

	// Запускаем воркеров в горутинах
	// Они будут ожидать получения данных для работы
	for w := 1; w <= 3; w++ {
		go handler.Worker(w, jobs)
	}

	// graceful shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT)

	// Запусвкаем сервер
	go func() {
		fmt.Println("Сервер запущен")
		if err = srv.Run(); err != nil {
			logger.Fatal().Err(err).Msg("failed to start server")
		}
	}()

	// Ждем нажатия Ctrl+C
	<-shutdown
}
