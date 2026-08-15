package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/akorwash/QuizBattle/api/controller"
	gameauth "github.com/akorwash/QuizBattle/auth"
	"github.com/akorwash/QuizBattle/config"
	"github.com/akorwash/QuizBattle/datastore"
	"github.com/akorwash/QuizBattle/repository"
	"github.com/akorwash/QuizBattle/service"
	"github.com/akorwash/QuizBattle/service/createaccount"
	"github.com/akorwash/QuizBattle/service/login"
	"github.com/akorwash/QuizBattle/service/updateaccount"
	"github.com/akorwash/QuizBattle/websockets"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

type App struct {
	Router      http.Handler
	config      config.Config
	server      *http.Server
	mongoClient *mongo.Client
	redisClient *redis.Client
	sockets     *websockets.Registry
	readinessMu sync.Mutex
	readinessAt time.Time
	readinessOK bool
}

func New(cfg config.Config) (*App, error) {
	startupContext, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	mongoClient, database, err := datastore.ConnectMongo(startupContext, cfg.MongoURI, cfg.MongoDatabase)
	if err != nil {
		return nil, err
	}
	cleanupMongo := func() { _ = mongoClient.Disconnect(context.Background()) }
	if err := repository.EnsureIndexes(startupContext, database); err != nil {
		cleanupMongo()
		return nil, err
	}
	questionBankRepository := repository.NewMongoQuestionBankRepository(database)
	if cfg.SeedDatabase {
		questions, err := datastore.LoadQuestionBank(cfg.QuestionBankPath, time.Now().UTC())
		if err != nil {
			cleanupMongo()
			return nil, err
		}
		if err := questionBankRepository.Import(startupContext, questions); err != nil {
			cleanupMongo()
			return nil, err
		}
	}

	var redisClient *redis.Client
	if cfg.RedisAddress != "" {
		redisClient, err = datastore.GetRedisContext(startupContext, datastore.RedisConfiguration{
			EndPoint: cfg.RedisAddress,
			Username: cfg.RedisUsername,
			Password: cfg.RedisPassword,
			UseTLS:   cfg.RedisTLS,
		})
		if err != nil {
			cleanupMongo()
			return nil, err
		}
	}

	tokens, err := gameauth.NewTokenManager(cfg.JWTSecret, cfg.SessionTTL)
	if err != nil {
		if redisClient != nil {
			_ = redisClient.Close()
		}
		cleanupMongo()
		return nil, err
	}
	sessionRepository := repository.NewMongoSessionRepository(database)
	authenticator := controller.NewAuthenticator(tokens, cfg.CookieSecure, sessionRepository)
	sockets := websockets.NewRegistry(cfg.AllowedOrigins, authenticator.SessionActive)

	userRepository := repository.NewMongoUserRepository(database)
	avatarRepository := repository.NewMongoAvatarRepository(database)
	gameRepository := repository.NewMongoGameRepository(database)
	economyRepository := repository.NewMongoEconomyRepository(database)
	matchRepository := repository.NewMongoMatchRepository(database)
	chatRepository := repository.NewMongoChatRepository(database)
	accountService := service.NewAccountService(userRepository)
	avatarService := service.NewAvatarService(avatarRepository)
	createAccountService := createaccount.New(userRepository)
	updateAccountService := updateaccount.New(userRepository)
	loginService := login.New(userRepository)
	gameService := service.NewGameService(gameRepository, userRepository, sockets)
	questionBankService := service.NewQuestionBankService(questionBankRepository)
	economyService := service.NewEconomyService(economyRepository, questionBankService)
	matchService := service.NewMatchService(matchRepository, economyRepository, economyService, questionBankService, gameRepository, sockets)
	chatService := service.NewChatService(chatRepository)
	sockets.SetChatMessageStore(chatService)

	app := &App{config: cfg, mongoClient: mongoClient, redisClient: redisClient, sockets: sockets}
	app.Router = app.routes(authenticator, accountService, avatarService, createAccountService, updateAccountService, loginService, gameService, economyService, matchService, chatService)
	return app, nil
}

func (app *App) routes(
	authenticator *controller.Authenticator,
	accountService *service.AccountService,
	avatarService *service.AvatarService,
	createAccountService *createaccount.CreateAccountServices,
	updateAccountService *updateaccount.UpdateAccountServices,
	loginService *login.LoginService,
	gameService *service.GameService,
	economyService *service.EconomyService,
	matchService *service.MatchService,
	chatService *service.ChatService,
) http.Handler {
	mux := http.NewServeMux()
	homeController := &controller.HomeController{}
	authController := &controller.AuthController{}
	userController := controller.NewUserController(authenticator, app.sockets)
	avatarController := &controller.AvatarController{}
	gameController := &controller.GameController{}
	economyController := &controller.EconomyController{}
	matchController := &controller.MatchController{}
	chatController := &controller.ChatController{}
	signUpLimit := 5
	if app.config.Environment == "development" || app.config.Environment == "test" {
		// Local multiplayer verification creates up to eight isolated players in
		// one run. Production keeps the stricter anti-abuse limit below.
		signUpLimit = 30
	}
	signUpRateLimiter := newIPRateLimiter(signUpLimit, time.Minute, app.config.TrustedProxyCIDRs...)
	loginRateLimiter := newIPRateLimiter(10, time.Minute, app.config.TrustedProxyCIDRs...)
	logoutRateLimiter := newIPRateLimiter(10, time.Minute, app.config.TrustedProxyCIDRs...)
	accountMutationLimiter := newIdentityRateLimiter(20, time.Minute)
	sessionReadLimiter := newIdentityRateLimiter(120, time.Minute)
	gameMutationLimiter := newIdentityRateLimiter(60, time.Minute)
	gameReadLimiter := newIdentityRateLimiter(120, time.Minute)
	economyMutationLimiter := newIdentityRateLimiter(60, time.Minute)
	economyReadLimiter := newIdentityRateLimiter(120, time.Minute)
	chatReadLimiter := newIdentityRateLimiter(120, time.Minute)
	matchMutationLimiter := newIdentityRateLimiter(120, time.Minute)
	websocketUpgradeLimiter := newIdentityRateLimiter(30, time.Minute)
	websocketAttemptLimiter := newIPRateLimiter(90, time.Minute, app.config.TrustedProxyCIDRs...)

	mux.HandleFunc("GET /{$}", homeController.HomePage)
	mux.HandleFunc("GET /about", homeController.AboutPage)
	mux.HandleFunc("GET /contact", homeController.ContactPage)
	mux.HandleFunc("GET /auth/signin", authController.SignInPage)
	mux.HandleFunc("GET /auth/signup", authController.SignUpPage)

	mux.Handle("POST /user/createuser", signUpRateLimiter.Middleware(userController.CreateUser(createAccountService)))
	mux.Handle("POST /user/login", loginRateLimiter.Middleware(userController.Login(loginService)))
	mux.Handle("POST /user/logout", logoutRateLimiter.Middleware(http.HandlerFunc(userController.Logout)))
	mux.Handle("GET /api/v1/session", authenticator.Middleware(sessionReadLimiter.Middleware(userController.Session(accountService))))
	mux.Handle("POST /api/v1/user", authenticator.Middleware(accountMutationLimiter.Middleware(userController.UpdateUser(updateAccountService))))
	mux.Handle("PUT /api/v1/user/avatar", authenticator.Middleware(accountMutationLimiter.Middleware(avatarController.Upload(avatarService))))
	mux.Handle("DELETE /api/v1/user/avatar", authenticator.Middleware(accountMutationLimiter.Middleware(avatarController.Delete(avatarService))))
	mux.Handle("GET /api/v1/user/avatar/{id}", authenticator.Middleware(sessionReadLimiter.Middleware(avatarController.Get(avatarService))))
	mux.Handle("GET /api/v1/chat/messages", authenticator.Middleware(chatReadLimiter.Middleware(chatController.Messages(chatService))))
	mux.Handle("GET /user/profile", authenticator.PageMiddleware(http.HandlerFunc(userController.UserProfilePage)))
	mux.Handle("GET /user/profile/{username}", authenticator.PageMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/user/profile", http.StatusPermanentRedirect)
	})))

	// Questions are intentionally not exposed by raw ID. The future round
	// engine must select them server-side and reveal answers only after a turn.
	mux.Handle("POST /api/v1/game", authenticator.Middleware(gameMutationLimiter.Middleware(gameController.CreateGame(gameService))))
	mux.Handle("POST /api/v1/game/{id}/join", authenticator.Middleware(gameMutationLimiter.Middleware(gameController.JoinGame(gameService))))
	mux.Handle("POST /api/v1/game/{id}/exit", authenticator.Middleware(gameMutationLimiter.Middleware(gameController.ExitGame(gameService))))
	mux.Handle("GET /api/v1/game/{id}", authenticator.Middleware(gameReadLimiter.Middleware(gameController.GetBattle(gameService))))
	mux.Handle("GET /api/v1/games/public", authenticator.Middleware(gameReadLimiter.Middleware(gameController.GetPublicBattles(gameService))))
	mux.Handle("GET /api/v1/games/mine", authenticator.Middleware(gameReadLimiter.Middleware(gameController.GetMyBattles(gameService))))
	mux.Handle("GET /api/v1/collection", authenticator.Middleware(economyReadLimiter.Middleware(economyController.Collection(economyService))))
	mux.Handle("GET /api/v1/market", authenticator.Middleware(economyReadLimiter.Middleware(economyController.Market(economyService))))
	mux.Handle("POST /api/v1/market/listings", authenticator.Middleware(economyMutationLimiter.Middleware(economyController.CreateListing(economyService))))
	mux.Handle("POST /api/v1/market/listings/{id}/buy", authenticator.Middleware(economyMutationLimiter.Middleware(economyController.BuyListing(economyService))))
	mux.Handle("POST /api/v1/market/listings/{id}/cancel", authenticator.Middleware(economyMutationLimiter.Middleware(economyController.CancelListing(economyService))))
	mux.Handle("GET /api/v1/trades", authenticator.Middleware(economyReadLimiter.Middleware(economyController.Trades(economyService))))
	mux.Handle("POST /api/v1/trades", authenticator.Middleware(economyMutationLimiter.Middleware(economyController.CreateTrade(economyService))))
	mux.Handle("POST /api/v1/trades/{id}/accept", authenticator.Middleware(economyMutationLimiter.Middleware(economyController.TradeCommand(economyService, "accept"))))
	mux.Handle("POST /api/v1/trades/{id}/reject", authenticator.Middleware(economyMutationLimiter.Middleware(economyController.TradeCommand(economyService, "reject"))))
	mux.Handle("POST /api/v1/trades/{id}/cancel", authenticator.Middleware(economyMutationLimiter.Middleware(economyController.TradeCommand(economyService, "cancel"))))
	mux.Handle("PUT /api/v1/game/{id}/deck", authenticator.Middleware(matchMutationLimiter.Middleware(matchController.CommitDeck(matchService))))
	mux.Handle("POST /api/v1/game/{id}/prepare", authenticator.Middleware(matchMutationLimiter.Middleware(matchController.Prepare(matchService))))
	mux.Handle("POST /api/v1/game/{id}/start", authenticator.Middleware(matchMutationLimiter.Middleware(matchController.Start(matchService))))
	mux.Handle("POST /api/v1/game/{id}/forfeit", authenticator.Middleware(matchMutationLimiter.Middleware(matchController.Forfeit(matchService))))
	mux.Handle("GET /api/v1/game/{id}/match", authenticator.Middleware(gameReadLimiter.Middleware(matchController.Snapshot(matchService))))
	mux.Handle("POST /api/v1/game/{id}/answer", authenticator.Middleware(matchMutationLimiter.Middleware(matchController.Answer(matchService))))
	mux.Handle("GET /game/play", authenticator.PageMiddleware(http.HandlerFunc(gameController.PlayPage)))
	mux.Handle("GET /battle/{id}", authenticator.PageMiddleware(http.HandlerFunc(gameController.BattlePage)))

	mux.Handle("GET /ws/events", websocketAttemptLimiter.Middleware(authenticator.Middleware(websocketUpgradeLimiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, _ := controller.Identity(r)
		app.sockets.ServeEvents(identity.UserID, identity.TokenID, identity.FullName, identity.ExpiresAt, w, r)
	})))))
	mux.Handle("GET /ws/world-chat", websocketAttemptLimiter.Middleware(authenticator.Middleware(websocketUpgradeLimiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, _ := controller.Identity(r)
		app.sockets.ServeWorldChat(identity.UserID, identity.TokenID, identity.Username, identity.FullName, identity.ExpiresAt, w, r)
	})))))
	mux.Handle("GET /ws/game/{id}", websocketAttemptLimiter.Middleware(authenticator.Middleware(websocketUpgradeLimiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, _ := controller.Identity(r)
		gameID, err := strconvPathID(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid battle ID", http.StatusBadRequest)
			return
		}
		if err := gameService.CanAccessBattle(identity.UserID, gameID); err != nil {
			http.Error(w, "battle access denied", http.StatusForbidden)
			return
		}
		app.sockets.ServeBattle(gameID, identity.UserID, identity.TokenID, identity.FullName, identity.ExpiresAt, w, r)
	})))))

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{\"status\":\"ok\"}\n"))
	})
	mux.HandleFunc("GET /readyz", app.readiness)
	staticFiles := http.StripPrefix("/static/", http.FileServer(http.Dir("./static")))
	mux.Handle("GET /static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/static/" || strings.Contains(r.URL.Path, "..") {
			http.NotFound(w, r)
			return
		}
		staticFiles.ServeHTTP(w, r)
	}))

	return chain(
		mux,
		recoverPanics,
		securityHeaders(app.config.CookieSecure),
		requireSameOrigin(app.config.AllowedOrigins),
		requestLogging,
		limitConcurrency(256),
	)
}

func (app *App) Run(ctx context.Context) error {
	app.server = &http.Server{
		Addr:              ":" + app.config.Port,
		Handler:           app.Router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}
	errorChannel := make(chan error, 1)
	go func() {
		errorChannel <- app.server.ListenAndServe()
	}()
	select {
	case err := <-errorChannel:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP: %w", err)
	case <-ctx.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := app.server.Shutdown(shutdownContext); err != nil {
			return fmt.Errorf("shutdown HTTP server: %w", err)
		}
		return nil
	}
}

func (app *App) Close(ctx context.Context) error {
	app.sockets.Close()
	var result error
	if app.redisClient != nil {
		result = errors.Join(result, app.redisClient.Close())
	}
	if app.mongoClient != nil {
		result = errors.Join(result, app.mongoClient.Disconnect(ctx))
	}
	return result
}

func (app *App) readiness(w http.ResponseWriter, r *http.Request) {
	app.readinessMu.Lock()
	defer app.readinessMu.Unlock()
	if time.Since(app.readinessAt) < 2*time.Second {
		app.writeReadiness(w, app.readinessOK)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	app.readinessOK = app.mongoClient.Ping(ctx, readpref.Primary()) == nil
	app.readinessAt = time.Now()
	app.writeReadiness(w, app.readinessOK)
}

func (app *App) writeReadiness(w http.ResponseWriter, ready bool) {
	w.Header().Set("Cache-Control", "no-store")
	if !ready {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte("{\"status\":\"ready\"}\n"))
}

func strconvPathID(value string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid ID")
	}
	return id, nil
}
