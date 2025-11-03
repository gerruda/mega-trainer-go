package bot

import (
	"context"
	"log"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"mega-trainer-go/internal/config"
	"mega-trainer-go/internal/delivery/telegram"
	"mega-trainer-go/internal/repository"
	"mega-trainer-go/internal/service"
	"mega-trainer-go/internal/storage/redis"
	"mega-trainer-go/pkg/database"
	"mega-trainer-go/pkg/logger"
	"mega-trainer-go/pkg/monitoring"
)

type Bot struct {
	cfg     *config.Config
	botAPI  *tgbotapi.BotAPI
	handler *telegram.Handler
}

func NewBot(cfg *config.Config) (*Bot, error) {
	botAPI, err := tgbotapi.NewBotAPI(cfg.Telegram.BotToken)
	if err != nil {
		return nil, err
	}

	// Инициализация логгера
	log := logger.New(&cfg.Logging)

	// Инициализация метрик
	metrics := monitoring.NewMetrics(cfg.App.Name)
	if cfg.Monitoring.PrometheusEnabled {
		metrics.StartServer(cfg.Monitoring.PrometheusPort)
	}

	// Подключение к БД
	pg, err := database.NewPostgres(&cfg.Database.Postgres)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer pg.Close()

	// Инициализация Redis
	redisClient := repository.NewRedisClient(cfg.Redis)

	// Проверяем соединение с Redis
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := repository.Ping(ctx, redisClient); err != nil {
		log.Fatal().Err(err).Msgf("Failed to connect to Redis: %v", err)
	}
	log.Println("Successfully connected to Redis")

	stateRepo := redis.NewRedisStateRepository(redisClient)

	// Сервисы
	calculatorService := service.NewCalculatorService()
	stateService := service.NewStateService(stateRepo)

	// Хендлер
	handler := telegram.NewHandler(botAPI, calculatorService, stateService)

	return &Bot{
		cfg:     cfg,
		botAPI:  botAPI,
		handler: handler,
	}, nil
}

func (b *Bot) Start(ctx context.Context) {
	log.Printf("Authorized on account %s", b.botAPI.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.botAPI.GetUpdatesChan(u)

	for update := range updates {
		go func(update tgbotapi.Update) {
			if err := b.handler.HandleUpdate(ctx, update); err != nil {
				log.Printf("Error handling update: %v", err)
			}
		}(update)
	}
}

func (b *Bot) StartWebhooks(ctx context.Context) {
	// // Устанавливаем webhook
	// webhookURL := os.Getenv("WEBHOOK_URL") // например: https://yourdomain.com/webhook
	// _, err = b.botAPI.SetWebhook(tgbotapi.NewWebhook(webhookURL))
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
}
