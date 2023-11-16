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

	//-------------------------------------------------------Настройка Nats------------------------------

	// Подключение к Nats
	nc, err := nats.Connect(cfg.NATS.URL)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to NATS")
	}
	defer func() {
		if err = nc.Drain(); err != nil {
			logger.Fatal().Err(err).Msg("failed to drain nats connection")
		}
	}()

	// Создаем Jetstream
	js, err := jetstream.New(nc)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to create new jetstream")
	}

	// Создаем Consumer
	cons, err := cfg.NewJS(context.Background(), js, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to create new Jetstream or Consumer")
	}

	// Канал для воркеров
	jobs := make(chan *models.SendMessage, 100)

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
	//-------------------------------------------------------Настройка Nats------------------------------

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
