package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"mega-trainer-go/internal/bot"
	"mega-trainer-go/internal/config"
)

func main() {
	// Загрузка конфигурации
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "configs/config.yaml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fitnessBot, err := bot.NewBot(cfg)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Обработка сигналов для graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	go func() {
		<-stop
		log.Println("Shutting down bot...")
		cancel()
	}()

	fitnessBot.Start(ctx)
	//
	// cfg, err := config.Load(configPath)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	//
	// // Инициализация логгера
	// log := logger.New(&cfg.Logging)
	//
	// // Инициализация метрик
	// metrics := monitoring.NewMetrics(cfg.App.Name)
	// if cfg.Monitoring.PrometheusEnabled {
	// 	metrics.StartServer(cfg.Monitoring.PrometheusPort)
	// }
	//
	// // Подключение к БД
	// pg, err := database.NewPostgres(&cfg.Database.Postgres)
	// if err != nil {
	// 	log.Fatal().Err(err).Msg("Failed to connect to database")
	// }
	// defer pg.Close()

	// Инициализация Redis
	// rdb := rdb.NewClient(&rdb.Options{
	// 	Addr:     cfg.Redis.Address,
	// 	Password: cfg.Redis.Password,
	// 	DB:       cfg.Redis.DB,
	// })
	//
	// fsmStore := redis.NewFSMStore(rdb, "mega-trainer", 24*time.Hour)
	// ctx := context.Background()

	// // Инициализация репозиториев
	// userRepo := postgres.NewUserRepository(pg.DB)
	//
	// // Создание бота
	// botAPI, err := tgbotapi.NewBotAPI(cfg.Telegram.BotToken)
	// if err != nil {
	// 	log.Fatal().Err(err).Msg("Failed to create bot")
	// }
	//
	// botAPI.Debug = cfg.Telegram.Debug
	//
	// log.Info().
	// 	Str("username", botAPI.Self.UserName).
	// 	Msg("Bot authorized successfully")
	//
	// // Создание обработчика
	// handler := bot.NewHandler(botAPI, redisStore, userRepo, metrics, log, cfg)
	//
	// // Graceful shutdown
	// ctx, stop := signal.NotifyContext(context.Background(),
	// 	os.Interrupt, syscall.SIGTERM)
	// defer stop()
	//
	// // Запуск бота
	// err = handler.StartPolling(60)
	// if err != nil {
	// 	log.Fatal("Failed to start polling:", err)
	// }
	// if cfg.Telegram.WebhookURL != "" {
	// 	handler.StartWebhook(ctx, cfg.Telegram.WebhookURL)
	// } else {
	// 	handler.StartPolling(ctx)
	// }

	// <-ctx.Done()
	// log.Info().Msg("Shutting down bot...")
	// ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	// defer stop()
	//
	// // Подключение к БД
	// // db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	// // if err != nil {
	// // 	log.Fatal(err)
	// // }
	// // defer db.Close()
	//
	// boti, err := tgbotapi.NewBotAPI(cfg.Telegram.BotToken)
	// if err != nil {
	// 	log.Fatal().Err(err)
	// }
	// boti.Debug = true
	//
	// log.Printf("Authorized on account %s", boti.Self.UserName)
	//
	// repo := postgres.NewRepository(pg.DB)
	// handler := bot.NewHandler(boti, repo)
	//
	// // UDATES
	// // u := tgbotapi.NewUpdate(0)
	// // u.Timeout = 60
	// //
	// // updates := boti.GetUpdatesChan(u)
	// //
	// // for {
	// // 	select {
	// // 	case <-ctx.Done():
	// // 		log.Println("Shutting down...")
	// // 		return
	// // 	case update := <-updates:
	// // 		go handler.HandleUpdate(ctx, update)
	// // 	}
	// // }
	//
	// // Устанавливаем webhook
	// webhookURL := os.Getenv("WEBHOOK_URL") // например: https://yourdomain.com/webhook
	// _, err = boti.SetWebhook(tgbotapi.NewWebhook(webhookURL))
	// if err != nil {
	// 	log.Fatal("SetWebhook:", err)
	// }
	//
	// repo := postgres.NewRepository(db)
	// handler := bot.NewHandler(botAPI, repo)
	//
	// // HTTP-хендлер для webhook
	// http.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
	// 	bytes, err := io.ReadAll(r.Body)
	// 	if err != nil {
	// 		http.Error(w, err.Error(), http.StatusBadRequest)
	// 		return
	// 	}
	//
	// 	var update tgbotapi.Update
	// 	if err := json.Unmarshal(bytes, &update); err != nil {
	// 		http.Error(w, err.Error(), http.StatusBadRequest)
	// 		return
	// 	}
	//
	// 	handler.HandleUpdate(ctx, update)
	// })
	//
	// port := os.Getenv("PORT")
	// if port == "" {
	// 	port = "8080"
	// }
	//
	// log.Printf("Webhook установлен на %s", webhookURL)
	// log.Printf("Сервер запущен на :%s", port)
	//
	// server := &http.Server{Addr: ":" + port}
	// go func() {
	// 	<-ctx.Done()
	// 	server.Shutdown(context.Background())
	// }()
	//
	// if err := server.ListenAndServe(); err != http.ErrServerClosed {
	// 	log.Fatal(err)
	// }
}
