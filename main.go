package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	cfg "github.com/golobby/config/v3"
	"github.com/golobby/config/v3/pkg/feeder"
	log "github.com/sirupsen/logrus"
	bot "github.com/tyzbit/go-discord-archiver/bot"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	config         bot.ArchiverBotConfig
	allSchemaTypes = []interface{}{
		&bot.ServerRegistration{},
		&bot.ServerConfig{},
		&bot.ArchiveEvent{},
		&bot.ArchiveEventEvent{},
		&bot.MessageEvent{},
		&bot.InteractionEvent{},
	}

	sqlitePath      string        = "/var/go-discord-archiver/local.sqlite"
	connMaxLifetime time.Duration = time.Hour
)

func init() {
	// Read from .env and override from the local environment
	dotEnvFeeder := feeder.DotEnv{Path: ".env"}
	envFeeder := feeder.Env{}

	_ = cfg.New().AddFeeder(dotEnvFeeder).AddStruct(&config).Feed()
	_ = cfg.New().AddFeeder(envFeeder).AddStruct(&config).Feed()

	// Info level by default
	LogLevelSelection := log.InfoLevel
	switch {
	case strings.EqualFold(config.LogLevel, "trace"):
		LogLevelSelection = log.TraceLevel
		log.SetReportCaller(true)
	case strings.EqualFold(config.LogLevel, "debug"):
		LogLevelSelection = log.DebugLevel
		log.SetReportCaller(true)
	case strings.EqualFold(config.LogLevel, "info"):
		LogLevelSelection = log.InfoLevel
	case strings.EqualFold(config.LogLevel, "warn"):
		LogLevelSelection = log.WarnLevel
	case strings.EqualFold(config.LogLevel, "error"):
		LogLevelSelection = log.ErrorLevel
	}
	log.SetLevel(LogLevelSelection)
	log.SetFormatter(&log.JSONFormatter{})
}

func main() {
	var db *gorm.DB
	var err error
	var dbType string

	// Increase verbosity of the database if the loglevel is higher than Info
	var logConfig logger.Interface
	if log.GetLevel() > log.DebugLevel {
		logConfig = logger.Default.LogMode(logger.Info)
	}

	if config.DBHost != "" && config.DBName != "" && config.DBPassword != "" && config.DBUser != "" {
		dbType = "mysql"
		dsn := fmt.Sprintf("%v:%v@tcp(%v)/%v?parseTime=True", config.DBUser, config.DBPassword, config.DBHost, config.DBName)
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logConfig})
	} else {
		dbType = "sqlite"
		// Create the folder path if it doesn't exist
		_, err = os.Stat(sqlitePath)
		if errors.Is(err, fs.ErrNotExist) {
			dirPath := filepath.Dir(sqlitePath)
			if err := os.MkdirAll(dirPath, 0660); err != nil {
				log.Error("unable to make directory path ", dirPath, " err: ", err)
				sqlitePath = "./local.db"
			}
		}
		db, err = gorm.Open(sqlite.Open(sqlitePath), &gorm.Config{Logger: logConfig})
	}

	if err != nil {
		log.Fatal("unable to connect to database (using "+dbType+"), err: ", err)
	}

	dbInstance, err := db.DB()
	if err != nil {
		log.Fatal("unable to configure db: ", err)
	}
	dbInstance.SetConnMaxLifetime(connMaxLifetime)

	log.Info("using ", dbType, " for the database")

	// Create a new Discord session using the provided bot token
	dg, err := discordgo.New("Bot " + config.Token)
	if err != nil {
		log.Fatal("error creating Discord session: ", err)
	}

	// ArchiverBot is an instance of this bot. It has many methods attached to
	// it for controlling the bot. db is the database object, dg is the
	// discordgo object
	archiveBot := bot.ArchiverBot{
		DB:     db,
		DG:     dg,
		Config: config,
	}

	// Set up DB if necessary
	for _, schemaType := range allSchemaTypes {
		err := db.AutoMigrate(schemaType)
		if err != nil {
			log.Fatal("unable to automigrate ", reflect.TypeOf(&schemaType).Elem().Name(), "err: ", err)
		}
	}

	// Start healthcheck handler
	go archiveBot.StartHealthAPI()

	// These handlers get called whenever there's a corresponding
	// Discord event
	dg.AddHandler(archiveBot.BotReadyHandler)
	dg.AddHandler(archiveBot.GuildCreateHandler)
	dg.AddHandler(archiveBot.MessageReactionAddHandler)
	dg.AddHandler(archiveBot.InteractionHandler)

	// We have to be explicit about what we want to receive. In addition,
	// some intents require additional permissions, which must be granted
	// to the bot when it's added or after the fact by a guild admin
	discordIntents := discordgo.IntentsGuildMessages | discordgo.IntentsGuilds |
		discordgo.IntentsDirectMessages | discordgo.IntentsDirectMessageReactions |
		discordgo.IntentsGuildMessageReactions
	dg.Identify.Intents = discordIntents

	// Open a websocket connection to Discord and begin listening
	if err := dg.Open(); err != nil {
		log.Fatal("error opening connection to discord: ", err)
	}

	// Wait here until CTRL-C or other term signal is received
	log.Info("bot started")

	// Cleanly close down the Discord session
	defer dg.Close()

	// Listen for signals from the OS
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc
}
